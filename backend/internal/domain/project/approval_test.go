package project

import (
	"errors"
	"testing"
	"time"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
)

func newApproval(approvers ...string) *Approval {
	a := &Approval{ID: "a1", TaskID: "t1", RequestedBy: "req", State: ApprovalPending}
	for _, id := range approvers {
		a.Approvers = append(a.Approvers, Approver{UserID: id})
	}
	return a
}

func TestValidateApprovalRequest(t *testing.T) {
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	past, future := now.Add(-time.Hour), now.Add(time.Hour)
	many := make([]string, MaxApprovers+1)
	for i := range many {
		many[i] = string(rune('a' + i))
	}
	cases := []struct {
		name      string
		approvers []string
		deadline  *time.Time
		wantErr   error
		wantIDs   int
	}{
		{"ok, deduped", []string{"u1", "u2", "u1"}, &future, nil, 2},
		{"requester named", []string{"u1", "req"}, nil, ErrSelfApproval, 0},
		{"none", nil, nil, ErrApproversInvalid, 0},
		{"blank only", []string{"  "}, nil, ErrApproversInvalid, 0},
		{"too many", many, nil, ErrApproversInvalid, 0},
		{"deadline passed", []string{"u1"}, &past, ErrDeadlinePast, 0},
	}
	for _, c := range cases {
		ids, err := ValidateApprovalRequest("req", c.approvers, c.deadline, "", now)
		if c.wantErr != nil {
			if !errors.Is(err, c.wantErr) {
				t.Errorf("%s: err = %v, want %v", c.name, err, c.wantErr)
			}
			continue
		}
		if err != nil || len(ids) != c.wantIDs {
			t.Errorf("%s: ids=%v err=%v", c.name, ids, err)
		}
	}
}

func TestApprovalDecide(t *testing.T) {
	now := time.Now()

	t.Run("self approval refused even if listed", func(t *testing.T) {
		a := newApproval("req", "u1")
		if err := a.Decide("req", DecisionApproved, "", now); !errors.Is(err, ErrSelfApproval) {
			t.Fatalf("err = %v, want ErrSelfApproval", err)
		}
		if a.State != ApprovalPending {
			t.Fatalf("state = %s", a.State)
		}
	})
	t.Run("all approve", func(t *testing.T) {
		a := newApproval("u1", "u2")
		if err := a.Decide("u1", DecisionApproved, "", now); err != nil || a.State != ApprovalPending {
			t.Fatalf("after first: err=%v state=%s", err, a.State)
		}
		if err := a.Decide("u2", DecisionApproved, "lgtm", now); err != nil || a.State != ApprovalApproved || a.DecidedAt == nil {
			t.Fatalf("after second: err=%v state=%s", err, a.State)
		}
	})
	t.Run("one change request settles it", func(t *testing.T) {
		a := newApproval("u1", "u2")
		if err := a.Decide("u1", DecisionChangesRequested, "fix the title", now); err != nil || a.State != ApprovalChangesRequested {
			t.Fatalf("err=%v state=%s", err, a.State)
		}
		if err := a.Decide("u2", DecisionApproved, "", now); !errors.Is(err, ErrApprovalNotPending) {
			t.Fatalf("late decision err = %v", err)
		}
	})
	t.Run("validation and membership", func(t *testing.T) {
		a := newApproval("u1")
		if err := a.Decide("u1", DecisionChangesRequested, "  ", now); !errors.Is(err, ErrApprovalComment) {
			t.Errorf("blank comment err = %v", err)
		}
		if err := a.Decide("u1", "maybe", "", now); !errors.Is(err, ErrDecisionInvalid) {
			t.Errorf("bad decision err = %v", err)
		}
		if err := a.Decide("stranger", DecisionApproved, "", now); !errors.Is(err, ErrNotApprover) {
			t.Errorf("stranger err = %v", err)
		}
		b := newApproval("u1", "u2")
		_ = b.Decide("u1", DecisionApproved, "", now)
		if err := b.Decide("u1", DecisionApproved, "", now); !errors.Is(err, ErrAlreadyDecided) {
			t.Errorf("second answer err = %v", err)
		}
	})
}

func TestApprovalCancelAndStale(t *testing.T) {
	now := time.Now()
	a := newApproval("u1")
	if err := a.Cancel("u1", false, now); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("approver cancel err = %v", err)
	}
	if err := a.Cancel("req", false, now); err != nil || a.State != ApprovalCancelled {
		t.Fatalf("requester cancel: %v %s", err, a.State)
	}
	if err := a.Cancel("req", false, now); !errors.Is(err, ErrApprovalNotPending) {
		t.Fatalf("second cancel err = %v", err)
	}
	if newApproval("u1").Stale() {
		t.Fatal("pending is not stale")
	}
	b := &Approval{State: ApprovalApproved, StaleAt: &now}
	if !b.Stale() {
		t.Fatal("approved with stale_at must be stale")
	}
}
