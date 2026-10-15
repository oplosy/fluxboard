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

// Task approvals (0045/0046, ADR-037) on real Postgres: the self-approval
// policy holds through the repository, a decision on a completed task goes
// stale when the task is reopened or edited, concurrent decisions never lose
// one, and another tenant sees none of it.
func TestTaskApprovals(t *testing.T) {
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
	requester, a1, a2, a3 := id(), id(), id(), id()
	orgID, otherOrg, proj, board, todo, done := id(), id(), id(), id(), id(), id()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := owner.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("fixture %q: %v", sql, err)
		}
	}
	for _, u := range []string{requester, a1, a2, a3} {
		exec(`INSERT INTO users (id, email, name) VALUES ($1, $2, 'approval test')`, u, "apr-"+u[:8]+"@example.test")
	}
	t.Cleanup(func() {
		for _, o := range []string{orgID, otherOrg} {
			_, _ = owner.Exec(context.Background(), `DELETE FROM organizations WHERE id = $1`, o)
		}
		for _, u := range []string{requester, a1, a2, a3} {
			_, _ = owner.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, u)
		}
	})
	exec(`INSERT INTO organizations (id, slug, name) VALUES ($1, $2, 'approval test')`, orgID, "ap-"+orgID[:8])
	exec(`INSERT INTO organizations (id, slug, name) VALUES ($1, $2, 'approval other')`, otherOrg, "ao-"+otherOrg[:8])
	exec(`INSERT INTO projects (id, org_id, key, name, created_by) VALUES ($1, $2, 'AP', 'Approvals', $3)`, proj, orgID, requester)
	exec(`INSERT INTO boards (id, org_id, project_id) VALUES ($1, $2, $3)`, board, orgID, proj)
	exec(`INSERT INTO board_columns (id, org_id, board_id, name, rank) VALUES ($1, $3, $4, 'To Do', 'a'), ($2, $3, $4, 'Done', 'b')`,
		todo, done, orgID, board)

	n := 0
	task := func(column string) string {
		t.Helper()
		n++
		tid := id()
		exec(`INSERT INTO tasks (id, org_id, project_id, column_id, number, title, rank, created_by)
		      VALUES ($1, $2, $3, $4, $5, 'task', $6, $7)`, tid, orgID, proj, column, n, "r"+tid[:8], requester)
		return tid
	}

	repo := NewApprovalRepo(NewTenantPool(app))
	now := time.Now()
	request := func(taskID string, approvers ...string) *project.Approval {
		t.Helper()
		a := &project.Approval{ID: id(), OrgID: orgID, TaskID: taskID, RequestedBy: requester, State: project.ApprovalPending}
		for _, u := range approvers {
			a.Approvers = append(a.Approvers, project.Approver{UserID: u})
		}
		if err := repo.Create(ctx, orgID, a); err != nil {
			t.Fatalf("create: %v", err)
		}
		return a
	}
	decide := func(a *project.Approval, user string, d project.ApprovalDecision) (*project.Approval, error) {
		return repo.Update(ctx, orgID, a.TaskID, a.ID, func(x *project.Approval) error {
			return x.Decide(user, d, "ok", now)
		})
	}

	// Self-approval is refused and nothing is written.
	tSelf := task(todo)
	self := request(tSelf, a1, requester)
	if _, err := decide(self, requester, project.DecisionApproved); !errors.Is(err, project.ErrSelfApproval) {
		t.Fatalf("self approval err = %v, want ErrSelfApproval", err)
	}
	got, err := repo.Get(ctx, orgID, tSelf, self.ID)
	if err != nil || got.State != project.ApprovalPending {
		t.Fatalf("after refused self approval: %+v %v", got, err)
	}

	// One open request per task.
	dup := &project.Approval{ID: id(), OrgID: orgID, TaskID: tSelf, RequestedBy: requester, State: project.ApprovalPending,
		Approvers: []project.Approver{{UserID: a2}}}
	if err := repo.Create(ctx, orgID, dup); !errors.Is(err, project.ErrApprovalOpen) {
		t.Fatalf("second open request err = %v, want ErrApprovalOpen", err)
	}

	// Waiting list: the approver sees it until answering.
	if w, _ := repo.ListWaiting(ctx, orgID, a1); len(w) != 1 || w[0].ApprovalID != self.ID {
		t.Fatalf("waiting for a1 = %+v", w)
	}

	// Concurrent decisions by three approvers settle to approved.
	tConc := task(todo)
	conc := request(tConc, a1, a2, a3)
	var wg sync.WaitGroup
	for _, u := range []string{a1, a2, a3} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := decide(conc, u, project.DecisionApproved); err != nil {
				t.Errorf("decide %s: %v", u, err)
			}
		}()
	}
	wg.Wait()
	got, _ = repo.Get(ctx, orgID, tConc, conc.ID)
	if got.State != project.ApprovalApproved {
		t.Fatalf("concurrent decisions ended %s, want approved", got.State)
	}

	// Stale: approved on a completed task, then reopened.
	tDone := task(done)
	ap := request(tDone, a1)
	if _, err := decide(ap, a1, project.DecisionApproved); err != nil {
		t.Fatal(err)
	}
	if got, _ = repo.Get(ctx, orgID, tDone, ap.ID); got.Stale() {
		t.Fatal("fresh approval must not be stale")
	}
	exec(`UPDATE tasks SET column_id = $2 WHERE id = $1`, tDone, todo)
	if got, _ = repo.Get(ctx, orgID, tDone, ap.ID); !got.Stale() {
		t.Fatal("reopening a completed task must mark its approval stale")
	}

	// Stale: editing a completed task counts; a rank change or an edit before
	// completion does not.
	tEdit := task(done)
	ap2 := request(tEdit, a1)
	if _, err := decide(ap2, a1, project.DecisionApproved); err != nil {
		t.Fatal(err)
	}
	exec(`UPDATE tasks SET rank = rank || 'x' WHERE id = $1`, tEdit)
	if got, _ = repo.Get(ctx, orgID, tEdit, ap2.ID); got.Stale() {
		t.Fatal("a rank change must not mark the approval stale")
	}
	exec(`UPDATE tasks SET title = 'changed' WHERE id = $1`, tEdit)
	if got, _ = repo.Get(ctx, orgID, tEdit, ap2.ID); !got.Stale() {
		t.Fatal("editing a completed task must mark its approval stale")
	}
	tOpen := task(todo)
	ap3 := request(tOpen, a1)
	if _, err := decide(ap3, a1, project.DecisionApproved); err != nil {
		t.Fatal(err)
	}
	exec(`UPDATE tasks SET title = 'changed' WHERE id = $1`, tOpen)
	if got, _ = repo.Get(ctx, orgID, tOpen, ap3.ID); got.Stale() {
		t.Fatal("editing a task that was not completed must not mark it stale")
	}

	// Tenant isolation: the other org can neither read nor change anything.
	if got, err := repo.Get(ctx, otherOrg, tConc, conc.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("cross-tenant get = %+v, %v; want ErrNotFound", got, err)
	}
	if as, err := repo.ListByTask(ctx, otherOrg, tConc); err != nil || len(as) != 0 {
		t.Fatalf("cross-tenant list = %v, %v", as, err)
	}
	if w, err := repo.ListWaiting(ctx, otherOrg, a1); err != nil || len(w) != 0 {
		t.Fatalf("cross-tenant waiting = %v, %v", w, err)
	}
	_, err = repo.Update(ctx, otherOrg, tSelf, self.ID, func(x *project.Approval) error {
		return x.Cancel(requester, true, now)
	})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("cross-tenant cancel err = %v, want ErrNotFound", err)
	}
	if got, _ = repo.Get(ctx, orgID, tSelf, self.ID); got.State != project.ApprovalPending {
		t.Fatalf("approval changed by another tenant: %s", got.State)
	}
}
