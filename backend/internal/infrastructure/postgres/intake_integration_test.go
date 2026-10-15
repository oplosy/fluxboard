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
	"github.com/mesutokul/fluxboard/backend/internal/domain/tenant"
	"github.com/mesutokul/fluxboard/backend/internal/pkg/token"
	"github.com/mesutokul/fluxboard/backend/internal/usecase/projectuc"
)

// Request inbox on real Postgres: dedupe (incl. concurrent), per-form rate
// limit, tenant isolation, status-guarded triage, completion detection through
// tasks.completed_at, and the submitter never becoming a user / audit actor.
func TestIntakeRequests(t *testing.T) {
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
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := owner.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("fixture %q: %v", sql, err)
		}
	}
	user := id()
	exec(`INSERT INTO users (id, email, name) VALUES ($1, $2, 'inbox test')`, user, "inbox-"+user[:8]+"@example.test")
	type org struct{ id, proj, board, todo, done, form string }
	rawToken := "inbox-" + id()
	mkOrg := func(tag string, raw string) org {
		o := org{id: id(), proj: id(), board: id(), todo: id(), done: id(), form: id()}
		exec(`INSERT INTO organizations (id, slug, name) VALUES ($1, $2, $3)`, o.id, tag+"-"+o.id[:8], "inbox "+tag)
		exec(`INSERT INTO projects (id, org_id, key, name, created_by) VALUES ($1, $2, 'IN', 'Inbox', $3)`, o.proj, o.id, user)
		exec(`INSERT INTO boards (id, org_id, project_id) VALUES ($1, $2, $3)`, o.board, o.id, o.proj)
		exec(`INSERT INTO board_columns (id, org_id, board_id, name, rank) VALUES ($1, $3, $4, 'To Do', 'a'), ($2, $3, $4, 'Done', 'b')`, o.todo, o.done, o.id, o.board)
		exec(`INSERT INTO project_forms (id, org_id, project_id, name, target_column_id, token_hash, created_by)
		      VALUES ($1, $2, $3, 'Bugs', $4, $5, $6)`, o.form, o.id, o.proj, o.todo, token.Hash(raw), user)
		return o
	}
	a, b := mkOrg("ia", rawToken), mkOrg("ib", "inbox-"+id())
	t.Cleanup(func() {
		for _, o := range []string{a.id, b.id} {
			_, _ = owner.Exec(context.Background(), `DELETE FROM audit_log WHERE org_id = $1`, o)
			_, _ = owner.Exec(context.Background(), `DELETE FROM organizations WHERE id = $1`, o)
		}
		_, _ = owner.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, user)
	})

	tp := NewTenantPool(app)
	repo := NewIntakeRepo(tp)
	mk := func(o org, title, email string) *project.IntakeRequest {
		return &project.IntakeRequest{
			ID: id(), ProjectID: o.proj, FormID: o.form, FormSchemaVersion: 1, FormSnapshot: []byte(`{"target_column_id":"` + o.todo + `"}`),
			Title: title, Priority: project.PriorityNone, SubmitterName: "Ada", SubmitterEmail: email,
			ContentHash: []byte("hash-" + title),
		}
	}
	const win, rateWin = time.Hour, time.Hour

	// Dedupe: same form + email (case-insensitive) + content inside the window.
	first, created, err := repo.Submit(ctx, a.id, mk(a, "dup", "Ada@Example.com"), win, 100, rateWin)
	if err != nil || !created {
		t.Fatalf("first submit: created=%v err=%v", created, err)
	}
	again, created, err := repo.Submit(ctx, a.id, mk(a, "dup", "ada@example.com"), win, 100, rateWin)
	if err != nil || created || again != first {
		t.Fatalf("duplicate: id=%s want %s created=%v err=%v", again, first, created, err)
	}
	if _, created, _ = repo.Submit(ctx, a.id, mk(a, "other", "ada@example.com"), win, 100, rateWin); !created {
		t.Fatal("different content must be a new request")
	}
	if _, created, _ = repo.Submit(ctx, a.id, mk(a, "dup", "grace@example.com"), win, 100, rateWin); !created {
		t.Fatal("different email must be a new request")
	}

	// Concurrent identical posts collapse to a single row.
	var wg sync.WaitGroup
	ids := make([]string, 8)
	for i := range ids {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ids[i], _, _ = repo.Submit(ctx, a.id, mk(a, "race", "race@example.com"), win, 100, rateWin)
		}()
	}
	wg.Wait()
	var n int
	if err := owner.QueryRow(ctx, `SELECT count(*) FROM intake_requests WHERE org_id = $1 AND title = 'race'`, a.id).Scan(&n); err != nil || n != 1 {
		t.Fatalf("race rows = %d err=%v, want 1", n, err)
	}

	// Per-form rate limit counts new requests, never duplicates.
	var total int
	_ = owner.QueryRow(ctx, `SELECT count(*) FROM intake_requests WHERE org_id = $1`, a.id).Scan(&total)
	if _, _, err := repo.Submit(ctx, a.id, mk(a, "over", "o@example.com"), win, total, rateWin); !errors.Is(err, domain.ErrRateLimited) {
		t.Fatalf("over limit err = %v, want ErrRateLimited", err)
	}
	if got, created, err := repo.Submit(ctx, a.id, mk(a, "dup", "ada@example.com"), win, total, rateWin); err != nil || created || got != first {
		t.Fatalf("duplicate at the limit: id=%s created=%v err=%v", got, created, err)
	}

	// Tenant isolation: org B cannot see or touch org A's request.
	if _, err := repo.Get(ctx, b.id, first); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("cross-tenant Get err = %v, want ErrNotFound", err)
	}
	if rows, err := repo.List(ctx, b.id, a.proj, "", 50); err != nil || len(rows) != 0 {
		t.Fatalf("cross-tenant List rows=%d err=%v", len(rows), err)
	}
	if err := repo.Decline(ctx, b.id, first, "nope", user); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("cross-tenant Decline err = %v, want no row matched", err)
	}
	if got, err := repo.Get(ctx, a.id, first); err != nil || got.Status != project.IntakeNew {
		t.Fatalf("request after cross-tenant decline: %+v err=%v", got, err)
	}

	// Service level: submit by token, accept, complete, mail due, audit.
	svc := projectuc.New(projectuc.Deps{
		Projects: NewProjectRepo(tp), Members: NewProjectMemberRepo(tp), Boards: NewBoardRepo(tp),
		Columns: NewColumnRepo(tp), Tasks: NewTaskRepo(tp), Forms: NewFormRepo(app, tp),
		Intake: repo, Audit: NewAuditRepo(app),
	})
	reqID, err := svc.SubmitForm(ctx, rawToken, projectuc.SubmitFormInput{
		Title: "Via service", SubmitterName: "Zed Submitter", SubmitterEmail: "zed@submitter.test",
	})
	if err != nil {
		t.Fatalf("SubmitForm: %v", err)
	}
	var tasksBefore int
	_ = owner.QueryRow(ctx, `SELECT count(*) FROM tasks WHERE org_id = $1`, a.id).Scan(&tasksBefore)
	if tasksBefore != 0 {
		t.Fatalf("submit created %d tasks, want 0 before triage", tasksBefore)
	}
	task, err := svc.AcceptRequest(ctx, a.id, user, a.proj, reqID, tenant.RoleAdmin)
	if err != nil {
		t.Fatalf("AcceptRequest: %v", err)
	}
	if task.CreatedBy != user || task.ColumnID != a.todo {
		t.Fatalf("task = %+v, want created_by accepting user in the form's column", task)
	}
	if _, err := svc.AcceptRequest(ctx, a.id, user, a.proj, reqID, tenant.RoleAdmin); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("second accept err = %v, want ErrConflict", err)
	}
	var tasksAfter int
	_ = owner.QueryRow(ctx, `SELECT count(*) FROM tasks WHERE org_id = $1`, a.id).Scan(&tasksAfter)
	if tasksAfter != 1 {
		t.Fatalf("tasks after double accept = %d, want 1", tasksAfter)
	}

	// Submitter is plain text only: no user row, no audit actor, no mention.
	var users, auditActors, auditMentions int
	_ = owner.QueryRow(ctx, `SELECT count(*) FROM users WHERE email = 'zed@submitter.test'`).Scan(&users)
	_ = owner.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE org_id = $1 AND action LIKE 'intake.%' AND actor_user_id = $2`, a.id, user).Scan(&auditActors)
	_ = owner.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE org_id = $1 AND (metadata::text LIKE '%zed@submitter.test%' OR metadata::text LIKE '%Zed Submitter%')`, a.id).Scan(&auditMentions)
	if users != 0 || auditActors != 1 || auditMentions != 0 {
		t.Fatalf("users=%d auditByAccepter=%d auditMentioningSubmitter=%d, want 0/1/0", users, auditActors, auditMentions)
	}

	// Completing the task (moving it into the done column) completes the request.
	if n, err := repo.CompleteDone(ctx, a.id); err != nil || n != 0 {
		t.Fatalf("CompleteDone before task done: n=%d err=%v", n, err)
	}
	exec(`UPDATE tasks SET column_id = $2 WHERE id = $1`, task.ID, a.done)
	if n, err := repo.CompleteDone(ctx, a.id); err != nil || n != 1 {
		t.Fatalf("CompleteDone after task done: n=%d err=%v", n, err)
	}
	due, err := repo.MailDue(ctx, a.id, 50)
	if err != nil {
		t.Fatal(err)
	}
	var sawAck, sawOutcome bool
	for _, m := range due {
		if m.ID == reqID {
			sawAck, sawOutcome = m.AckDue, m.OutcomeDue
		}
	}
	if !sawAck || !sawOutcome {
		t.Fatalf("mail due ack=%v outcome=%v, want both", sawAck, sawOutcome)
	}
	if err := repo.MarkAcked(ctx, a.id, reqID); err != nil {
		t.Fatal(err)
	}
	if err := repo.MarkOutcomeSent(ctx, a.id, reqID); err != nil {
		t.Fatal(err)
	}
	due, _ = repo.MailDue(ctx, a.id, 50)
	for _, m := range due {
		if m.ID == reqID {
			t.Fatalf("request still owes mail after marks: %+v", m)
		}
	}
}
