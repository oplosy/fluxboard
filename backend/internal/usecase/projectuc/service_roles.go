package projectuc

import (
	"context"
	"strings"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/project"
	"github.com/mesutokul/fluxboard/backend/internal/domain/tenant"
	"github.com/mesutokul/fluxboard/backend/internal/pkg/rank"
)

// Column roles and the review gate (ADR-033 decisions 5–6, FR-AGT-006/007).

// Commenter posts a comment as userID (taskuc.Service). Declared here so the
// review flow can leave its note without importing the task usecase.
type Commenter interface {
	AddComment(ctx context.Context, orgID, userID, taskID, body string, orgRole tenant.OrgRole) (*project.Comment, error)
}

// Review outcomes.
var (
	errNotInReview = domain.NewCodedError(domain.ErrValidation, "not_in_review",
		"This task is not waiting for review.")
	errReviewComment = domain.NewFieldError("comment", "comment_required",
		"Say what needs to change so the assignee can pick it up.")
	errRoleInvalid = domain.NewFieldError("role", "column_role_invalid",
		"Role must be one of backlog, todo, in_progress, review, done.")
)

// isAgent reports whether userID is an agent (false when no lookup is wired).
func (s *Service) isAgent(ctx context.Context, orgID, userID string) (bool, error) {
	if s.agents == nil {
		return false, nil
	}
	return s.agents.IsAgent(ctx, orgID, userID)
}

// refuseAgentDone returns ErrHumanApprovalRequired when an agent targets the
// board's done column.
func (s *Service) refuseAgentDone(ctx context.Context, orgID, userID, boardID, columnID string) error {
	cols, err := s.columns.ListByBoard(ctx, orgID, boardID)
	if err != nil {
		return err
	}
	if project.DoneColumnID(cols) != columnID {
		return nil
	}
	agent, err := s.isAgent(ctx, orgID, userID)
	if err != nil {
		return err
	}
	if agent {
		return project.ErrHumanApprovalRequired
	}
	return nil
}

// boardColumns returns a project's board columns, rank-ordered.
func (s *Service) boardColumns(ctx context.Context, orgID, projectID string) ([]project.Column, error) {
	board, err := s.boards.GetByProject(ctx, orgID, projectID)
	if err != nil {
		return nil, err
	}
	return s.columns.ListByBoard(ctx, orgID, board.ID)
}

// MoveTaskToRole moves a task to the end of the column holding role — how
// agents move work, by meaning rather than by column name. It returns the
// target column. The same rules as MoveTask apply, including the done gate.
func (s *Service) MoveTaskToRole(ctx context.Context, orgID, userID, taskID string, role project.ColumnRole, ifVersion *int64, orgRole tenant.OrgRole) (*project.Column, error) {
	if !role.Valid() {
		return nil, errRoleInvalid
	}
	t, err := s.tasks.Get(ctx, orgID, taskID)
	if err != nil {
		return nil, err
	}
	if _, _, err := s.access(ctx, orgID, t.ProjectID, userID, orgRole, project.RoleContributor); err != nil {
		return nil, err
	}
	cols, err := s.boardColumns(ctx, orgID, t.ProjectID)
	if err != nil {
		return nil, err
	}
	col := project.ColumnForRole(cols, role)
	if col == nil {
		return nil, project.ErrNoColumnForRole
	}
	if col.ID == t.ColumnID {
		return col, nil // already there; keep its place
	}
	existing, err := s.tasks.ListByColumn(ctx, orgID, col.ID)
	if err != nil {
		return nil, err
	}
	last := ""
	if n := len(existing); n > 0 {
		last = existing[n-1].Rank
	}
	if err := s.MoveTask(ctx, orgID, userID, taskID, col.ID, rank.Append(last), ifVersion, orgRole); err != nil {
		return nil, err
	}
	return col, nil
}

// SetColumnRole gives a column a workflow role, or clears it with "" (LEAD).
// The role moves off whichever column held it.
func (s *Service) SetColumnRole(ctx context.Context, orgID, userID, projectID, columnID string, role project.ColumnRole, orgRole tenant.OrgRole) error {
	if role != "" && !role.Valid() {
		return errRoleInvalid
	}
	if err := s.requireColumn(ctx, orgID, userID, projectID, columnID, orgRole); err != nil {
		return err
	}
	col, err := s.columns.Get(ctx, orgID, columnID)
	if err != nil {
		return err
	}
	return s.columns.SetRole(ctx, orgID, col.BoardID, columnID, role)
}

// ReviewTask records a person's decision on a task waiting in review:
// approve moves it to done; send back moves it to in progress (todo when the
// board has none) with the reviewer's comment, keeping its assignee so the
// agent picks it up again. Agents cannot review (human-approved Done).
func (s *Service) ReviewTask(ctx context.Context, orgID, userID, taskID string, approve bool, comment string, orgRole tenant.OrgRole) (*project.Column, error) {
	comment = strings.TrimSpace(comment)
	if !approve && comment == "" {
		return nil, errReviewComment
	}
	if agent, err := s.isAgent(ctx, orgID, userID); err != nil {
		return nil, err
	} else if agent {
		return nil, project.ErrHumanApprovalRequired
	}
	t, err := s.tasks.Get(ctx, orgID, taskID)
	if err != nil {
		return nil, err
	}
	if _, _, err := s.access(ctx, orgID, t.ProjectID, userID, orgRole, project.RoleContributor); err != nil {
		return nil, err
	}
	cols, err := s.boardColumns(ctx, orgID, t.ProjectID)
	if err != nil {
		return nil, err
	}
	if project.EffectiveRoles(cols)[t.ColumnID] != project.ColumnReview {
		return nil, errNotInReview
	}
	if comment != "" && s.commenter == nil {
		return nil, domain.ErrForbidden // review comments unavailable
	}
	target := project.ColumnDone
	if !approve {
		target = project.ColumnInProgress
		if project.ColumnForRole(cols, target) == nil {
			target = project.ColumnTodo
		}
	}
	col, err := s.MoveTaskToRole(ctx, orgID, userID, taskID, target, nil, orgRole)
	if err != nil {
		return nil, err
	}
	if comment != "" {
		prefix := "Sent back for changes:\n\n"
		if approve {
			prefix = "Approved:\n\n"
		}
		if _, err := s.commenter.AddComment(ctx, orgID, userID, taskID, prefix+comment, orgRole); err != nil {
			return nil, err
		}
	}
	return col, nil
}

// ColumnForRole returns the project's column holding role (VIEWER), or
// project.ErrNoColumnForRole.
func (s *Service) ColumnForRole(ctx context.Context, orgID, userID, projectID string, role project.ColumnRole, orgRole tenant.OrgRole) (*project.Column, error) {
	if !role.Valid() {
		return nil, errRoleInvalid
	}
	if _, _, err := s.access(ctx, orgID, projectID, userID, orgRole, project.RoleViewer); err != nil {
		return nil, err
	}
	cols, err := s.boardColumns(ctx, orgID, projectID)
	if err != nil {
		return nil, err
	}
	col := project.ColumnForRole(cols, role)
	if col == nil {
		return nil, project.ErrNoColumnForRole
	}
	return col, nil
}
