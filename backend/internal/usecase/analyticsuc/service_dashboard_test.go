package analyticsuc

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/analytics"
	"github.com/mesutokul/fluxboard/backend/internal/domain/project"
	"github.com/mesutokul/fluxboard/backend/internal/domain/tenant"
)

// visibleProjects lists the open project to everyone and the private one only to ADMIN+.
type visibleProjects struct{ projectsRepo }

func (*visibleProjects) List(_ context.Context, _, _ string, scope project.Scope, _ bool) ([]project.Project, error) {
	out := []project.Project{{ID: "proj1", Key: "ONE", Name: "One", Color: "#111"}}
	if scope.All {
		out = append(out, project.Project{ID: "secret", Key: "SEC", Name: "Secret"})
	}
	return out, nil
}

type fakeDash struct {
	gotIDs   []string
	gotFrom  time.Time
	layout   []analytics.Widget
	layoutOK bool
}

func (f *fakeDash) Counts(_ context.Context, _ string, ids []string, _, from time.Time) ([]analytics.ProjectCounts, []analytics.AssigneeCount, []analytics.WeeklyCompleted, error) {
	f.gotIDs, f.gotFrom = ids, from
	return []analytics.ProjectCounts{{ProjectID: "proj1", Open: 3, Done: 2, Overdue: 1}}, nil, nil, nil
}
func (f *fakeDash) GetLayout(context.Context, string, string) ([]analytics.Widget, error) {
	if !f.layoutOK {
		return nil, domain.ErrNotFound
	}
	return f.layout, nil
}
func (f *fakeDash) SaveLayout(_ context.Context, _, _ string, ws []analytics.Widget) error {
	f.layout, f.layoutOK = ws, true
	return nil
}

func TestDashboardOnlyAggregatesVisibleProjects(t *testing.T) {
	d := &fakeDash{}
	now := time.Date(2026, 10, 23, 15, 0, 0, 0, time.UTC) // a Friday
	svc := New(Deps{Projects: &visibleProjects{}, Dash: d, Now: func() time.Time { return now }})

	member, err := svc.Dashboard(context.Background(), "org1", "u1", tenant.RoleMember)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(d.gotIDs, []string{"proj1"}) {
		t.Fatalf("member aggregated %v, want only the visible project", d.gotIDs)
	}
	if want := time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC); !d.gotFrom.Equal(want) || !member.WeekFrom.Equal(want) {
		t.Fatalf("week from = %v, want Monday %v", d.gotFrom, want)
	}
	if len(member.Projects) != 1 || member.Projects[0].Open != 3 || member.Projects[0].Name != "One" {
		t.Fatalf("projects = %+v", member.Projects)
	}

	admin, err := svc.Dashboard(context.Background(), "org1", "u1", tenant.RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	if len(admin.Projects) != 2 || admin.Projects[1].ProjectID != "secret" || admin.Projects[1].Open != 0 {
		t.Fatalf("admin projects = %+v, want the private project with zero counts", admin.Projects)
	}
}

func TestDashboardLayoutDefaultAndSave(t *testing.T) {
	d := &fakeDash{}
	svc := New(Deps{Dash: d})
	got, err := svc.DashboardLayout(context.Background(), "org1", "u1")
	if err != nil || len(got) != len(analytics.DefaultLayout()) {
		t.Fatalf("default layout = %+v, %v", got, err)
	}
	if _, err := svc.SaveDashboardLayout(context.Background(), "org1", "u1", []analytics.Widget{{ID: "a", Type: "pie"}}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("bad widget err = %v, want ErrValidation", err)
	}
	if d.layoutOK {
		t.Fatal("an invalid layout must not be stored")
	}
	if _, err := svc.SaveDashboardLayout(context.Background(), "org1", "u1", []analytics.Widget{{ID: "a", Type: analytics.WidgetOverdue}}); err != nil {
		t.Fatal(err)
	}
	if got, _ := svc.DashboardLayout(context.Background(), "org1", "u1"); len(got) != 1 || got[0].ID != "a" {
		t.Fatalf("saved layout = %+v", got)
	}
}
