package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/github"
	"github.com/mesutokul/fluxboard/backend/internal/infrastructure/postgres/gen"
)

// GithubRepo is the Postgres-backed github.Repository ([T], RLS).
type GithubRepo struct{ tp *TenantPool }

// NewGithubRepo builds a GithubRepo over the tenant pool.
func NewGithubRepo(tp *TenantPool) *GithubRepo { return &GithubRepo{tp: tp} }

var _ github.Repository = (*GithubRepo)(nil)

func (r *GithubRepo) Get(ctx context.Context, orgID string) (*github.Integration, error) {
	oid, err := parseUUID(orgID)
	if err != nil {
		return nil, fmt.Errorf("github get: %w", err)
	}
	var out *github.Integration
	err = r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		row, err := q.GetGithubIntegration(ctx, oid)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		if err != nil {
			return err
		}
		projects, err := q.ListGithubMergeProjects(ctx, oid)
		if err != nil {
			return err
		}
		out = &github.Integration{
			OrgID: row.OrgID.String(), SecretEnc: row.SecretEnc, Repos: row.Repos,
			MoveProjects: make([]string, 0, len(projects)), CreatedBy: row.CreatedBy.String(),
		}
		for _, p := range projects {
			out.MoveProjects = append(out.MoveProjects, p.String())
		}
		return nil
	})
	return out, err
}

func (r *GithubRepo) SaveSecret(ctx context.Context, orgID, secretEnc, actorID string) error {
	oid, err := parseUUID(orgID)
	if err != nil {
		return fmt.Errorf("github save secret: %w", err)
	}
	by, err := parseUUID(actorID)
	if err != nil {
		return fmt.Errorf("github save secret: actor: %w", err)
	}
	return r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		return q.UpsertGithubSecret(ctx, gen.UpsertGithubSecretParams{OrgID: oid, SecretEnc: secretEnc, CreatedBy: by})
	})
}

func (r *GithubRepo) SaveSettings(ctx context.Context, orgID string, repos, moveProjectIDs []string) error {
	oid, err := parseUUID(orgID)
	if err != nil {
		return fmt.Errorf("github save settings: %w", err)
	}
	ids := make([]uuid.UUID, 0, len(moveProjectIDs))
	seen := map[uuid.UUID]bool{}
	for _, s := range moveProjectIDs {
		id, err := uuid.Parse(s)
		if err != nil {
			return domain.ErrValidation
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	if repos == nil {
		repos = []string{}
	}
	return r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		n, err := q.SetGithubRepos(ctx, gen.SetGithubReposParams{Repos: repos, OrgID: oid})
		if err != nil {
			return err
		}
		if n == 0 {
			return domain.ErrNotFound
		}
		if err := q.ClearGithubMergeProjects(ctx, oid); err != nil {
			return err
		}
		added, err := q.AddGithubMergeProjects(ctx, gen.AddGithubMergeProjectsParams{OrgID: oid, ProjectIds: ids})
		if err != nil {
			return err
		}
		if int(added) != len(ids) { // a project id is not in this org
			return domain.ErrValidation
		}
		return nil
	})
}

func (r *GithubRepo) Delete(ctx context.Context, orgID string) error {
	oid, err := parseUUID(orgID)
	if err != nil {
		return fmt.Errorf("github delete: %w", err)
	}
	return r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		if err := q.ClearGithubMergeProjects(ctx, oid); err != nil {
			return err
		}
		return q.DeleteGithubIntegration(ctx, oid)
	})
}

func (r *GithubRepo) ResolveTasks(ctx context.Context, orgID string, keys []string) ([]github.TaskRef, error) {
	oid, err := parseUUID(orgID)
	if err != nil {
		return nil, fmt.Errorf("github resolve: %w", err)
	}
	var out []github.TaskRef
	err = r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		rows, err := q.ResolveTasksByKeys(ctx, gen.ResolveTasksByKeysParams{OrgID: oid, Keys: keys})
		if err != nil {
			return err
		}
		for _, row := range rows {
			out = append(out, github.TaskRef{TaskID: row.ID.String(), ProjectID: row.ProjectID.String()})
		}
		return nil
	})
	return out, err
}

func (r *GithubRepo) Apply(ctx context.Context, orgID, deliveryID string, ev *github.PullRequestEvent, taskIDs []string) (bool, error) {
	oid, err := parseUUID(orgID)
	if err != nil {
		return false, fmt.Errorf("github apply: %w", err)
	}
	tids := make([]uuid.UUID, 0, len(taskIDs))
	for _, s := range taskIDs {
		id, err := parseUUID(s)
		if err != nil {
			return false, fmt.Errorf("github apply: task: %w", err)
		}
		tids = append(tids, id)
	}
	duplicate := false
	err = r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		n, err := q.RecordGithubDelivery(ctx, gen.RecordGithubDeliveryParams{OrgID: oid, DeliveryID: deliveryID})
		if err != nil {
			return err
		}
		if n == 0 {
			duplicate = true
			return nil
		}
		if err := q.PruneGithubDeliveries(ctx, oid); err != nil {
			return err
		}
		if err := q.RefreshPullRequestRows(ctx, gen.RefreshPullRequestRowsParams{
			Title: ev.Title, State: ev.State, Url: ev.URL, PrUpdatedAt: ev.UpdatedAt,
			OrgID: oid, Repo: ev.Repo, Number: int32(ev.Number),
		}); err != nil {
			return err
		}
		for _, tid := range tids {
			if err := q.UpsertTaskPullRequest(ctx, gen.UpsertTaskPullRequestParams{
				OrgID: oid, TaskID: tid, Repo: ev.Repo, Number: int32(ev.Number),
				Title: ev.Title, State: ev.State, Url: ev.URL, PrUpdatedAt: ev.UpdatedAt,
			}); err != nil {
				return err
			}
		}
		return nil
	})
	return duplicate, err
}

func (r *GithubRepo) ListByTask(ctx context.Context, orgID, taskID string) ([]github.LinkedPR, error) {
	oid, err := parseUUID(orgID)
	if err != nil {
		return nil, fmt.Errorf("github list: %w", err)
	}
	tid, err := parseUUID(taskID)
	if err != nil {
		return nil, fmt.Errorf("github list: task: %w", err)
	}
	out := []github.LinkedPR{}
	err = r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		rows, err := q.ListTaskPullRequests(ctx, gen.ListTaskPullRequestsParams{OrgID: oid, TaskID: tid})
		if err != nil {
			return err
		}
		for _, row := range rows {
			out = append(out, github.LinkedPR{
				Repo: row.Repo, Number: int(row.Number), Title: row.Title,
				State: row.State, URL: row.Url, UpdatedAt: row.PrUpdatedAt,
			})
		}
		return nil
	})
	return out, err
}
