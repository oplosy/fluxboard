// Package automation holds the domain types for org-scoped automation rules
// (docs/08, FR-AUTO): "when X happens, do Y" rules evaluated on the task
// write path. Trigger/action vocabularies are closed sets validated in the
// domain so unknown kinds fail fast at rule creation, not at fire time.
package automation

import (
	"context"
	"time"
)

// Triggers.
const (
	TriggerTaskCreated  = "task.created"
	TriggerTaskMoved    = "task.moved"
	TriggerTaskAssigned = "task.assigned"
	// Time triggers (ADR-036): evaluated by the scheduled worker job, firing
	// at most once per task per rule.
	TriggerTaskDueSoon = "task.due_soon" // due within TriggerConfig.Hours
	TriggerTaskOverdue = "task.overdue"  // due date passed, task still open
)

// IsTimeTrigger reports whether t is evaluated by the scheduler, not a write.
func IsTimeTrigger(t string) bool {
	return t == TriggerTaskDueSoon || t == TriggerTaskOverdue
}

// Loop guard and budget defaults (ADR-036).
const (
	// MaxDepth is how many automation-made writes may chain: a human write is
	// depth 0, a rule fired by it runs at depth 1, and so on. A rule that
	// would run at MaxDepth is skipped.
	MaxDepth = 3
	// MaxHours bounds the due-soon window (30 days).
	MaxHours = 720
)

type depthKey struct{}

// WithDepth returns ctx carrying the automation cascade depth.
func WithDepth(ctx context.Context, depth int) context.Context {
	return context.WithValue(ctx, depthKey{}, depth)
}

// DepthFrom returns the cascade depth of ctx (0 for a human write).
func DepthFrom(ctx context.Context) int {
	d, _ := ctx.Value(depthKey{}).(int)
	return d
}

// Run statuses.
const (
	RunPending = "pending"
	RunSuccess = "success"
	RunFailed  = "failed"
	RunSkipped = "skipped"
)

// Run is one recorded rule firing (run history).
type Run struct {
	ID         string
	OrgID      string
	RuleID     string
	Trigger    string
	TaskID     string
	TaskTitle  string // joined on read, with the task's route identity
	TaskNumber int
	ProjectKey string
	ActorID    string // "" for time triggers
	Status     string
	Error      string
	Depth      int
	CreatedAt  time.Time
}

// RunRepository persists run history and answers the time-trigger scan.
type RunRepository interface {
	// Insert records a run. For a time trigger it returns false when the rule
	// already claimed that task (once-only), inserting nothing.
	Insert(ctx context.Context, orgID string, r *Run) (bool, error)
	Finish(ctx context.Context, orgID, id, status, errMsg string) error
	ListByRule(ctx context.Context, orgID, ruleID string, limit int) ([]Run, error)
	// CountSince counts runs that executed (not skipped) since t: the budget window.
	CountSince(ctx context.Context, orgID string, since time.Time) (int, error)
	// DueTasks returns open task ids due in [from, to) the rule has not fired on.
	DueTasks(ctx context.Context, orgID, ruleID string, from, to time.Time) ([]string, error)
}

// ActionRequest asks the task usecase to perform a rule's action as a normal
// task write (activity, realtime event, notification, rule cascade).
type ActionRequest struct {
	RuleID  string
	ActorID string // attribution: the rule's creator
	TaskID  string
	Action  string
	Config  ActionConfig
}

// TaskActions is implemented by taskuc: automation actions take the same
// write path as human edits, minus the per-user access checks.
type TaskActions interface {
	ApplyAutomation(ctx context.Context, orgID string, a ActionRequest) error
}

// Actions.
const (
	ActionAssign      = "assign"
	ActionSetPriority = "set_priority"
	ActionMove        = "move"
	ActionAddLabel    = "add_label"
)

// TriggerConfig conditions a trigger. ColumnID applies to TriggerTaskMoved:
// the rule fires only when the task lands in that column.
type TriggerConfig struct {
	ColumnID string `json:"column_id,omitempty"`
	// Hours is the TriggerTaskDueSoon window (1..MaxHours).
	Hours int `json:"hours,omitempty"`
}

// ActionConfig carries action parameters. Only the field matching the rule's
// action is read: UserID for assign, Priority for set_priority, ColumnID for
// move, LabelID for add_label.
type ActionConfig struct {
	UserID   string `json:"user_id,omitempty"`
	Priority string `json:"priority,omitempty"`
	ColumnID string `json:"column_id,omitempty"`
	LabelID  string `json:"label_id,omitempty"`
}

// Rule is one automation rule in an organization.
type Rule struct {
	ID            string
	OrgID         string
	Name          string
	Enabled       bool
	Trigger       string
	TriggerConfig TriggerConfig
	Action        string
	ActionConfig  ActionConfig
	CreatedBy     string
	CreatedAt     time.Time
}

// ValidTrigger reports whether t is a known trigger kind.
func ValidTrigger(t string) bool {
	switch t {
	case TriggerTaskCreated, TriggerTaskMoved, TriggerTaskAssigned, TriggerTaskDueSoon, TriggerTaskOverdue:
		return true
	}
	return false
}

// ValidAction reports whether a is a known action kind.
func ValidAction(a string) bool {
	switch a {
	case ActionAssign, ActionSetPriority, ActionMove, ActionAddLabel:
		return true
	}
	return false
}

// RuleRepository persists automation rules (FR-AUTO).
type RuleRepository interface {
	Create(ctx context.Context, orgID string, r *Rule) error
	Get(ctx context.Context, orgID, id string) (*Rule, error)
	// List returns every rule in creation order (settings UI).
	List(ctx context.Context, orgID string) ([]Rule, error)
	// ListEnabled returns enabled rules for one trigger (evaluate path).
	ListEnabled(ctx context.Context, orgID, trigger string) ([]Rule, error)
	Update(ctx context.Context, orgID string, r *Rule) error
	// Delete removes a rule. ErrNotFound if absent.
	Delete(ctx context.Context, orgID string, id string) error
}

// Event is one automation-relevant occurrence on the task write path.
type Event struct {
	OrgID      string
	Trigger    string
	TaskID     string
	ActorID    string
	ToColumnID string // task.moved: the destination column
	AssigneeID string // task.assigned: the new assignee ("" when cleared)
}

// Evaluator runs enabled rules for an event. Implemented by the automation
// usecase; taskuc/projectuc depend on this narrow interface (never the
// concrete service) so the dependency rule stays intact.
type Evaluator interface {
	Evaluate(ctx context.Context, e Event)
}
