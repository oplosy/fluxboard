package projectuc

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/audit"
	"github.com/mesutokul/fluxboard/backend/internal/domain/project"
	"github.com/mesutokul/fluxboard/backend/internal/domain/tenant"
	"github.com/mesutokul/fluxboard/backend/internal/pkg/token"
)

// fakeIntake is an in-memory project.IntakeRepository. Dedupe/rate-limit SQL is
// covered by the Postgres integration test; here it only stores what it is given.
type fakeIntake struct {
	rows map[string]*project.IntakeRequest
	done []string // ids CompleteDone should flip
}

func newFakeIntake() *fakeIntake { return &fakeIntake{rows: map[string]*project.IntakeRequest{}} }

func (f *fakeIntake) Submit(_ context.Context, orgID string, r *project.IntakeRequest, _ time.Duration, _ int, _ time.Duration) (string, bool, error) {
	cp := *r
	cp.OrgID, cp.Status, cp.ReceivedAt = orgID, project.IntakeNew, time.Now()
	f.rows[r.ID] = &cp
	return r.ID, true, nil
}

func (f *fakeIntake) Get(_ context.Context, _, id string) (*project.IntakeRequest, error) {
	r, ok := f.rows[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	cp := *r
	return &cp, nil
}

func (f *fakeIntake) List(_ context.Context, _, projectID string, _ project.IntakeStatus, _ int) ([]project.IntakeRequest, error) {
	var out []project.IntakeRequest
	for _, r := range f.rows {
		if r.ProjectID == projectID {
			out = append(out, *r)
		}
	}
	return out, nil
}

func (f *fakeIntake) Triage(_ context.Context, _, id, owner string, p project.Priority) error {
	r := f.rows[id]
	if r.Status != project.IntakeNew {
		return domain.ErrConflict
	}
	r.OwnerID, r.Priority = owner, p
	return nil
}

func (f *fakeIntake) Accept(_ context.Context, _, id, taskID, by string) error {
	r := f.rows[id]
	if r.Status != project.IntakeNew {
		return domain.ErrConflict
	}
	r.Status, r.TaskID, r.DecidedBy = project.IntakeAccepted, taskID, by
	return nil
}

func (f *fakeIntake) Decline(_ context.Context, _, id, reason, by string) error {
	r := f.rows[id]
	if r.Status != project.IntakeNew {
		return domain.ErrConflict
	}
	r.Status, r.DeclineReason, r.DecidedBy = project.IntakeDeclined, reason, by
	return nil
}

func (f *fakeIntake) CompleteDone(_ context.Context, _ string) (int, error) {
	for _, id := range f.done {
		f.rows[id].Status = project.IntakeDone
	}
	n := len(f.done)
	f.done = nil
	return n, nil
}

func (f *fakeIntake) MailDue(_ context.Context, _ string, _ int) ([]project.IntakeMail, error) {
	var out []project.IntakeMail
	for _, r := range f.rows {
		if r.SubmitterEmail == "" {
			continue
		}
		ack := r.AckSentAt == nil
		outc := r.OutcomeSentAt == nil && (r.Status == project.IntakeDeclined || r.Status == project.IntakeDone)
		if ack || outc {
			out = append(out, project.IntakeMail{
				ID: r.ID, Title: r.Title, SubmitterName: r.SubmitterName, SubmitterEmail: r.SubmitterEmail,
				FormName: "Bugs", Status: r.Status, DeclineReason: r.DeclineReason, AckDue: ack, OutcomeDue: outc,
			})
		}
	}
	return out, nil
}

func (f *fakeIntake) MarkAcked(_ context.Context, _, id string) error {
	now := time.Now()
	f.rows[id].AckSentAt = &now
	return nil
}

func (f *fakeIntake) MarkOutcomeSent(_ context.Context, _, id string) error {
	now := time.Now()
	f.rows[id].OutcomeSentAt = &now
	return nil
}

type fakeMail struct{ subjects, bodies, to []string }

func (m *fakeMail) SendIntakeMail(_ context.Context, to, subject, body string) error {
	m.to, m.subjects, m.bodies = append(m.to, to), append(m.subjects, subject), append(m.bodies, body)
	return nil
}

type auditSink struct{ entries []audit.Entry }

func (a *auditSink) Append(_ context.Context, e audit.Entry) error {
	a.entries = append(a.entries, e)
	return nil
}

func intakeService(raw string) (*Service, *fakeIntake, *fakeFormTasks, *fakeMail, *auditSink) {
	forms := newFakeForms(&project.ProjectForm{
		ID: "f1", OrgID: "o", ProjectID: "p1", Name: "Bugs", Description: "Report a bug",
		TargetColumnID: "c1", TokenHash: token.Hash(raw), CreatedBy: "form-owner", IsActive: true, SchemaVersion: 3,
	})
	tasks, intake, mail, aud := &fakeFormTasks{}, newFakeIntake(), &fakeMail{}, &auditSink{}
	lead := project.RoleLead
	s := New(Deps{
		Projects: &fakeProjects{p: &project.Project{ID: "p1", Visibility: project.VisibilityOrg}},
		Members:  &fakeMembers{role: &lead},
		Boards:   &fakeFormBoards{}, Columns: &fakeFormColumns{},
		Tasks: tasks, Forms: forms, Intake: intake, IntakeMail: mail, Audit: aud,
	})
	return s, intake, tasks, mail, aud
}

func TestSubmitForm_CreatesRequestNotTask(t *testing.T) {
	ctx := context.Background()
	s, intake, tasks, _, _ := intakeService("tok")

	id, err := s.SubmitForm(ctx, "tok", SubmitFormInput{
		Title: "Login broken", Description: "500 on submit", Priority: "high",
		SubmitterName: "Ada", SubmitterEmail: "ada@example.com",
	})
	if err != nil {
		t.Fatalf("SubmitForm: %v", err)
	}
	if len(tasks.tasks) != 0 {
		t.Fatal("submit must not create a task before triage")
	}
	r := intake.rows[id]
	if r.Status != project.IntakeNew || r.FormID != "f1" || r.FormSchemaVersion != 3 {
		t.Errorf("request = %+v", r)
	}
	if r.SubmitterName != "Ada" || r.SubmitterEmail != "ada@example.com" || r.Priority != project.PriorityHigh {
		t.Errorf("submitter/priority = %+v", r)
	}
	if !strings.Contains(string(r.FormSnapshot), `"name":"Bugs"`) {
		t.Errorf("snapshot = %s", r.FormSnapshot)
	}
}

func TestAcceptRequest_CreatesTaskAuthoredByAccepter(t *testing.T) {
	ctx := context.Background()
	s, intake, tasks, _, aud := intakeService("tok")
	id, _ := s.SubmitForm(ctx, "tok", SubmitFormInput{Title: "T", Description: "body", SubmitterName: "Ada", SubmitterEmail: "ada@example.com"})

	got, err := s.AcceptRequest(ctx, "o", "triager", "p1", id, tenant.RoleMember)
	if err != nil {
		t.Fatalf("AcceptRequest: %v", err)
	}
	if got.CreatedBy != "triager" {
		t.Errorf("created_by = %s, want the accepting user (never the submitter)", got.CreatedBy)
	}
	if !strings.Contains(got.Description, "Ada <ada@example.com>") || !strings.Contains(got.Description, "body") {
		t.Errorf("description = %q", got.Description)
	}
	if r := intake.rows[id]; r.Status != project.IntakeAccepted || r.TaskID != got.ID {
		t.Errorf("request = %+v", r)
	}
	if len(aud.entries) != 1 || aud.entries[0].ActorUserID != "triager" || aud.entries[0].Action != audit.ActionIntakeAccept {
		t.Errorf("audit = %+v", aud.entries)
	}
	if len(tasks.tasks) != 1 {
		t.Fatalf("tasks = %d", len(tasks.tasks))
	}
	if _, err := s.AcceptRequest(ctx, "o", "triager", "p1", id, tenant.RoleMember); err != domain.ErrConflict {
		t.Errorf("second accept err = %v, want ErrConflict", err)
	}
}

func TestDeclineRequest_NeedsReasonAndIsAudited(t *testing.T) {
	ctx := context.Background()
	s, intake, _, _, aud := intakeService("tok")
	id, _ := s.SubmitForm(ctx, "tok", SubmitFormInput{Title: "T"})

	if _, err := s.DeclineRequest(ctx, "o", "u", "p1", id, "  ", tenant.RoleMember); err != domain.ErrValidation {
		t.Fatalf("empty reason err = %v", err)
	}
	if _, err := s.DeclineRequest(ctx, "o", "u", "p1", id, "Out of scope", tenant.RoleMember); err != nil {
		t.Fatalf("decline: %v", err)
	}
	if r := intake.rows[id]; r.Status != project.IntakeDeclined || r.DeclineReason != "Out of scope" {
		t.Errorf("request = %+v", r)
	}
	if len(aud.entries) != 1 || aud.entries[0].Action != audit.ActionIntakeDecline || aud.entries[0].ActorUserID != "u" {
		t.Errorf("audit = %+v", aud.entries)
	}
}

func TestSendIntakeMails_AckThenCompletionOnce(t *testing.T) {
	ctx := context.Background()
	s, intake, _, mail, _ := intakeService("tok")
	id, _ := s.SubmitForm(ctx, "tok", SubmitFormInput{Title: "Login\r\nBcc: x@evil", SubmitterName: "Ada", SubmitterEmail: "ada@example.com"})
	if _, err := s.SubmitForm(ctx, "tok", SubmitFormInput{Title: "Anonymous"}); err != nil {
		t.Fatal(err)
	}

	n, err := s.SendIntakeMails(ctx, "o")
	if err != nil || n != 1 {
		t.Fatalf("first sweep n=%d err=%v, want one ack (address-less request owes nothing)", n, err)
	}
	if strings.ContainsAny(mail.subjects[0], "\r\n") {
		t.Errorf("subject has line breaks: %q", mail.subjects[0])
	}
	if n, _ := s.SendIntakeMails(ctx, "o"); n != 0 {
		t.Fatalf("second sweep sent %d, want 0 (ack already sent)", n)
	}

	// The linked task completes: the sweep flips the request and mails once.
	intake.rows[id].Status = project.IntakeAccepted
	intake.done = []string{id}
	if n, _ := s.SendIntakeMails(ctx, "o"); n != 1 {
		t.Fatalf("completion sweep sent %d, want 1", n)
	}
	if n, _ := s.SendIntakeMails(ctx, "o"); n != 0 {
		t.Fatalf("repeat sweep sent %d, want 0", n)
	}
	if !strings.Contains(mail.subjects[1], "done") || mail.to[1] != "ada@example.com" {
		t.Errorf("completion mail = %q to %q", mail.subjects[1], mail.to[1])
	}
}
