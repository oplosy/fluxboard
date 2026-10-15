package taskuc

import (
	"context"
	"time"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/audit"
	"github.com/mesutokul/fluxboard/backend/internal/domain/notify"
	"github.com/mesutokul/fluxboard/backend/internal/domain/project"
	"github.com/mesutokul/fluxboard/backend/internal/domain/tenant"
)

// ---- Task approvals (ADR-037) -----------------------------------------------
// A contributor asks named approvers to sign off a task; each decides once.
// Reads need VIEWER, requesting CONTRIBUTOR. Deciding needs being a named
// approver (and a project contributor). Staleness is stamped by a database
// trigger, not here.

// RequestApprovalInput is the payload for RequestApproval.
type RequestApprovalInput struct {
	ApproverIDs []string
	Deadline    *time.Time
	Note        string
}

// ListApprovals returns the task's requests, newest first (VIEWER).
func (s *Service) ListApprovals(ctx context.Context, orgID, userID, taskID string, orgRole tenant.OrgRole) ([]project.Approval, error) {
	if s.approvals == nil {
		return nil, domain.ErrForbidden // approvals disabled
	}
	if _, _, err := s.taskAccess(ctx, orgID, taskID, userID, orgRole, project.RoleViewer); err != nil {
		return nil, err
	}
	return s.approvals.ListByTask(ctx, orgID, taskID)
}

// RequestApproval asks approvers to sign off a task (CONTRIBUTOR). Approvers
// must be people with at least CONTRIBUTOR on the project.
func (s *Service) RequestApproval(ctx context.Context, orgID, userID, taskID string, in RequestApprovalInput, orgRole tenant.OrgRole) (*project.Approval, error) {
	if s.approvals == nil {
		return nil, domain.ErrForbidden // approvals disabled
	}
	t, _, err := s.taskAccess(ctx, orgID, taskID, userID, orgRole, project.RoleContributor)
	if err != nil {
		return nil, err
	}
	ids, err := project.ValidateApprovalRequest(userID, in.ApproverIDs, in.Deadline, in.Note, s.now())
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		if err := s.requireApprover(ctx, orgID, t.ProjectID, id); err != nil {
			return nil, err
		}
	}
	a := &project.Approval{
		ID: newID(), OrgID: orgID, TaskID: taskID, RequestedBy: userID, Note: in.Note,
		Deadline: in.Deadline, State: project.ApprovalPending,
	}
	for _, id := range ids {
		a.Approvers = append(a.Approvers, project.Approver{UserID: id})
	}
	if err := s.approvals.Create(ctx, orgID, a); err != nil {
		return nil, err
	}
	s.auditApproval(ctx, orgID, userID, audit.ActionApprovalRequest, a.ID, map[string]any{
		"task_id": taskID, "approvers": ids,
	})
	s.notifyApproval(ctx, orgID, userID, ids, "Approval requested",
		"Your approval is requested on “"+t.Title+"”.", taskID)
	return s.approvals.Get(ctx, orgID, taskID, a.ID)
}

// requireApprover checks that id is a person with at least CONTRIBUTOR on the
// project through an explicit membership.
// shortcut: org admins without a membership row cannot be named; add them to
// the project first.
func (s *Service) requireApprover(ctx context.Context, orgID, projectID, id string) error {
	m, err := s.members.Get(ctx, orgID, projectID, id)
	if err != nil || !m.Role.AtLeast(project.RoleContributor) {
		return project.ErrApproversInvalid
	}
	if s.agents != nil {
		if isAgent, err := s.agents.IsAgent(ctx, orgID, id); err != nil {
			return err
		} else if isAgent {
			return project.ErrApproversInvalid
		}
	}
	return nil
}

// DecideApproval records the caller's decision on a request. Self-approval,
// agents and non-approvers are refused; the requester is notified.
func (s *Service) DecideApproval(ctx context.Context, orgID, userID, taskID, approvalID string, d project.ApprovalDecision, comment string, orgRole tenant.OrgRole) (*project.Approval, error) {
	if s.approvals == nil {
		return nil, domain.ErrForbidden // approvals disabled
	}
	if s.agents != nil {
		if isAgent, err := s.agents.IsAgent(ctx, orgID, userID); err != nil {
			return nil, err
		} else if isAgent {
			return nil, project.ErrHumanApprovalRequired
		}
	}
	t, _, err := s.taskAccess(ctx, orgID, taskID, userID, orgRole, project.RoleContributor)
	if err != nil {
		return nil, err
	}
	now := s.now()
	a, err := s.approvals.Update(ctx, orgID, taskID, approvalID, func(a *project.Approval) error {
		return a.Decide(userID, d, comment, now)
	})
	if err != nil {
		return nil, err
	}
	s.auditApproval(ctx, orgID, userID, audit.ActionApprovalDecide, a.ID, map[string]any{
		"task_id": taskID, "decision": string(d), "state": string(a.State),
	})
	title := "Approval decided"
	body := "Your approval request on “" + t.Title + "” is now " + string(a.State) + "."
	if d == project.DecisionChangesRequested {
		title, body = "Changes requested", "Changes were requested on “"+t.Title+"”: "+comment
	}
	s.notifyApproval(ctx, orgID, userID, []string{a.RequestedBy}, title, body, taskID)
	return a, nil
}

// CancelApproval withdraws an open request (its requester, or a project lead).
func (s *Service) CancelApproval(ctx context.Context, orgID, userID, taskID, approvalID string, orgRole tenant.OrgRole) error {
	if s.approvals == nil {
		return domain.ErrForbidden // approvals disabled
	}
	_, role, err := s.taskAccess(ctx, orgID, taskID, userID, orgRole, project.RoleContributor)
	if err != nil {
		return err
	}
	now := s.now()
	a, err := s.approvals.Update(ctx, orgID, taskID, approvalID, func(a *project.Approval) error {
		return a.Cancel(userID, role.AtLeast(project.RoleLead), now)
	})
	if err != nil {
		return err
	}
	s.auditApproval(ctx, orgID, userID, audit.ActionApprovalCancel, a.ID, map[string]any{"task_id": taskID})
	return nil
}

// ListWaitingApprovals returns the open requests the caller has yet to answer.
func (s *Service) ListWaitingApprovals(ctx context.Context, orgID, userID string) ([]project.WaitingApproval, error) {
	if s.approvals == nil {
		return nil, domain.ErrForbidden // approvals disabled
	}
	return s.approvals.ListWaiting(ctx, orgID, userID)
}

// auditApproval writes a best-effort audit row.
func (s *Service) auditApproval(ctx context.Context, orgID, actor string, action audit.Action, approvalID string, meta map[string]any) {
	if s.audit == nil {
		return
	}
	if err := s.audit.Append(ctx, audit.Entry{
		OrgID: orgID, ActorUserID: actor, Action: action,
		TargetType: "task_approval", TargetID: approvalID, Metadata: meta, Severity: audit.SeverityInfo,
	}); err != nil {
		s.logger.Warn("approval audit append failed", "action", action, "err", err)
	}
}

func (s *Service) notifyApproval(ctx context.Context, orgID, actor string, to []string, title, body, taskID string) {
	if s.notifier == nil {
		return
	}
	s.notifier.NotifyTargets(ctx, orgID, actor, notify.CategoryApproval, to, title, body, "task", taskID)
}
