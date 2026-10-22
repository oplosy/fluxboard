package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/project"
	"github.com/mesutokul/fluxboard/backend/internal/infrastructure/postgres/gen"
)

// EpicRepo is the Postgres-backed project.EpicRepository ([T], RLS).
type EpicRepo struct{ tp *TenantPool }

// NewEpicRepo builds an EpicRepo over the tenant pool.
func NewEpicRepo(tp *TenantPool) *EpicRepo { return &EpicRepo{tp: tp} }

var _ project.EpicRepository = (*EpicRepo)(nil)

func mapEpic(row gen.Epic) *project.Epic {
	return &project.Epic{
		ID: row.ID.String(), OrgID: row.OrgID.String(), ProjectID: row.ProjectID.String(),
		Name: row.Name, Color: row.Color, CreatedBy: row.CreatedBy.String(), CreatedAt: row.CreatedAt,
	}
}

func (r *EpicRepo) Create(ctx context.Context, orgID string, e *project.Epic) error {
	id, err := parseUUID(e.ID)
	if err != nil {
		return fmt.Errorf("epic create: %w", err)
	}
	pid, err := parseUUID(e.ProjectID)
	if err != nil {
		return fmt.Errorf("epic create: project: %w", err)
	}
	by, err := parseUUID(e.CreatedBy)
	if err != nil {
		return fmt.Errorf("epic create: by: %w", err)
	}
	return r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		oid, _ := parseUUID(orgID)
		return q.CreateEpic(ctx, gen.CreateEpicParams{
			ID: id, OrgID: oid, ProjectID: pid, Name: e.Name, Color: e.Color, CreatedBy: by,
		})
	})
}

func (r *EpicRepo) Get(ctx context.Context, orgID, id string) (*project.Epic, error) {
	eid, err := parseUUID(id)
	if err != nil {
		return nil, fmt.Errorf("epic get: %w", err)
	}
	var out *project.Epic
	err = r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		oid, _ := parseUUID(orgID)
		row, err := q.GetEpic(ctx, gen.GetEpicParams{OrgID: oid, ID: eid})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		out = mapEpic(row)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *EpicRepo) ListByProject(ctx context.Context, orgID, projectID string) ([]project.Epic, error) {
	pid, err := parseUUID(projectID)
	if err != nil {
		return nil, fmt.Errorf("epic list: %w", err)
	}
	out := make([]project.Epic, 0)
	err = r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		oid, _ := parseUUID(orgID)
		rows, err := q.ListEpicsByProject(ctx, gen.ListEpicsByProjectParams{OrgID: oid, ProjectID: pid})
		if err != nil {
			return err
		}
		for _, row := range rows {
			out = append(out, *mapEpic(row))
		}
		return nil
	})
	return out, err
}

func (r *EpicRepo) Update(ctx context.Context, orgID string, e *project.Epic) error {
	eid, err := parseUUID(e.ID)
	if err != nil {
		return fmt.Errorf("epic update: %w", err)
	}
	return r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		oid, _ := parseUUID(orgID)
		n, err := q.UpdateEpic(ctx, gen.UpdateEpicParams{Name: e.Name, Color: e.Color, OrgID: oid, ID: eid})
		if err != nil {
			return err
		}
		if n == 0 {
			return domain.ErrNotFound
		}
		return nil
	})
}

func (r *EpicRepo) Delete(ctx context.Context, orgID, id string) error {
	eid, err := parseUUID(id)
	if err != nil {
		return fmt.Errorf("epic delete: %w", err)
	}
	return r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		oid, _ := parseUUID(orgID)
		n, err := q.DeleteEpic(ctx, gen.DeleteEpicParams{OrgID: oid, ID: eid})
		if err != nil {
			return err
		}
		if n == 0 {
			return domain.ErrNotFound
		}
		return nil
	})
}

func (r *EpicRepo) SetTaskEpic(ctx context.Context, orgID, taskID string, epicID *string) error {
	tid, err := parseUUID(taskID)
	if err != nil {
		return fmt.Errorf("epic assign: task: %w", err)
	}
	epic, err := nullableUUID(epicID)
	if err != nil {
		return fmt.Errorf("epic assign: epic: %w", err)
	}
	return r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		oid, _ := parseUUID(orgID)
		n, err := q.SetTaskEpic(ctx, gen.SetTaskEpicParams{EpicID: epic, OrgID: oid, ID: tid})
		if err != nil {
			return err
		}
		if n == 0 {
			return domain.ErrNotFound
		}
		return nil
	})
}
