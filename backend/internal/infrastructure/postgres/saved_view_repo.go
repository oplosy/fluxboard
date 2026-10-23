package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/project"
	"github.com/mesutokul/fluxboard/backend/internal/infrastructure/postgres/gen"
)

// SavedViewRepo is the Postgres-backed project.SavedViewRepository ([T], RLS).
type SavedViewRepo struct{ tp *TenantPool }

// NewSavedViewRepo builds a SavedViewRepo over the tenant pool.
func NewSavedViewRepo(tp *TenantPool) *SavedViewRepo { return &SavedViewRepo{tp: tp} }

var _ project.SavedViewRepository = (*SavedViewRepo)(nil)

func mapSavedView(row gen.SavedView) (*project.SavedView, error) {
	v := &project.SavedView{
		ID: row.ID.String(), OrgID: row.OrgID.String(), ProjectID: row.ProjectID.String(),
		OwnerID: row.OwnerID.String(), Name: row.Name, Shared: row.Shared, CreatedAt: row.CreatedAt,
	}
	if err := json.Unmarshal(row.Filters, &v.Filters); err != nil {
		return nil, fmt.Errorf("saved view %s: filters: %w", v.ID, err)
	}
	return v, nil
}

func (r *SavedViewRepo) Create(ctx context.Context, orgID string, v *project.SavedView) error {
	id, err := parseUUID(v.ID)
	if err != nil {
		return fmt.Errorf("view create: %w", err)
	}
	pid, err := parseUUID(v.ProjectID)
	if err != nil {
		return fmt.Errorf("view create: project: %w", err)
	}
	owner, err := parseUUID(v.OwnerID)
	if err != nil {
		return fmt.Errorf("view create: owner: %w", err)
	}
	filters, err := json.Marshal(v.Filters)
	if err != nil {
		return fmt.Errorf("view create: filters: %w", err)
	}
	return r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		oid, _ := parseUUID(orgID)
		return q.CreateSavedView(ctx, gen.CreateSavedViewParams{
			ID: id, OrgID: oid, ProjectID: pid, OwnerID: owner, Name: v.Name, Shared: v.Shared, Filters: filters,
		})
	})
}

func (r *SavedViewRepo) Get(ctx context.Context, orgID, id string) (*project.SavedView, error) {
	vid, err := parseUUID(id)
	if err != nil {
		return nil, fmt.Errorf("view get: %w", err)
	}
	var out *project.SavedView
	err = r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		oid, _ := parseUUID(orgID)
		row, err := q.GetSavedView(ctx, gen.GetSavedViewParams{OrgID: oid, ID: vid})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		out, err = mapSavedView(row)
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *SavedViewRepo) ListVisible(ctx context.Context, orgID, projectID, userID string) ([]project.SavedView, error) {
	pid, err := parseUUID(projectID)
	if err != nil {
		return nil, fmt.Errorf("view list: project: %w", err)
	}
	uid, err := parseUUID(userID)
	if err != nil {
		return nil, fmt.Errorf("view list: user: %w", err)
	}
	out := make([]project.SavedView, 0)
	err = r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		oid, _ := parseUUID(orgID)
		rows, err := q.ListVisibleSavedViews(ctx, gen.ListVisibleSavedViewsParams{OrgID: oid, ProjectID: pid, UserID: uid})
		if err != nil {
			return err
		}
		for _, row := range rows {
			v, err := mapSavedView(row)
			if err != nil {
				return err
			}
			out = append(out, *v)
		}
		return nil
	})
	return out, err
}

func (r *SavedViewRepo) Update(ctx context.Context, orgID string, v *project.SavedView) error {
	vid, err := parseUUID(v.ID)
	if err != nil {
		return fmt.Errorf("view update: %w", err)
	}
	filters, err := json.Marshal(v.Filters)
	if err != nil {
		return fmt.Errorf("view update: filters: %w", err)
	}
	return r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		oid, _ := parseUUID(orgID)
		n, err := q.UpdateSavedView(ctx, gen.UpdateSavedViewParams{
			Name: v.Name, Shared: v.Shared, Filters: filters, OrgID: oid, ID: vid,
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

func (r *SavedViewRepo) Delete(ctx context.Context, orgID, id string) error {
	vid, err := parseUUID(id)
	if err != nil {
		return fmt.Errorf("view delete: %w", err)
	}
	return r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		oid, _ := parseUUID(orgID)
		n, err := q.DeleteSavedView(ctx, gen.DeleteSavedViewParams{OrgID: oid, ID: vid})
		if err != nil {
			return err
		}
		if n == 0 {
			return domain.ErrNotFound
		}
		return nil
	})
}
