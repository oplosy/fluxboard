package taskuc

import (
	"context"
	"fmt"
	"strings"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/automation"
	"github.com/mesutokul/fluxboard/backend/internal/domain/notify"
	"github.com/mesutokul/fluxboard/backend/internal/domain/project"
	"github.com/mesutokul/fluxboard/backend/internal/pkg/rank"
)

var _ automation.TaskActions = (*Service)(nil)

// ApplyAutomation performs one automation action as a normal task write
// (ADR-036): same persistence, activity log, realtime event, notification and
// rule cascade as a human edit. There is no per-user access check — the rule
// was authorised by an admin when it was created — but the guards that are
// about data integrity still apply (board membership of the column, the
// dependency guard on the done column). The activity is attributed to the
// rule and its creator (req.ActorID).
func (s *Service) ApplyAutomation(ctx context.Context, orgID string, req automation.ActionRequest) error {
	t, err := s.tasks.Get(ctx, orgID, req.TaskID)
	if err != nil {
		return err
	}
	switch req.Action {
	case automation.ActionAssign:
		uid := strings.TrimSpace(req.Config.UserID)
		if uid == "" {
			return fmt.Errorf("assign: empty user_id")
		}
		in := updateInputOf(t)
		in.AssigneeID = &uid
		_, err = s.applyUpdate(ctx, orgID, req.ActorID, req.RuleID, t, in)
		return err
	case automation.ActionSetPriority:
		p := project.Priority(req.Config.Priority)
		if !p.Valid() {
			return fmt.Errorf("set_priority: bad priority")
		}
		in := updateInputOf(t)
		in.Priority = p
		_, err = s.applyUpdate(ctx, orgID, req.ActorID, req.RuleID, t, in)
		return err
	case automation.ActionMove:
		return s.automationMove(ctx, orgID, req, t)
	case automation.ActionAddLabel:
		lid := strings.TrimSpace(req.Config.LabelID)
		if lid == "" {
			return fmt.Errorf("add_label: empty label_id")
		}
		if err := s.labels.Attach(ctx, orgID, t.ID, lid); err != nil {
			return err
		}
		s.appendActivity(ctx, orgID, req.ActorID, req.RuleID, t.ID, "label", nil, &lid)
		s.publish(ctx, orgID, notify.EventTaskUpdated, req.ActorID, map[string]any{
			"task_id": t.ID, "project_id": t.ProjectID, "fields": []string{"labels"},
		})
		return nil
	default:
		return fmt.Errorf("unknown action %q", req.Action)
	}
}

// updateInputOf is the full current value of t's editable fields, so a
// single-field automation change diffs to exactly that field.
func updateInputOf(t *project.Task) UpdateTaskInput {
	return UpdateTaskInput{
		Title: t.Title, Description: t.Description, AssigneeID: t.AssigneeID,
		Priority: t.Priority, StartDate: t.StartDate, DueDate: t.DueDate,
	}
}

// automationMove relocates t to the end of the target column on its own board.
func (s *Service) automationMove(ctx context.Context, orgID string, req automation.ActionRequest, t *project.Task) error {
	columnID := strings.TrimSpace(req.Config.ColumnID)
	if err := s.columnInProject(ctx, orgID, t.ProjectID, columnID); err != nil {
		return err
	}
	if columnID == t.ColumnID {
		return nil // already there
	}
	// Dependency guard (FR-LINKS): the done column stays closed while open
	// blockers exist, for rules as for people.
	cols, err := s.projectColumns(ctx, orgID, t.ProjectID)
	if err != nil {
		return err
	}
	if project.DoneColumnID(cols) == columnID {
		ready, err := s.unblocked(ctx, orgID, t.ID, map[string]string{t.ProjectID: columnID})
		if err != nil {
			return err
		}
		if !ready {
			return fmt.Errorf("%w: task has open blockers", domain.ErrConflict)
		}
	}
	existing, err := s.tasks.ListByColumn(ctx, orgID, columnID)
	if err != nil {
		return err
	}
	last := ""
	if n := len(existing); n > 0 {
		last = existing[n-1].Rank
	}
	from := t.ColumnID
	newRank := rank.Append(last)
	if err := s.tasks.Move(ctx, orgID, t.ID, columnID, newRank); err != nil {
		return err
	}
	s.appendActivity(ctx, orgID, req.ActorID, req.RuleID, t.ID, "column", &from, &columnID)
	s.publish(ctx, orgID, notify.EventTaskMoved, req.ActorID, map[string]any{
		"task_id": t.ID, "project_id": t.ProjectID,
		"from_column": from, "to_column": columnID, "rank": newRank,
	})
	s.automate(ctx, automation.Event{
		OrgID: orgID, Trigger: automation.TriggerTaskMoved,
		TaskID: t.ID, ActorID: req.ActorID, ToColumnID: columnID,
	})
	return nil
}
