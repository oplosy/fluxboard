package projectuc

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/audit"
	"github.com/mesutokul/fluxboard/backend/internal/domain/automation"
	"github.com/mesutokul/fluxboard/backend/internal/domain/notify"
	"github.com/mesutokul/fluxboard/backend/internal/domain/project"
	"github.com/mesutokul/fluxboard/backend/internal/domain/tenant"
	"github.com/mesutokul/fluxboard/backend/internal/pkg/rank"
)

// ---- Request inbox (research report §6.3/§7) ---------------------------------
// A public form submission is a request: the submitter's name/email stay plain
// text on the row (never a user, never an audit actor); an org user triages it.
// Accepting creates the task, authored by the accepting user. The submitter only
// ever sees the mails below — they carry the request's own title, the form name
// and the decline reason typed for them, never task comments or activity.

// Abuse limits: an identical resubmission inside IntakeDedupeWindow collapses
// to the first request; a form takes at most IntakeRatePerForm new requests per
// IntakeRateWindow (on top of the per-IP throttle at the route).
const (
	IntakeDedupeWindow = 24 * time.Hour
	IntakeRatePerForm  = 30
	IntakeRateWindow   = 10 * time.Minute

	// MaxDeclineReasonLen bounds the reason mailed to the submitter.
	MaxDeclineReasonLen = 500
	intakeListLimit     = 200
	intakeMailBatch     = 100
)

// IntakeMailer delivers one plain submitter mail (implemented by mailer.Mailer).
type IntakeMailer interface {
	SendIntakeMail(ctx context.Context, to, subject, body string) error
}

// TriageRequestInput edits an open request.
type TriageRequestInput struct {
	OwnerID  string // "" clears
	Priority string
}

func (s *Service) intakeEnabled() bool { return s.intake != nil }

// ListRequests returns a project's requests newest-first (CONTRIBUTOR — they
// carry submitter contact details). status "" = all.
func (s *Service) ListRequests(ctx context.Context, orgID, userID, projectID, status string, orgRole tenant.OrgRole) ([]project.IntakeRequest, error) {
	if !s.intakeEnabled() {
		return nil, domain.ErrForbidden
	}
	if _, _, err := s.access(ctx, orgID, projectID, userID, orgRole, project.RoleContributor); err != nil {
		return nil, err
	}
	st := project.IntakeStatus(status)
	if status != "" && !st.Valid() {
		return nil, domain.ErrValidation
	}
	return s.intake.List(ctx, orgID, projectID, st, intakeListLimit)
}

func (s *Service) requestInProject(ctx context.Context, orgID, projectID, id string) (*project.IntakeRequest, error) {
	r, err := s.intake.Get(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	if r.ProjectID != projectID {
		return nil, domain.ErrNotFound
	}
	return r, nil
}

// TriageRequest sets owner/priority on a new request (CONTRIBUTOR).
func (s *Service) TriageRequest(ctx context.Context, orgID, userID, projectID, id string, in TriageRequestInput, orgRole tenant.OrgRole) (*project.IntakeRequest, error) {
	if !s.intakeEnabled() {
		return nil, domain.ErrForbidden
	}
	if _, _, err := s.access(ctx, orgID, projectID, userID, orgRole, project.RoleContributor); err != nil {
		return nil, err
	}
	pr := project.Priority(strings.TrimSpace(in.Priority))
	if !pr.Valid() {
		return nil, domain.ErrValidation
	}
	if in.OwnerID != "" {
		if _, err := s.members.Get(ctx, orgID, projectID, in.OwnerID); err != nil {
			return nil, domain.ErrValidation // owner must be a project member
		}
	}
	if _, err := s.requestInProject(ctx, orgID, projectID, id); err != nil {
		return nil, err
	}
	if err := s.intake.Triage(ctx, orgID, id, in.OwnerID, pr); err != nil {
		return nil, err
	}
	return s.intake.Get(ctx, orgID, id)
}

// AcceptRequest turns a new request into a task in the form's target column
// (CONTRIBUTOR) and links the two. Returns the task.
func (s *Service) AcceptRequest(ctx context.Context, orgID, userID, projectID, id string, orgRole tenant.OrgRole) (*project.Task, error) {
	if !s.intakeEnabled() {
		return nil, domain.ErrForbidden
	}
	if _, _, err := s.access(ctx, orgID, projectID, userID, orgRole, project.RoleContributor); err != nil {
		return nil, err
	}
	r, err := s.requestInProject(ctx, orgID, projectID, id)
	if err != nil {
		return nil, err
	}
	if r.Status != project.IntakeNew {
		return nil, domain.ErrConflict
	}
	var snap struct {
		TargetColumnID string `json:"target_column_id"`
	}
	if err := json.Unmarshal(r.FormSnapshot, &snap); err != nil || snap.TargetColumnID == "" {
		return nil, domain.ErrValidation
	}
	if err := s.checkTargetColumn(ctx, orgID, projectID, snap.TargetColumnID); err != nil {
		return nil, err
	}
	existing, err := s.tasks.ListByColumn(ctx, orgID, snap.TargetColumnID)
	if err != nil {
		return nil, err
	}
	last := ""
	if n := len(existing); n > 0 {
		last = existing[n-1].Rank
	}
	t := &project.Task{
		ID: newID(), OrgID: orgID, ProjectID: projectID, ColumnID: snap.TargetColumnID,
		Title: r.Title, Description: intakeBody(r.SubmitterName, r.SubmitterEmail, r.Description),
		Priority: r.Priority, Rank: rank.Append(last), CreatedBy: userID,
	}
	if r.OwnerID != "" {
		t.AssigneeID = &r.OwnerID
	}
	if err := s.tasks.Create(ctx, orgID, t); err != nil {
		return nil, err
	}
	if err := s.intake.Accept(ctx, orgID, id, t.ID, userID); err != nil {
		// Lost a race to another accept/decline: don't leave a stray task.
		if derr := s.tasks.SoftDelete(ctx, orgID, t.ID, s.now()); derr != nil {
			s.logger.Warn("intake accept: rollback task failed", "task", t.ID, "err", derr)
		}
		return nil, err
	}
	s.publish(ctx, orgID, notify.EventTaskCreated, userID, map[string]any{
		"task_id": t.ID, "project_id": t.ProjectID, "column_id": t.ColumnID, "title": t.Title,
	})
	s.automate(ctx, automation.Event{OrgID: orgID, Trigger: automation.TriggerTaskCreated, TaskID: t.ID, ActorID: userID})
	s.auditIntake(ctx, orgID, userID, audit.ActionIntakeAccept, id, map[string]any{"task_id": t.ID})
	return s.tasks.Get(ctx, orgID, t.ID)
}

// DeclineRequest closes a new request with a reason that is mailed to the
// submitter (CONTRIBUTOR).
func (s *Service) DeclineRequest(ctx context.Context, orgID, userID, projectID, id, reason string, orgRole tenant.OrgRole) (*project.IntakeRequest, error) {
	if !s.intakeEnabled() {
		return nil, domain.ErrForbidden
	}
	if _, _, err := s.access(ctx, orgID, projectID, userID, orgRole, project.RoleContributor); err != nil {
		return nil, err
	}
	reason = strings.TrimSpace(reason)
	if reason == "" || len(reason) > MaxDeclineReasonLen {
		return nil, domain.ErrValidation
	}
	if _, err := s.requestInProject(ctx, orgID, projectID, id); err != nil {
		return nil, err
	}
	if err := s.intake.Decline(ctx, orgID, id, reason, userID); err != nil {
		return nil, err
	}
	s.auditIntake(ctx, orgID, userID, audit.ActionIntakeDecline, id, nil)
	return s.intake.Get(ctx, orgID, id)
}

// checkTargetColumn verifies the form's column still exists on the project's
// board; a gone column means the owner must re-point the form.
func (s *Service) checkTargetColumn(ctx context.Context, orgID, projectID, columnID string) error {
	col, err := s.columns.Get(ctx, orgID, columnID)
	if err != nil {
		return domain.ErrValidation
	}
	board, err := s.boards.GetByProject(ctx, orgID, projectID)
	if err != nil || col.BoardID != board.ID {
		return domain.ErrValidation
	}
	return nil
}

// auditIntake records an org user's triage decision (best-effort). The actor is
// always the deciding org user — a submitter has no identity to record.
func (s *Service) auditIntake(ctx context.Context, orgID, actor string, action audit.Action, requestID string, meta map[string]any) {
	if s.audit == nil {
		return
	}
	if err := s.audit.Append(ctx, audit.Entry{
		OrgID: orgID, ActorUserID: actor, Action: action,
		TargetType: "intake_request", TargetID: requestID, Metadata: meta, Severity: audit.SeverityInfo,
	}); err != nil {
		s.logger.Warn("intake audit append failed", "action", action, "err", err)
	}
}

// SendIntakeMails runs one sweep for an org (worker, every minute): flips
// accepted requests whose task completed to done, then sends the owed
// acknowledgment / completion / decline mails and marks each as sent. Returns
// the number of mails sent. A failed send is logged and retried next sweep.
// shortcut: an address that always fails is retried every sweep; add an
// attempt counter if bounces become common.
func (s *Service) SendIntakeMails(ctx context.Context, orgID string) (int, error) {
	if !s.intakeEnabled() {
		return 0, nil
	}
	if _, err := s.intake.CompleteDone(ctx, orgID); err != nil {
		return 0, err
	}
	if s.intakeMail == nil {
		return 0, nil
	}
	due, err := s.intake.MailDue(ctx, orgID, intakeMailBatch)
	if err != nil {
		return 0, err
	}
	sent := 0
	for _, m := range due {
		if m.AckDue {
			subject, body := ackMail(m)
			if err := s.intakeMail.SendIntakeMail(ctx, m.SubmitterEmail, subject, body); err != nil {
				s.logger.Warn("intake ack mail failed", "request", m.ID, "err", err)
				continue
			}
			if err := s.intake.MarkAcked(ctx, orgID, m.ID); err != nil {
				return sent, err
			}
			sent++
		}
		if m.OutcomeDue {
			subject, body := outcomeMail(m)
			if err := s.intakeMail.SendIntakeMail(ctx, m.SubmitterEmail, subject, body); err != nil {
				s.logger.Warn("intake outcome mail failed", "request", m.ID, "err", err)
				continue
			}
			if err := s.intake.MarkOutcomeSent(ctx, orgID, m.ID); err != nil {
				return sent, err
			}
			sent++
		}
	}
	return sent, nil
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func greeting(name string) string {
	if n := oneLine(name); n != "" {
		return "Hi " + n + ",\n\n"
	}
	return "Hello,\n\n"
}

func ackMail(m project.IntakeMail) (subject, body string) {
	form := oneLine(m.FormName)
	if form == "" {
		form = "our form"
	}
	return "We received your request: " + oneLine(m.Title),
		greeting(m.SubmitterName) + "Thanks - we received your request \"" + oneLine(m.Title) + "\" via " + form +
			".\nWe'll email you again when it has been completed."
}

func outcomeMail(m project.IntakeMail) (subject, body string) {
	title := oneLine(m.Title)
	if m.Status == project.IntakeDeclined {
		return "Update on your request: " + title,
			greeting(m.SubmitterName) + "Your request \"" + title + "\" was declined.\n\nReason: " + m.DeclineReason
	}
	return "Your request is done: " + title,
		greeting(m.SubmitterName) + "Your request \"" + title + "\" has been completed."
}
