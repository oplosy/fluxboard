//go:build integration

package postgres

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/project"
)

// Wiki pages (0057/0058) on real Postgres: the version check refuses a stale
// save, the mention index is replaced with the body, soft-deleted tasks drop
// out of a page's task list, and another org sees nothing (RLS).
func TestWikiRepo(t *testing.T) {
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
	user, orgID, otherOrg, proj, board, col := id(), id(), id(), id(), id(), id()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := owner.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("fixture %q: %v", sql, err)
		}
	}
	exec(`INSERT INTO users (id, email, name) VALUES ($1, $2, 'wiki test')`, user, "wiki-"+user[:8]+"@example.test")
	t.Cleanup(func() {
		_, _ = owner.Exec(context.Background(), `DELETE FROM organizations WHERE id = ANY($1)`, []string{orgID, otherOrg})
		_, _ = owner.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, user)
	})
	exec(`INSERT INTO organizations (id, slug, name) VALUES ($1, $2, 'wiki test'), ($3, $4, 'other')`, orgID, "wk-"+orgID[:8], otherOrg, "wk-"+otherOrg[:8])
	exec(`INSERT INTO projects (id, org_id, key, name, created_by) VALUES ($1, $2, 'WK', 'Wiki', $3)`, proj, orgID, user)
	exec(`INSERT INTO boards (id, org_id, project_id) VALUES ($1, $2, $3)`, board, orgID, proj)
	exec(`INSERT INTO board_columns (id, org_id, board_id, name, rank) VALUES ($1, $2, $3, 'To Do', 'a')`, col, orgID, board)
	for n := 1; n <= 2; n++ {
		exec(`INSERT INTO tasks (id, org_id, project_id, column_id, number, title, rank, created_by)
		      VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`, id(), orgID, proj, col, n, "task "+string(rune('0'+n)), "r"+string(rune('0'+n)), user)
	}

	repo := NewWikiRepo(NewTenantPool(app))
	page := &project.WikiPage{ID: id(), ProjectID: proj, Title: "Plan", Body: "WK-1 WK-2 WK-9", CreatedBy: user, UpdatedBy: user}
	if err := repo.Create(ctx, orgID, page, []int{1, 2, 9}); err != nil {
		t.Fatalf("create: %v", err)
	}
	refs, err := repo.TaskRefs(ctx, orgID, page.ID)
	if err != nil || len(refs) != 2 || refs[0].Number != 1 || refs[1].Title != "task 2" {
		t.Fatalf("refs (WK-9 does not exist) = %+v, %v", refs, err)
	}
	got, err := repo.MentioningTask(ctx, orgID, proj, 2)
	if err != nil || len(got) != 1 || got[0].ID != page.ID || got[0].Body != "" {
		t.Fatalf("mentioning WK-2 = %+v, %v", got, err)
	}

	// A save of the version just read wins and replaces the index.
	page.Body, page.Title = "only WK-2", "Plan v2"
	if err := repo.Update(ctx, orgID, page, 1, []int{2}); err != nil {
		t.Fatalf("update: %v", err)
	}
	if got, _ := repo.MentioningTask(ctx, orgID, proj, 1); len(got) != 0 {
		t.Errorf("WK-1 still indexed after the body dropped it: %+v", got)
	}
	// ... and the same version again is stale.
	if err := repo.Update(ctx, orgID, page, 1, []int{2}); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("stale update: %v, want ErrConflict", err)
	}
	cur, err := repo.Get(ctx, orgID, page.ID)
	if err != nil || cur.Version != 2 || cur.Title != "Plan v2" {
		t.Fatalf("get = %+v, %v", cur, err)
	}
	ghost := *page
	ghost.ID = id()
	if err := repo.Update(ctx, orgID, &ghost, 1, nil); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("update missing: %v, want ErrNotFound", err)
	}

	// A trashed task stops being listed.
	exec(`UPDATE tasks SET deleted_at = now() WHERE project_id = $1 AND number = 2`, proj)
	if refs, _ := repo.TaskRefs(ctx, orgID, page.ID); len(refs) != 0 {
		t.Errorf("trashed task still listed: %+v", refs)
	}

	list, err := repo.ListByProject(ctx, orgID, proj, "PLAN")
	if err != nil || len(list) != 1 {
		t.Fatalf("title search = %+v, %v", list, err)
	}
	if list, _ = repo.ListByProject(ctx, orgID, proj, "nope"); len(list) != 0 {
		t.Errorf("non-matching search = %+v", list)
	}

	// Another org's tenant context sees and changes nothing.
	if _, err := repo.Get(ctx, otherOrg, page.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("cross-org get: %v, want ErrNotFound", err)
	}
	if list, _ := repo.ListByProject(ctx, otherOrg, proj, ""); len(list) != 0 {
		t.Errorf("cross-org list = %+v", list)
	}
	if err := repo.Delete(ctx, otherOrg, page.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("cross-org delete: %v, want ErrNotFound", err)
	}

	if err := repo.Delete(ctx, orgID, page.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	var left int
	if err := owner.QueryRow(ctx, `SELECT count(*) FROM wiki_page_tasks WHERE page_id = $1`, page.ID).Scan(&left); err != nil || left != 0 {
		t.Errorf("mention rows left after delete = %d, %v", left, err)
	}
}
