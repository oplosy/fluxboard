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
	"github.com/mesutokul/fluxboard/backend/internal/domain/chat"
)

// Chat channels (0057/0058, ADR-038) on real Postgres: a delivery and its
// chat:send outbox row are written together, the task deep-link join resolves,
// attempts are recorded, and another tenant sees none of it.
func TestChatChannels(t *testing.T) {
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
	user, orgID, otherOrg, proj, board, col, task := id(), id(), id(), id(), id(), id(), id()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := owner.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("fixture %q: %v", sql, err)
		}
	}
	exec(`INSERT INTO users (id, email, name) VALUES ($1, $2, 'chat test')`, user, "chat-"+user[:8]+"@example.test")
	t.Cleanup(func() {
		for _, o := range []string{orgID, otherOrg} {
			_, _ = owner.Exec(context.Background(), `DELETE FROM organizations WHERE id = $1`, o)
		}
		_, _ = owner.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, user)
	})
	exec(`INSERT INTO organizations (id, slug, name) VALUES ($1, $2, 'chat test')`, orgID, "ct-"+orgID[:8])
	exec(`INSERT INTO organizations (id, slug, name) VALUES ($1, $2, 'chat other')`, otherOrg, "co-"+otherOrg[:8])
	exec(`INSERT INTO projects (id, org_id, key, name, created_by) VALUES ($1, $2, 'CH', 'Chat', $3)`, proj, orgID, user)
	exec(`INSERT INTO boards (id, org_id, project_id) VALUES ($1, $2, $3)`, board, orgID, proj)
	exec(`INSERT INTO board_columns (id, org_id, board_id, name, rank) VALUES ($1, $2, $3, 'To Do', 'a')`, col, orgID, board)
	exec(`INSERT INTO tasks (id, org_id, project_id, column_id, number, title, rank, created_by)
	      VALUES ($1, $2, $3, $4, 7, 'task', 'r1', $5)`, task, orgID, proj, col, user)

	repo := NewChatRepo(NewTenantPool(app))
	ch := &chat.Channel{
		ID: id(), OrgID: orgID, Provider: chat.ProviderSlack, Name: "dev", URLEnc: "ciphertext",
		URLHint: "hooks.slack.com/…abcd", Categories: []string{"mention"}, Enabled: true, CreatedBy: user,
	}
	if err := repo.Create(ctx, orgID, ch); err != nil {
		t.Fatalf("create: %v", err)
	}
	if got, err := repo.Get(ctx, orgID, ch.ID); err != nil || got.URLEnc != "ciphertext" || len(got.Categories) != 1 {
		t.Fatalf("get: %+v %v", got, err)
	}

	d := &chat.Delivery{ID: id(), OrgID: orgID, ChannelID: ch.ID, Category: "mention", Title: "t", Body: "b"}
	if err := repo.Enqueue(ctx, orgID, d); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	var queued int
	if err := owner.QueryRow(ctx, `SELECT count(*) FROM outbox WHERE org_id = $1 AND kind = 'chat:send' AND payload->>'delivery_id' = $2`, orgID, d.ID).Scan(&queued); err != nil || queued != 1 {
		t.Fatalf("outbox rows = %d (%v); want 1", queued, err)
	}

	if err := repo.RecordAttempt(ctx, orgID, d.ID, chat.StatusSent, ""); err != nil {
		t.Fatalf("record: %v", err)
	}
	list, err := repo.ListDeliveries(ctx, orgID, 10)
	if err != nil || len(list) != 1 || list[0].Status != chat.StatusSent || list[0].Attempts != 1 || list[0].SentAt == nil || list[0].ChannelName != "dev" {
		t.Fatalf("list: %+v %v", list, err)
	}

	slug, key, num, err := repo.TaskPath(ctx, orgID, task)
	if err != nil || slug != "ct-"+orgID[:8] || key != "CH" || num != 7 {
		t.Fatalf("task path: %s %s %d %v", slug, key, num, err)
	}

	// Another tenant sees and changes nothing.
	if _, err := repo.Get(ctx, otherOrg, ch.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Get across orgs: %v", err)
	}
	if _, err := repo.GetDelivery(ctx, otherOrg, d.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("GetDelivery across orgs: %v", err)
	}
	if _, _, _, err := repo.TaskPath(ctx, otherOrg, task); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("TaskPath across orgs: %v", err)
	}
	if err := repo.Delete(ctx, otherOrg, ch.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Delete across orgs: %v", err)
	}
	if l, _ := repo.List(ctx, otherOrg); len(l) != 0 {
		t.Errorf("List leaked %d channels", len(l))
	}

	// Deleting the channel removes its deliveries.
	if err := repo.Delete(ctx, orgID, ch.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if l, _ := repo.ListDeliveries(ctx, orgID, 10); len(l) != 0 {
		t.Errorf("deliveries survived channel delete: %d", len(l))
	}
}
