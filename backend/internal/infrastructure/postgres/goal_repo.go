package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/project"
	"github.com/mesutokul/fluxboard/backend/internal/infrastructure/postgres/gen"
)

// GoalRepo is the Postgres-backed project.GoalRepository ([T], RLS).
type GoalRepo struct{ tp *TenantPool }

// NewGoalRepo builds a GoalRepo over the tenant pool.
func NewGoalRepo(tp *TenantPool) *GoalRepo { return &GoalRepo{tp: tp} }

var _ project.GoalRepository = (*GoalRepo)(nil)

func mapGoal(row gen.Goal) *project.Goal {
	return &project.Goal{
		ID: row.ID.String(), OrgID: row.OrgID.String(), Title: row.Title, Description: row.Description,
		OwnerID: row.OwnerID.String(), TargetDate: datePtr(row.TargetDate), CreatedAt: row.CreatedAt,
	}
}

func (r *GoalRepo) Create(ctx context.Context, orgID string, g *project.Goal) error {
	id, err := parseUUID(g.ID)
	if err != nil {
		return fmt.Errorf("goal create: %w", err)
	}
	owner, err := parseUUID(g.OwnerID)
	if err != nil {
		return fmt.Errorf("goal create: owner: %w", err)
	}
	return r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		oid, _ := parseUUID(orgID)
		return q.CreateGoal(ctx, gen.CreateGoalParams{
			ID: id, OrgID: oid, Title: g.Title, Description: g.Description,
			OwnerID: owner, TargetDate: nullDate(g.TargetDate),
		})
	})
}

func (r *GoalRepo) Get(ctx context.Context, orgID, id string) (*project.Goal, error) {
	gid, err := parseUUID(id)
	if err != nil {
		return nil, fmt.Errorf("goal get: %w", err)
	}
	var out *project.Goal
	err = r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		oid, _ := parseUUID(orgID)
		row, err := q.GetGoal(ctx, gen.GetGoalParams{OrgID: oid, ID: gid})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		out = mapGoal(row)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *GoalRepo) List(ctx context.Context, orgID string) ([]project.Goal, error) {
	out := make([]project.Goal, 0)
	err := r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		oid, _ := parseUUID(orgID)
		rows, err := q.ListGoals(ctx, oid)
		if err != nil {
			return err
		}
		for _, row := range rows {
			out = append(out, *mapGoal(row))
		}
		return nil
	})
	return out, err
}

func (r *GoalRepo) Update(ctx context.Context, orgID string, g *project.Goal) error {
	gid, err := parseUUID(g.ID)
	if err != nil {
		return fmt.Errorf("goal update: %w", err)
	}
	return r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		oid, _ := parseUUID(orgID)
		n, err := q.UpdateGoal(ctx, gen.UpdateGoalParams{
			Title: g.Title, Description: g.Description, TargetDate: nullDate(g.TargetDate), OrgID: oid, ID: gid,
		})
		if err != nil {
			return err
		}
		if n == 0 {
			return domain.ErrNotFound
		}
		return nil
	})
}

func (r *GoalRepo) Delete(ctx context.Context, orgID, id string) error {
	gid, err := parseUUID(id)
	if err != nil {
		return fmt.Errorf("goal delete: %w", err)
	}
	return r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		oid, _ := parseUUID(orgID)
		n, err := q.DeleteGoal(ctx, gen.DeleteGoalParams{OrgID: oid, ID: gid})
		if err != nil {
			return err
		}
		if n == 0 {
			return domain.ErrNotFound
		}
		return nil
	})
}

// linkTarget maps a kind + item id onto the (epic_id, task_id) column pair.
func linkTarget(kind project.GoalItemKind, itemID string) (epic, task pgtype.UUID, err error) {
	id, err := parseUUID(itemID)
	if err != nil {
		return epic, task, fmt.Errorf("goal link: item: %w", err)
	}
	switch kind {
	case project.GoalItemEpic:
		epic = pgtype.UUID{Bytes: id, Valid: true}
	case project.GoalItemTask:
		task = pgtype.UUID{Bytes: id, Valid: true}
	default:
		return epic, task, domain.ErrValidation
	}
	return epic, task, nil
}

func (r *GoalRepo) Link(ctx context.Context, orgID, goalID string, kind project.GoalItemKind, itemID string) error {
	gid, err := parseUUID(goalID)
	if err != nil {
		return fmt.Errorf("goal link: %w", err)
	}
	epic, task, err := linkTarget(kind, itemID)
	if err != nil {
		return err
	}
	return r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		oid, _ := parseUUID(orgID)
		return q.AddGoalLink(ctx, gen.AddGoalLinkParams{
			ID: uuid.New(), OrgID: oid, GoalID: gid, EpicID: epic, TaskID: task,
		})
	})
}

func (r *GoalRepo) Unlink(ctx context.Context, orgID, goalID string, kind project.GoalItemKind, itemID string) error {
	gid, err := parseUUID(goalID)
	if err != nil {
		return fmt.Errorf("goal unlink: %w", err)
	}
	epic, task, err := linkTarget(kind, itemID)
	if err != nil {
		return err
	}
	return r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		oid, _ := parseUUID(orgID)
		n, err := q.DeleteGoalLink(ctx, gen.DeleteGoalLinkParams{OrgID: oid, GoalID: gid, EpicID: epic, TaskID: task})
		if err != nil {
			return err
		}
		if n == 0 {
			return domain.ErrNotFound
		}
		return nil
	})
}

func (r *GoalRepo) Links(ctx context.Context, orgID, goalID string) ([]project.GoalLink, error) {
	var filter pgtype.UUID
	if goalID != "" {
		gid, err := parseUUID(goalID)
		if err != nil {
			return nil, fmt.Errorf("goal links: %w", err)
		}
		filter = pgtype.UUID{Bytes: gid, Valid: true}
	}
	out := make([]project.GoalLink, 0)
	err := r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		oid, _ := parseUUID(orgID)
		rows, err := q.ListGoalLinks(ctx, gen.ListGoalLinksParams{OrgID: oid, GoalID: filter})
		if err != nil {
			return err
		}
		for _, row := range rows {
			l := project.GoalLink{GoalID: row.GoalID.String(), ProjectID: row.ProjectID.String()}
			if row.EpicID.Valid {
				l.Kind, l.ItemID = project.GoalItemEpic, uuid.UUID(row.EpicID.Bytes).String()
			} else {
				l.Kind, l.ItemID = project.GoalItemTask, uuid.UUID(row.TaskID.Bytes).String()
			}
			out = append(out, l)
		}
		return nil
	})
	return out, err
}
