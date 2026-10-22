package postgres

import (
	"context"
	"fmt"

	"github.com/mesutokul/fluxboard/backend/internal/domain/project"
	"github.com/mesutokul/fluxboard/backend/internal/infrastructure/postgres/gen"
)

// TaskWatcherRepo is the Postgres-backed project.WatcherRepository ([T], RLS).
type TaskWatcherRepo struct{ tp *TenantPool }

// NewTaskWatcherRepo builds a TaskWatcherRepo over the tenant pool.
func NewTaskWatcherRepo(tp *TenantPool) *TaskWatcherRepo { return &TaskWatcherRepo{tp: tp} }

var _ project.WatcherRepository = (*TaskWatcherRepo)(nil)

func (r *TaskWatcherRepo) Add(ctx context.Context, orgID, taskID, userID string) error {
	tid, err := parseUUID(taskID)
	if err != nil {
		return fmt.Errorf("task watcher add: %w", err)
	}
	uid, err := parseUUID(userID)
	if err != nil {
		return fmt.Errorf("task watcher add: user: %w", err)
	}
	return r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		oid, _ := parseUUID(orgID)
		return q.AddTaskWatcher(ctx, gen.AddTaskWatcherParams{TaskID: tid, UserID: uid, OrgID: oid})
	})
}

func (r *TaskWatcherRepo) Remove(ctx context.Context, orgID, taskID, userID string) error {
	tid, err := parseUUID(taskID)
	if err != nil {
		return fmt.Errorf("task watcher remove: %w", err)
	}
	uid, err := parseUUID(userID)
	if err != nil {
		return fmt.Errorf("task watcher remove: user: %w", err)
	}
	return r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		oid, _ := parseUUID(orgID)
		return q.DeleteTaskWatcher(ctx, gen.DeleteTaskWatcherParams{OrgID: oid, TaskID: tid, UserID: uid})
	})
}

func (r *TaskWatcherRepo) ListByTask(ctx context.Context, orgID, taskID string) ([]project.Watcher, error) {
	tid, err := parseUUID(taskID)
	if err != nil {
		return nil, fmt.Errorf("task watcher list: %w", err)
	}
	out := make([]project.Watcher, 0)
	err = r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		oid, _ := parseUUID(orgID)
		rows, err := q.ListTaskWatchers(ctx, gen.ListTaskWatchersParams{OrgID: oid, TaskID: tid})
		if err != nil {
			return err
		}
		for _, row := range rows {
			out = append(out, project.Watcher{
				UserID: row.ID.String(), Email: row.Email, Name: row.Name, CreatedAt: row.CreatedAt,
			})
		}
		return nil
	})
	return out, err
}
