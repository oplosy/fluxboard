package projectuc

import (
	"context"
	"errors"
	"testing"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/project"
	"github.com/mesutokul/fluxboard/backend/internal/domain/tenant"
)

// A board Backlog · Todo · In Progress · Review · Done with one task in Review.

type roleBoards struct{ project.BoardRepository }

func (roleBoards) GetByProject(context.Context, string, string) (*project.Board, error) {
	return &project.Board{ID: "b1", ProjectID: "p1"}, nil
}

type roleColumns struct {
	project.ColumnRepository
	cols []project.Column
}

func (c *roleColumns) ListByBoard(context.Context, string, string) ([]project.Column, error) {
	return c.cols, nil
}
func (c *roleColumns) Get(_ context.Context, _, id string) (*project.Column, error) {
	for i := range c.cols {
		if c.cols[i].ID == id {
			return &c.cols[i], nil
		}
	}
	return nil, domain.ErrNotFound
}
func (c *roleColumns) SetRole(_ context.Context, _, _, id string, role project.ColumnRole) error {
	for i := range c.cols {
		if role != "" && c.cols[i].Role == role {
			c.cols[i].Role = ""
		}
	}
	for i := range c.cols {
		if c.cols[i].ID == id {
			c.cols[i].Role = role
			return nil
		}
	}
	return domain.ErrNotFound
}

type roleTasks struct {
	project.TaskRepository
	tasks map[string]*project.Task
}

func (r *roleTasks) Get(_ context.Context, _, id string) (*project.Task, error) {
	t, ok := r.tasks[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	cp := *t
	return &cp, nil
}
func (r *roleTasks) ListByColumn(context.Context, string, string) ([]project.Task, error) {
	return nil, nil
}
func (r *roleTasks) Move(_ context.Context, _, id, columnID, rank string) error {
	r.tasks[id].ColumnID, r.tasks[id].Rank = columnID, rank
	return nil
}

func pr(r project.ProjectRole) *project.ProjectRole { return &r }

type agentSet map[string]bool

func (a agentSet) IsAgent(_ context.Context, _, id string) (bool, error) { return a[id], nil }

type noteLog struct{ bodies []string }

func (n *noteLog) AddComment(_ context.Context, _, _, _, body string, _ tenant.OrgRole) (*project.Comment, error) {
	n.bodies = append(n.bodies, body)
	return &project.Comment{Body: body}, nil
}

func roleSvc() (*Service, *roleTasks, *roleColumns, *noteLog) {
	columns := &roleColumns{cols: []project.Column{
		{ID: "c-backlog", BoardID: "b1", Name: "Backlog", Rank: "a"},
		{ID: "c-todo", BoardID: "b1", Name: "Todo", Rank: "b"},
		{ID: "c-doing", BoardID: "b1", Name: "In Progress", Rank: "c"},
		{ID: "c-review", BoardID: "b1", Name: "Review", Rank: "d"},
		{ID: "c-done", BoardID: "b1", Name: "Done", Rank: "e"},
	}}
	tasks := &roleTasks{tasks: map[string]*project.Task{
		"t1": {ID: "t1", ProjectID: "p1", ColumnID: "c-review", Rank: "m"},
	}}
	notes := &noteLog{}
	s := New(Deps{
		Projects: &fakeProjects{p: &project.Project{ID: "p1", Visibility: project.VisibilityOrg}},
		Members:  &fakeMembers{role: pr(project.RoleContributor)},
		Boards:   roleBoards{}, Columns: columns, Tasks: tasks,
		Agents: agentSet{"bot": true}, Commenter: notes,
	})
	return s, tasks, columns, notes
}

func TestAgentsCannotMoveToDone(t *testing.T) {
	s, tasks, _, _ := roleSvc()
	ctx := context.Background()
	err := s.MoveTask(ctx, "o1", "bot", "t1", "c-done", "z", nil, tenant.RoleMember)
	if !errors.Is(err, project.ErrHumanApprovalRequired) {
		t.Fatalf("agent → done: %v", err)
	}
	if _, err := s.MoveTaskToRole(ctx, "o1", "bot", "t1", project.ColumnDone, nil, tenant.RoleMember); !errors.Is(err, project.ErrHumanApprovalRequired) {
		t.Fatalf("agent → done by role: %v", err)
	}
	if tasks.tasks["t1"].ColumnID != "c-review" {
		t.Fatal("a refused move must not move the task")
	}
	// Agents may move anywhere else; people may move to done.
	if col, err := s.MoveTaskToRole(ctx, "o1", "bot", "t1", project.ColumnInProgress, nil, tenant.RoleMember); err != nil || col.ID != "c-doing" {
		t.Fatalf("agent → in progress: %v %v", col, err)
	}
	if err := s.MoveTask(ctx, "o1", "alice", "t1", "c-done", "z", nil, tenant.RoleMember); err != nil {
		t.Fatalf("person → done: %v", err)
	}
}

func TestReviewTask(t *testing.T) {
	ctx := context.Background()

	s, tasks, _, notes := roleSvc()
	if _, err := s.ReviewTask(ctx, "o1", "bot", "t1", true, "", tenant.RoleMember); !errors.Is(err, project.ErrHumanApprovalRequired) {
		t.Fatalf("agent reviewing: %v", err)
	}
	if _, err := s.ReviewTask(ctx, "o1", "alice", "t1", false, "  ", tenant.RoleMember); err == nil {
		t.Fatal("send-back without a comment must be refused")
	}
	col, err := s.ReviewTask(ctx, "o1", "alice", "t1", false, "Add tests", tenant.RoleMember)
	if err != nil || col.ID != "c-doing" || tasks.tasks["t1"].ColumnID != "c-doing" {
		t.Fatalf("send back: %v %v", col, err)
	}
	if len(notes.bodies) != 1 || notes.bodies[0] != "Sent back for changes:\n\nAdd tests" {
		t.Fatalf("review note: %q", notes.bodies)
	}
	if _, err := s.ReviewTask(ctx, "o1", "alice", "t1", true, "", tenant.RoleMember); err == nil {
		t.Fatal("a task no longer in review cannot be reviewed")
	}

	s, tasks, _, notes = roleSvc()
	if col, err := s.ReviewTask(ctx, "o1", "alice", "t1", true, "", tenant.RoleMember); err != nil || col.ID != "c-done" {
		t.Fatalf("approve: %v %v", col, err)
	}
	if tasks.tasks["t1"].ColumnID != "c-done" || len(notes.bodies) != 0 {
		t.Fatalf("approve without comment posts nothing: %q", notes.bodies)
	}
}

func TestReviewSendBackFallsBackToTodo(t *testing.T) {
	s, tasks, columns, _ := roleSvc()
	columns.cols = append(columns.cols[:2], columns.cols[3:]...) // no In Progress
	if _, err := s.ReviewTask(context.Background(), "o1", "alice", "t1", false, "redo", tenant.RoleMember); err != nil {
		t.Fatal(err)
	}
	if tasks.tasks["t1"].ColumnID != "c-todo" {
		t.Fatalf("send back without an in-progress column: %s", tasks.tasks["t1"].ColumnID)
	}
}

func TestSetColumnRole(t *testing.T) {
	s, _, columns, _ := roleSvc()
	ctx := context.Background()
	lead := &fakeMembers{role: pr(project.RoleLead)}
	s.members = lead
	if err := s.SetColumnRole(ctx, "o1", "alice", "p1", "c-doing", "archive", tenant.RoleMember); err == nil {
		t.Fatal("unknown role must be refused")
	}
	// Marking In Progress as review takes the role off the Review column.
	if err := s.SetColumnRole(ctx, "o1", "alice", "p1", "c-doing", project.ColumnReview, tenant.RoleMember); err != nil {
		t.Fatal(err)
	}
	if c := project.ColumnForRole(columns.cols, project.ColumnReview); c == nil || c.ID != "c-doing" {
		t.Fatalf("review column = %v", c)
	}
	s.members = &fakeMembers{role: pr(project.RoleContributor)}
	if err := s.SetColumnRole(ctx, "o1", "bob", "p1", "c-todo", project.ColumnTodo, tenant.RoleMember); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("contributor setting roles: %v", err)
	}
}
