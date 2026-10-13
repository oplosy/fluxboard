package taskuc

import (
	"context"
	"errors"
	"time"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/notify"
	"github.com/mesutokul/fluxboard/backend/internal/domain/project"
	"github.com/mesutokul/fluxboard/backend/internal/domain/tenant"
)

// Agent claims and version-checked writes (ADR-033 decisions 3–4,
// FR-AGT-004/005).

// errNoConcurrency refuses a versioned write or claim when the service was
// built without a ConcurrencyRepository (never the case in cmd/api).
var errNoConcurrency = domain.NewCodedError(domain.ErrValidation, "unsupported",
	"Versioned writes and claims are not available.")

// writeTask saves t, guarded by ifVersion when given. A stale version comes
// back as *project.VersionConflict carrying the current task.
func (s *Service) writeTask(ctx context.Context, orgID string, t *project.Task, ifVersion *int64) error {
	if ifVersion == nil {
		return s.tasks.Update(ctx, orgID, t)
	}
	if s.conc == nil {
		return errNoConcurrency
	}
	return s.versionConflict(ctx, orgID, t.ID, s.conc.UpdateIfVersion(ctx, orgID, t, *ifVersion))
}

// versionConflict turns ErrVersionConflict into *VersionConflict with the
// task as it is now; any other error passes through.
func (s *Service) versionConflict(ctx context.Context, orgID, taskID string, err error) error {
	if !errors.Is(err, project.ErrVersionConflict) {
		return err
	}
	cur, gerr := s.tasks.Get(ctx, orgID, taskID)
	if gerr != nil {
		return gerr
	}
	return &project.VersionConflict{Current: cur}
}

// leaseFor validates a requested lease (0 ⇒ the default).
func leaseFor(d time.Duration) (time.Duration, error) {
	if d == 0 {
		return project.DefaultLease, nil
	}
	if d < project.MinLease || d > project.MaxLease {
		return 0, domain.NewFieldError("lease_minutes", "lease_invalid",
			"A lease must be between 1 and 240 minutes.")
	}
	return d, nil
}

// ClaimTask claims a task for the calling agent and assigns it, or renews the
// agent's own claim. Another agent's live claim, or an assignee who is neither
// this agent nor a lapsed claimant, is *project.ClaimConflict naming the
// holder. Needs CONTRIBUTOR on the project (a claim edits the assignee).
func (s *Service) ClaimTask(ctx context.Context, orgID, agentID, taskID string, lease time.Duration, orgRole tenant.OrgRole) (*project.Task, error) {
	if s.conc == nil {
		return nil, errNoConcurrency
	}
	lease, err := leaseFor(lease)
	if err != nil {
		return nil, err
	}
	before, _, err := s.taskAccess(ctx, orgID, taskID, agentID, orgRole, project.RoleContributor)
	if err != nil {
		return nil, err
	}
	now := s.now().UTC()
	ok, err := s.conc.Claim(ctx, orgID, taskID, agentID, now.Add(lease))
	if err != nil {
		return nil, err
	}
	if !ok {
		cur, err := s.tasks.Get(ctx, orgID, taskID)
		if err != nil {
			return nil, err
		}
		return nil, claimConflict(cur, now)
	}
	if before.AssigneeID == nil || *before.AssigneeID != agentID {
		s.logAssignee(ctx, orgID, agentID, before, &agentID)
	}
	return s.tasks.Get(ctx, orgID, taskID)
}

// HeartbeatTask extends the calling agent's live claim. A lapsed or foreign
// claim is project.ErrClaimLost: the agent must claim again.
func (s *Service) HeartbeatTask(ctx context.Context, orgID, agentID, taskID string, lease time.Duration, orgRole tenant.OrgRole) (*project.Task, error) {
	if s.conc == nil {
		return nil, errNoConcurrency
	}
	lease, err := leaseFor(lease)
	if err != nil {
		return nil, err
	}
	if _, _, err := s.taskAccess(ctx, orgID, taskID, agentID, orgRole, project.RoleContributor); err != nil {
		return nil, err
	}
	ok, err := s.conc.Heartbeat(ctx, orgID, taskID, agentID, s.now().UTC().Add(lease))
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, project.ErrClaimLost
	}
	return s.tasks.Get(ctx, orgID, taskID)
}

// ReleaseTask gives the calling agent's claim back and unassigns the task if
// the agent is still its assignee.
func (s *Service) ReleaseTask(ctx context.Context, orgID, agentID, taskID string, orgRole tenant.OrgRole) (*project.Task, error) {
	if s.conc == nil {
		return nil, errNoConcurrency
	}
	before, _, err := s.taskAccess(ctx, orgID, taskID, agentID, orgRole, project.RoleContributor)
	if err != nil {
		return nil, err
	}
	ok, err := s.conc.Release(ctx, orgID, taskID, agentID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, project.ErrClaimLost
	}
	if before.AssigneeID != nil && *before.AssigneeID == agentID {
		s.logAssignee(ctx, orgID, agentID, before, nil)
	}
	return s.tasks.Get(ctx, orgID, taskID)
}

// ReleaseExpiredClaims clears an org's lapsed claims (sweeper job).
func (s *Service) ReleaseExpiredClaims(ctx context.Context, orgID string) (int, error) {
	if s.conc == nil {
		return 0, nil
	}
	return s.conc.ReleaseExpired(ctx, orgID)
}

// claimConflict names who holds t: a live claimant, else the assignee.
func claimConflict(t *project.Task, now time.Time) error {
	if t.ClaimActive(now) {
		return &project.ClaimConflict{HolderID: *t.ClaimedBy, LeaseExpiresAt: t.LeaseExpiresAt}
	}
	if t.AssigneeID != nil {
		return &project.ClaimConflict{HolderID: *t.AssigneeID}
	}
	// Changed between the claim and this read: report it as taken, not lost.
	return project.ErrClaimHeld
}

// logAssignee records an assignee change made by a claim or release in the
// activity log and on the realtime stream (best-effort, like UpdateTask).
func (s *Service) logAssignee(ctx context.Context, orgID, actorID string, before *project.Task, to *string) {
	a := &project.Activity{
		ID: newID(), TaskID: before.ID, ActorID: actorID, Field: "assignee",
		OldValue: before.AssigneeID, NewValue: to,
	}
	if err := s.activity.Append(ctx, orgID, a); err != nil {
		s.logger.Warn("activity append failed", "task", before.ID, "field", "assignee", "err", err)
	}
	s.publish(ctx, orgID, notify.EventTaskUpdated, actorID, map[string]any{
		"task_id": before.ID, "project_id": before.ProjectID, "fields": []string{"assignee"},
	})
}
