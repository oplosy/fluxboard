// Package automationuc holds the application service for automation rules
// (docs/08, FR-AUTO, ADR-036). Event triggers are evaluated synchronously on
// the task write path: taskuc (create/update) and projectuc (move) call
// Evaluate best-effort after publishing their realtime event. Time triggers
// are evaluated by the worker's scheduled job (ScanTimeTriggers). Every firing
// is recorded in the run history, and actions run through the task usecase's
// normal write path (automation.TaskActions) with a depth-limited loop guard
// and a per-org hourly action budget.
package automationuc

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/automation"
	"github.com/mesutokul/fluxboard/backend/internal/domain/project"
	"github.com/mesutokul/fluxboard/backend/internal/pkg/uuidv7"
)

// DefaultBudget is the per-org cap on executed automation actions per hour.
const DefaultBudget = 500

// budgetWindow is the sliding window the budget counts over.
const budgetWindow = time.Hour

// Deps are the collaborators the service needs. Actions is wired after
// construction (SetActions): taskuc depends on this service as its Evaluator,
// so the reverse edge cannot be a constructor argument.
type Deps struct {
	Rules  automation.RuleRepository
	Runs   automation.RunRepository
	Budget int // executed actions per org per hour; 0 ⇒ DefaultBudget
	Logger *slog.Logger
	Now    func() time.Time
}

// Service implements automation rule CRUD and event evaluation.
type Service struct {
	rules   automation.RuleRepository
	runs    automation.RunRepository
	actions automation.TaskActions
	budget  int
	logger  *slog.Logger
	now     func() time.Time
}

// New builds a Service from Deps.
func New(d Deps) *Service {
	logger := d.Logger
	if logger == nil {
		logger = slog.Default()
	}
	budget := d.Budget
	if budget <= 0 {
		budget = DefaultBudget
	}
	now := d.Now
	if now == nil {
		now = time.Now
	}
	return &Service{rules: d.Rules, runs: d.Runs, budget: budget, logger: logger, now: now}
}

// SetActions wires the task usecase that performs rule actions.
func (s *Service) SetActions(a automation.TaskActions) { s.actions = a }

// ---- CRUD (ADMIN+, gated in the handler) ------------------------------------

// CreateInput carries the rule builder form.
type CreateInput struct {
	Name          string
	Trigger       string
	TriggerConfig automation.TriggerConfig
	Action        string
	ActionConfig  automation.ActionConfig
	CreatedBy     string
}

func (in CreateInput) validate() error {
	if strings.TrimSpace(in.Name) == "" {
		return fmt.Errorf("%w: name required", domain.ErrValidation)
	}
	if !automation.ValidTrigger(in.Trigger) {
		return fmt.Errorf("%w: unknown trigger", domain.ErrValidation)
	}
	if in.Trigger == automation.TriggerTaskDueSoon && (in.TriggerConfig.Hours < 1 || in.TriggerConfig.Hours > automation.MaxHours) {
		return fmt.Errorf("%w: due_soon needs hours between 1 and %d", domain.ErrValidation, automation.MaxHours)
	}
	if !automation.ValidAction(in.Action) {
		return fmt.Errorf("%w: unknown action", domain.ErrValidation)
	}
	return checkActionConfig(in.Action, in.ActionConfig)
}

// Create validates the vocabulary and stores the rule.
func (s *Service) Create(ctx context.Context, orgID string, in CreateInput) (*automation.Rule, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	r := &automation.Rule{
		ID: uuidv7.New().String(), OrgID: orgID, Name: strings.TrimSpace(in.Name),
		Enabled: true, Trigger: in.Trigger, TriggerConfig: in.TriggerConfig,
		Action: in.Action, ActionConfig: in.ActionConfig, CreatedBy: in.CreatedBy,
	}
	if err := s.rules.Create(ctx, orgID, r); err != nil {
		return nil, err
	}
	return s.rules.Get(ctx, orgID, r.ID)
}

// List returns every rule in creation order (settings UI).
func (s *Service) List(ctx context.Context, orgID string) ([]automation.Rule, error) {
	return s.rules.List(ctx, orgID)
}

// Update replaces a rule's definition. Unknown trigger/action rejected.
func (s *Service) Update(ctx context.Context, orgID, id string, in CreateInput) (*automation.Rule, error) {
	r, err := s.rules.Get(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	if err := in.validate(); err != nil {
		return nil, err
	}
	r.Name = strings.TrimSpace(in.Name)
	r.Trigger = in.Trigger
	r.TriggerConfig = in.TriggerConfig
	r.Action = in.Action
	r.ActionConfig = in.ActionConfig
	if err := s.rules.Update(ctx, orgID, r); err != nil {
		return nil, err
	}
	return r, nil
}

// SetEnabled flips a rule without touching its definition.
func (s *Service) SetEnabled(ctx context.Context, orgID, id string, enabled bool) (*automation.Rule, error) {
	r, err := s.rules.Get(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	r.Enabled = enabled
	if err := s.rules.Update(ctx, orgID, r); err != nil {
		return nil, err
	}
	return r, nil
}

// Delete removes a rule.
func (s *Service) Delete(ctx context.Context, orgID, id string) error {
	return s.rules.Delete(ctx, orgID, id)
}

// Runs returns a rule's most recent firings, newest first (settings UI).
func (s *Service) Runs(ctx context.Context, orgID, ruleID string, limit int) ([]automation.Run, error) {
	if _, err := s.rules.Get(ctx, orgID, ruleID); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return s.runs.ListByRule(ctx, orgID, ruleID, limit)
}

func checkActionConfig(action string, c automation.ActionConfig) error {
	switch action {
	case automation.ActionAssign:
		if strings.TrimSpace(c.UserID) == "" {
			return fmt.Errorf("%w: assign needs user_id", domain.ErrValidation)
		}
	case automation.ActionSetPriority:
		if !project.Priority(c.Priority).Valid() || c.Priority == string(project.PriorityNone) {
			return fmt.Errorf("%w: set_priority needs a real priority", domain.ErrValidation)
		}
	case automation.ActionMove:
		if strings.TrimSpace(c.ColumnID) == "" {
			return fmt.Errorf("%w: move needs column_id", domain.ErrValidation)
		}
	case automation.ActionAddLabel:
		if strings.TrimSpace(c.LabelID) == "" {
			return fmt.Errorf("%w: add_label needs label_id", domain.ErrValidation)
		}
	}
	return nil
}

// ---- Evaluate ---------------------------------------------------------------

// Evaluate runs enabled rules for e.Trigger against the task. Best-effort:
// per-rule failures are recorded and logged, never returned, so automation
// can never fail the write that triggered it.
func (s *Service) Evaluate(ctx context.Context, e automation.Event) {
	if s.rules == nil || s.runs == nil || s.actions == nil {
		return
	}
	rules, err := s.rules.ListEnabled(ctx, e.OrgID, e.Trigger)
	if err != nil {
		s.logger.Warn("automation list failed", "err", err, "org", e.OrgID, "trigger", e.Trigger)
		return
	}
	for i := range rules {
		r := &rules[i]
		if !matchTrigger(r, e) {
			continue
		}
		s.fire(ctx, e.OrgID, r, e.TaskID, e.ActorID)
	}
}

func matchTrigger(r *automation.Rule, e automation.Event) bool {
	if r.Trigger == automation.TriggerTaskMoved && r.TriggerConfig.ColumnID != "" {
		return r.TriggerConfig.ColumnID == e.ToColumnID
	}
	return true
}

// ScanTimeTriggers fires the due-soon and overdue rules of one org (worker
// job). Each (rule, task) fires at most once: the run row is the claim, so a
// repeated or concurrent scan finds nothing left to do. Returns how many
// actions fired. Stops early when the org's action budget is spent; the
// untouched tasks are picked up by the next scan.
func (s *Service) ScanTimeTriggers(ctx context.Context, orgID string) (int, error) {
	if s.rules == nil || s.runs == nil || s.actions == nil {
		return 0, nil
	}
	now := s.now().UTC()
	fired := 0
	for _, trig := range []string{automation.TriggerTaskDueSoon, automation.TriggerTaskOverdue} {
		rules, err := s.rules.ListEnabled(ctx, orgID, trig)
		if err != nil {
			return fired, err
		}
		for i := range rules {
			r := &rules[i]
			from, to := time.Unix(0, 0).UTC(), now // overdue: everything before now
			if trig == automation.TriggerTaskDueSoon {
				from, to = now, now.Add(time.Duration(r.TriggerConfig.Hours)*time.Hour)
			}
			ids, err := s.runs.DueTasks(ctx, orgID, r.ID, from, to)
			if err != nil {
				return fired, err
			}
			for _, id := range ids {
				switch s.fire(ctx, orgID, r, id, "") {
				case fireDone:
					fired++
				case fireBudget:
					return fired, nil
				}
			}
		}
	}
	return fired, nil
}

type fireResult int

const (
	fireNone   fireResult = iota // skipped, lost the claim, or could not record
	fireDone                     // the action ran (success or failed)
	fireBudget                   // the org's budget is spent
)

// fire records and executes one rule on one task, enforcing the loop guard and
// the org budget. Skips of event triggers are recorded so "why did it not
// fire?" has an answer; a budget skip of a time trigger is not (it retries).
func (s *Service) fire(ctx context.Context, orgID string, r *automation.Rule, taskID, actorID string) fireResult {
	depth := automation.DepthFrom(ctx)
	run := &automation.Run{
		ID: uuidv7.New().String(), OrgID: orgID, RuleID: r.ID, Trigger: r.Trigger,
		TaskID: taskID, ActorID: actorID, Status: automation.RunPending, Depth: depth,
	}
	skip := func(why string) {
		run.Status, run.Error = automation.RunSkipped, why
		if _, err := s.runs.Insert(ctx, orgID, run); err != nil {
			s.logger.Warn("automation run record failed", "err", err, "rule", r.ID)
		}
	}
	if depth >= automation.MaxDepth {
		skip(fmt.Sprintf("loop guard: cascade depth limit %d reached", automation.MaxDepth))
		return fireNone
	}
	n, err := s.runs.CountSince(ctx, orgID, s.now().Add(-budgetWindow))
	if err != nil {
		s.logger.Warn("automation budget check failed", "err", err, "org", orgID)
		return fireNone
	}
	if n >= s.budget {
		if !automation.IsTimeTrigger(r.Trigger) {
			skip(fmt.Sprintf("action budget of %d per hour reached", s.budget))
		}
		return fireBudget
	}
	ok, err := s.runs.Insert(ctx, orgID, run)
	if err != nil {
		s.logger.Warn("automation run record failed", "err", err, "rule", r.ID, "task", taskID)
		return fireNone
	}
	if !ok {
		return fireNone // another scan already claimed this (rule, task)
	}
	err = s.actions.ApplyAutomation(automation.WithDepth(ctx, depth+1), orgID, automation.ActionRequest{
		RuleID: r.ID, ActorID: r.CreatedBy, TaskID: taskID, Action: r.Action, Config: r.ActionConfig,
	})
	status, msg := automation.RunSuccess, ""
	if err != nil {
		status, msg = automation.RunFailed, err.Error()
		s.logger.Warn("automation rule failed", "err", err, "rule", r.ID, "task", taskID)
	}
	if ferr := s.runs.Finish(ctx, orgID, run.ID, status, msg); ferr != nil {
		s.logger.Warn("automation run finish failed", "err", ferr, "run", run.ID)
	}
	return fireDone
}
