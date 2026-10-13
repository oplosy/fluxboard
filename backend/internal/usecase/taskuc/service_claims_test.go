package taskuc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/project"
	"github.com/mesutokul/fluxboard/backend/internal/domain/tenant"
)

// claimStore is an in-memory task store that is both the TaskRepository (Get,
// Update) and the ConcurrencyRepository, applying the same rules as the SQL.
type claimStore struct {
	project.TaskRepository
	tasks map[string]*project.Task
	now   time.Time
}

func (s *claimStore) Get(_ context.Context, _, id string) (*project.Task, error) {
	t, ok := s.tasks[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	cp := *t
	return &cp, nil
}

func (s *claimStore) Update(_ context.Context, _ string, t *project.Task) error {
	cur := s.tasks[t.ID]
	cp := *t
	cp.Version = cur.Version + 1
	s.tasks[t.ID] = &cp
	return nil
}

func (s *claimStore) UpdateIfVersion(ctx context.Context, orgID string, t *project.Task, v int64) error {
	cur, ok := s.tasks[t.ID]
	if !ok {
		return domain.ErrNotFound
	}
	if cur.Version != v {
		return project.ErrVersionConflict
	}
	return s.Update(ctx, orgID, t)
}

func (s *claimStore) MoveIfVersion(context.Context, string, string, string, string, int64) error {
	return nil
}

func (s *claimStore) Claim(_ context.Context, _, id, agent string, exp time.Time) (bool, error) {
	t, ok := s.tasks[id]
	if !ok {
		return false, domain.ErrNotFound
	}
	held := t.ClaimActive(s.now) && *t.ClaimedBy != agent
	assignedElsewhere := t.AssigneeID != nil && *t.AssigneeID != agent &&
		(t.ClaimedBy == nil || *t.AssigneeID != *t.ClaimedBy)
	if held || assignedElsewhere {
		return false, nil
	}
	if t.AssigneeID == nil || *t.AssigneeID != agent {
		t.Version++
	}
	a := agent
	t.ClaimedBy, t.AssigneeID, t.LeaseExpiresAt = &a, &a, &exp
	return true, nil
}

func (s *claimStore) Heartbeat(_ context.Context, _, id, agent string, exp time.Time) (bool, error) {
	t := s.tasks[id]
	if !t.ClaimActive(s.now) || *t.ClaimedBy != agent {
		return false, nil
	}
	t.LeaseExpiresAt = &exp
	return true, nil
}

func (s *claimStore) Release(_ context.Context, _, id, agent string) (bool, error) {
	t := s.tasks[id]
	if t.ClaimedBy == nil || *t.ClaimedBy != agent {
		return false, nil
	}
	t.ClaimedBy, t.LeaseExpiresAt = nil, nil
	if t.AssigneeID != nil && *t.AssigneeID == agent {
		t.AssigneeID = nil
		t.Version++
	}
	return true, nil
}

func (s *claimStore) EndClaim(_ context.Context, _, id, agent string) (bool, error) {
	t := s.tasks[id]
	if t.ClaimedBy == nil || *t.ClaimedBy != agent {
		return false, nil
	}
	t.ClaimedBy, t.LeaseExpiresAt = nil, nil
	return true, nil
}

func (s *claimStore) ReleaseExpired(context.Context, string) (int, error) { return 0, nil }

type activityLog struct {
	project.ActivityRepository
	entries []*project.Activity
}

func (a *activityLog) Append(_ context.Context, _ string, e *project.Activity) error {
	a.entries = append(a.entries, e)
	return nil
}

var claimNow = time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)

func claimSvc() (*Service, *claimStore, *activityLog) {
	store := &claimStore{tasks: map[string]*project.Task{
		"t1": {ID: "t1", ProjectID: "p1", Title: "one", Priority: project.PriorityNone, Version: 1},
	}, now: claimNow}
	acts := &activityLog{}
	s := New(Deps{
		Tasks: store, Concurrency: store, Activity: acts,
		Projects: fakeProjects{p: &project.Project{ID: "p1", Visibility: project.VisibilityOrg}},
		Members:  fakeMembers{role: role(project.RoleContributor)},
		Now:      func() time.Time { return claimNow },
	})
	return s, store, acts
}

func TestClaimAssignsAndLogs(t *testing.T) {
	s, store, acts := claimSvc()
	ctx := context.Background()
	got, err := s.ClaimTask(ctx, "o1", "agentA", "t1", 0, tenant.RoleMember)
	if err != nil {
		t.Fatal(err)
	}
	if *got.ClaimedBy != "agentA" || *got.AssigneeID != "agentA" || !got.LeaseExpiresAt.Equal(claimNow.Add(project.DefaultLease)) {
		t.Fatalf("claim = %+v", got)
	}
	if len(acts.entries) != 1 || acts.entries[0].Field != "assignee" || *acts.entries[0].NewValue != "agentA" {
		t.Fatalf("activity = %+v", acts.entries)
	}
	// Renewing its own claim neither fails nor logs a second assignment.
	if _, err := s.ClaimTask(ctx, "o1", "agentA", "t1", 10*time.Minute, tenant.RoleMember); err != nil {
		t.Fatal(err)
	}
	if len(acts.entries) != 1 {
		t.Fatalf("renewal logged an assignment: %+v", acts.entries)
	}

	// Another agent gets a ClaimConflict naming the holder and its lease.
	_, err = s.ClaimTask(ctx, "o1", "agentB", "t1", 0, tenant.RoleMember)
	var cc *project.ClaimConflict
	if !errors.As(err, &cc) || cc.HolderID != "agentA" || cc.LeaseExpiresAt == nil || !errors.Is(err, project.ErrClaimHeld) {
		t.Fatalf("second agent: %v", err)
	}
	// A person-assigned task names the person, with no lease.
	p := "person"
	store.tasks["t1"].ClaimedBy, store.tasks["t1"].LeaseExpiresAt, store.tasks["t1"].AssigneeID = nil, nil, &p
	_, err = s.ClaimTask(ctx, "o1", "agentB", "t1", 0, tenant.RoleMember)
	if !errors.As(err, &cc) || cc.HolderID != "person" || cc.LeaseExpiresAt != nil {
		t.Fatalf("person-assigned: %v", err)
	}
}

func TestHeartbeatAndRelease(t *testing.T) {
	s, store, acts := claimSvc()
	ctx := context.Background()
	if _, err := s.HeartbeatTask(ctx, "o1", "agentA", "t1", 0, tenant.RoleMember); !errors.Is(err, project.ErrClaimLost) {
		t.Fatalf("heartbeat without a claim: %v", err)
	}
	if _, err := s.ClaimTask(ctx, "o1", "agentA", "t1", 0, tenant.RoleMember); err != nil {
		t.Fatal(err)
	}
	got, err := s.HeartbeatTask(ctx, "o1", "agentA", "t1", time.Hour, tenant.RoleMember)
	if err != nil || !got.LeaseExpiresAt.Equal(claimNow.Add(time.Hour)) {
		t.Fatalf("heartbeat = %v, %v", got, err)
	}
	if _, err := s.ReleaseTask(ctx, "o1", "agentB", "t1", tenant.RoleMember); !errors.Is(err, project.ErrClaimLost) {
		t.Fatalf("release by non-holder: %v", err)
	}
	got, err = s.ReleaseTask(ctx, "o1", "agentA", "t1", tenant.RoleMember)
	if err != nil || got.ClaimedBy != nil || got.AssigneeID != nil {
		t.Fatalf("release = %+v, %v", got, err)
	}
	if n := len(acts.entries); n != 2 || acts.entries[1].NewValue != nil {
		t.Fatalf("release should log the unassignment: %+v", acts.entries)
	}
	// A lapsed claim cannot be renewed.
	store.tasks["t1"].ClaimedBy = ptr("agentA")
	lapsed := claimNow.Add(-time.Second)
	store.tasks["t1"].LeaseExpiresAt = &lapsed
	if _, err := s.HeartbeatTask(ctx, "o1", "agentA", "t1", 0, tenant.RoleMember); !errors.Is(err, project.ErrClaimLost) {
		t.Fatalf("heartbeat on a lapsed claim: %v", err)
	}
}

func TestClaimValidatesLeaseAndAccess(t *testing.T) {
	s, _, _ := claimSvc()
	ctx := context.Background()
	for _, d := range []time.Duration{30 * time.Second, 5 * time.Hour} {
		_, err := s.ClaimTask(ctx, "o1", "agentA", "t1", d, tenant.RoleMember)
		var ce *domain.CodedError
		if !errors.As(err, &ce) || ce.Code != "lease_invalid" {
			t.Fatalf("lease %v: %v", d, err)
		}
	}
	viewer := New(Deps{
		Tasks:       &claimStore{tasks: map[string]*project.Task{"t1": {ID: "t1", ProjectID: "p1"}}},
		Concurrency: &claimStore{}, Projects: fakeProjects{p: &project.Project{ID: "p1", Visibility: project.VisibilityOrg}},
		Members: fakeMembers{}, Now: func() time.Time { return claimNow },
	})
	if _, err := viewer.ClaimTask(ctx, "o1", "agentA", "t1", 0, tenant.RoleMember); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("a viewer must not claim: %v", err)
	}
}

func TestUpdateTaskIfVersion(t *testing.T) {
	s, _, _ := claimSvc()
	ctx := context.Background()
	stale := int64(7)
	_, err := s.UpdateTask(ctx, "o1", "u1", "t1", UpdateTaskInput{Title: "new", IfVersion: &stale}, tenant.RoleMember)
	var vc *project.VersionConflict
	if !errors.As(err, &vc) || vc.Current.Version != 1 || !errors.Is(err, project.ErrVersionConflict) {
		t.Fatalf("stale update: %v", err)
	}
	fresh := int64(1)
	got, err := s.UpdateTask(ctx, "o1", "u1", "t1", UpdateTaskInput{Title: "new", IfVersion: &fresh}, tenant.RoleMember)
	if err != nil || got.Title != "new" || got.Version != 2 {
		t.Fatalf("fresh update = %+v, %v", got, err)
	}
	// Without a version the write is unconditional, as before.
	if _, err := s.UpdateTask(ctx, "o1", "u1", "t1", UpdateTaskInput{Title: "newer"}, tenant.RoleMember); err != nil {
		t.Fatal(err)
	}
}

func ptr(s string) *string { return &s }
