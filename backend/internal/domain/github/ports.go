package github

import "context"

// Repository persists the GitHub integration of an org ([T], RLS). Every method
// is org-scoped; Get returns domain.ErrNotFound when not configured.
type Repository interface {
	Get(ctx context.Context, orgID string) (*Integration, error)
	// SaveSecret creates the integration or replaces its secret (rotation).
	SaveSecret(ctx context.Context, orgID, secretEnc, actorID string) error
	// SaveSettings replaces the allow-list and the move-on-merge projects in one
	// transaction. domain.ErrValidation when a project id is not in the org;
	// domain.ErrNotFound when the integration does not exist.
	SaveSettings(ctx context.Context, orgID string, repos, moveProjectIDs []string) error
	Delete(ctx context.Context, orgID string) error
	// ResolveTasks maps "WEB-12" style keys to live tasks of the org.
	ResolveTasks(ctx context.Context, orgID string, keys []string) ([]TaskRef, error)
	// Apply records deliveryID and upserts the PR's links to taskIDs in one
	// transaction. duplicate=true (and nothing written) when the delivery was
	// already recorded.
	Apply(ctx context.Context, orgID, deliveryID string, ev *PullRequestEvent, taskIDs []string) (duplicate bool, err error)
	ListByTask(ctx context.Context, orgID, taskID string) ([]LinkedPR, error)
}
