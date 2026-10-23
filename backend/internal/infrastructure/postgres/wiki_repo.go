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

// WikiRepo is the Postgres-backed project.WikiRepository ([T], RLS).
type WikiRepo struct{ tp *TenantPool }

// NewWikiRepo builds a WikiRepo over the tenant pool.
func NewWikiRepo(tp *TenantPool) *WikiRepo { return &WikiRepo{tp: tp} }

var _ project.WikiRepository = (*WikiRepo)(nil)

func mapWikiPage(row gen.WikiPage) *project.WikiPage {
	return &project.WikiPage{
		ID: row.ID.String(), OrgID: row.OrgID.String(), ProjectID: row.ProjectID.String(),
		Title: row.Title, Body: row.Body, Version: int(row.Version),
		CreatedBy: row.CreatedBy.String(), UpdatedBy: row.UpdatedBy.String(),
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

// replaceMentions rewrites a page's mention index inside the caller's transaction.
func replaceMentions(ctx context.Context, q *gen.Queries, orgID, projectID, pageID string, numbers []int) error {
	oid, _ := parseUUID(orgID)
	pid, _ := parseUUID(projectID)
	id, _ := parseUUID(pageID)
	if err := q.ClearWikiPageTasks(ctx, gen.ClearWikiPageTasksParams{OrgID: oid, PageID: id}); err != nil {
		return err
	}
	for _, n := range numbers {
		if err := q.AddWikiPageTask(ctx, gen.AddWikiPageTaskParams{OrgID: oid, ProjectID: pid, PageID: id, TaskNumber: int32(n)}); err != nil {
			return err
		}
	}
	return nil
}

func (r *WikiRepo) Create(ctx context.Context, orgID string, p *project.WikiPage, taskNumbers []int) error {
	id, err := parseUUID(p.ID)
	if err != nil {
		return fmt.Errorf("wiki create: %w", err)
	}
	pid, err := parseUUID(p.ProjectID)
	if err != nil {
		return fmt.Errorf("wiki create: project: %w", err)
	}
	by, err := parseUUID(p.CreatedBy)
	if err != nil {
		return fmt.Errorf("wiki create: by: %w", err)
	}
	return r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		oid, _ := parseUUID(orgID)
		if err := q.CreateWikiPage(ctx, gen.CreateWikiPageParams{
			ID: id, OrgID: oid, ProjectID: pid, Title: p.Title, Body: p.Body, CreatedBy: by,
		}); err != nil {
			return err
		}
		return replaceMentions(ctx, q, orgID, p.ProjectID, p.ID, taskNumbers)
	})
}

func (r *WikiRepo) Get(ctx context.Context, orgID, id string) (*project.WikiPage, error) {
	pid, err := parseUUID(id)
	if err != nil {
		return nil, fmt.Errorf("wiki get: %w", err)
	}
	var out *project.WikiPage
	err = r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		oid, _ := parseUUID(orgID)
		row, err := q.GetWikiPage(ctx, gen.GetWikiPageParams{OrgID: oid, ID: pid})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		out = mapWikiPage(row)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *WikiRepo) ListByProject(ctx context.Context, orgID, projectID, query string) ([]project.WikiPage, error) {
	pid, err := parseUUID(projectID)
	if err != nil {
		return nil, fmt.Errorf("wiki list: %w", err)
	}
	out := make([]project.WikiPage, 0)
	err = r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		oid, _ := parseUUID(orgID)
		rows, err := q.ListWikiPagesByProject(ctx, gen.ListWikiPagesByProjectParams{OrgID: oid, ProjectID: pid, Query: query})
		if err != nil {
			return err
		}
		for _, row := range rows {
			out = append(out, *mapWikiPage(gen.WikiPage(row)))
		}
		return nil
	})
	return out, err
}

func (r *WikiRepo) Update(ctx context.Context, orgID string, p *project.WikiPage, expectedVersion int, taskNumbers []int) error {
	id, err := parseUUID(p.ID)
	if err != nil {
		return fmt.Errorf("wiki update: %w", err)
	}
	by, err := parseUUID(p.UpdatedBy)
	if err != nil {
		return fmt.Errorf("wiki update: by: %w", err)
	}
	return r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		oid, _ := parseUUID(orgID)
		n, err := q.UpdateWikiPage(ctx, gen.UpdateWikiPageParams{
			Title: p.Title, Body: p.Body, UpdatedBy: by, OrgID: oid, ID: id, IfVersion: int32(expectedVersion),
		})
		if err != nil {
			return err
		}
		if n == 0 { // missing, or someone saved first
			if _, err := q.GetWikiPage(ctx, gen.GetWikiPageParams{OrgID: oid, ID: id}); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return domain.ErrNotFound
				}
				return err
			}
			return domain.ErrConflict
		}
		return replaceMentions(ctx, q, orgID, p.ProjectID, p.ID, taskNumbers)
	})
}

func (r *WikiRepo) Delete(ctx context.Context, orgID, id string) error {
	pid, err := parseUUID(id)
	if err != nil {
		return fmt.Errorf("wiki delete: %w", err)
	}
	return r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		oid, _ := parseUUID(orgID)
		n, err := q.DeleteWikiPage(ctx, gen.DeleteWikiPageParams{OrgID: oid, ID: pid})
		if err != nil {
			return err
		}
		if n == 0 {
			return domain.ErrNotFound
		}
		return nil
	})
}

func (r *WikiRepo) TaskRefs(ctx context.Context, orgID, pageID string) ([]project.WikiTaskRef, error) {
	pid, err := parseUUID(pageID)
	if err != nil {
		return nil, fmt.Errorf("wiki refs: %w", err)
	}
	out := make([]project.WikiTaskRef, 0)
	err = r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		oid, _ := parseUUID(orgID)
		rows, err := q.ListWikiPageTaskRefs(ctx, gen.ListWikiPageTaskRefsParams{OrgID: oid, PageID: pid})
		if err != nil {
			return err
		}
		for _, row := range rows {
			out = append(out, project.WikiTaskRef{Number: int(row.Number), Title: row.Title})
		}
		return nil
	})
	return out, err
}

func (r *WikiRepo) MentioningTask(ctx context.Context, orgID, projectID string, number int) ([]project.WikiPage, error) {
	pid, err := parseUUID(projectID)
	if err != nil {
		return nil, fmt.Errorf("wiki mentions: %w", err)
	}
	out := make([]project.WikiPage, 0)
	err = r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		oid, _ := parseUUID(orgID)
		rows, err := q.ListWikiPagesMentioningTask(ctx, gen.ListWikiPagesMentioningTaskParams{OrgID: oid, ProjectID: pid, TaskNumber: int32(number)})
		if err != nil {
			return err
		}
		for _, row := range rows {
			out = append(out, *mapWikiPage(gen.WikiPage(row)))
		}
		return nil
	})
	return out, err
}
