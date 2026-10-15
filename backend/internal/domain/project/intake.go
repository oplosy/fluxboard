package project

import (
	"context"
	"time"
)

// IntakeStatus is a request's triage state. new -> accepted (a task exists) ->
// done (that task completed), or new -> declined.
type IntakeStatus string

const (
	IntakeNew      IntakeStatus = "new"
	IntakeAccepted IntakeStatus = "accepted"
	IntakeDeclined IntakeStatus = "declined"
	IntakeDone     IntakeStatus = "done"
)

// Valid reports whether s is a known status.
func (s IntakeStatus) Valid() bool {
	switch s {
	case IntakeNew, IntakeAccepted, IntakeDeclined, IntakeDone:
		return true
	}
	return false
}

// IntakeRequest is one public-form submission awaiting or past triage. The
// submitter is plain text on the request — never a user row, never an actor.
type IntakeRequest struct {
	ID                string
	OrgID             string
	ProjectID         string
	FormID            string // "" once the form was deleted
	FormSchemaVersion int
	FormSnapshot      []byte // JSON {name, description, target_column_id} as the submitter saw it
	Title             string
	Description       string
	Priority          Priority
	SubmitterName     string
	SubmitterEmail    string
	ContentHash       []byte
	Status            IntakeStatus
	OwnerID           string
	DeclineReason     string
	TaskID            string
	DecidedBy         string
	DecidedAt         *time.Time
	ReceivedAt        time.Time
	AckSentAt         *time.Time
	OutcomeSentAt     *time.Time
}

// IntakeMail is a request that owes its submitter a mail.
type IntakeMail struct {
	ID             string
	Title          string
	SubmitterName  string
	SubmitterEmail string
	FormName       string
	Status         IntakeStatus
	DeclineReason  string
	AckDue         bool // acknowledgment not yet sent
	OutcomeDue     bool // declined/done and the outcome mail not yet sent
}

// IntakeRepository persists requests (intake_requests, RLS-scoped).
type IntakeRepository interface {
	// Submit inserts r unless the same form + email + content arrived within
	// dedupeWindow (the existing id is returned, created=false) or the form took
	// rateMax requests within rateWindow (domain.ErrRateLimited). One tx.
	Submit(ctx context.Context, orgID string, r *IntakeRequest, dedupeWindow time.Duration, rateMax int, rateWindow time.Duration) (id string, created bool, err error)
	Get(ctx context.Context, orgID, id string) (*IntakeRequest, error)
	// List returns a project's requests newest-first; status "" = all.
	List(ctx context.Context, orgID, projectID string, status IntakeStatus, limit int) ([]IntakeRequest, error)
	// Triage sets owner ("" clears) and priority while the request is new;
	// domain.ErrConflict otherwise.
	Triage(ctx context.Context, orgID, id, ownerID string, priority Priority) error
	// Accept links taskID and flips new -> accepted; ErrConflict if no longer new.
	Accept(ctx context.Context, orgID, id, taskID, by string) error
	// Decline flips new -> declined; ErrConflict if no longer new.
	Decline(ctx context.Context, orgID, id, reason, by string) error
	// CompleteDone flips accepted -> done for requests whose task completed.
	CompleteDone(ctx context.Context, orgID string) (int, error)
	// MailDue lists requests owing a mail (submitters with an address only).
	MailDue(ctx context.Context, orgID string, limit int) ([]IntakeMail, error)
	MarkAcked(ctx context.Context, orgID, id string) error
	MarkOutcomeSent(ctx context.Context, orgID, id string) error
}
