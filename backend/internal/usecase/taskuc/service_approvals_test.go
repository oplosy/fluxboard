package taskuc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/audit"
	"github.com/mesutokul/fluxboard/backend/internal/domain/notify"
	"github.com/mesutokul/fluxboard/backend/internal/domain/project"
	"github.com/mesutokul/fluxboard/backend/internal/domain/tenant"
)

type memApprovals struct {
	project.ApprovalRepository
	byID map[string]*project.Approval
}

func (m *memApprovals) Create(_ context.Context, _ string, a *project.Approval) error {
	cp := *a
	m.byID[a.ID] = &cp
	return nil
}

func (m *memApprovals) Get(_ context.Context, _, taskID, id string) (*project.Approval, error) {
	a, ok := m.byID[id]
	if !ok || a.TaskID != taskID {
		return nil, domain.ErrNotFound
	}
	cp := *a
	return &cp, nil
}

func (m *memApprovals) Update(_ context.Context, _, taskID, id string, fn func(*project.Approval) error) (*project.Approval, error) {
	a, ok := m.byID[id]
	if !ok || a.TaskID != taskID {
		return nil, domain.ErrNotFound
	}
	cp := *a
	cp.Approvers = append([]project.Approver(nil), a.Approvers...)
	if err := fn(&cp); err != nil {
		return nil, err
	}
	m.byID[id] = &cp
	return &cp, nil
}

type sentNote struct {
	to    []string
	title string
}

type spyNotifier struct {
	Notifier
	sent []sentNote
}

func (n *spyNotifier) NotifyTargets(_ context.Context, _, _ string, cat notify.Category, to []string, title, _, _, _ string) {
	if cat == notify.CategoryApproval {
		n.sent = append(n.sent, sentNote{to: to, title: title})
	}
}

type spyAudit struct{ actions []audit.Action }

func (a *spyAudit) Append(_ context.Context, e audit.Entry) error {
	a.actions = append(a.actions, e.Action)
	return nil
}

type fakeAgents map[string]bool

func (f fakeAgents) IsAgent(_ context.Context, _, id string) (bool, error) { return f[id], nil }

type approvalFixture struct {
	svc   *Service
	repo  *memApprovals
	notes *spyNotifier
	audit *spyAudit
}

func newApprovalFixture(memberRole project.ProjectRole, agents fakeAgents) approvalFixture {
	store := &claimStore{tasks: map[string]*project.Task{
		"t1": {ID: "t1", ProjectID: "p1", Title: "Ship it"},
	}}
	f := approvalFixture{
		repo:  &memApprovals{byID: map[string]*project.Approval{}},
		notes: &spyNotifier{},
		audit: &spyAudit{},
	}
	f.svc = New(Deps{
		Tasks: store, Approvals: f.repo, Notifier: f.notes, Audit: f.audit, Agents: agents,
		Projects: fakeProjects{p: &project.Project{ID: "p1", Visibility: project.VisibilityOrg}},
		Members:  fakeMembers{role: role(memberRole)},
		Now:      func() time.Time { return time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC) },
	})
	return f
}

func TestRequestApproval(t *testing.T) {
	ctx := context.Background()

	t.Run("notifies approvers and audits", func(t *testing.T) {
		f := newApprovalFixture(project.RoleContributor, nil)
		a, err := f.svc.RequestApproval(ctx, "o", "req", "t1", RequestApprovalInput{ApproverIDs: []string{"u1", "u2"}}, tenant.RoleMember)
		if err != nil || a.State != project.ApprovalPending || len(a.Approvers) != 2 {
			t.Fatalf("request: %+v %v", a, err)
		}
		if len(f.notes.sent) != 1 || len(f.notes.sent[0].to) != 2 {
			t.Fatalf("notifications = %+v", f.notes.sent)
		}
		if len(f.audit.actions) != 1 || f.audit.actions[0] != audit.ActionApprovalRequest {
			t.Fatalf("audit = %v", f.audit.actions)
		}
	})
	t.Run("requester cannot name themselves", func(t *testing.T) {
		f := newApprovalFixture(project.RoleContributor, nil)
		_, err := f.svc.RequestApproval(ctx, "o", "req", "t1", RequestApprovalInput{ApproverIDs: []string{"req"}}, tenant.RoleMember)
		if !errors.Is(err, project.ErrSelfApproval) {
			t.Fatalf("err = %v, want ErrSelfApproval", err)
		}
	})
	t.Run("viewer cannot be an approver", func(t *testing.T) {
		f := newApprovalFixture(project.RoleViewer, nil)
		_, err := f.svc.RequestApproval(ctx, "o", "req", "t1", RequestApprovalInput{ApproverIDs: []string{"u1"}}, tenant.RoleAdmin)
		if !errors.Is(err, project.ErrApproversInvalid) {
			t.Fatalf("err = %v, want ErrApproversInvalid", err)
		}
	})
	t.Run("an agent cannot be an approver", func(t *testing.T) {
		f := newApprovalFixture(project.RoleContributor, fakeAgents{"bot": true})
		_, err := f.svc.RequestApproval(ctx, "o", "req", "t1", RequestApprovalInput{ApproverIDs: []string{"bot"}}, tenant.RoleMember)
		if !errors.Is(err, project.ErrApproversInvalid) {
			t.Fatalf("err = %v, want ErrApproversInvalid", err)
		}
	})
}

func TestDecideApproval(t *testing.T) {
	ctx := context.Background()
	f := newApprovalFixture(project.RoleContributor, fakeAgents{"bot": true})
	a, err := f.svc.RequestApproval(ctx, "o", "req", "t1", RequestApprovalInput{ApproverIDs: []string{"u1", "bot"}}, tenant.RoleMember)
	if err == nil {
		t.Fatalf("agent approver must be refused, got %+v", a)
	}
	a, err = f.svc.RequestApproval(ctx, "o", "req", "t1", RequestApprovalInput{ApproverIDs: []string{"u1"}}, tenant.RoleMember)
	if err != nil {
		t.Fatal(err)
	}
	f.notes.sent = nil

	if _, err := f.svc.DecideApproval(ctx, "o", "req", "t1", a.ID, project.DecisionApproved, "", tenant.RoleMember); !errors.Is(err, project.ErrSelfApproval) {
		t.Fatalf("requester deciding: err = %v, want ErrSelfApproval", err)
	}
	if _, err := f.svc.DecideApproval(ctx, "o", "stranger", "t1", a.ID, project.DecisionApproved, "", tenant.RoleMember); !errors.Is(err, project.ErrNotApprover) {
		t.Fatalf("stranger deciding: err = %v, want ErrNotApprover", err)
	}
	if _, err := f.svc.DecideApproval(ctx, "o", "bot", "t1", a.ID, project.DecisionApproved, "", tenant.RoleMember); !errors.Is(err, project.ErrHumanApprovalRequired) {
		t.Fatalf("agent deciding: err = %v, want ErrHumanApprovalRequired", err)
	}
	if len(f.notes.sent) != 0 {
		t.Fatalf("refused decisions must not notify: %+v", f.notes.sent)
	}

	got, err := f.svc.DecideApproval(ctx, "o", "u1", "t1", a.ID, project.DecisionChangesRequested, "tighten the copy", tenant.RoleMember)
	if err != nil || got.State != project.ApprovalChangesRequested {
		t.Fatalf("decide: %+v %v", got, err)
	}
	if len(f.notes.sent) != 1 || f.notes.sent[0].to[0] != "req" {
		t.Fatalf("requester must be notified: %+v", f.notes.sent)
	}
	if last := f.audit.actions[len(f.audit.actions)-1]; last != audit.ActionApprovalDecide {
		t.Fatalf("last audit = %s", last)
	}
}
