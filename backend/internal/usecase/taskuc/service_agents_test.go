package taskuc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mesutokul/fluxboard/backend/internal/domain/project"
	"github.com/mesutokul/fluxboard/backend/internal/domain/tenant"
)

// nextStore is claimStore plus board order for ListByProject.
type nextStore struct {
	*claimStore
	order []string
}

func (s *nextStore) ListByProject(context.Context, string, string) ([]project.Task, error) {
	out := make([]project.Task, 0, len(s.order))
	for _, id := range s.order {
		out = append(out, *s.tasks[id])
	}
	return out, nil
}

type nextBoards struct{ project.BoardRepository }

func (nextBoards) GetByProject(_ context.Context, _, projectID string) (*project.Board, error) {
	return &project.Board{ID: "b-" + projectID, ProjectID: projectID}, nil
}

type nextColumns struct{ project.ColumnRepository }

func (nextColumns) ListByBoard(_ context.Context, _, boardID string) ([]project.Column, error) {
	p := boardID[2:]
	return []project.Column{
		{ID: p + "-todo", Name: "Todo"}, {ID: p + "-doing", Name: "In Progress"},
		{ID: p + "-review", Name: "Review"}, {ID: p + "-done", Name: "Done"},
	}, nil
}

type nextLinks struct {
	project.TaskLinkRepository
	blockers map[string][]project.TaskLink
}

func (l nextLinks) Blockers(_ context.Context, _, taskID string) ([]project.TaskLink, error) {
	return l.blockers[taskID], nil
}

type someAgents map[string]bool

func (a someAgents) IsAgent(_ context.Context, _, id string) (bool, error) { return a[id], nil }

func nextSvc(tasks map[string]*project.Task, order []string, blockers map[string][]project.TaskLink) (*Service, *nextStore) {
	store := &nextStore{claimStore: &claimStore{tasks: tasks, now: claimNow}, order: order}
	return New(Deps{
		Tasks: store, Concurrency: store, Activity: &activityLog{},
		Boards: nextBoards{}, Columns: nextColumns{}, Links: nextLinks{blockers: blockers},
		Projects: fakeProjects{p: &project.Project{ID: "p1", Visibility: project.VisibilityOrg}},
		Members:  fakeMembers{role: role(project.RoleContributor)},
		Agents:   someAgents{"me": true, "other": true},
		Now:      func() time.Time { return claimNow },
	}), store
}

func TestNextTaskOrderAndReadiness(t *testing.T) {
	me, other, person := "me", "other", "alice"
	later := claimNow.Add(time.Hour)
	tasks := map[string]*project.Task{
		"low":       {ID: "low", ProjectID: "p1", ColumnID: "p1-todo", Priority: project.PriorityLow},
		"urgent":    {ID: "urgent", ProjectID: "p1", ColumnID: "p1-todo", Priority: project.PriorityUrgent},
		"blocked":   {ID: "blocked", ProjectID: "p1", ColumnID: "p1-todo", Priority: project.PriorityUrgent},
		"held":      {ID: "held", ProjectID: "p1", ColumnID: "p1-todo", Priority: project.PriorityUrgent, ClaimedBy: &other, AssigneeID: &other, LeaseExpiresAt: &later},
		"persons":   {ID: "persons", ProjectID: "p1", ColumnID: "p1-todo", Priority: project.PriorityUrgent, AssigneeID: &person},
		"sentback":  {ID: "sentback", ProjectID: "p1", ColumnID: "p1-doing", Priority: project.PriorityNone, AssigneeID: &me},
		"theirwork": {ID: "theirwork", ProjectID: "p1", ColumnID: "p1-doing", Priority: project.PriorityUrgent, AssigneeID: &other},
	}
	order := []string{"low", "urgent", "blocked", "held", "persons", "sentback", "theirwork"}
	blockers := map[string][]project.TaskLink{
		"blocked": {{LinkedTaskID: "x", ProjectID: "p2", ColumnID: "p2-doing"}}, // blocker in another project, not done
		"urgent":  {{LinkedTaskID: "y", ProjectID: "p2", ColumnID: "p2-done"}},  // blocker done
	}
	s, _ := nextSvc(tasks, order, blockers)
	ctx := context.Background()

	// Sent-back work first, even at lower priority.
	got, err := s.NextTask(ctx, "o1", me, "p1", false, 0, tenant.RoleMember)
	if err != nil || got == nil || got.ID != "sentback" {
		t.Fatalf("first pick = %v, %v; want sentback", got, err)
	}
	// Claiming takes it; the next call moves on to the most urgent ready todo.
	if got, _ = s.NextTask(ctx, "o1", me, "p1", true, 0, tenant.RoleMember); got.ID != "sentback" || *got.ClaimedBy != me {
		t.Fatalf("claimed = %+v", got)
	}
	tasks["sentback"].ColumnID = "p1-review" // handed over
	if got, _ = s.NextTask(ctx, "o1", me, "p1", true, 0, tenant.RoleMember); got == nil || got.ID != "urgent" {
		t.Fatalf("second pick = %v; want urgent (blocked, held and person-assigned skipped)", got)
	}
	// While it still holds "urgent", asking again returns that task, not more work.
	if got, _ = s.NextTask(ctx, "o1", me, "p1", true, 0, tenant.RoleMember); got == nil || got.ID != "urgent" {
		t.Fatalf("held pick = %v; want the task it already holds", got)
	}
	tasks["urgent"].ColumnID = "p1-review" // handed over
	if got, _ = s.NextTask(ctx, "o1", me, "p1", true, 0, tenant.RoleMember); got == nil || got.ID != "low" {
		t.Fatalf("third pick = %v; want low", got)
	}
	tasks["low"].ColumnID = "p1-review"
	if got, err = s.NextTask(ctx, "o1", me, "p1", true, 0, tenant.RoleMember); got != nil || err != nil {
		t.Fatalf("nothing left: %v, %v", got, err)
	}
}

func TestNextTaskSkipsLostRace(t *testing.T) {
	other := "other"
	tasks := map[string]*project.Task{
		"a": {ID: "a", ProjectID: "p1", ColumnID: "p1-todo", Priority: project.PriorityHigh},
		"b": {ID: "b", ProjectID: "p1", ColumnID: "p1-todo", Priority: project.PriorityLow},
	}
	s, store := nextSvc(tasks, []string{"a", "b"}, nil)
	// "a" looks free when listed but another agent takes it before our claim.
	store.claimStore = &claimStore{tasks: tasks, now: claimNow}
	racing := &racingStore{nextStore: store, steal: "a", by: other}
	s.tasks, s.conc = racing, racing
	got, err := s.NextTask(context.Background(), "o1", "me", "p1", true, 0, tenant.RoleMember)
	if err != nil || got == nil || got.ID != "b" {
		t.Fatalf("after losing a: %v, %v", got, err)
	}
}

// racingStore lets another agent claim steal just before our claim lands.
type racingStore struct {
	*nextStore
	steal, by string
}

func (r *racingStore) Claim(ctx context.Context, orgID, id, agent string, exp time.Time) (bool, error) {
	if id == r.steal {
		_, _ = r.nextStore.Claim(ctx, orgID, id, r.by, exp)
	}
	return r.nextStore.Claim(ctx, orgID, id, agent, exp)
}

func TestAgentsCannotCreateOrBulkMoveIntoDone(t *testing.T) {
	tasks := map[string]*project.Task{"a": {ID: "a", ProjectID: "p1", ColumnID: "p1-todo"}}
	s, _ := nextSvc(tasks, []string{"a"}, nil)
	s.columns = doneColumns{}
	ctx := context.Background()
	_, err := s.CreateTask(ctx, "o1", "me", CreateTaskInput{ProjectID: "p1", ColumnID: "p1-done", Title: "x"}, tenant.RoleMember)
	if !errors.Is(err, project.ErrHumanApprovalRequired) {
		t.Fatalf("agent creating into done: %v", err)
	}
	if err := s.refuseAgentDone(ctx, "o1", "alice", "p1", "p1-done"); err != nil {
		t.Fatalf("a person may target done: %v", err)
	}
	if err := s.refuseAgentDone(ctx, "o1", "me", "p1", "p1-todo"); err != nil {
		t.Fatalf("an agent may target todo: %v", err)
	}
}

// doneColumns adds Get to nextColumns for the create path.
type doneColumns struct{ nextColumns }

func (doneColumns) Get(_ context.Context, _, id string) (*project.Column, error) {
	return &project.Column{ID: id, BoardID: "b-p1"}, nil
}
