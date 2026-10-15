package project

import (
	"context"
	"strings"
	"time"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
)

// Task approvals (ADR-037). A request names approvers and an optional
// deadline; each approver decides once. This is a sign-off record on a task,
// separate from the column-based review gate (ADR-033), which moves work.

// ApprovalState is the request's overall state, derived from its approvers.
type ApprovalState string

const (
	ApprovalPending          ApprovalState = "pending"
	ApprovalApproved         ApprovalState = "approved"
	ApprovalChangesRequested ApprovalState = "changes_requested"
	ApprovalCancelled        ApprovalState = "cancelled"
)

// ApprovalDecision is one approver's answer.
type ApprovalDecision string

const (
	DecisionApproved         ApprovalDecision = "approved"
	DecisionChangesRequested ApprovalDecision = "changes_requested"
)

// Valid reports whether d is a known decision.
func (d ApprovalDecision) Valid() bool {
	return d == DecisionApproved || d == DecisionChangesRequested
}

// MaxApprovers bounds one request's approver list.
const MaxApprovers = 10

// MaxApprovalNoteLen bounds the request note and a decision comment.
const MaxApprovalNoteLen = 2000

// Approval outcomes.
var (
	ErrSelfApproval = domain.NewCodedError(domain.ErrForbidden, "self_approval",
		"You cannot approve your own request.")
	ErrNotApprover = domain.NewCodedError(domain.ErrForbidden, "not_approver",
		"You are not an approver on this request.")
	ErrApprovalNotPending = domain.NewCodedError(domain.ErrConflict, "approval_not_pending",
		"This approval request is no longer open.")
	ErrApprovalOpen = domain.NewCodedError(domain.ErrConflict, "approval_open",
		"This task already has an open approval request.")
	ErrAlreadyDecided = domain.NewCodedError(domain.ErrConflict, "already_decided",
		"You already answered this request.")
	ErrApprovalComment = domain.NewFieldError("comment", "comment_required",
		"Say what needs to change.")
	ErrDecisionInvalid = domain.NewFieldError("decision", "decision_invalid",
		"Decision must be approved or changes_requested.")
	ErrApproversInvalid = domain.NewFieldError("approver_ids", "approvers_invalid",
		"Choose between 1 and 10 approvers, other than yourself.")
	ErrDeadlinePast = domain.NewFieldError("deadline", "deadline_past",
		"The deadline must be in the future.")
)

// Approver is one person asked to decide.
type Approver struct {
	UserID    string
	Decision  ApprovalDecision // "" while waiting
	Comment   string
	DecidedAt *time.Time
}

// Approval is one request on a task.
type Approval struct {
	ID          string
	OrgID       string
	TaskID      string
	RequestedBy string
	Note        string
	Deadline    *time.Time
	State       ApprovalState
	// StaleAt is set when the task was reopened or edited after completion
	// while this request stood approved.
	StaleAt   *time.Time
	DecidedAt *time.Time
	CreatedAt time.Time
	Approvers []Approver
}

// Stale reports an approved request that no longer vouches for the task.
func (a *Approval) Stale() bool { return a.State == ApprovalApproved && a.StaleAt != nil }

// ValidateApprovalRequest checks a new request's approvers (de-duplicated,
// returned in order) and deadline.
func ValidateApprovalRequest(requestedBy string, approvers []string, deadline *time.Time, note string, now time.Time) ([]string, error) {
	seen := make(map[string]bool, len(approvers))
	out := make([]string, 0, len(approvers))
	for _, id := range approvers {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		if id == requestedBy {
			return nil, ErrSelfApproval
		}
		seen[id] = true
		out = append(out, id)
	}
	if len(out) == 0 || len(out) > MaxApprovers {
		return nil, ErrApproversInvalid
	}
	if len(note) > MaxApprovalNoteLen {
		return nil, domain.NewFieldError("note", "note_too_long", "The note is too long.")
	}
	if deadline != nil && !deadline.After(now) {
		return nil, ErrDeadlinePast
	}
	return out, nil
}

// Decide records userID's decision and re-derives the request's state: any
// changes_requested wins, else all approved means approved, else pending.
func (a *Approval) Decide(userID string, d ApprovalDecision, comment string, now time.Time) error {
	comment = strings.TrimSpace(comment)
	if !d.Valid() {
		return ErrDecisionInvalid
	}
	if d == DecisionChangesRequested && comment == "" {
		return ErrApprovalComment
	}
	if len(comment) > MaxApprovalNoteLen {
		return domain.NewFieldError("comment", "comment_too_long", "The comment is too long.")
	}
	if userID == a.RequestedBy {
		return ErrSelfApproval
	}
	if a.State != ApprovalPending {
		return ErrApprovalNotPending
	}
	var me *Approver
	for i := range a.Approvers {
		if a.Approvers[i].UserID == userID {
			me = &a.Approvers[i]
		}
	}
	if me == nil {
		return ErrNotApprover
	}
	if me.Decision != "" {
		return ErrAlreadyDecided
	}
	me.Decision, me.Comment, me.DecidedAt = d, comment, &now
	a.State = a.derive()
	if a.State != ApprovalPending {
		a.DecidedAt = &now
	}
	return nil
}

func (a *Approval) derive() ApprovalState {
	waiting := false
	for _, p := range a.Approvers {
		switch p.Decision {
		case DecisionChangesRequested:
			return ApprovalChangesRequested
		case "":
			waiting = true
		}
	}
	if waiting {
		return ApprovalPending
	}
	return ApprovalApproved
}

// Cancel withdraws an open request. Only its requester (or a project lead,
// passed as force) may.
func (a *Approval) Cancel(userID string, force bool, now time.Time) error {
	if a.State != ApprovalPending {
		return ErrApprovalNotPending
	}
	if userID != a.RequestedBy && !force {
		return domain.ErrForbidden
	}
	a.State = ApprovalCancelled
	a.DecidedAt = &now
	return nil
}

// WaitingApproval is an open request awaiting a given person, with the task
// facts a "waiting for my approval" list needs.
type WaitingApproval struct {
	ApprovalID  string
	TaskID      string
	ProjectID   string
	TaskNumber  int
	TaskTitle   string
	RequestedBy string
	Deadline    *time.Time
	CreatedAt   time.Time
}

// ApprovalRepository persists approvals ([T], RLS).
type ApprovalRepository interface {
	// Create inserts the request with its approvers (all waiting).
	// ErrApprovalOpen when the task already has a pending request.
	Create(ctx context.Context, orgID string, a *Approval) error
	// Get returns one request of a task with its approvers, or ErrNotFound.
	Get(ctx context.Context, orgID, taskID, approvalID string) (*Approval, error)
	// ListByTask returns the task's requests, newest first.
	ListByTask(ctx context.Context, orgID, taskID string) ([]Approval, error)
	// Update loads a request under a row lock, runs fn on it, and persists the
	// state and decisions fn left, in one transaction. Concurrent decisions on
	// one request serialize, so the derived state never misses one.
	Update(ctx context.Context, orgID, taskID, approvalID string, fn func(*Approval) error) (*Approval, error)
	// ListWaiting returns open requests userID has not answered yet, on live
	// tasks, nearest deadline first.
	ListWaiting(ctx context.Context, orgID, userID string) ([]WaitingApproval, error)
}
