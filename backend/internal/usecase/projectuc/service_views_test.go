package projectuc

import (
	"context"
	"testing"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/project"
	"github.com/mesutokul/fluxboard/backend/internal/domain/tenant"
)

type fakeViews struct {
	project.SavedViewRepository
	views   map[string]*project.SavedView
	deleted []string
}

func newFakeViews(vs ...*project.SavedView) *fakeViews {
	m := make(map[string]*project.SavedView)
	for _, v := range vs {
		m[v.ID] = v
	}
	return &fakeViews{views: m}
}

func (f *fakeViews) Create(_ context.Context, _ string, v *project.SavedView) error {
	cp := *v
	f.views[v.ID] = &cp
	return nil
}

func (f *fakeViews) Get(_ context.Context, _, id string) (*project.SavedView, error) {
	v, ok := f.views[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	cp := *v
	return &cp, nil
}

func (f *fakeViews) Update(_ context.Context, _ string, v *project.SavedView) error {
	cp := *v
	f.views[v.ID] = &cp
	return nil
}

func (f *fakeViews) Delete(_ context.Context, _, id string) error {
	f.deleted = append(f.deleted, id)
	return nil
}

func viewTestService(role project.ProjectRole, views *fakeViews) *Service {
	return New(Deps{
		Projects: &fakeProjects{p: &project.Project{ID: "p1", Visibility: project.VisibilityOrg}},
		Members:  &fakeMembers{role: &role},
		Views:    views,
	})
}

func TestCreateView(t *testing.T) {
	ctx := context.Background()
	f := project.ViewFilters{Priority: "high", GroupBy: "epic"}

	viewer := viewTestService(project.RoleViewer, newFakeViews())
	v, err := viewer.CreateView(ctx, "o", "u1", "p1", " Mine ", false, f, tenant.RoleMember)
	if err != nil || v.Name != "Mine" || v.Shared || v.OwnerID != "u1" || v.Filters.Assignee != "any" {
		t.Fatalf("personal view: %+v, %v", v, err)
	}
	if _, err := viewer.CreateView(ctx, "o", "u1", "p1", "Team", true, f, tenant.RoleMember); err != domain.ErrForbidden {
		t.Fatalf("viewer sharing: err %v, want ErrForbidden", err)
	}
	if _, err := viewer.CreateView(ctx, "o", "u1", "p1", "Bad", false, project.ViewFilters{GroupBy: "label"}, tenant.RoleMember); err != domain.ErrValidation {
		t.Fatalf("bad filters: err %v, want ErrValidation", err)
	}
	lead := viewTestService(project.RoleLead, newFakeViews())
	if v, err := lead.CreateView(ctx, "o", "u1", "p1", "Team", true, f, tenant.RoleMember); err != nil || !v.Shared {
		t.Fatalf("lead sharing: %+v, %v", v, err)
	}
}

func TestEditView_Permissions(t *testing.T) {
	ctx := context.Background()
	views := func() *fakeViews {
		return newFakeViews(
			&project.SavedView{ID: "mine", ProjectID: "p1", OwnerID: "u1", Name: "Mine"},
			&project.SavedView{ID: "theirs", ProjectID: "p1", OwnerID: "u2", Name: "Theirs"},
			&project.SavedView{ID: "team", ProjectID: "p1", OwnerID: "u2", Name: "Team", Shared: true},
		)
	}
	f := project.ViewFilters{}

	vs := views()
	viewer := viewTestService(project.RoleViewer, vs)
	if _, err := viewer.UpdateView(ctx, "o", "u1", "p1", "mine", "Renamed", false, f, tenant.RoleMember); err != nil {
		t.Fatalf("own view: %v", err)
	}
	if vs.views["mine"].Name != "Renamed" {
		t.Errorf("rename not stored: %+v", vs.views["mine"])
	}
	if _, err := viewer.UpdateView(ctx, "o", "u1", "p1", "mine", "Mine", true, f, tenant.RoleMember); err != domain.ErrForbidden {
		t.Errorf("viewer sharing own view: err %v, want ErrForbidden", err)
	}
	if err := viewer.DeleteView(ctx, "o", "u1", "p1", "theirs", tenant.RoleMember); err != domain.ErrNotFound {
		t.Errorf("someone else's personal view: err %v, want ErrNotFound", err)
	}
	if err := viewer.DeleteView(ctx, "o", "u1", "p1", "team", tenant.RoleMember); err != domain.ErrForbidden {
		t.Errorf("viewer deleting shared view: err %v, want ErrForbidden", err)
	}
	if err := viewer.DeleteView(ctx, "o", "u1", "p2", "mine", tenant.RoleMember); err != domain.ErrNotFound {
		t.Errorf("other project: err %v, want ErrNotFound", err)
	}
	if len(vs.deleted) != 0 {
		t.Fatalf("refused deletes still deleted %v", vs.deleted)
	}

	vs = views()
	lead := viewTestService(project.RoleLead, vs)
	if err := lead.DeleteView(ctx, "o", "u1", "p1", "team", tenant.RoleMember); err != nil {
		t.Errorf("lead deleting shared view: %v", err)
	}
	if err := lead.DeleteView(ctx, "o", "u1", "p1", "theirs", tenant.RoleMember); err != domain.ErrNotFound {
		t.Errorf("lead and someone else's personal view: err %v, want ErrNotFound", err)
	}
}
