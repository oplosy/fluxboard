package taskuc

import (
	"context"
	"fmt"

	"github.com/mesutokul/fluxboard/backend/internal/domain/automation"
	"github.com/mesutokul/fluxboard/backend/internal/domain/project"
	"github.com/mesutokul/fluxboard/backend/internal/domain/tenant"
)

// CanViewTask is nil when the user may see the task (project VIEWER+); the
// GitHub PR list uses it as its read gate (FR-TASK-016).
func (s *Service) CanViewTask(ctx context.Context, orgID, userID, taskID string, orgRole tenant.OrgRole) error {
	_, _, err := s.taskAccess(ctx, orgID, taskID, userID, orgRole, project.RoleViewer)
	return err
}

// MoveToDone moves a task to its project's done column on behalf of actorID
// (a merged pull request, FR-TASK-016). It takes the automation move path, so
// the dependency guard, activity log, event and rule cascade all apply; no
// per-user access check, the admin who enabled the integration authorised it.
// A project without a done column is left alone.
func (s *Service) MoveToDone(ctx context.Context, orgID, actorID, taskID string) error {
	t, err := s.tasks.Get(ctx, orgID, taskID)
	if err != nil {
		return err
	}
	cols, err := s.projectColumns(ctx, orgID, t.ProjectID)
	if err != nil {
		return err
	}
	done := project.DoneColumnID(cols)
	if done == "" {
		return nil
	}
	if err := s.automationMove(ctx, orgID, automation.ActionRequest{
		TaskID: taskID, ActorID: actorID, Config: automation.ActionConfig{ColumnID: done},
	}, t); err != nil {
		return fmt.Errorf("move task %s to done: %w", taskID, err)
	}
	return nil
}
