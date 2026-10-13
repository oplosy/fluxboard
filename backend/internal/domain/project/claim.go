package project

import (
	"context"
	"time"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
)

// Agent claims and optimistic versions on tasks (ADR-033 decisions 3–4,
// FR-AGT-004/005).

// DefaultLease is how long a claim lasts without a heartbeat.
const DefaultLease = 30 * time.Minute

// MinLease and MaxLease bound a requested lease.
const (
	MinLease = time.Minute
	MaxLease = 4 * time.Hour
)

// Stable error codes for claims and versions.
var (
	// ErrVersionConflict: the task changed since the caller read it.
	ErrVersionConflict = domain.NewCodedError(domain.ErrConflict, "version_conflict",
		"This task changed since you read it. Re-read it and retry.")
	// ErrClaimHeld: another agent holds a live claim, or a person is assigned.
	ErrClaimHeld = domain.NewCodedError(domain.ErrConflict, "task_claimed",
		"Someone else is working on this task.")
	// ErrClaimLost: the caller does not hold a live claim on the task (it lapsed,
	// was released, or was never taken).
	ErrClaimLost = domain.NewCodedError(domain.ErrConflict, "claim_lost",
		"You no longer hold this task. Claim it again before continuing.")
)

// ClaimActive reports whether t is claimed with a lease still running at now.
func (t *Task) ClaimActive(now time.Time) bool {
	return t.ClaimedBy != nil && t.LeaseExpiresAt != nil && t.LeaseExpiresAt.After(now)
}

// VersionConflict carries the current task with ErrVersionConflict so the
// caller can show it or retry without another read.
type VersionConflict struct{ Current *Task }

func (e *VersionConflict) Error() string { return ErrVersionConflict.Error() }
func (e *VersionConflict) Unwrap() error { return ErrVersionConflict }

// ClaimConflict carries who holds the task with ErrClaimHeld.
type ClaimConflict struct {
	HolderID       string     // the claiming agent, or the assignee
	LeaseExpiresAt *time.Time // set when the holder is a live claim
}

func (e *ClaimConflict) Error() string { return ErrClaimHeld.Error() }
func (e *ClaimConflict) Unwrap() error { return ErrClaimHeld }

// ConcurrencyRepository holds the conditional task writes: version-checked
// update/move and the claim lifecycle. Every method is a single conditional
// statement, so concurrent callers cannot both succeed.
type ConcurrencyRepository interface {
	// UpdateIfVersion is TaskRepository.Update guarded by the task's version:
	// ErrVersionConflict when it moved on, domain.ErrNotFound when absent.
	UpdateIfVersion(ctx context.Context, orgID string, t *Task, version int64) error
	// MoveIfVersion is TaskRepository.Move guarded the same way.
	MoveIfVersion(ctx context.Context, orgID, id, columnID, rank string, version int64) error
	// Claim claims (or renews) a task for agentID until expiresAt and assigns it.
	// It reports false when the task is held by someone else.
	Claim(ctx context.Context, orgID, id, agentID string, expiresAt time.Time) (bool, error)
	// Heartbeat extends a live claim agentID holds; false when it holds none.
	Heartbeat(ctx context.Context, orgID, id, agentID string, expiresAt time.Time) (bool, error)
	// Release drops agentID's claim (and its assignment); false when it holds none.
	Release(ctx context.Context, orgID, id, agentID string) (bool, error)
	// ReleaseExpired clears every lapsed claim in the org; returns the count.
	ReleaseExpired(ctx context.Context, orgID string) (int, error)
}
