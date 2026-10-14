//go:build integration

package taskuc

import (
	"context"
	"os"
	"sort"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mesutokul/fluxboard/backend/internal/domain/tenant"
	"github.com/mesutokul/fluxboard/backend/internal/infrastructure/postgres"
)

// Idempotent CSV import (FR-TASK-012) against real Postgres with RLS: the same
// Idempotency-Key never duplicates rows — on a plain retry, after a partial
// write, or under two concurrent attempts — and the import goes through the
// normal create path (per-project numbering, labels). Fixture rows are written
// as the owner; the service runs on the RLS-subject app role.
func TestImportIdempotent(t *testing.T) {
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
	email := "import-" + userID[:8] + "@example.test"
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := owner.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("fixture %q: %v", sql, err)
		}
	}
	exec(`INSERT INTO users (id, email, name) VALUES ($1, $2, 'import test')`, userID, email)
	t.Cleanup(func() {
		_, _ = owner.Exec(context.Background(), `DELETE FROM organizations WHERE id = $1`, orgID)
		_, _ = owner.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID)
	})
	exec(`INSERT INTO organizations (id, slug, name) VALUES ($1, $2, 'import test')`, orgID, "it-"+orgID[:8])
	exec(`INSERT INTO memberships (org_id, user_id, role) VALUES ($1, $2, 'MEMBER')`, orgID, userID)
	exec(`INSERT INTO projects (id, org_id, key, name, visibility, created_by) VALUES ($1, $2, 'IMP', 'Import', 'org', $3)`, projectID, orgID, userID)
	exec(`INSERT INTO project_members (org_id, project_id, user_id, role) VALUES ($1, $2, $3, 'CONTRIBUTOR')`, orgID, projectID, userID)
	exec(`INSERT INTO boards (id, org_id, project_id) VALUES ($1, $2, $3)`, boardID, orgID, projectID)
	exec(`INSERT INTO board_columns (id, org_id, board_id, name, rank) VALUES ($1, $3, $4, 'To Do', 'a'), ($2, $3, $4, 'Done', 'b')`,
		todo, done, orgID, boardID)

	tp := postgres.NewTenantPool(app)
	svc := New(Deps{
		Tasks: postgres.NewTaskRepo(tp), Labels: postgres.NewLabelRepo(tp), Projects: postgres.NewProjectRepo(tp),
		Members: postgres.NewProjectMemberRepo(tp), Boards: postgres.NewBoardRepo(tp), Columns: postgres.NewColumnRepo(tp),
		People: postgres.NewMembershipRepo(tp),
	})
	csvText := "Title,Status,Assignee,Labels\nFirst,,\nSecond,Done," + email + ",Imported\nThird,to do,,\"Imported, Extra\"\n"
	run := func(key, csv string) *ImportResult {
		t.Helper()
		res, err := svc.ImportTasks(ctx, orgID, userID, tenant.RoleMember, ImportInput{ProjectID: projectID, CSV: csv, IdemKey: key})
		if err != nil {
			t.Fatalf("import %s: %v", key, err)
		}
		return res
	}
	count := func() int {
		t.Helper()
		var n int
		if err := owner.QueryRow(ctx, `SELECT count(*) FROM tasks WHERE project_id = $1`, projectID).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	first := run("key-1", csvText)
	if first.Created != 3 || first.Existing != 0 || count() != 3 {
		t.Fatalf("first import: %+v, %d tasks", first, count())
	}
	numbers := []int{first.Tasks[0].Number, first.Tasks[1].Number, first.Tasks[2].Number}
	if !sort.IntsAreSorted(numbers) || numbers[0] != 1 || numbers[2] != 3 {
		t.Fatalf("numbers %v, want 1..3 from the project counter", numbers)
	}
	var assignee, column string
	if err := owner.QueryRow(ctx, `SELECT assignee_id::text, column_id::text FROM tasks WHERE id = $1`, first.Tasks[1].ID).Scan(&assignee, &column); err != nil {
		t.Fatal(err)
	}
	if assignee != userID || column != done {
		t.Fatalf("row 2: assignee %s column %s", assignee, column)
	}
	var links int
	if err := owner.QueryRow(ctx, `SELECT count(*) FROM task_labels WHERE org_id = $1`, orgID).Scan(&links); err != nil || links != 3 {
		t.Fatalf("label links = %d (%v), want 3", links, err)
	}

	t.Run("retry with the same key replays", func(t *testing.T) {
		again := run("key-1", csvText)
		if again.Created != 0 || again.Existing != 3 || count() != 3 {
			t.Fatalf("retry: %+v, %d tasks", again, count())
		}
		for i := range again.Tasks {
			if again.Tasks[i] != first.Tasks[i] {
				t.Fatalf("retry task %d = %+v, want %+v", i, again.Tasks[i], first.Tasks[i])
			}
		}
	})

	t.Run("retry after a partial write creates only the missing rows", func(t *testing.T) {
		res := run("key-2", csvText)
		// Simulate a crash after row 1: the later rows never landed.
		exec(`DELETE FROM tasks WHERE id = ANY($1)`, []string{res.Tasks[1].ID, res.Tasks[2].ID})
		resumed := run("key-2", csvText)
		if resumed.Created != 2 || resumed.Existing != 1 || count() != 6 {
			t.Fatalf("resume: %+v, %d tasks", resumed, count())
		}
	})

	t.Run("concurrent attempts with one key write each row once", func(t *testing.T) {
		var wg sync.WaitGroup
		errs := make(chan error, 2)
		for range 2 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := svc.ImportTasks(ctx, orgID, userID, tenant.RoleMember, ImportInput{ProjectID: projectID, CSV: csvText, IdemKey: "key-3"})
				errs <- err
			}()
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatal(err)
			}
		}
		if count() != 9 {
			t.Fatalf("%d tasks, want 9", count())
		}
	})

	t.Run("a new key is a new import", func(t *testing.T) {
		if res := run("key-4", csvText); res.Created != 3 || count() != 12 {
			t.Fatalf("new key: %+v, %d tasks", res, count())
		}
	})
}
