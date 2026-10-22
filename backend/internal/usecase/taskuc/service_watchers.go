package taskuc

import (
	"context"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/project"
	"github.com/mesutokul/fluxboard/backend/internal/domain/tenant"
)

// ---- Watchers (FR-TASK-014) ---------------------------------------------------
// Anyone who can see a task may watch it; callers only add or remove
// themselves. Watchers join the comment fan-out in AddComment.

// ListWatchers returns the task's watchers (VIEWER).
func (s *Service) ListWatchers(ctx context.Context, orgID, userID, taskID string, orgRole tenant.OrgRole) ([]project.Watcher, error) {
	if s.watchers == nil {
		return nil, domain.ErrForbidden // watchers disabled
	}
	if _, _, err := s.taskAccess(ctx, orgID, taskID, userID, orgRole, project.RoleViewer); err != nil {
		return nil, err
	}
	return s.watchers.ListByTask(ctx, orgID, taskID)
}

// Watch makes the caller watch the task (VIEWER, idempotent).
func (s *Service) Watch(ctx context.Context, orgID, userID, taskID string, orgRole tenant.OrgRole) error {
	if s.watchers == nil {
		return domain.ErrForbidden // watchers disabled
	}
	if _, _, err := s.taskAccess(ctx, orgID, taskID, userID, orgRole, project.RoleViewer); err != nil {
		return err
	}
	return s.watchers.Add(ctx, orgID, taskID, userID)
}

// Unwatch stops the caller watching the task (VIEWER, idempotent).
func (s *Service) Unwatch(ctx context.Context, orgID, userID, taskID string, orgRole tenant.OrgRole) error {
	if s.watchers == nil {
		return domain.ErrForbidden // watchers disabled
	}
	if _, _, err := s.taskAccess(ctx, orgID, taskID, userID, orgRole, project.RoleViewer); err != nil {
		return err
	}
	return s.watchers.Remove(ctx, orgID, taskID, userID)
}

// watcherIDs returns the task's watcher ids for fan-out. Best-effort: a lookup
// error is logged and yields none, never failing the comment.
func (s *Service) watcherIDs(ctx context.Context, orgID, taskID string) []string {
	if s.watchers == nil {
		return nil
	}
	ws, err := s.watchers.ListByTask(ctx, orgID, taskID)
	if err != nil {
		s.logger.Warn("watchers lookup failed", "task", taskID, "err", err)
		return nil
	}
	ids := make([]string, 0, len(ws))
	for _, w := range ws {
		ids = append(ids, w.UserID)
	}
	return ids
}
