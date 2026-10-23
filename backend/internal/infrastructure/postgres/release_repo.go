package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/project"
	"github.com/mesutokul/fluxboard/backend/internal/infrastructure/postgres/gen"
)

// ReleaseRepo is the Postgres-backed project.ReleaseRepository ([T], RLS).
type ReleaseRepo struct{ tp *TenantPool }

// NewReleaseRepo builds a ReleaseRepo over the tenant pool.
func NewReleaseRepo(tp *TenantPool) *ReleaseRepo { return &ReleaseRepo{tp: tp} }

var _ project.ReleaseRepository = (*ReleaseRepo)(nil)

// nullDate maps an optional day to a date param (nil ⇒ SQL NULL).
func nullDate(t *time.Time) pgtype.Date {
	if t == nil {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: t.UTC(), Valid: true}
}

// datePtr maps a nullable date column to midnight UTC (nil when SQL NULL).
func datePtr(d pgtype.Date) *time.Time {
	if !d.Valid {
		return nil
	}
	t := time.Date(d.Time.Year(), d.Time.Month(), d.Time.Day(), 0, 0, 0, 0, time.UTC)
	return &t
}

func mapRelease(row gen.Release) *project.Release {
	return &project.Release{
		ID: row.ID.String(), OrgID: row.OrgID.String(), ProjectID: row.ProjectID.String(),
		Name: row.Name, Description: row.Description,
		ReleaseDate: datePtr(row.ReleaseDate), ReleasedAt: tsPtr(row.ReleasedAt),
		CreatedBy: row.CreatedBy.String(), CreatedAt: row.CreatedAt,
	}
}

func (r *ReleaseRepo) Create(ctx context.Context, orgID string, rel *project.Release) error {
	id, err := parseUUID(rel.ID)
	if err != nil {
		return fmt.Errorf("release create: %w", err)
	}
	pid, err := parseUUID(rel.ProjectID)
	if err != nil {
		return fmt.Errorf("release create: project: %w", err)
	}
	by, err := parseUUID(rel.CreatedBy)
	if err != nil {
		return fmt.Errorf("release create: by: %w", err)
	}
	return r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		oid, _ := parseUUID(orgID)
		return q.CreateRelease(ctx, gen.CreateReleaseParams{
			ID: id, OrgID: oid, ProjectID: pid, Name: rel.Name, Description: rel.Description,
			ReleaseDate: nullDate(rel.ReleaseDate), CreatedBy: by,
		})
	})
}

func (r *ReleaseRepo) Get(ctx context.Context, orgID, id string) (*project.Release, error) {
	rid, err := parseUUID(id)
	if err != nil {
		return nil, fmt.Errorf("release get: %w", err)
	}
	var out *project.Release
	err = r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		oid, _ := parseUUID(orgID)
		row, err := q.GetRelease(ctx, gen.GetReleaseParams{OrgID: oid, ID: rid})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		out = mapRelease(row)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *ReleaseRepo) ListByProject(ctx context.Context, orgID, projectID string) ([]project.Release, error) {
	pid, err := parseUUID(projectID)
	if err != nil {
		return nil, fmt.Errorf("release list: %w", err)
	}
	out := make([]project.Release, 0)
	err = r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		oid, _ := parseUUID(orgID)
		rows, err := q.ListReleasesByProject(ctx, gen.ListReleasesByProjectParams{OrgID: oid, ProjectID: pid})
		if err != nil {
			return err
		}
		for _, row := range rows {
			out = append(out, *mapRelease(row))
		}
		return nil
	})
	return out, err
}

func (r *ReleaseRepo) Update(ctx context.Context, orgID string, rel *project.Release) error {
	rid, err := parseUUID(rel.ID)
	if err != nil {
		return fmt.Errorf("release update: %w", err)
	}
	return r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		oid, _ := parseUUID(orgID)
		n, err := q.UpdateRelease(ctx, gen.UpdateReleaseParams{
			Name: rel.Name, Description: rel.Description,
			ReleaseDate: nullDate(rel.ReleaseDate), ReleasedAt: nullTS(rel.ReleasedAt),
			OrgID: oid, ID: rid,
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

func (r *ReleaseRepo) Delete(ctx context.Context, orgID, id string) error {
	rid, err := parseUUID(id)
	if err != nil {
		return fmt.Errorf("release delete: %w", err)
	}
	return r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		oid, _ := parseUUID(orgID)
		n, err := q.DeleteRelease(ctx, gen.DeleteReleaseParams{OrgID: oid, ID: rid})
		if err != nil {
			return err
		}
		if n == 0 {
			return domain.ErrNotFound
		}
		return nil
	})
}

func (r *ReleaseRepo) SetTaskRelease(ctx context.Context, orgID, taskID string, releaseID *string) error {
	tid, err := parseUUID(taskID)
	if err != nil {
		return fmt.Errorf("release assign: task: %w", err)
	}
	rel, err := nullableUUID(releaseID)
	if err != nil {
		return fmt.Errorf("release assign: release: %w", err)
	}
	return r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		oid, _ := parseUUID(orgID)
		n, err := q.SetTaskRelease(ctx, gen.SetTaskReleaseParams{ReleaseID: rel, OrgID: oid, ID: tid})
		if err != nil {
			return err
		}
		if n == 0 {
			return domain.ErrNotFound
		}
		return nil
	})
}
