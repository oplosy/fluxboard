//go:build integration

package postgres

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/analytics"
	"github.com/mesutokul/fluxboard/backend/internal/domain/project"
)

// Org dashboard (FR-AN-005, ADR-038) on real Postgres under the app role: the
// aggregates count open / done / overdue per project and per assignee, never
// reach another org's project, and a layout belongs to one user.
func TestDashboard(t *testing.T) {
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

	user, other, orgA, orgB := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := owner.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("fixture %q: %v", sql, err)
		}
	}
	for _, u := range []string{user, other} {
		exec(`INSERT INTO users (id, email, name) VALUES ($1, $2, 'dash test')`, u, "dash-"+u[:8]+"@example.test")
	}
	t.Cleanup(func() {
		for _, o := range []string{orgA, orgB} {
			_, _ = owner.Exec(context.Background(), `DELETE FROM organizations WHERE id = $1`, o)
		}
		for _, u := range []string{user, other} {
			_, _ = owner.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, u)
		}
	})
	tp := NewTenantPool(app)
	seeder := NewProjectSeedRepo(tp)
	seed := func(org, key string) *project.ProjectSeed {
		exec(`INSERT INTO organizations (id, slug, name) VALUES ($1, $2, 'dash test')`, org, "d-"+org[:8])
		id := uuid.NewString
		s := &project.ProjectSeed{Project: project.Project{ID: id(), Key: key, Name: key, Visibility: project.VisibilityOrg}, LeadID: user}
		s.Board = project.Board{ID: id(), ProjectID: s.Project.ID, Name: "Board"}
		s.Columns = []project.Column{{ID: id(), Name: "Todo", Rank: "a", Role: project.ColumnBacklog}, {ID: id(), Name: "Done", Rank: "b", Role: project.ColumnDone}}
		s.Tasks = []project.Task{
			{ID: id(), ColumnID: s.Columns[0].ID, Title: "open late", Priority: project.PriorityLow, Rank: "a"},
			{ID: id(), ColumnID: s.Columns[0].ID, Title: "open", Priority: project.PriorityLow, Rank: "b"},
			{ID: id(), ColumnID: s.Columns[1].ID, Title: "done", Priority: project.PriorityLow, Rank: "a"},
		}
		if err := seeder.Seed(ctx, org, s); err != nil {
			t.Fatal(err)
		}
		return s
	}
	a, b := seed(orgA, "DSA"), seed(orgB, "DSB")
	exec(`UPDATE tasks SET due_date = now() - interval '2 days', assignee_id = $2 WHERE id = $1`, a.Tasks[0].ID, user)

	repo := NewDashboardRepo(tp)
	now := time.Now().UTC()
	// orgB's project id is passed on purpose: org filter + RLS must drop it.
	counts, assignees, _, err := repo.Counts(ctx, orgA, []string{a.Project.ID, b.Project.ID}, now, now.AddDate(0, 0, -56))
	if err != nil {
		t.Fatal(err)
	}
	if len(counts) != 1 || counts[0].ProjectID != a.Project.ID || counts[0].Open != 2 || counts[0].Done != 1 || counts[0].Overdue != 1 {
		t.Fatalf("counts = %+v, want only org A's project: open 2, done 1, overdue 1", counts)
	}
	byUser := map[string]analytics.AssigneeCount{}
	for _, x := range assignees {
		byUser[x.UserID] = x
	}
	if byUser[user].Open != 1 || byUser[user].Overdue != 1 || byUser[""].Open != 1 || len(assignees) != 2 {
		t.Fatalf("assignees = %+v", assignees)
	}

	if _, err := repo.GetLayout(ctx, orgA, user); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("no layout yet: err = %v, want ErrNotFound", err)
	}
	want := []analytics.Widget{{ID: "w1", Type: analytics.WidgetOverdue, ProjectIDs: []string{a.Project.ID}}}
	if err := repo.SaveLayout(ctx, orgA, user, want); err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetLayout(ctx, orgA, user)
	if err != nil || len(got) != 1 || got[0].ID != "w1" || got[0].ProjectIDs[0] != a.Project.ID {
		t.Fatalf("layout = %+v, %v", got, err)
	}
	if _, err := repo.GetLayout(ctx, orgA, other); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("another user's layout: err = %v, want ErrNotFound", err)
	}
	if _, err := repo.GetLayout(ctx, orgB, user); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("same user, other org: err = %v, want ErrNotFound", err)
	}
	if err := repo.SaveLayout(ctx, orgA, "apikey:123", want); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("api key principal: err = %v, want ErrForbidden", err)
	}
}
