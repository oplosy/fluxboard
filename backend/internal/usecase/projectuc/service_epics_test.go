package projectuc

import (
	"context"
	"errors"
	"testing"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/project"
	"github.com/mesutokul/fluxboard/backend/internal/domain/tenant"
)

type fakeEpics struct {
	project.EpicRepository
	epics map[string]*project.Epic
	set   map[string]*string
}

func newFakeEpics(es ...*project.Epic) *fakeEpics {
	m := make(map[string]*project.Epic)
	for _, e := range es {
		m[e.ID] = e
	}
	return &fakeEpics{epics: m, set: make(map[string]*string)}
}

func (f *fakeEpics) Create(_ context.Context, _ string, e *project.Epic) error {
	cp := *e
	f.epics[e.ID] = &cp
	return nil
}

func (f *fakeEpics) Get(_ context.Context, _, id string) (*project.Epic, error) {
	e, ok := f.epics[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	cp := *e
	return &cp, nil
}

func (f *fakeEpics) ListByProject(_ context.Context, _, projectID string) ([]project.Epic, error) {
	var out []project.Epic
	for _, e := range f.epics {
		if e.ProjectID == projectID {
			out = append(out, *e)
		}
	}
	return out, nil
}

func (f *fakeEpics) Update(_ context.Context, _ string, e *project.Epic) error {
	cp := *e
	f.epics[e.ID] = &cp
	return nil
}

func (f *fakeEpics) SetTaskEpic(_ context.Context, _, taskID string, epicID *string) error {
	f.set[taskID] = epicID
	return nil
}

func epicTestService(role project.ProjectRole, epics *fakeEpics, tasks ...*project.Task) *Service {
	return New(Deps{
		Projects: &fakeProjects{p: &project.Project{ID: "p1", Visibility: project.VisibilityOrg}},
		Members:  &fakeMembers{role: &role},
		Boards:   &fakeSprintBoards{},
		Columns:  &fakeSprintColumns{},
		Tasks:    &fakeSprintTasks{tasks: tasks},
		Epics:    epics,
	})
}

func TestListEpics_Progress(t *testing.T) {
	eid := "e1"
	epics := newFakeEpics(&project.Epic{ID: eid, ProjectID: "p1", Name: "Checkout"})
	s := epicTestService(project.RoleViewer, epics,
		&project.Task{ID: "t-done", ProjectID: "p1", ColumnID: "c2", EpicID: &eid},
		&project.Task{ID: "t-open", ProjectID: "p1", ColumnID: "c1", EpicID: &eid},
		&project.Task{ID: "t-none", ProjectID: "p1", ColumnID: "c2"},
	)
	got, err := s.ListEpics(context.Background(), "o", "u", "p1", tenant.RoleMember)
	if err != nil || len(got) != 1 {
		t.Fatalf("ListEpics = %+v, %v", got, err)
	}
	if got[0].Total != 2 || got[0].Done != 1 {
		t.Fatalf("progress = %d/%d, want 1/2", got[0].Done, got[0].Total)
	}
}

func TestCreateEpic(t *testing.T) {
	ctx := context.Background()
	t.Run("lead creates with normalized fields", func(t *testing.T) {
		s := epicTestService(project.RoleLead, newFakeEpics())
		e, err := s.CreateEpic(ctx, "o", "u", "p1", " Checkout ", "#AABBCC", tenant.RoleMember)
		if err != nil || e.Name != "Checkout" || e.Color != "#aabbcc" {
			t.Fatalf("CreateEpic = %+v, %v", e, err)
		}
	})
	t.Run("contributor cannot create", func(t *testing.T) {
		s := epicTestService(project.RoleContributor, newFakeEpics())
		if _, err := s.CreateEpic(ctx, "o", "u", "p1", "Checkout", "", tenant.RoleMember); err == nil {
			t.Fatal("want an access error")
		}
	})
}

func TestAssignEpic(t *testing.T) {
	ctx := context.Background()
	eid, other := "e1", "e-other"
	epics := func() *fakeEpics {
		return newFakeEpics(
			&project.Epic{ID: eid, ProjectID: "p1"},
			&project.Epic{ID: other, ProjectID: "p2"},
		)
	}
	tasks := []*project.Task{
		{ID: "t1", ProjectID: "p1"},
		{ID: "t2", ProjectID: "p1"},
		{ID: "t-foreign", ProjectID: "p2"},
	}

	t.Run("contributor files tasks and clears", func(t *testing.T) {
		f := epics()
		s := epicTestService(project.RoleContributor, f, tasks...)
		if err := s.AssignEpic(ctx, "o", "u", "p1", []string{"t1", "t2"}, &eid, tenant.RoleMember); err != nil {
			t.Fatal(err)
		}
		if f.set["t1"] == nil || *f.set["t1"] != eid || f.set["t2"] == nil {
			t.Fatalf("set = %v", f.set)
		}
		if err := s.AssignEpic(ctx, "o", "u", "p1", []string{"t1"}, nil, tenant.RoleMember); err != nil || f.set["t1"] != nil {
			t.Fatalf("clear: %v %v", f.set["t1"], err)
		}
	})
	t.Run("epic of another project is not found", func(t *testing.T) {
		s := epicTestService(project.RoleContributor, epics(), tasks...)
		if err := s.AssignEpic(ctx, "o", "u", "p1", []string{"t1"}, &other, tenant.RoleMember); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("err = %v, want ErrNotFound", err)
		}
	})
	t.Run("a foreign task writes nothing", func(t *testing.T) {
		f := epics()
		s := epicTestService(project.RoleContributor, f, tasks...)
		err := s.AssignEpic(ctx, "o", "u", "p1", []string{"t1", "t-foreign"}, &eid, tenant.RoleMember)
		if !errors.Is(err, domain.ErrValidation) || len(f.set) != 0 {
			t.Fatalf("err = %v, set = %v", err, f.set)
		}
	})
	t.Run("viewer cannot assign", func(t *testing.T) {
		s := epicTestService(project.RoleViewer, epics(), tasks...)
		if err := s.AssignEpic(ctx, "o", "u", "p1", []string{"t1"}, &eid, tenant.RoleMember); err == nil {
			t.Fatal("want an access error")
		}
	})
}

func TestUpdateEpic_KeepsProgress(t *testing.T) {
	eid := "e1"
	s := epicTestService(project.RoleLead, newFakeEpics(&project.Epic{ID: eid, ProjectID: "p1", Name: "Old"}),
		&project.Task{ID: "t-done", ProjectID: "p1", ColumnID: "c2", EpicID: &eid},
		&project.Task{ID: "t-open", ProjectID: "p1", ColumnID: "c1", EpicID: &eid},
	)
	e, err := s.UpdateEpic(context.Background(), "o", "u", "p1", eid, "New", "", tenant.RoleMember)
	if err != nil || e.Name != "New" || e.Total != 2 || e.Done != 1 {
		t.Fatalf("UpdateEpic = %+v, %v", e, err)
	}
}
