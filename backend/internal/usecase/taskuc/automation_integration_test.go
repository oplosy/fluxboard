//go:build integration

package taskuc

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mesutokul/fluxboard/backend/internal/infrastructure/postgres"
	"github.com/mesutokul/fluxboard/backend/internal/usecase/automationuc"
)

// Time-triggered automations against real Postgres with RLS (ADR-036): a
// due-soon and an overdue rule fire at most once per task per rule however
// often (or concurrently) the scan runs; the action goes through the normal
// task write path (activity attributed to the rule); done and trashed tasks
// are left alone.
func TestAutomationTimeTriggersFireOnce(t *testing.T) {
	appDSN, ownerDSN := os.Getenv("TEST_DATABASE_URL"), os.Getenv("TEST_DATABASE_OWNER_URL")
	if appDSN == "" || ownerDSN == "" {
		t.Fatal("TEST_DATABASE_URL and TEST_DATABASE_OWNER_URL must be set (see make test-integration)")
	}
	ctx := context.Background()
	connect := func(dsn string) *pgxpool.Pool {
		p, err := pgxpool.New(ctx, dsn)
		if err != nil {
			t.Fatalf("connect: %v", err)
		}
		t.Cleanup(p.Close)
		return p
	}
	app, owner := connect(appDSN), connect(ownerDSN)

	id := uuid.NewString
	userID, orgID, projectID, boardID, todo, done := id(), id(), id(), id(), id(), id()
	soonID, lateID, doneID, trashedID, farID := id(), id(), id(), id(), id()
	ruleSoon, ruleLate := id(), id()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := owner.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("fixture %q: %v", sql, err)
		}
	}
	exec(`INSERT INTO users (id, email, name) VALUES ($1, $2, 'automation test')`, userID, "auto-"+userID[:8]+"@example.test")
	t.Cleanup(func() {
		_, _ = owner.Exec(context.Background(), `DELETE FROM organizations WHERE id = $1`, orgID)
		_, _ = owner.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID)
	})
	exec(`INSERT INTO organizations (id, slug, name) VALUES ($1, $2, 'automation test')`, orgID, "au-"+orgID[:8])
	exec(`INSERT INTO memberships (org_id, user_id, role) VALUES ($1, $2, 'ADMIN')`, orgID, userID)
	exec(`INSERT INTO projects (id, org_id, key, name, visibility, created_by) VALUES ($1, $2, 'AUT', 'Auto', 'org', $3)`, projectID, orgID, userID)
	exec(`INSERT INTO boards (id, org_id, project_id) VALUES ($1, $2, $3)`, boardID, orgID, projectID)
	exec(`INSERT INTO board_columns (id, org_id, board_id, name, rank) VALUES ($1, $3, $4, 'To Do', 'a'), ($2, $3, $4, 'Done', 'b')`,
		todo, done, orgID, boardID)

	now := time.Now().UTC()
	n := 0
	task := func(tid, column string, due time.Time) {
		t.Helper()
		n++
		exec(`INSERT INTO tasks (id, org_id, project_id, column_id, number, title, priority, due_date, rank, created_by)
		      VALUES ($1, $2, $3, $4, $5, $6, 'none', $7, $8, $9)`,
			tid, orgID, projectID, column, n, "task", due, string(rune('a'+n)), userID)
	}
	task(soonID, todo, now.Add(2*time.Hour))     // due soon
	task(farID, todo, now.Add(72*time.Hour))     // outside the 24h window
	task(lateID, todo, now.Add(-3*time.Hour))    // overdue
	task(doneID, todo, now.Add(-3*time.Hour))    // overdue but finished
	task(trashedID, todo, now.Add(-3*time.Hour)) // overdue but trashed
	exec(`UPDATE tasks SET column_id = $2 WHERE id = $1`, doneID, done)
	exec(`UPDATE tasks SET deleted_at = now() WHERE id = $1`, trashedID)

	exec(`INSERT INTO automation_rules (id, org_id, name, enabled, trigger, trigger_config, action, action_config, created_by)
	      VALUES ($1, $3, 'soon', true, 'task.due_soon', '{"hours": 24}', 'set_priority', '{"priority": "high"}', $4),
	             ($2, $3, 'late', true, 'task.overdue', '{}', 'set_priority', '{"priority": "urgent"}', $4)`,
		ruleSoon, ruleLate, orgID, userID)

	tp := postgres.NewTenantPool(app)
	automationSvc := automationuc.New(automationuc.Deps{
		Rules: postgres.NewAutomationRepo(tp), Runs: postgres.NewAutomationRunRepo(tp),
	})
	svc := New(Deps{
		Tasks: postgres.NewTaskRepo(tp), Labels: postgres.NewLabelRepo(tp), Projects: postgres.NewProjectRepo(tp),
		Members: postgres.NewProjectMemberRepo(tp), Boards: postgres.NewBoardRepo(tp), Columns: postgres.NewColumnRepo(tp),
		Activity: postgres.NewActivityRepo(tp), Automation: automationSvc,
	})
	automationSvc.SetActions(svc)

	// Sequential repeats and concurrent scans (two workers) must all agree.
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := automationSvc.ScanTimeTriggers(ctx, orgID); err != nil {
				t.Errorf("scan: %v", err)
			}
		}()
	}
	wg.Wait()
	for range 2 {
		if _, err := automationSvc.ScanTimeTriggers(ctx, orgID); err != nil {
			t.Fatalf("scan: %v", err)
		}
	}

	count := func(sql string, args ...any) int {
		t.Helper()
		var c int
		if err := owner.QueryRow(ctx, sql, args...).Scan(&c); err != nil {
			t.Fatal(err)
		}
		return c
	}
	if c := count(`SELECT count(*) FROM automation_runs WHERE org_id = $1 AND status = 'success'`, orgID); c != 2 {
		t.Fatalf("success runs = %d, want 2 (one per rule+task)", c)
	}
	if c := count(`SELECT count(*) FROM automation_runs WHERE org_id = $1`, orgID); c != 2 {
		t.Fatalf("total runs = %d, want 2", c)
	}
	for tid, want := range map[string]string{soonID: "high", lateID: "urgent", farID: "none", doneID: "none", trashedID: "none"} {
		var p string
		if err := owner.QueryRow(ctx, `SELECT priority FROM tasks WHERE id = $1`, tid).Scan(&p); err != nil || p != want {
			t.Fatalf("task %s priority = %q (%v), want %q", tid, p, err, want)
		}
	}
	// Went through the normal write path: one activity row per change, naming the rule.
	if c := count(`SELECT count(*) FROM task_activity WHERE org_id = $1 AND field = 'priority' AND automation_rule_id IN ($2, $3)`, orgID, ruleSoon, ruleLate); c != 2 {
		t.Fatalf("rule-attributed activity rows = %d, want 2", c)
	}
	// A rule edited back to a new threshold must not refire for a claimed task.
	exec(`UPDATE tasks SET priority = 'none' WHERE id = $1`, soonID)
	if _, err := automationSvc.ScanTimeTriggers(ctx, orgID); err != nil {
		t.Fatal(err)
	}
	var p string
	if err := owner.QueryRow(ctx, `SELECT priority FROM tasks WHERE id = $1`, soonID).Scan(&p); err != nil || p != "none" {
		t.Fatalf("due-soon rule refired on a claimed task: priority=%q (%v)", p, err)
	}
}
