//go:build integration

package postgres

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mesutokul/fluxboard/backend/internal/domain/project"
	"github.com/mesutokul/fluxboard/backend/internal/usecase/taskuc"
)

// Recurring-task generation on real Postgres (research report P1): repeated,
// concurrent and replayed runs create each (rule, date) occurrence once; the
// rule's timezone decides the day; a monthly 31st clamps to February's end;
// "after completion" waits for the latest occurrence to reach Done.
func TestRecurrenceGenerationIsDuplicateFree(t *testing.T) {
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
	user, orgID, proj, board, backlog, todo, done := id(), id(), id(), id(), id(), id(), id()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := owner.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("fixture %q: %v", sql, err)
		}
	}
	exec(`INSERT INTO users (id, email, name) VALUES ($1, $2, 'recur test')`, user, "recur-"+user[:8]+"@example.test")
	t.Cleanup(func() {
		_, _ = owner.Exec(context.Background(), `DELETE FROM organizations WHERE id = $1`, orgID)
		_, _ = owner.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, user)
	})
	exec(`INSERT INTO organizations (id, slug, name) VALUES ($1, $2, 'recur test')`, orgID, "rc-"+orgID[:8])
	exec(`INSERT INTO projects (id, org_id, key, name, created_by) VALUES ($1, $2, 'RC', 'Recur', $3)`, proj, orgID, user)
	exec(`INSERT INTO boards (id, org_id, project_id) VALUES ($1, $2, $3)`, board, orgID, proj)
	// Occurrences go to the to-do column even though it is not the first.
	exec(`INSERT INTO board_columns (id, org_id, board_id, name, rank) VALUES ($1, $3, $4, 'Backlog', 'a'), ($2, $3, $4, 'To Do', 'b'), ($5, $3, $4, 'Done', 'c')`,
		backlog, todo, orgID, board, done)
	template := func(title string) string {
		t.Helper()
		tid, n := id(), 0
		if err := owner.QueryRow(ctx, `UPDATE projects SET task_counter = task_counter + 1 WHERE id = $1 RETURNING task_counter`, proj).Scan(&n); err != nil {
			t.Fatal(err)
		}
		exec(`INSERT INTO tasks (id, org_id, project_id, column_id, number, title, rank, created_by, priority)
		      VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'high')`, tid, orgID, proj, done, n, title, "t"+tid[:8], user)
		return tid
	}

	tp := NewTenantPool(app)
	repo := NewRecurrenceRepo(tp)
	var now time.Time
	svc := taskuc.New(taskuc.Deps{Tasks: NewTaskRepo(tp), Columns: NewColumnRepo(tp), Recurrences: repo, Now: func() time.Time { return now }})
	generate := func() int {
		t.Helper()
		created, err := svc.GenerateOccurrences(ctx, orgID)
		if err != nil {
			t.Fatalf("generate: %v", err)
		}
		return created
	}
	occurrences := func(templateID string) []string {
		t.Helper()
		rows, err := owner.Query(ctx, `SELECT o.occurrence_on::text FROM tasks o
		  JOIN task_recurrences r ON r.id = o.recurrence_id WHERE r.task_id = $1 ORDER BY 1`, templateID)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var out []string
		for rows.Next() {
			var d string
			if err := rows.Scan(&d); err != nil {
				t.Fatal(err)
			}
			out = append(out, d)
		}
		return out
	}
	setRule := func(templateID string, r project.Recurrence) {
		t.Helper()
		r.ID, r.OrgID, r.TaskID, r.CreatedBy = id(), orgID, templateID, user
		if err := repo.Upsert(ctx, orgID, &r); err != nil {
			t.Fatalf("upsert rule: %v", err)
		}
	}
	day := func(s string) time.Time { d, _ := time.Parse(time.DateOnly, s); return d }

	// Daily in Istanbul (UTC+3): 21:30 UTC on Oct 14 is already Oct 15 there.
	daily := template("water the plants")
	setRule(daily, project.Recurrence{Freq: project.FreqDaily, Timezone: "Europe/Istanbul",
		Mode: project.ModeSchedule, LastOn: day("2026-10-14")})
	now = time.Date(2026, 10, 14, 20, 30, 0, 0, time.UTC)
	if got := generate(); got != 0 {
		t.Fatalf("before local midnight: created %d, want 0", got)
	}
	now = time.Date(2026, 10, 14, 21, 30, 0, 0, time.UTC)
	if got := generate(); got != 1 {
		t.Fatalf("after local midnight: created %d, want 1", got)
	}
	if got := generate(); got != 0 {
		t.Fatalf("repeat run: created %d, want 0", got)
	}

	// Concurrent runs (overlapping workers / an Asynq retry racing the next tick).
	now = time.Date(2026, 10, 15, 22, 0, 0, 0, time.UTC) // Oct 16 in Istanbul
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := svc.GenerateOccurrences(ctx, orgID); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent generate: %v", err)
	}

	// A replay whose last_on bookkeeping was lost still hits the unique identity.
	exec(`UPDATE task_recurrences SET last_on = '2026-10-14' WHERE task_id = $1`, daily)
	if got := generate(); got != 0 {
		t.Fatalf("replay: created %d, want 0", got)
	}
	if got := occurrences(daily); len(got) != 2 || got[0] != "2026-10-15" || got[1] != "2026-10-16" {
		t.Fatalf("daily occurrences = %v, want [2026-10-15 2026-10-16]", got)
	}

	// The copy lands in the to-do column with the template's fields and its date due.
	var col, title, priority string
	var due time.Time
	if err := owner.QueryRow(ctx, `SELECT column_id::text, title, priority, due_date FROM tasks o
	    JOIN task_recurrences r ON r.id = o.recurrence_id WHERE r.task_id = $1 AND o.occurrence_on = '2026-10-16'`, daily).
		Scan(&col, &title, &priority, &due); err != nil {
		t.Fatal(err)
	}
	if col != todo || title != "water the plants" || priority != "high" || !due.Equal(day("2026-10-16")) {
		t.Fatalf("occurrence = (%s, %q, %s, %s), want (To Do, template title, high, 2026-10-16)", col, title, priority, due)
	}

	// Paused rules generate nothing.
	exec(`UPDATE task_recurrences SET paused = true WHERE task_id = $1`, daily)
	now = time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	if got := generate(); got != 0 {
		t.Fatalf("paused: created %d, want 0", got)
	}

	// Monthly on the 31st clamps to Feb 28, once.
	monthly := template("close the books")
	setRule(monthly, project.Recurrence{Freq: project.FreqMonthly, MonthDay: 31, Timezone: "UTC",
		Mode: project.ModeSchedule, LastOn: day("2027-01-31")})
	for _, at := range []string{"2027-02-27", "2027-02-28", "2027-02-28", "2027-03-01", "2027-03-30"} {
		now = day(at).Add(12 * time.Hour)
		generate()
	}
	if got := occurrences(monthly); len(got) != 1 || got[0] != "2027-02-28" {
		t.Fatalf("monthly occurrences = %v, want [2027-02-28]", got)
	}

	// After completion: nothing while the latest instance is open.
	after := template("review backups")
	exec(`UPDATE tasks SET column_id = $2 WHERE id = $1`, after, todo) // reopen the template
	setRule(after, project.Recurrence{Freq: project.FreqDaily, Timezone: "UTC",
		Mode: project.ModeAfterCompletion, LastOn: day("2026-10-01")})
	now = time.Now().Add(48 * time.Hour)
	if got := occurrences(after); generate() != 0 || len(got) != 0 {
		t.Fatalf("after completion with an open template: occurrences %v, want none", got)
	}
	exec(`UPDATE tasks SET column_id = $2 WHERE id = $1`, after, done) // completed_at = now()
	generate()
	if got := occurrences(after); len(got) != 1 {
		t.Fatalf("after completion: occurrences %v, want one", got)
	}
	generate() // the new occurrence is open: no further copies
	if got := occurrences(after); len(got) != 1 {
		t.Fatalf("after completion with an open occurrence: occurrences %v, want one", got)
	}
}
