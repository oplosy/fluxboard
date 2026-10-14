//go:build integration

package postgres

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// tasks.completed_at (0038) on real Postgres and the rollup that reads it
// (FR-AN-001): entering the done column stamps it, leaving clears it, column
// reorders and edits leave it alone, and ComputeDay turns it into the day's
// completed count and cycle time.
func TestTaskCompletedAtAndRollup(t *testing.T) {
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
	user, orgID, proj, board := id(), id(), id(), id()
	todo, doing, done, archive := id(), id(), id(), id()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := owner.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("fixture %q: %v", sql, err)
		}
	}
	exec(`INSERT INTO users (id, email, name) VALUES ($1, $2, 'done test')`, user, "done-"+user[:8]+"@example.test")
	t.Cleanup(func() {
		_, _ = owner.Exec(context.Background(), `DELETE FROM organizations WHERE id = $1`, orgID)
		_, _ = owner.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, user)
	})
	exec(`INSERT INTO organizations (id, slug, name) VALUES ($1, $2, 'done test')`, orgID, "dt-"+orgID[:8])
	exec(`INSERT INTO projects (id, org_id, key, name, created_by) VALUES ($1, $2, 'DT', 'Done', $3)`, proj, orgID, user)
	exec(`INSERT INTO boards (id, org_id, project_id) VALUES ($1, $2, $3)`, board, orgID, proj)
	// "Done" is found by name although it is not the last column.
	exec(`INSERT INTO board_columns (id, org_id, board_id, name, rank) VALUES
	      ($1, $5, $6, 'To Do', 'a'), ($2, $5, $6, 'Doing', 'b'), ($3, $5, $6, 'Done', 'c'), ($4, $5, $6, 'Archive', 'd')`,
		todo, doing, done, archive, orgID, board)

	n := 0
	task := func(column string, createdAgo time.Duration) string {
		t.Helper()
		n++
		tid := id()
		exec(`INSERT INTO tasks (id, org_id, project_id, column_id, number, title, rank, created_by, created_at)
		      VALUES ($1, $2, $3, $4, $5, 'task', $6, $7, now() - make_interval(secs => $8))`,
			tid, orgID, proj, column, n, "r"+tid[:8], user, createdAgo.Seconds())
		return tid
	}
	completedAt := func(tid string) *time.Time {
		t.Helper()
		var at *time.Time
		if err := owner.QueryRow(ctx, `SELECT completed_at FROM tasks WHERE id = $1`, tid).Scan(&at); err != nil {
			t.Fatal(err)
		}
		return at
	}
	move := func(tid, column string) { exec(`UPDATE tasks SET column_id = $2 WHERE id = $1`, tid, column) }

	moved := task(todo, 2*time.Hour)
	if completedAt(moved) != nil {
		t.Fatal("a task in To Do must not be completed")
	}
	move(moved, done)
	first := completedAt(moved)
	if first == nil {
		t.Fatal("moving into Done must stamp completed_at")
	}

	born := task(done, 4*time.Hour)
	if completedAt(born) == nil {
		t.Fatal("a task created in Done must be completed")
	}

	reopened := task(todo, time.Hour)
	move(reopened, done)
	move(reopened, doing)
	if completedAt(reopened) != nil {
		t.Fatal("leaving Done must clear completed_at")
	}

	trashed := task(done, time.Hour)
	exec(`UPDATE tasks SET deleted_at = now() WHERE id = $1`, trashed)

	// Edits and column reorders do not rewrite history; callers cannot set it.
	exec(`UPDATE tasks SET title = 'renamed', completed_at = NULL WHERE id = $1`, moved)
	exec(`UPDATE board_columns SET rank = 'z' WHERE id = $1`, done)
	if at := completedAt(moved); at == nil || !at.Equal(*first) {
		t.Fatalf("completed_at after edit/reorder = %v, want %v", at, *first)
	}

	// An explicit done role wins over the name.
	exec(`UPDATE board_columns SET role = 'done' WHERE id = $1`, archive)
	archived := task(todo, time.Hour)
	move(archived, archive)
	if completedAt(archived) == nil {
		t.Fatal("moving into the column holding the done role must stamp completed_at")
	}

	stat, err := NewStatsRepo(NewTenantPool(app)).ComputeDay(ctx, orgID, proj, time.Now())
	if err != nil {
		t.Fatalf("compute day: %v", err)
	}
	// moved (2h), born (4h), archived (1h); reopened and trashed do not count.
	if stat.CompletedCount != 3 {
		t.Fatalf("CompletedCount = %d, want 3", stat.CompletedCount)
	}
	if stat.AvgCycleSeconds == nil {
		t.Fatal("AvgCycleSeconds nil, want the mean of three completions")
	}
	if got, want := time.Duration(*stat.AvgCycleSeconds)*time.Second, (7*time.Hour)/3; got < want-time.Minute || got > want+time.Minute {
		t.Fatalf("AvgCycleSeconds = %v, want about %v", got, want)
	}

	yesterday, err := NewStatsRepo(NewTenantPool(app)).ComputeDay(ctx, orgID, proj, time.Now().AddDate(0, 0, -1))
	if err != nil {
		t.Fatal(err)
	}
	if yesterday.CompletedCount != 0 || yesterday.AvgCycleSeconds != nil {
		t.Fatalf("yesterday = %d completions, cycle %v; want none", yesterday.CompletedCount, yesterday.AvgCycleSeconds)
	}
}
