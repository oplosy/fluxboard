package projectuc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/project"
	"github.com/mesutokul/fluxboard/backend/internal/domain/tenant"
)

type fakeGoals struct {
	project.GoalRepository
	goals map[string]*project.Goal
	links []project.GoalLink
}

func (f *fakeGoals) Create(_ context.Context, _ string, g *project.Goal) error {
	cp := *g
	f.goals[g.ID] = &cp
	return nil
}

func (f *fakeGoals) Get(_ context.Context, _, id string) (*project.Goal, error) {
	g, ok := f.goals[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	cp := *g
	return &cp, nil
}

func (f *fakeGoals) List(_ context.Context, _ string) ([]project.Goal, error) {
	var out []project.Goal
	for _, g := range f.goals {
		out = append(out, *g)
	}
	return out, nil
}

func (f *fakeGoals) Update(_ context.Context, _ string, g *project.Goal) error {
	cp := *g
	f.goals[g.ID] = &cp
	return nil
}

func (f *fakeGoals) Delete(_ context.Context, _, id string) error {
	delete(f.goals, id)
	return nil
}

func (f *fakeGoals) Link(_ context.Context, _, goalID string, kind project.GoalItemKind, itemID string) error {
	f.links = append(f.links, project.GoalLink{GoalID: goalID, Kind: kind, ItemID: itemID, ProjectID: goalTestItemProject[itemID]})
	return nil
}

func (f *fakeGoals) Links(_ context.Context, _, goalID string) ([]project.GoalLink, error) {
	var out []project.GoalLink
	for _, l := range f.links {
		if goalID == "" || l.GoalID == goalID {
			out = append(out, l)
		}
	}
	return out, nil
}

// goalTestItemProject maps a linked item id to its project, like the SQL join does.
var goalTestItemProject = map[string]string{"e1": "p1", "t1": "p1", "t2": "p1", "t3": "p1", "ep": "p2", "tp": "p2"}

type fakeGoalProjects struct {
	project.ProjectRepository
	all []project.Project
}

func (f *fakeGoalProjects) Get(_ context.Context, _, id string) (*project.Project, error) {
	for i := range f.all {
		if f.all[i].ID == id {
			return &f.all[i], nil
		}
	}
	return nil, domain.ErrNotFound
}

func (f *fakeGoalProjects) List(_ context.Context, _, _ string, sc project.Scope, _ bool) ([]project.Project, error) {
	var out []project.Project
	for _, p := range f.all {
		if sc.All || (sc.OrgVisible && p.Visibility == project.VisibilityOrg) {
			out = append(out, p)
		}
	}
	return out, nil
}

type fakeGoalMembers struct{ project.ProjectMemberRepository }

func (fakeGoalMembers) Get(_ context.Context, _, _, _ string) (*project.ProjectMember, error) {
	return nil, domain.ErrNotFound
}

type fakeGoalTasks struct {
	project.TaskRepository
	tasks []project.Task
}

func (f *fakeGoalTasks) Get(_ context.Context, _, id string) (*project.Task, error) {
	for i := range f.tasks {
		if f.tasks[i].ID == id {
			return &f.tasks[i], nil
		}
	}
	return nil, domain.ErrNotFound
}

func (f *fakeGoalTasks) ListByProject(_ context.Context, _, projectID string) ([]project.Task, error) {
	var out []project.Task
	for _, t := range f.tasks {
		if t.ProjectID == projectID {
			out = append(out, t)
		}
	}
	return out, nil
}

type fakeGoalEpics struct {
	project.EpicRepository
	epics []project.Epic
}

func (f *fakeGoalEpics) Get(_ context.Context, _, id string) (*project.Epic, error) {
	for i := range f.epics {
		if f.epics[i].ID == id {
			return &f.epics[i], nil
		}
	}
	return nil, domain.ErrNotFound
}

func (f *fakeGoalEpics) ListByProject(_ context.Context, _, projectID string) ([]project.Epic, error) {
	var out []project.Epic
	for _, e := range f.epics {
		if e.ProjectID == projectID {
			out = append(out, e)
		}
	}
	return out, nil
}

// goalTestService seeds an org-visible project p1 (KEY1) and a private p2
// (SECRET) the test callers are not members of. c2 is the done column.
//
//	e1 (p1): t1 done 5pt, t2 open 3pt     t3 (p1, no epic): done 2pt
//	ep (p2): tp done 4pt, titled "private task"
func goalTestService() (*Service, *fakeGoals) {
	e1, ep := "e1", "ep"
	five, three, two, four := 5, 3, 2, 4
	goals := &fakeGoals{goals: map[string]*project.Goal{}}
	s := New(Deps{
		Projects: &fakeGoalProjects{all: []project.Project{
			{ID: "p1", Key: "KEY1", Visibility: project.VisibilityOrg},
			{ID: "p2", Key: "SECRET", Visibility: project.VisibilityPrivate},
		}},
		Members: fakeGoalMembers{},
		Boards:  &fakeSprintBoards{},
		Columns: &fakeSprintColumns{},
		Tasks: &fakeGoalTasks{tasks: []project.Task{
			{ID: "t1", ProjectID: "p1", ColumnID: "c2", Number: 1, Title: "one", EpicID: &e1, Estimate: &five},
			{ID: "t2", ProjectID: "p1", ColumnID: "c1", Number: 2, Title: "two", EpicID: &e1, Estimate: &three},
			{ID: "t3", ProjectID: "p1", ColumnID: "c2", Number: 3, Title: "three", Estimate: &two},
			{ID: "tp", ProjectID: "p2", ColumnID: "c2", Number: 1, Title: "private task", EpicID: &ep, Estimate: &four},
		}},
		Epics: &fakeGoalEpics{epics: []project.Epic{
			{ID: "e1", ProjectID: "p1", Name: "Epic one"},
			{ID: "ep", ProjectID: "p2", Name: "private epic"},
		}},
		Goals: goals,
		Now:   func() time.Time { return time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC) },
	})
	return s, goals
}

func newGoal(t *testing.T, s *Service, owner string, role tenant.OrgRole) string {
	t.Helper()
	v, err := s.CreateGoal(context.Background(), "o1", owner, GoalInput{Title: "Ship v2"}, role)
	if err != nil {
		t.Fatalf("create goal: %v", err)
	}
	return v.Goal.ID
}

func link(t *testing.T, s *Service, goalID, kind, item string) {
	t.Helper()
	if err := s.LinkGoalItem(context.Background(), "o1", "admin", goalID, project.GoalItemKind(kind), item, tenant.RoleAdmin); err != nil {
		t.Fatalf("link %s: %v", item, err)
	}
}

func TestGoalProgress_CountsEachTaskOnce(t *testing.T) {
	s, _ := goalTestService()
	id := newGoal(t, s, "owner", tenant.RoleMember)
	// t1 is linked directly and through its epic; t3 only directly.
	link(t, s, id, "epic", "e1")
	link(t, s, id, "task", "t1")
	link(t, s, id, "task", "t3")

	v, err := s.GetGoal(context.Background(), "o1", "owner", id, tenant.RoleMember)
	if err != nil {
		t.Fatal(err)
	}
	g := v.Goal
	if g.Total != 3 || g.Done != 2 || g.Points != 10 || g.DonePoints != 7 {
		t.Fatalf("progress = %d/%d tasks %d/%d points, want 2/3 and 7/10", g.Done, g.Total, g.DonePoints, g.Points)
	}
	if g.Status != project.GoalOnTrack {
		t.Fatalf("status = %s", g.Status)
	}
	if len(v.Items) != 3 {
		t.Fatalf("items = %d, want 3", len(v.Items))
	}
	if ep := v.Items[0]; ep.Kind != project.GoalItemEpic || ep.Total != 2 || ep.Done != 1 || ep.ProjectKey != "KEY1" {
		t.Fatalf("epic item = %+v", ep)
	}
}

func TestGoalStatus_DoneAndAtRisk(t *testing.T) {
	s, _ := goalTestService()
	ctx := context.Background()
	id := newGoal(t, s, "owner", tenant.RoleMember)
	link(t, s, id, "task", "t1") // done
	link(t, s, id, "task", "t3") // done
	v, _ := s.GetGoal(ctx, "o1", "owner", id, tenant.RoleMember)
	if v.Goal.Status != project.GoalDone {
		t.Fatalf("all done: status = %s", v.Goal.Status)
	}
	link(t, s, id, "task", "t2") // open
	past := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	v, err := s.UpdateGoal(ctx, "o1", "owner", id, GoalInput{Title: "Ship v2", TargetDate: &past}, tenant.RoleMember)
	if err != nil {
		t.Fatal(err)
	}
	if v.Goal.Status != project.GoalAtRisk {
		t.Fatalf("late: status = %s", v.Goal.Status)
	}
}

func TestGoalVisibility_PrivateProjectItemsHidden(t *testing.T) {
	s, _ := goalTestService()
	ctx := context.Background()
	id := newGoal(t, s, "owner", tenant.RoleMember)
	link(t, s, id, "epic", "e1")
	link(t, s, id, "epic", "ep") // admin links a private epic
	link(t, s, id, "task", "tp")

	member, err := s.GetGoal(ctx, "o1", "owner", id, tenant.RoleMember)
	if err != nil {
		t.Fatal(err)
	}
	if member.Goal.Total != 2 || len(member.Items) != 1 || member.Items[0].ID != "e1" {
		t.Fatalf("member sees total=%d items=%+v, want only the KEY1 epic", member.Goal.Total, member.Items)
	}
	list, _ := s.ListGoals(ctx, "o1", "owner", tenant.RoleMember)
	if len(list) != 1 || list[0].Total != 2 || list[0].Points != 8 {
		t.Fatalf("member list = %+v", list)
	}

	admin, _ := s.GetGoal(ctx, "o1", "admin", id, tenant.RoleAdmin)
	if admin.Goal.Total != 3 || len(admin.Items) != 3 {
		t.Fatalf("admin sees total=%d items=%d, want 3/3", admin.Goal.Total, len(admin.Items))
	}
}

func TestLinkGoalItem_HiddenProjectIsNotFound(t *testing.T) {
	s, _ := goalTestService()
	id := newGoal(t, s, "owner", tenant.RoleMember)
	for _, c := range []struct{ kind, item string }{{"epic", "ep"}, {"task", "tp"}} {
		err := s.LinkGoalItem(context.Background(), "o1", "owner", id, project.GoalItemKind(c.kind), c.item, tenant.RoleMember)
		if !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("link %s %s by member: %v, want not found", c.kind, c.item, err)
		}
	}
	if err := s.LinkGoalItem(context.Background(), "o1", "owner", id, "bogus", "t1", tenant.RoleMember); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("bad kind: %v", err)
	}
}

func TestGoalPermissions(t *testing.T) {
	s, goals := goalTestService()
	ctx := context.Background()
	if _, err := s.CreateGoal(ctx, "o1", "g", GoalInput{Title: "x"}, tenant.RoleGuest); !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("guest create: %v", err)
	}
	if _, err := s.CreateGoal(ctx, "o1", "m", GoalInput{Title: " "}, tenant.RoleMember); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("blank title: %v", err)
	}
	id := newGoal(t, s, "owner", tenant.RoleMember)

	in := GoalInput{Title: "renamed"}
	if _, err := s.UpdateGoal(ctx, "o1", "other", id, in, tenant.RoleMember); !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("non-owner update: %v", err)
	}
	if err := s.LinkGoalItem(ctx, "o1", "other", id, project.GoalItemTask, "t1", tenant.RoleMember); !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("non-owner link: %v", err)
	}
	if err := s.UnlinkGoalItem(ctx, "o1", "other", id, project.GoalItemTask, "t1", tenant.RoleMember); !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("non-owner unlink: %v", err)
	}
	if err := s.DeleteGoal(ctx, "o1", "other", id, tenant.RoleMember); !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("non-owner delete: %v", err)
	}
	if _, err := s.UpdateGoal(ctx, "o1", "owner", id, in, tenant.RoleMember); err != nil {
		t.Errorf("owner update: %v", err)
	}
	if _, err := s.UpdateGoal(ctx, "o1", "admin", id, GoalInput{Title: "by admin"}, tenant.RoleAdmin); err != nil {
		t.Errorf("admin update: %v", err)
	}
	if err := s.DeleteGoal(ctx, "o1", "admin", id, tenant.RoleAdmin); err != nil || len(goals.goals) != 0 {
		t.Errorf("admin delete: %v (left %d)", err, len(goals.goals))
	}
	if _, err := s.GetGoal(ctx, "o1", "owner", id, tenant.RoleMember); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("get deleted: %v", err)
	}
}
