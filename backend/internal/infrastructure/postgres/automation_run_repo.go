package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/mesutokul/fluxboard/backend/internal/domain/automation"
	"github.com/mesutokul/fluxboard/backend/internal/infrastructure/postgres/gen"
)

// AutomationRunRepo is the Postgres-backed automation.RunRepository ([T], RLS).
type AutomationRunRepo struct{ tp *TenantPool }

// NewAutomationRunRepo builds an AutomationRunRepo over the tenant pool.
func NewAutomationRunRepo(tp *TenantPool) *AutomationRunRepo { return &AutomationRunRepo{tp: tp} }

var _ automation.RunRepository = (*AutomationRunRepo)(nil)

func (r *AutomationRunRepo) Insert(ctx context.Context, orgID string, run *automation.Run) (bool, error) {
	id, err := parseUUID(run.ID)
	if err != nil {
		return false, fmt.Errorf("automation run insert: id: %w", err)
	}
	rid, err := parseUUID(run.RuleID)
	if err != nil {
		return false, fmt.Errorf("automation run insert: rule: %w", err)
	}
	tid, err := parseUUID(run.TaskID)
	if err != nil {
		return false, fmt.Errorf("automation run insert: task: %w", err)
	}
	var actor pgtype.UUID
	if run.ActorID != "" {
		a, err := parseUUID(run.ActorID)
		if err != nil {
			return false, fmt.Errorf("automation run insert: actor: %w", err)
		}
		actor = pgtype.UUID{Bytes: a, Valid: true}
	}
	var errMsg *string
	if run.Error != "" {
		errMsg = &run.Error
	}
	var n int64
	err = r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		oid, _ := parseUUID(orgID)
		var qerr error
		n, qerr = q.InsertAutomationRun(ctx, gen.InsertAutomationRunParams{
			ID: id, OrgID: oid, RuleID: rid, Trigger: run.Trigger, TaskID: tid,
			ActorID: actor, Status: run.Status, Error: errMsg, Depth: int32(run.Depth),
		})
		return qerr
	})
	return n > 0, err
}

func (r *AutomationRunRepo) Finish(ctx context.Context, orgID, id, status, errMsg string) error {
	rid, err := parseUUID(id)
	if err != nil {
		return fmt.Errorf("automation run finish: %w", err)
	}
	var e *string
	if errMsg != "" {
		e = &errMsg
	}
	return r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		oid, _ := parseUUID(orgID)
		return q.FinishAutomationRun(ctx, gen.FinishAutomationRunParams{OrgID: oid, ID: rid, Status: status, Error: e})
	})
}

func (r *AutomationRunRepo) ListByRule(ctx context.Context, orgID, ruleID string, limit int) ([]automation.Run, error) {
	rid, err := parseUUID(ruleID)
	if err != nil {
		return nil, fmt.Errorf("automation runs list: %w", err)
	}
	var out []automation.Run
	err = r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		oid, _ := parseUUID(orgID)
		rows, err := q.ListAutomationRuns(ctx, gen.ListAutomationRunsParams{OrgID: oid, RuleID: rid, MaxRows: int32(limit)})
		if err != nil {
			return err
		}
		for _, row := range rows {
			run := automation.Run{
				ID: row.ID.String(), OrgID: row.OrgID.String(), RuleID: row.RuleID.String(),
				Trigger: row.Trigger, TaskID: row.TaskID.String(), TaskTitle: row.TaskTitle,
				TaskNumber: int(row.TaskNumber), ProjectKey: row.ProjectKey,
				Status: row.Status, Depth: int(row.Depth), CreatedAt: row.CreatedAt,
			}
			if row.ActorID.Valid {
				run.ActorID = uuid.UUID(row.ActorID.Bytes).String()
			}
			if row.Error != nil {
				run.Error = *row.Error
			}
			out = append(out, run)
		}
		return nil
	})
	return out, err
}

func (r *AutomationRunRepo) CountSince(ctx context.Context, orgID string, since time.Time) (int, error) {
	var n int64
	err := r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		oid, _ := parseUUID(orgID)
		var qerr error
		n, qerr = q.CountAutomationActionsSince(ctx, gen.CountAutomationActionsSinceParams{OrgID: oid, Since: since})
		return qerr
	})
	return int(n), err
}

func (r *AutomationRunRepo) DueTasks(ctx context.Context, orgID, ruleID string, from, to time.Time) ([]string, error) {
	rid, err := parseUUID(ruleID)
	if err != nil {
		return nil, fmt.Errorf("automation due tasks: %w", err)
	}
	var out []string
	err = r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		oid, _ := parseUUID(orgID)
		ids, err := q.ListTasksDueBetween(ctx, gen.ListTasksDueBetweenParams{
			OrgID: oid, RuleID: rid,
			FromAt: pgtype.Timestamptz{Time: from, Valid: true},
			ToAt:   pgtype.Timestamptz{Time: to, Valid: true},
		})
		if err != nil {
			return err
		}
		for _, id := range ids {
			out = append(out, id.String())
		}
		return nil
	})
	return out, err
}
