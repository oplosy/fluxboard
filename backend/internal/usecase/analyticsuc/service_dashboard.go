package analyticsuc

import (
	"context"
	"errors"
	"time"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/analytics"
	"github.com/mesutokul/fluxboard/backend/internal/domain/project"
	"github.com/mesutokul/fluxboard/backend/internal/domain/tenant"
)

// Dashboard returns the org dashboard data (FR-AN-005, ADR-038) for the
// projects the caller can see: the same visibility as the project list, so a
// private project the caller is not in never reaches an aggregate. Visible
// projects with no tasks are listed with zero counts.
func (s *Service) Dashboard(ctx context.Context, orgID, userID string, orgRole tenant.OrgRole) (analytics.DashboardData, error) {
	visible, err := s.projects.List(ctx, orgID, userID, project.ScopeFor(orgRole), false)
	if err != nil {
		return analytics.DashboardData{}, err
	}
	now := s.now().UTC()
	// Monday of the first of the last DashboardWeeks weeks, including this one.
	today := now.Truncate(24 * time.Hour)
	weekFrom := today.AddDate(0, 0, -((int(today.Weekday())+6)%7)-7*(analytics.DashboardWeeks-1))
	out := analytics.DashboardData{
		Projects: make([]analytics.ProjectCounts, 0, len(visible)), Assignees: []analytics.AssigneeCount{},
		Weekly: []analytics.WeeklyCompleted{}, WeekFrom: weekFrom,
	}
	if len(visible) == 0 {
		return out, nil
	}
	ids := make([]string, len(visible))
	for i, p := range visible {
		ids[i] = p.ID
	}
	counts, assignees, weekly, err := s.dash.Counts(ctx, orgID, ids, now, weekFrom)
	if err != nil {
		return analytics.DashboardData{}, err
	}
	byProject := make(map[string]analytics.ProjectCounts, len(counts))
	for _, c := range counts {
		byProject[c.ProjectID] = c
	}
	for _, p := range visible {
		c := byProject[p.ID]
		c.ProjectID, c.Key, c.Name, c.Color = p.ID, p.Key, p.Name, p.Color
		out.Projects = append(out.Projects, c)
	}
	out.Assignees, out.Weekly = append(out.Assignees, assignees...), append(out.Weekly, weekly...)
	return out, nil
}

// DashboardLayout returns the caller's saved widgets, or the default layout.
func (s *Service) DashboardLayout(ctx context.Context, orgID, userID string) ([]analytics.Widget, error) {
	ws, err := s.dash.GetLayout(ctx, orgID, userID)
	if errors.Is(err, domain.ErrNotFound) {
		return analytics.DefaultLayout(), nil
	}
	return ws, err
}

// SaveDashboardLayout validates and stores the caller's widgets.
func (s *Service) SaveDashboardLayout(ctx context.Context, orgID, userID string, widgets []analytics.Widget) ([]analytics.Widget, error) {
	ws, err := analytics.NormalizeLayout(widgets)
	if err != nil {
		return nil, err
	}
	if err := s.dash.SaveLayout(ctx, orgID, userID, ws); err != nil {
		return nil, err
	}
	return ws, nil
}
