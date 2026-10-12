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

// A task carries several labels; search must still return it once and count it
// once (the label facet used to be a join, multiplying rows per label). The
// fixture is written with the owner role (TEST_DATABASE_OWNER_URL) under fresh
// ids and read back through TaskRepo on the RLS-subject app role.
func TestSearchReturnsEachTaskOnce(t *testing.T) {
	ownerDSN := os.Getenv("TEST_DATABASE_OWNER_URL")
	if ownerDSN == "" {
		t.Fatal("TEST_DATABASE_OWNER_URL not set; this test writes its fixture as the owner role (see make test-integration)")
	}
	app, _ := newIntegrationDB(t)
	ctx := context.Background()
	owner, err := pgxpool.New(ctx, ownerDSN)
	if err != nil {
		t.Fatalf("connect owner: %v", err)
	}
	t.Cleanup(owner.Close)

	id := func() string { return uuid.NewString() }
	userID, orgID, projectID, boardID, columnID := id(), id(), id(), id(), id()
	bug, ui := id(), id()
	multi, plain := id(), id()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := owner.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("fixture %q: %v", sql, err)
		}
	}
	exec(`INSERT INTO users (id, email, name) VALUES ($1, $2, 'search test')`, userID, "search-"+userID[:8]+"@example.test")
	t.Cleanup(func() {
		// Deleting the org cascades its tenant rows; the user goes after them.
		_, _ = owner.Exec(context.Background(), `DELETE FROM organizations WHERE id = $1`, orgID)
		_, _ = owner.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID)
	})
	exec(`INSERT INTO organizations (id, slug, name) VALUES ($1, $2, 'search test')`, orgID, "st-"+orgID[:8])
	exec(`INSERT INTO projects (id, org_id, key, name, visibility, created_by) VALUES ($1, $2, 'SRCH', 'Search', 'org', $3)`, projectID, orgID, userID)
	exec(`INSERT INTO boards (id, org_id, project_id) VALUES ($1, $2, $3)`, boardID, orgID, projectID)
	exec(`INSERT INTO board_columns (id, org_id, board_id, name, rank) VALUES ($1, $2, $3, 'Todo', 'a')`, columnID, orgID, boardID)
	exec(`INSERT INTO labels (id, org_id, name) VALUES ($1, $3, 'bug'), ($2, $3, 'ui')`, bug, ui, orgID)
	exec(`INSERT INTO tasks (id, org_id, project_id, column_id, number, title, rank, created_by, assignee_id)
	      VALUES ($1, $3, $4, $5, 1, 'searchable widget with two labels', 'a', $6, $6),
	             ($2, $3, $4, $5, 2, 'searchable widget without labels', 'b', $6, $6)`,
		multi, plain, orgID, projectID, columnID, userID)
	exec(`INSERT INTO task_labels (task_id, label_id, org_id) VALUES ($1, $2, $4), ($1, $3, $4)`, multi, bug, ui, orgID)

	repo := NewTaskRepo(NewTenantPool(app))
	all := project.Scope{All: true, OrgVisible: true}
	cases := []struct {
		name string
		f    project.SearchFilter
		want []string
	}{
		{"text query", project.SearchFilter{Query: "searchable"}, []string{multi, plain}},
		{"facet only", project.SearchFilter{AssigneeID: userID}, []string{multi, plain}},
		{"label facet", project.SearchFilter{LabelID: bug}, []string{multi}},
	}
	for _, c := range cases {
		c.f.UserID, c.f.Scope, c.f.Limit = userID, all, 50
		res, err := repo.Search(ctx, orgID, c.f)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		got := map[string]int{}
		for _, task := range res.Tasks {
			got[task.ID]++
		}
		if len(res.Tasks) != len(c.want) || res.Total != len(c.want) {
			t.Errorf("%s: %d hits, total %d, want %d of each (hits per task: %v)", c.name, len(res.Tasks), res.Total, len(c.want), got)
		}
		for _, w := range c.want {
			if got[w] != 1 {
				t.Errorf("%s: task %s returned %d times, want once", c.name, w, got[w])
			}
		}
	}
}
