package automationuc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/automation"
)

type fakeRules struct {
	automation.RuleRepository
	rules []automation.Rule
}

func (f *fakeRules) Create(_ context.Context, _ string, r *automation.Rule) error {
	f.rules = append(f.rules, *r)
	return nil
}

func (f *fakeRules) Get(_ context.Context, _, id string) (*automation.Rule, error) {
	for i := range f.rules {
		if f.rules[i].ID == id {
			return &f.rules[i], nil
		}
	}
	return nil, domain.ErrNotFound
}

func (f *fakeRules) ListEnabled(_ context.Context, _ string, trigger string) ([]automation.Rule, error) {
	var out []automation.Rule
	for _, r := range f.rules {
		if r.Enabled && r.Trigger == trigger {
			out = append(out, r)
		}
	}
	return out, nil
}

// fakeRuns is an in-memory RunRepository with the once-only claim of the
// time-trigger unique index.
type fakeRuns struct {
	runs []automation.Run
	due  map[string][]string // ruleID -> task ids the scan query returns
}

func (f *fakeRuns) Insert(_ context.Context, _ string, r *automation.Run) (bool, error) {
	if automation.IsTimeTrigger(r.Trigger) {
		for _, x := range f.runs {
			if x.RuleID == r.RuleID && x.TaskID == r.TaskID && automation.IsTimeTrigger(x.Trigger) {
				return false, nil
			}
		}
	}
	f.runs = append(f.runs, *r)
	return true, nil
}

func (f *fakeRuns) Finish(_ context.Context, _, id, status, msg string) error {
	for i := range f.runs {
		if f.runs[i].ID == id {
			f.runs[i].Status, f.runs[i].Error = status, msg
		}
	}
	return nil
}

func (f *fakeRuns) ListByRule(_ context.Context, _, ruleID string, _ int) ([]automation.Run, error) {
	var out []automation.Run
	for _, r := range f.runs {
		if r.RuleID == ruleID {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *fakeRuns) CountSince(_ context.Context, _ string, _ time.Time) (int, error) {
	n := 0
	for _, r := range f.runs {
		if r.Status != automation.RunSkipped {
			n++
		}
	}
	return n, nil
}

// DueTasks ignores claims on purpose: the claim in Insert must hold even when
// the scan query returns an already-fired task (a concurrent scan).
func (f *fakeRuns) DueTasks(_ context.Context, _, ruleID string, _, _ time.Time) ([]string, error) {
	return f.due[ruleID], nil
}

type fakeActions struct {
	calls []automation.ActionRequest
	depth []int
	err   error
	// onApply lets a test re-enter Evaluate, as taskuc.automate does.
	onApply func(ctx context.Context)
}

func (f *fakeActions) ApplyAutomation(ctx context.Context, _ string, a automation.ActionRequest) error {
	f.calls = append(f.calls, a)
	f.depth = append(f.depth, automation.DepthFrom(ctx))
	if f.onApply != nil {
		f.onApply(ctx)
	}
	return f.err
}

func testService() (*Service, *fakeRules, *fakeRuns, *fakeActions) {
	rules, runs, actions := &fakeRules{}, &fakeRuns{due: map[string][]string{}}, &fakeActions{}
	svc := New(Deps{Rules: rules, Runs: runs, Budget: 3})
	svc.SetActions(actions)
	return svc, rules, runs, actions
}

func assignRule(id, trigger string) automation.Rule {
	return automation.Rule{
		ID: id, OrgID: "o", Name: id, Enabled: true, Trigger: trigger,
		Action: automation.ActionAssign, ActionConfig: automation.ActionConfig{UserID: "u9"}, CreatedBy: "creator",
	}
}

func TestCreate_RejectsBadDefinitions(t *testing.T) {
	svc, _, _, _ := testService()
	assign := automation.ActionConfig{UserID: "u"}
	cases := map[string]CreateInput{
		"unknown trigger":      {Name: "x", Trigger: "nope", Action: automation.ActionAssign, ActionConfig: assign},
		"unknown action":       {Name: "x", Trigger: automation.TriggerTaskCreated, Action: "nope"},
		"assign w/o user":      {Name: "x", Trigger: automation.TriggerTaskCreated, Action: automation.ActionAssign},
		"due_soon w/o hours":   {Name: "x", Trigger: automation.TriggerTaskDueSoon, Action: automation.ActionAssign, ActionConfig: assign},
		"due_soon over 30 day": {Name: "x", Trigger: automation.TriggerTaskDueSoon, TriggerConfig: automation.TriggerConfig{Hours: automation.MaxHours + 1}, Action: automation.ActionAssign, ActionConfig: assign},
	}
	for name, in := range cases {
		if _, err := svc.Create(context.Background(), "o", in); !errors.Is(err, domain.ErrValidation) {
			t.Errorf("%s: want ErrValidation, got %v", name, err)
		}
	}
	ok := CreateInput{Name: "x", Trigger: automation.TriggerTaskDueSoon, TriggerConfig: automation.TriggerConfig{Hours: 24}, Action: automation.ActionAssign, ActionConfig: assign}
	if _, err := svc.Create(context.Background(), "o", ok); err != nil {
		t.Fatalf("valid due_soon rule rejected: %v", err)
	}
}

func TestEvaluate_RecordsSuccessWithAttribution(t *testing.T) {
	svc, rules, runs, actions := testService()
	rules.rules = []automation.Rule{assignRule("r1", automation.TriggerTaskCreated)}
	svc.Evaluate(context.Background(), automation.Event{OrgID: "o", Trigger: automation.TriggerTaskCreated, TaskID: "t1", ActorID: "actor"})
	if len(actions.calls) != 1 || actions.calls[0].ActorID != "creator" || actions.calls[0].RuleID != "r1" {
		t.Fatalf("action not attributed to the rule and its creator: %+v", actions.calls)
	}
	if actions.depth[0] != 1 {
		t.Fatalf("action must run one level deeper, got depth %d", actions.depth[0])
	}
	if len(runs.runs) != 1 || runs.runs[0].Status != automation.RunSuccess || runs.runs[0].ActorID != "actor" || runs.runs[0].TaskID != "t1" {
		t.Fatalf("run not recorded as success: %+v", runs.runs)
	}
}

func TestEvaluate_RecordsFailure(t *testing.T) {
	svc, rules, runs, actions := testService()
	actions.err = errors.New("column gone")
	rules.rules = []automation.Rule{assignRule("r1", automation.TriggerTaskCreated)}
	svc.Evaluate(context.Background(), automation.Event{OrgID: "o", Trigger: automation.TriggerTaskCreated, TaskID: "t1"})
	if len(runs.runs) != 1 || runs.runs[0].Status != automation.RunFailed || runs.runs[0].Error != "column gone" {
		t.Fatalf("failure not recorded: %+v", runs.runs)
	}
}

func TestEvaluate_MoveColumnFilterAndDisabled(t *testing.T) {
	svc, rules, runs, actions := testService()
	moved := assignRule("r1", automation.TriggerTaskMoved)
	moved.TriggerConfig.ColumnID = "c2"
	off := assignRule("r2", automation.TriggerTaskMoved)
	off.Enabled = false
	rules.rules = []automation.Rule{moved, off}
	svc.Evaluate(context.Background(), automation.Event{OrgID: "o", Trigger: automation.TriggerTaskMoved, TaskID: "t1", ToColumnID: "c9"})
	if len(actions.calls) != 0 || len(runs.runs) != 0 {
		t.Fatalf("rule fired for a non-matching column or while disabled: %+v", actions.calls)
	}
	svc.Evaluate(context.Background(), automation.Event{OrgID: "o", Trigger: automation.TriggerTaskMoved, TaskID: "t1", ToColumnID: "c2"})
	if len(actions.calls) != 1 {
		t.Fatalf("want 1 action, got %d", len(actions.calls))
	}
}

// A rule whose action re-triggers itself (assign -> task.assigned -> assign...)
// must stop at MaxDepth and record the skip.
func TestEvaluate_LoopGuardStopsCascade(t *testing.T) {
	svc, rules, runs, actions := testService()
	svc.budget = 1000
	rules.rules = []automation.Rule{assignRule("loop", automation.TriggerTaskAssigned)}
	ev := automation.Event{OrgID: "o", Trigger: automation.TriggerTaskAssigned, TaskID: "t1"}
	actions.onApply = func(ctx context.Context) { svc.Evaluate(ctx, ev) }
	svc.Evaluate(context.Background(), ev)
	if len(actions.calls) != automation.MaxDepth {
		t.Fatalf("want %d executed actions, got %d", automation.MaxDepth, len(actions.calls))
	}
	last := runs.runs[len(runs.runs)-1]
	if last.Status != automation.RunSkipped || last.Depth != automation.MaxDepth {
		t.Fatalf("last run must be a depth skip: %+v", last)
	}
}

func TestEvaluate_BudgetSkipsAndRecords(t *testing.T) {
	svc, rules, runs, actions := testService() // budget 3
	rules.rules = []automation.Rule{assignRule("r1", automation.TriggerTaskCreated)}
	for i := 0; i < 5; i++ {
		svc.Evaluate(context.Background(), automation.Event{OrgID: "o", Trigger: automation.TriggerTaskCreated, TaskID: "t1"})
	}
	if len(actions.calls) != 3 {
		t.Fatalf("budget of 3 not enforced: %d actions", len(actions.calls))
	}
	skipped := 0
	for _, r := range runs.runs {
		if r.Status == automation.RunSkipped {
			skipped++
		}
	}
	if skipped != 2 {
		t.Fatalf("want 2 recorded budget skips, got %d", skipped)
	}
}

func TestScanTimeTriggers_FiresOncePerTaskPerRule(t *testing.T) {
	svc, rules, runs, actions := testService()
	svc.budget = 1000
	rules.rules = []automation.Rule{
		assignRule("soon", automation.TriggerTaskDueSoon),
		assignRule("late", automation.TriggerTaskOverdue),
	}
	rules.rules[0].TriggerConfig.Hours = 24
	runs.due["soon"] = []string{"t1", "t2"}
	runs.due["late"] = []string{"t1"}

	for i := 0; i < 3; i++ { // repeated scans return the same tasks
		if _, err := svc.ScanTimeTriggers(context.Background(), "o"); err != nil {
			t.Fatal(err)
		}
	}
	if len(actions.calls) != 3 {
		t.Fatalf("want 3 firings (t1+t2 soon, t1 late), got %d", len(actions.calls))
	}
}

func TestScanTimeTriggers_StopsAtBudgetWithoutRecording(t *testing.T) {
	svc, rules, runs, actions := testService() // budget 3
	rules.rules = []automation.Rule{assignRule("late", automation.TriggerTaskOverdue)}
	runs.due["late"] = []string{"t1", "t2", "t3", "t4", "t5"}
	n, err := svc.ScanTimeTriggers(context.Background(), "o")
	if err != nil || n != 3 || len(actions.calls) != 3 {
		t.Fatalf("want 3 fired within budget, got n=%d calls=%d err=%v", n, len(actions.calls), err)
	}
	for _, r := range runs.runs {
		if r.Status == automation.RunSkipped {
			t.Fatalf("a time trigger over budget must not be recorded (it retries next scan): %+v", r)
		}
	}
}
