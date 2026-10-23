// Package analyticsuc serves project analytics and the org usage dashboard from the
// nightly rollup and usage aggregates (FR-AN-001/002). It NEVER runs live aggregates
// over the operational tables — only the analytics read ports. Domain-only deps.
package analyticsuc

import (
	"context"
	"errors"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/analytics"
	"github.com/mesutokul/fluxboard/backend/internal/domain/billing"
	"github.com/mesutokul/fluxboard/backend/internal/domain/project"
	"github.com/mesutokul/fluxboard/backend/internal/domain/tenant"
)

// defaultWindow is the project-analytics range applied when the caller passes none.
const defaultWindow = 30 * 24 * time.Hour

// Deps wires the analytics service.
type Deps struct {
	Stats    analytics.StatsRepository
	Projects project.ProjectRepository       // project-access check (private projects)
	Members  project.ProjectMemberRepository // same policy as taskuc/aiuc
	Usage    analytics.UsageReader
	Subs     billing.SubscriptionRepository
	Plans    billing.PlanRepository
	Dash     analytics.DashboardRepository // org dashboard (FR-AN-005)
	Now      func() time.Time
}

// Service implements the analytics reads.
type Service struct {
	stats    analytics.StatsRepository
	projects project.ProjectRepository
	members  project.ProjectMemberRepository
	usage    analytics.UsageReader
	subs     billing.SubscriptionRepository
	plans    billing.PlanRepository
	dash     analytics.DashboardRepository
	now      func() time.Time
}

// New builds the service.
func New(d Deps) *Service {
	if d.Now == nil {
		d.Now = time.Now
	}
	return &Service{stats: d.Stats, projects: d.Projects, members: d.Members, usage: d.Usage, subs: d.Subs, plans: d.Plans, dash: d.Dash, now: d.Now}
}

// ProjectAnalytics returns the daily series + window totals for a project. A zero
// from/to defaults to the last 30 UTC days. Totals and the mean cycle time are
// computed from the rollup rows (no live aggregation). The caller needs VIEWER
// on the project: org RLS alone would let any member read a private project's
// stats, and an unknown or foreign project must 404 rather than answer empty.
func (s *Service) ProjectAnalytics(ctx context.Context, orgID, userID string, orgRole tenant.OrgRole, projectID string, from, to time.Time) (analytics.ProjectAnalytics, error) {
	if err := s.requireViewer(ctx, orgID, userID, orgRole, projectID); err != nil {
		return analytics.ProjectAnalytics{}, err
	}
	now := s.now().UTC()
	if to.IsZero() {
		to = now
	}
	if from.IsZero() {
		from = to.Add(-defaultWindow)
	}
	from, to = from.UTC().Truncate(24*time.Hour), to.UTC().Truncate(24*time.Hour)

	series, err := s.stats.ListDaily(ctx, orgID, projectID, from, to)
	if err != nil {
		return analytics.ProjectAnalytics{}, err
	}
	out := analytics.ProjectAnalytics{ProjectID: projectID, From: from, To: to, Series: series}
	// Each day's mean is weighted by its completions, so a day with one task
	// does not count as much as a day with twenty.
	var cycleSum, cycleTasks int64
	for _, d := range series {
		out.TotalCreated += d.CreatedCount
		out.TotalCompleted += d.CompletedCount
		if d.AvgCycleSeconds != nil && d.CompletedCount > 0 {
			cycleSum += *d.AvgCycleSeconds * int64(d.CompletedCount)
			cycleTasks += int64(d.CompletedCount)
		}
	}
	if cycleTasks > 0 {
		avg := cycleSum / cycleTasks
		out.AvgCycleSeconds = &avg
	}
	return out, nil
}

// UsageDashboard returns the current calendar month's metered dimensions plus an
// invoice estimate (the plan's base monthly price; metered add-ons are surfaced by
// the billing pipeline, not estimated here). Seats/Storage are point-in-time; API
// calls are the month-to-date sum.
func (s *Service) UsageDashboard(ctx context.Context, orgID string) (analytics.UsageDashboard, error) {
	now := s.now().UTC()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	today := now.Truncate(24 * time.Hour)

	var (
		seats    int64
		storage  int64
		apiCalls int64
		sub      *billing.Subscription
		subErr   error
	)
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		var err error
		seats, err = s.usage.Latest(gctx, orgID, analytics.MetricActiveMembers, today)
		return err
	})
	g.Go(func() error {
		var err error
		storage, err = s.usage.Latest(gctx, orgID, analytics.MetricStorageBytes, today)
		return err
	})
	g.Go(func() error {
		var err error
		apiCalls, err = s.usage.SumRange(gctx, orgID, analytics.MetricAPICalls, monthStart, today)
		return err
	})
	g.Go(func() error {
		sub, subErr = s.subs.Get(gctx, orgID)
		if subErr != nil && !errors.Is(subErr, domain.ErrNotFound) {
			return subErr
		}
		return nil
	})
	if err := g.Wait(); err != nil {
		return analytics.UsageDashboard{}, err
	}

	planCode := billing.PlanFree
	if subErr == nil && sub != nil {
		if billing.SubStatus(sub.Status).Entitled() {
			planCode = sub.PlanCode
		}
	}
	var estimate int64
	if p, err := s.plans.Get(ctx, planCode); err == nil && p != nil {
		estimate = p.MonthlyPrice
	}

	return analytics.UsageDashboard{
		PeriodStart:    monthStart,
		PeriodEnd:      now,
		Seats:          seats,
		StorageBytes:   storage,
		APICalls:       apiCalls,
		EstimatedTotal: estimate,
		Plan:           string(planCode),
	}, nil
}

// requireViewer requires VIEWER on the project (project.Access): hidden or
// missing projects are ErrNotFound so they are indistinguishable.
func (s *Service) requireViewer(ctx context.Context, orgID, userID string, orgRole tenant.OrgRole, projectID string) error {
	_, _, err := project.Access(ctx, s.projects, s.members, orgID, projectID, userID, orgRole, project.RoleViewer)
	return err
}
