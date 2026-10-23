//go:build integration

package postgres

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mesutokul/fluxboard/backend/internal/domain/project"
)

// EpicProgress / ReleaseProgress on real Postgres: tasks and points per
// epic or release, the done column's share, deleted tasks left out, and no
// done column counting nothing as done.
func TestEpicAndReleaseProgress(t *testing.T) {
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
	user, orgID, proj, board, todo, done := id(), id(), id(), id(), id(), id()
	epic, emptyEpic, rel := id(), id(), id()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := owner.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("fixture %q: %v", sql, err)
		}
	}
	exec(`INSERT INTO users (id, email, name) VALUES ($1, $2, 'progress test')`, user, "prog-"+user[:8]+"@example.test")
	t.Cleanup(func() {
		_, _ = owner.Exec(context.Background(), `DELETE FROM organizations WHERE id = $1`, orgID)
		_, _ = owner.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, user)
	})
	exec(`INSERT INTO organizations (id, slug, name) VALUES ($1, $2, 'progress test')`, orgID, "pg-"+orgID[:8])
	exec(`INSERT INTO projects (id, org_id, key, name, created_by) VALUES ($1, $2, 'PG', 'Progress', $3)`, proj, orgID, user)
	exec(`INSERT INTO boards (id, org_id, project_id) VALUES ($1, $2, $3)`, board, orgID, proj)
	exec(`INSERT INTO board_columns (id, org_id, board_id, name, rank) VALUES ($1, $3, $4, 'To Do', 'a'), ($2, $3, $4, 'Done', 'b')`,
		todo, done, orgID, board)
	exec(`INSERT INTO epics (id, org_id, project_id, name, created_by) VALUES ($1, $3, $4, 'E', $5), ($2, $3, $4, 'Empty', $5)`,
		epic, emptyEpic, orgID, proj, user)
	exec(`INSERT INTO releases (id, org_id, project_id, name, created_by) VALUES ($1, $2, $3, 'v1', $4)`, rel, orgID, proj, user)

	n := 0
	task := func(column string, estimate *int, deleted bool) {
		t.Helper()
		n++
		tid := id()
		exec(`INSERT INTO tasks (id, org_id, project_id, column_id, number, title, rank, created_by, estimate, epic_id, release_id, deleted_at)
		      VALUES ($1, $2, $3, $4, $5, 'task', $6, $7, $8, $9, $10, CASE WHEN $11 THEN now() END)`,
			tid, orgID, proj, column, n, "r"+tid[:8], user, estimate, epic, rel, deleted)
	}
	five, three := 5, 3
	task(done, &five, false)
	task(todo, &three, false)
	task(todo, nil, false)
	task(done, &five, true) // deleted: not counted

	want := project.Progress{Total: 3, Done: 1, Points: 8, DonePoints: 5}
	tp := NewTenantPool(app)
	epics, err := NewEpicRepo(tp).Progress(ctx, orgID, proj, done)
	if err != nil {
		t.Fatal(err)
	}
	if epics[epic] != want {
		t.Fatalf("epic progress = %+v, want %+v", epics[epic], want)
	}
	if _, ok := epics[emptyEpic]; ok {
		t.Fatal("an epic without tasks must be absent")
	}
	rels, err := NewReleaseRepo(tp).Progress(ctx, orgID, proj, done)
	if err != nil {
		t.Fatal(err)
	}
	if rels[rel] != want {
		t.Fatalf("release progress = %+v, want %+v", rels[rel], want)
	}
	noDone, err := NewEpicRepo(tp).Progress(ctx, orgID, proj, "")
	if err != nil {
		t.Fatal(err)
	}
	if p := noDone[epic]; p.Total != 3 || p.Done != 0 || p.DonePoints != 0 {
		t.Fatalf("no done column: %+v, want nothing done", p)
	}
	// Another org sees none of it.
	other, err := NewEpicRepo(tp).Progress(ctx, id(), proj, done)
	if err != nil || len(other) != 0 {
		t.Fatalf("other org = %+v, %v; want empty", other, err)
	}
}
