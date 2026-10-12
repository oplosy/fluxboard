package analyticsuc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/analytics"
	"github.com/mesutokul/fluxboard/backend/internal/domain/project"
	"github.com/mesutokul/fluxboard/backend/internal/domain/tenant"
)

type fakeStats struct {
	rows    []analytics.DailyStat
	gotFrom time.Time
	gotTo   time.Time
}

func (f *fakeStats) ListDaily(_ context.Context, _, _ string, from, to time.Time) ([]analytics.DailyStat, error) {
	f.gotFrom, f.gotTo = from, to
	return f.rows, nil
}

func i64(v int64) *int64 { return &v }

// projectsRepo serves proj1 (org-visible) and secret (private, member "lead").
type projectsRepo struct{ byID map[string]*project.Project }

func fakeProjects() *projectsRepo {
	return &projectsRepo{byID: map[string]*project.Project{
		"proj1":  {ID: "proj1", OrgID: "org1", Visibility: project.VisibilityOrg},
		"secret": {ID: "secret", OrgID: "org1", Visibility: project.VisibilityPrivate},
	}}
}

func (f *projectsRepo) Get(_ context.Context, _, id string) (*project.Project, error) {
	if p, ok := f.byID[id]; ok {
		cp := *p
		return &cp, nil
	}
	return nil, domain.ErrNotFound
}
func (f *projectsRepo) Create(context.Context, string, *project.Project) error { return nil }
func (f *projectsRepo) List(context.Context, string, string, bool, bool) ([]project.Project, error) {
	return nil, nil
}
func (f *projectsRepo) Update(context.Context, string, *project.Project) error        { return nil }
func (f *projectsRepo) SetArchived(context.Context, string, string, *time.Time) error { return nil }
func (f *projectsRepo) NextNumber(context.Context, string, string) (int, error)       { return 1, nil }

type fakeMembers struct{}

func (fakeMembers) Get(_ context.Context, _, projectID, userID string) (*project.ProjectMember, error) {
	if projectID == "secret" && userID == "lead" {
		return &project.ProjectMember{Role: project.RoleLead}, nil
	}
	return nil, domain.ErrNotFound
}
func (fakeMembers) Add(context.Context, string, string, string, project.ProjectRole) error {
	return nil
}
func (fakeMembers) List(context.Context, string, string) ([]project.ProjectMember, error) {
	return nil, nil
}
func (fakeMembers) Remove(context.Context, string, string, string) error { return nil }

// ProjectAnalytics totals + mean cycle must equal a raw recompute over the rollup.
func TestProjectAnalyticsMatchesRawRecompute(t *testing.T) {
	rows := []analytics.DailyStat{
		{CreatedCount: 3, CompletedCount: 1, AvgCycleSeconds: i64(100)},
		{CreatedCount: 2, CompletedCount: 4, AvgCycleSeconds: nil}, // a day with no completions → no cycle
		{CreatedCount: 5, CompletedCount: 2, AvgCycleSeconds: i64(300)},
	}
	stats := &fakeStats{rows: rows}
	svc := New(Deps{Stats: stats, Projects: fakeProjects(), Members: fakeMembers{}})

	out, err := svc.ProjectAnalytics(context.Background(), "org1", "u1", tenant.RoleMember, "proj1", time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("analytics: %v", err)
	}

	// Raw recompute.
	var wantCreated, wantCompleted int
	var cycleSum, cycleDays int64
	for _, d := range rows {
		wantCreated += d.CreatedCount
		wantCompleted += d.CompletedCount
		if d.AvgCycleSeconds != nil {
			cycleSum += *d.AvgCycleSeconds
			cycleDays++
		}
	}
	if out.TotalCreated != wantCreated {
		t.Fatalf("TotalCreated = %d, want %d", out.TotalCreated, wantCreated)
	}
	if out.TotalCompleted != wantCompleted {
		t.Fatalf("TotalCompleted = %d, want %d", out.TotalCompleted, wantCompleted)
	}
	if out.AvgCycleSeconds == nil {
		t.Fatal("AvgCycleSeconds nil, want mean over recording days")
	}
	if want := cycleSum / cycleDays; *out.AvgCycleSeconds != want {
		t.Fatalf("AvgCycleSeconds = %d, want %d", *out.AvgCycleSeconds, want)
	}
	if len(out.Series) != len(rows) {
		t.Fatalf("Series len = %d, want %d", len(out.Series), len(rows))
	}
}

// A zero from/to defaults to the last 30 UTC days (truncated to day boundaries).
func TestProjectAnalyticsDefaultWindow(t *testing.T) {
	stats := &fakeStats{}
	now := time.Date(2026, 7, 8, 15, 30, 0, 0, time.UTC)
	svc := New(Deps{Stats: stats, Projects: fakeProjects(), Members: fakeMembers{}, Now: func() time.Time { return now }})

	if _, err := svc.ProjectAnalytics(context.Background(), "org1", "u1", tenant.RoleMember, "proj1", time.Time{}, time.Time{}); err != nil {
		t.Fatalf("analytics: %v", err)
	}
	wantTo := now.Truncate(24 * time.Hour)
	wantFrom := now.Add(-defaultWindow).Truncate(24 * time.Hour)
	if !stats.gotTo.Equal(wantTo) {
		t.Fatalf("to = %v, want %v", stats.gotTo, wantTo)
	}
	if !stats.gotFrom.Equal(wantFrom) {
		t.Fatalf("from = %v, want %v", stats.gotFrom, wantFrom)
	}
}

// No rollup rows → zero totals and a nil mean (nothing recorded a cycle).
func TestProjectAnalyticsEmpty(t *testing.T) {
	svc := New(Deps{Stats: &fakeStats{rows: nil}, Projects: fakeProjects(), Members: fakeMembers{}})
	out, err := svc.ProjectAnalytics(context.Background(), "org1", "u1", tenant.RoleMember, "proj1", time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("analytics: %v", err)
	}
	if out.TotalCreated != 0 || out.TotalCompleted != 0 || out.AvgCycleSeconds != nil {
		t.Fatalf("empty analytics not zero-valued: %+v", out)
	}
}

// A private project's stats are hidden from org members outside it, and an
// unknown or foreign project 404s instead of answering an empty series.
func TestProjectAnalyticsRequiresProjectAccess(t *testing.T) {
	svc := New(Deps{Stats: &fakeStats{}, Projects: fakeProjects(), Members: fakeMembers{}})
	ctx := context.Background()
	cases := []struct {
		name, user, project string
		role                tenant.OrgRole
		want                error
	}{
		{"member, private project", "u1", "secret", tenant.RoleMember, domain.ErrNotFound},
		{"guest, org project", "u1", "proj1", tenant.RoleGuest, domain.ErrNotFound},
		{"unknown or foreign project", "u1", "other-org-project", tenant.RoleAdmin, domain.ErrNotFound},
		{"project lead, private project", "lead", "secret", tenant.RoleMember, nil},
		{"org admin, private project", "u1", "secret", tenant.RoleAdmin, nil},
	}
	for _, c := range cases {
		_, err := svc.ProjectAnalytics(ctx, "org1", c.user, c.role, c.project, time.Time{}, time.Time{})
		if !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, want %v", c.name, err, c.want)
		}
	}
}
