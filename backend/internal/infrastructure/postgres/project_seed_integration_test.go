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

// Project templates (FR-PROJ-007, ADR-035) on real Postgres under the app role:
// the seed lands whole (columns+roles, labels, tasks), an existing org label is
// reused, and a failure part-way rolls back every row.
func TestProjectSeed(t *testing.T) {
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

	user, orgID := uuid.NewString(), uuid.NewString()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := owner.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("fixture %q: %v", sql, err)
		}
	}
	exec(`INSERT INTO users (id, email, name) VALUES ($1, $2, 'seed test')`, user, "seed-"+user[:8]+"@example.test")
	t.Cleanup(func() {
		_, _ = owner.Exec(context.Background(), `DELETE FROM organizations WHERE id = $1`, orgID)
		_, _ = owner.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, user)
	})
	exec(`INSERT INTO organizations (id, slug, name) VALUES ($1, $2, 'seed test')`, orgID, "st-"+orgID[:8])
	existing := uuid.NewString()
	exec(`INSERT INTO labels (id, org_id, name, color) VALUES ($1, $2, 'bug', '#000000')`, existing, orgID)

	tp := NewTenantPool(app)
	repo := NewProjectSeedRepo(tp)
	newSeed := func(key string) *project.ProjectSeed {
		id := uuid.NewString
		s := &project.ProjectSeed{
			Project: project.Project{ID: id(), Key: key, Name: "Support", Visibility: project.VisibilityOrg},
			LeadID:  user,
		}
		s.Board = project.Board{ID: id(), ProjectID: s.Project.ID, Name: "Board"}
		s.Columns = []project.Column{
			{ID: id(), Name: "New", Rank: "a", Role: project.ColumnBacklog},
			{ID: id(), Name: "Waiting", Rank: "b"},
			{ID: id(), Name: "Resolved", Rank: "c", Role: project.ColumnDone},
		}
		s.Labels = []project.Label{{ID: id(), Name: "bug", Color: "#ef4444"}, {ID: id(), Name: "urgent", Color: "#dc2626"}}
		t1, t2 := id(), id()
		s.Tasks = []project.Task{
			{ID: t1, ColumnID: s.Columns[0].ID, Title: "one", Priority: project.PriorityHigh, Rank: "a"},
			{ID: t2, ColumnID: s.Columns[2].ID, Title: "two", Priority: project.PriorityLow, Rank: "a"},
		}
		s.TaskLabels = map[string][]string{t1: {"bug", "urgent"}}
		return s
	}

	s := newSeed("SUP")
	if err := repo.Seed(ctx, orgID, s); err != nil {
		t.Fatal(err)
	}
	count := func(sql string, args ...any) int {
		t.Helper()
		var n int
		if err := owner.QueryRow(ctx, sql, args...).Scan(&n); err != nil {
			t.Fatalf("%q: %v", sql, err)
		}
		return n
	}
	if n := count(`SELECT count(*) FROM board_columns WHERE board_id = $1`, s.Board.ID); n != 3 {
		t.Fatalf("columns = %d", n)
	}
	cols, err := NewColumnRepo(tp).ListByBoard(ctx, orgID, s.Board.ID)
	if err != nil || cols[0].Role != project.ColumnBacklog || cols[1].Role != "" || cols[2].Role != project.ColumnDone {
		t.Fatalf("columns = %+v, %v", cols, err)
	}
	if n := count(`SELECT count(*) FROM tasks WHERE project_id = $1`, s.Project.ID); n != 2 {
		t.Fatalf("tasks = %d", n)
	}
	if s.Tasks[0].Number != 1 || s.Tasks[1].Number != 2 {
		t.Fatalf("numbers = %d, %d", s.Tasks[0].Number, s.Tasks[1].Number)
	}
	if n := count(`SELECT count(*) FROM project_members WHERE project_id = $1 AND user_id = $2 AND role = 'LEAD'`, s.Project.ID, user); n != 1 {
		t.Fatal("creator is not LEAD")
	}
	// The org's existing "bug" label is reused (color untouched); "urgent" is new.
	if n := count(`SELECT count(*) FROM labels WHERE org_id = $1`, orgID); n != 2 {
		t.Fatalf("labels = %d", n)
	}
	if n := count(`SELECT count(*) FROM task_labels WHERE task_id = $1 AND label_id = $2`, s.Tasks[0].ID, existing); n != 1 {
		t.Fatal("existing label not attached")
	}
	if n := count(`SELECT count(*) FROM labels WHERE id = $1 AND color = '#000000'`, existing); n != 1 {
		t.Fatal("existing label was modified")
	}

	// Same key again: conflict, nothing new written.
	if err := repo.Seed(ctx, orgID, newSeed("SUP")); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("duplicate key err = %v", err)
	}

	// A failure after the project, board, columns and a task are written rolls
	// all of them back.
	bad := newSeed("BAD")
	bad.TaskLabels[bad.Tasks[0].ID] = []string{"missing"}
	if err := repo.Seed(ctx, orgID, bad); err == nil {
		t.Fatal("seed with an unknown task label must fail")
	}
	if n := count(`SELECT count(*) FROM projects WHERE org_id = $1`, orgID); n != 1 {
		t.Fatalf("projects after failed seed = %d, want 1", n)
	}
	if n := count(`SELECT count(*) FROM boards WHERE id = $1`, bad.Board.ID) +
		count(`SELECT count(*) FROM tasks WHERE id = $1`, bad.Tasks[0].ID) +
		count(`SELECT count(*) FROM labels WHERE org_id = $1`, orgID); n != 2 {
		t.Fatalf("leftover rows after failed seed (want only the 2 labels): %d", n)
	}
}
