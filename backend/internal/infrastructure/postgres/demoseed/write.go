package demoseed

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// Write loads the fixture in one transaction on the owner pool (cross-tenant
// writes; the owner is not subject to the non-FORCE RLS, ADR-014). Re-running
// resets the seed orgs to the fixture: they are deleted (every tenant row they
// own cascades) and rebuilt. Users are upserted by fixed ID instead, so a seed
// user's activity elsewhere never blocks a re-run. Rows that merely share a
// seed email or slug but have a different ID are refused, never deleted.
func Write(ctx context.Context, db pgx.Tx, f Fixture, passwordHash, totpSecret string) error {
	if err := checkCollisions(ctx, db, f); err != nil {
		return err
	}

	orgIDs := make([]string, len(f.Orgs))
	for i, o := range f.Orgs {
		orgIDs[i] = o.ID
	}
	if _, err := db.Exec(ctx, `DELETE FROM organizations WHERE id = ANY($1::uuid[])`, orgIDs); err != nil {
		return fmt.Errorf("reset seed orgs: %w", err)
	}

	b := &batch{}
	for _, u := range f.Users {
		role, secret, enabled := "user", "", false
		if u.PlatformAdmin {
			role = "admin"
			secret, enabled = totpSecret, totpSecret != ""
		}
		b.add("user "+u.Email, `
INSERT INTO users (id, email, password_hash, name, email_verified, platform_role, totp_secret, totp_enabled)
VALUES ($1, $2, $3, $4, true, $5, nullif($6, ''), $7)
ON CONFLICT (id) DO UPDATE SET
  email = EXCLUDED.email, password_hash = EXCLUDED.password_hash, name = EXCLUDED.name,
  email_verified = true, platform_role = EXCLUDED.platform_role,
  totp_secret = EXCLUDED.totp_secret, totp_enabled = EXCLUDED.totp_enabled,
  locked_at = NULL, updated_at = now()`,
			u.ID, u.Email, passwordHash, u.Name, role, secret, enabled)
	}
	for _, o := range f.Orgs {
		b.add("org "+o.Slug, `
INSERT INTO organizations (id, slug, name, stripe_customer_id, created_at, updated_at)
VALUES ($1, $2, $3, nullif($4, ''), $5, $5)`, o.ID, o.Slug, o.Name, o.StripeCustomerID, o.CreatedAt)
	}
	for _, m := range f.Memberships {
		b.add("membership", `INSERT INTO memberships (org_id, user_id, role) VALUES ($1, $2, $3)`, m.OrgID, m.UserID, m.Role)
	}
	for _, p := range f.Projects {
		b.add("project "+p.Key, `
INSERT INTO projects (id, org_id, key, name, visibility, archived_at, task_counter, created_by, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9)`,
			p.ID, p.OrgID, p.Key, p.Name, p.Visibility, p.ArchivedAt, p.TaskCounter, p.CreatedBy, p.CreatedAt)
		b.add("board "+p.Key, `
INSERT INTO boards (id, org_id, project_id, created_at, updated_at) VALUES ($1, $2, $3, $4, $4)`,
			p.BoardID, p.OrgID, p.ID, p.CreatedAt)
	}
	for _, m := range f.ProjectMembers {
		b.add("project member", `
INSERT INTO project_members (project_id, org_id, user_id, role) VALUES ($1, $2, $3, $4)`, m.ProjectID, m.OrgID, m.UserID, m.Role)
	}
	for _, c := range f.Columns {
		b.add("column "+c.Name, `
INSERT INTO board_columns (id, org_id, board_id, name, rank) VALUES ($1, $2, $3, $4, $5)`, c.ID, c.OrgID, c.BoardID, c.Name, c.Rank)
	}
	for _, l := range f.Labels {
		b.add("label "+l.Name, `INSERT INTO labels (id, org_id, name, color) VALUES ($1, $2, $3, $4)`, l.ID, l.OrgID, l.Name, l.Color)
	}
	for _, t := range f.Tasks {
		b.add(fmt.Sprintf("task %d", t.Number), `
INSERT INTO tasks (id, org_id, project_id, column_id, number, title, description, assignee_id, priority,
                   start_date, due_date, rank, created_by, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)`,
			t.ID, t.OrgID, t.ProjectID, t.ColumnID, t.Number, t.Title, t.Description, t.AssigneeID, t.Priority,
			t.StartDate, t.DueDate, t.Rank, t.CreatedBy, t.CreatedAt, t.UpdatedAt)
	}
	for _, tl := range f.TaskLabels {
		b.add("task label", `INSERT INTO task_labels (task_id, label_id, org_id) VALUES ($1, $2, $3)`, tl.TaskID, tl.LabelID, tl.OrgID)
	}
	for _, s := range f.Subscriptions {
		b.add("subscription "+s.PlanCode, `
INSERT INTO subscriptions (id, org_id, plan_code, stripe_subscription_id, stripe_customer_id, status, current_period_end)
VALUES ($1, $2, $3, $4, $5, $6, $7)`, s.ID, s.OrgID, s.PlanCode, s.StripeSubID, s.StripeCustomerID, s.Status, s.CurrentPeriodEnd)
	}
	for _, u := range f.Usage {
		b.add("usage "+string(u.Metric), `
INSERT INTO usage_records (org_id, metric, period_date, value, pushed_at) VALUES ($1, $2, $3, $4, $5)`,
			u.OrgID, string(u.Metric), u.Day, u.Value, u.PushedAt)
	}
	return b.send(ctx, db)
}

// checkCollisions refuses to touch data that only looks like the seed: a user
// with a seed email or an org with a seed slug under a different ID belongs to
// someone else (e.g. a manual sign-up as owner@demo.dev).
func checkCollisions(ctx context.Context, db pgx.Tx, f Fixture) error {
	var problems []string
	for _, u := range f.Users {
		var id string
		err := db.QueryRow(ctx, `SELECT id::text FROM users WHERE email = $1 AND id <> $2`, u.Email, u.ID).Scan(&id)
		if err == nil {
			problems = append(problems, fmt.Sprintf("user %s exists with id %s", u.Email, id))
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("check user %s: %w", u.Email, err)
		}
	}
	for _, o := range f.Orgs {
		var id string
		err := db.QueryRow(ctx, `SELECT id::text FROM organizations WHERE slug = $1 AND id <> $2`, o.Slug, o.ID).Scan(&id)
		if err == nil {
			problems = append(problems, fmt.Sprintf("org slug %s exists with id %s", o.Slug, id))
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("check org %s: %w", o.Slug, err)
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("seed: refusing to overwrite rows that are not seed-owned (%s); remove them or reset the database",
			strings.Join(problems, "; "))
	}
	return nil
}

// batch queues statements and reports the first failure with its label.
type batch struct {
	b      pgx.Batch
	labels []string
}

func (b *batch) add(label, sql string, args ...any) {
	b.b.Queue(sql, args...)
	b.labels = append(b.labels, label)
}

func (b *batch) send(ctx context.Context, db pgx.Tx) error {
	br := db.SendBatch(ctx, &b.b)
	for _, label := range b.labels {
		if _, err := br.Exec(); err != nil {
			_ = br.Close()
			return fmt.Errorf("seed %s: %w", label, err)
		}
	}
	return br.Close()
}
