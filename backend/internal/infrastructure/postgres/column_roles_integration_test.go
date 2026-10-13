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

// Column roles (ADR-033 decision 5) on real Postgres: one column per role per
// board, moved atomically; clearing; other boards unaffected.
func TestColumnRoles(t *testing.T) {
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
	user, orgID := id(), id()
	p1, p2, b1, b2 := id(), id(), id(), id()
	qa, review, other := id(), id(), id()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := owner.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("fixture %q: %v", sql, err)
		}
	}
	exec(`INSERT INTO users (id, email, name) VALUES ($1, $2, 'roles test')`, user, "roles-"+user[:8]+"@example.test")
	t.Cleanup(func() {
		_, _ = owner.Exec(context.Background(), `DELETE FROM organizations WHERE id = $1`, orgID)
		_, _ = owner.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, user)
	})
	exec(`INSERT INTO organizations (id, slug, name) VALUES ($1, $2, 'roles test')`, orgID, "rt-"+orgID[:8])
	exec(`INSERT INTO projects (id, org_id, key, name, created_by) VALUES ($1, $3, 'RA', 'A', $4), ($2, $3, 'RB', 'B', $4)`, p1, p2, orgID, user)
	exec(`INSERT INTO boards (id, org_id, project_id) VALUES ($1, $3, $4), ($2, $3, $5)`, b1, b2, orgID, p1, p2)
	exec(`INSERT INTO board_columns (id, org_id, board_id, name, rank) VALUES
	      ($1, $4, $5, 'QA', 'a'), ($2, $4, $5, 'Review', 'b'), ($3, $4, $6, 'Review', 'a')`, qa, review, other, orgID, b1, b2)

	repo := NewColumnRepo(NewTenantPool(app))
	roles := func(board string) map[string]project.ColumnRole {
		t.Helper()
		cols, err := repo.ListByBoard(ctx, orgID, board)
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]project.ColumnRole{}
		for _, c := range cols {
			out[c.ID] = c.Role
		}
		return out
	}

	if err := repo.SetRole(ctx, orgID, b1, review, project.ColumnReview); err != nil {
		t.Fatal(err)
	}
	if err := repo.SetRole(ctx, orgID, b2, other, project.ColumnReview); err != nil {
		t.Fatalf("the same role on another board: %v", err)
	}
	// Moving review to QA frees it on the Review column, in one step.
	if err := repo.SetRole(ctx, orgID, b1, qa, project.ColumnReview); err != nil {
		t.Fatal(err)
	}
	if r := roles(b1); r[qa] != project.ColumnReview || r[review] != "" {
		t.Fatalf("board 1 roles = %v", r)
	}
	if r := roles(b2); r[other] != project.ColumnReview {
		t.Fatalf("board 2 must keep its role: %v", r)
	}
	if err := repo.SetRole(ctx, orgID, b1, qa, ""); err != nil {
		t.Fatal(err)
	}
	if r := roles(b1); r[qa] != "" {
		t.Fatalf("cleared role = %q", r[qa])
	}
	// A column of another board is not found through this board.
	if err := repo.SetRole(ctx, orgID, b1, other, project.ColumnDone); err == nil {
		t.Fatal("setting a role on another board's column must fail")
	}
	// The database refuses an unknown role outright.
	if _, err := owner.Exec(ctx, `UPDATE board_columns SET role = 'archive' WHERE id = $1`, qa); err == nil {
		t.Fatal("unknown role accepted by the database")
	}
}
