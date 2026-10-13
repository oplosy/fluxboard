//go:build integration

package postgres

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/project"
)

// Claims and versions (ADR-033, FR-AGT-004/005) against real Postgres: the
// trigger, the conditional UPDATEs and the race between two agents. Fixture
// rows are written as the owner; the repo runs on the RLS-subject app role.
func TestTaskClaimsAndVersions(t *testing.T) {
	ownerDSN := os.Getenv("TEST_DATABASE_OWNER_URL")
	if ownerDSN == "" {
		t.Fatal("TEST_DATABASE_OWNER_URL not set; this test writes its fixture as the owner role")
	}
	app, _ := newIntegrationDB(t)
	ctx := context.Background()
	owner, err := pgxpool.New(ctx, ownerDSN)
	if err != nil {
		t.Fatalf("connect owner: %v", err)
	}
	t.Cleanup(owner.Close)

	id := func() string { return uuid.NewString() }
	person, agentA, agentB := id(), id(), id()
	orgID, projectID, boardID, todo, doing := id(), id(), id(), id(), id()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := owner.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("fixture %q: %v", sql, err)
		}
	}
	for _, u := range []string{person, agentA, agentB} {
		exec(`INSERT INTO users (id, email, name) VALUES ($1, $2, 'claims test')`, u, "claims-"+u[:8]+"@example.test")
	}
	t.Cleanup(func() {
		_, _ = owner.Exec(context.Background(), `DELETE FROM organizations WHERE id = $1`, orgID)
		_, _ = owner.Exec(context.Background(), `DELETE FROM users WHERE id = ANY($1)`, []string{person, agentA, agentB})
	})
	exec(`INSERT INTO organizations (id, slug, name) VALUES ($1, $2, 'claims test')`, orgID, "ct-"+orgID[:8])
	exec(`INSERT INTO projects (id, org_id, key, name, visibility, created_by) VALUES ($1, $2, 'CLM', 'Claims', 'org', $3)`, projectID, orgID, person)
	exec(`INSERT INTO boards (id, org_id, project_id) VALUES ($1, $2, $3)`, boardID, orgID, projectID)
	exec(`INSERT INTO board_columns (id, org_id, board_id, name, rank) VALUES ($1, $3, $4, 'Todo', 'a'), ($2, $3, $4, 'Doing', 'b')`,
		todo, doing, orgID, boardID)
	newTask := func(n int, rank string, assignee *string) string {
		tid := id()
		exec(`INSERT INTO tasks (id, org_id, project_id, column_id, number, title, rank, created_by, assignee_id)
		      VALUES ($1, $2, $3, $4, $5, 'claims task', $6, $7, $8)`, tid, orgID, projectID, todo, n, rank, person, assignee)
		return tid
	}
	repo := NewTaskRepo(NewTenantPool(app))
	get := func(tid string) *project.Task {
		t.Helper()
		tk, err := repo.Get(ctx, orgID, tid)
		if err != nil {
			t.Fatalf("get %s: %v", tid, err)
		}
		return tk
	}
	in := func(d time.Duration) time.Time { return time.Now().Add(d) }

	t.Run("version bumps on content, not on rank or claim", func(t *testing.T) {
		tid := newTask(1, "a", nil)
		tk := get(tid)
		if tk.Version != 1 {
			t.Fatalf("new task version = %d, want 1", tk.Version)
		}
		if err := repo.Move(ctx, orgID, tid, todo, "aa"); err != nil {
			t.Fatal(err)
		}
		if ok, err := repo.Claim(ctx, orgID, tid, agentA, in(time.Hour)); err != nil || !ok {
			t.Fatalf("claim: %v %v", ok, err)
		}
		// The claim assigned the task: that is a content change.
		if v := get(tid).Version; v != 2 {
			t.Fatalf("after reorder + claim(assign) version = %d, want 2", v)
		}
		if ok, _ := repo.Heartbeat(ctx, orgID, tid, agentA, in(2*time.Hour)); !ok {
			t.Fatal("heartbeat by holder failed")
		}
		if v := get(tid).Version; v != 2 {
			t.Fatalf("heartbeat bumped version to %d", v)
		}
	})

	t.Run("stale versions are refused", func(t *testing.T) {
		tid := newTask(2, "b", nil)
		tk := get(tid)
		tk.Title = "first edit"
		if err := repo.UpdateIfVersion(ctx, orgID, tk, 1); err != nil {
			t.Fatalf("fresh update: %v", err)
		}
		tk.Title = "lost update"
		if err := repo.UpdateIfVersion(ctx, orgID, tk, 1); !errors.Is(err, project.ErrVersionConflict) {
			t.Fatalf("stale update: want ErrVersionConflict, got %v", err)
		}
		if err := repo.MoveIfVersion(ctx, orgID, tid, doing, "a", 1); !errors.Is(err, project.ErrVersionConflict) {
			t.Fatalf("stale move: want ErrVersionConflict, got %v", err)
		}
		if err := repo.MoveIfVersion(ctx, orgID, tid, doing, "a", 2); err != nil {
			t.Fatalf("fresh move: %v", err)
		}
		if got := get(tid); got.Title != "first edit" || got.ColumnID != doing || got.Version != 3 {
			t.Fatalf("task = %q in %s v%d", got.Title, got.ColumnID, got.Version)
		}
		if err := repo.UpdateIfVersion(ctx, orgID, &project.Task{ID: id(), Title: "x", Priority: "none"}, 1); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("missing task: want ErrNotFound, got %v", err)
		}
	})

	t.Run("claims block others until the lease lapses", func(t *testing.T) {
		tid := newTask(3, "c", nil)
		if ok, _ := repo.Claim(ctx, orgID, tid, agentA, in(time.Hour)); !ok {
			t.Fatal("A could not claim a free task")
		}
		if ok, _ := repo.Claim(ctx, orgID, tid, agentB, in(time.Hour)); ok {
			t.Fatal("B claimed a task A holds")
		}
		if ok, _ := repo.Heartbeat(ctx, orgID, tid, agentB, in(time.Hour)); ok {
			t.Fatal("B renewed A's claim")
		}
		// A's lease runs out: B may take over and becomes the assignee.
		exec(`UPDATE tasks SET lease_expires_at = now() - interval '1 second' WHERE id = $1`, tid)
		if ok, _ := repo.Heartbeat(ctx, orgID, tid, agentA, in(time.Hour)); ok {
			t.Fatal("A renewed a lapsed claim")
		}
		if ok, _ := repo.Claim(ctx, orgID, tid, agentB, in(time.Hour)); !ok {
			t.Fatal("B could not take over a lapsed claim")
		}
		if got := get(tid); *got.ClaimedBy != agentB || *got.AssigneeID != agentB {
			t.Fatalf("after takeover claimed_by=%v assignee=%v", *got.ClaimedBy, *got.AssigneeID)
		}
		if ok, _ := repo.Release(ctx, orgID, tid, agentA); ok {
			t.Fatal("A released B's claim")
		}
		if ok, _ := repo.Release(ctx, orgID, tid, agentB); !ok {
			t.Fatal("B could not release its claim")
		}
		if got := get(tid); got.ClaimedBy != nil || got.AssigneeID != nil {
			t.Fatalf("release left claimed_by=%v assignee=%v", got.ClaimedBy, got.AssigneeID)
		}
	})

	t.Run("a person's assignment blocks claims", func(t *testing.T) {
		tid := newTask(4, "d", &person)
		if ok, _ := repo.Claim(ctx, orgID, tid, agentA, in(time.Hour)); ok {
			t.Fatal("agent claimed a task assigned to a person")
		}
		// Assigned to the agent by a person: that agent may claim it, others not.
		exec(`UPDATE tasks SET assignee_id = $2 WHERE id = $1`, tid, agentA)
		if ok, _ := repo.Claim(ctx, orgID, tid, agentB, in(time.Hour)); ok {
			t.Fatal("B claimed a task assigned to A")
		}
		if ok, _ := repo.Claim(ctx, orgID, tid, agentA, in(time.Hour)); !ok {
			t.Fatal("A could not claim a task assigned to it")
		}
		// A person reassigns while A's claim is live; after it lapses the
		// person's assignment still blocks everyone.
		exec(`UPDATE tasks SET assignee_id = $2, lease_expires_at = now() - interval '1 second' WHERE id = $1`, tid, person)
		if ok, _ := repo.Claim(ctx, orgID, tid, agentB, in(time.Hour)); ok {
			t.Fatal("B claimed a task a person took back")
		}
	})

	t.Run("two agents racing: exactly one wins", func(t *testing.T) {
		for round := 0; round < 10; round++ {
			tid := newTask(100+round, "r"+string(rune('a'+round)), nil)
			var wg sync.WaitGroup
			wins := make(chan string, 2)
			for _, ag := range []string{agentA, agentB} {
				wg.Add(1)
				go func(ag string) {
					defer wg.Done()
					if ok, err := repo.Claim(ctx, orgID, tid, ag, in(time.Hour)); err == nil && ok {
						wins <- ag
					}
				}(ag)
			}
			wg.Wait()
			close(wins)
			var winners []string
			for w := range wins {
				winners = append(winners, w)
			}
			if len(winners) != 1 {
				t.Fatalf("round %d: %d winners", round, len(winners))
			}
			if got := get(tid); *got.ClaimedBy != winners[0] || *got.AssigneeID != winners[0] {
				t.Fatalf("round %d: winner %s but claimed_by=%s assignee=%s", round, winners[0], *got.ClaimedBy, *got.AssigneeID)
			}
		}
	})

	t.Run("sweeper clears lapsed claims and their assignment only", func(t *testing.T) {
		lapsed, live, reassigned := newTask(200, "s1", nil), newTask(201, "s2", nil), newTask(202, "s3", nil)
		for _, tid := range []string{lapsed, live, reassigned} {
			if ok, _ := repo.Claim(ctx, orgID, tid, agentA, in(time.Hour)); !ok {
				t.Fatalf("claim %s", tid)
			}
		}
		exec(`UPDATE tasks SET lease_expires_at = now() - interval '1 second' WHERE id = ANY($1)`, []string{lapsed, reassigned})
		exec(`UPDATE tasks SET assignee_id = $2 WHERE id = $1`, reassigned, person)
		if _, err := repo.ReleaseExpired(ctx, orgID); err != nil {
			t.Fatal(err)
		}
		if got := get(lapsed); got.ClaimedBy != nil || got.AssigneeID != nil {
			t.Fatalf("lapsed: claimed_by=%v assignee=%v", got.ClaimedBy, got.AssigneeID)
		}
		if got := get(live); got.ClaimedBy == nil || *got.AssigneeID != agentA {
			t.Fatal("sweeper touched a live claim")
		}
		if got := get(reassigned); got.ClaimedBy != nil || got.AssigneeID == nil || *got.AssigneeID != person {
			t.Fatalf("reassigned: the person's assignment must stay (assignee=%v)", got.AssigneeID)
		}
	})
}
