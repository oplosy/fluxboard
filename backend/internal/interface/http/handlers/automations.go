// Automation rules surface (docs/08, FR-AUTO). Org-scoped CRUD under
// /orgs/{orgId}/automations; writes need the ADMIN automations gate.
package handlers

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/automation"
	mw "github.com/mesutokul/fluxboard/backend/internal/interface/http/middleware"
	"github.com/mesutokul/fluxboard/backend/internal/interface/http/response"
	"github.com/mesutokul/fluxboard/backend/internal/usecase/automationuc"
)

// AutomationHandlers serves the automation-rules surface.
type AutomationHandlers struct {
	svc    *automationuc.Service
	logger *slog.Logger
}

// NewAutomationHandlers builds AutomationHandlers.
func NewAutomationHandlers(svc *automationuc.Service, logger *slog.Logger) *AutomationHandlers {
	if logger == nil {
		logger = slog.Default()
	}
	return &AutomationHandlers{svc: svc, logger: logger}
}

type automationRuleResp struct {
	ID            string                   `json:"id"`
	Name          string                   `json:"name"`
	Enabled       bool                     `json:"enabled"`
	Trigger       string                   `json:"trigger"`
	TriggerConfig automation.TriggerConfig `json:"trigger_config"`
	Action        string                   `json:"action"`
	ActionConfig  automation.ActionConfig  `json:"action_config"`
	CreatedBy     string                   `json:"created_by"`
	CreatedAt     time.Time                `json:"created_at"`
}

func toAutomationRuleResp(r *automation.Rule) automationRuleResp {
	return automationRuleResp{
		ID: r.ID, Name: r.Name, Enabled: r.Enabled, Trigger: r.Trigger,
		TriggerConfig: r.TriggerConfig, Action: r.Action, ActionConfig: r.ActionConfig,
		CreatedBy: r.CreatedBy, CreatedAt: r.CreatedAt,
	}
}

type automationRuleReq struct {
	Name          string                   `json:"name"`
	Trigger       string                   `json:"trigger"`
	TriggerConfig automation.TriggerConfig `json:"trigger_config"`
	Action        string                   `json:"action"`
	ActionConfig  automation.ActionConfig  `json:"action_config"`
}

// ListRules returns every rule in creation order (ADMIN+).
func (h *AutomationHandlers) ListRules(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	rules, err := h.svc.List(r.Context(), tc.OrgID)
	if err != nil {
		response.Error(w, err)
		return
	}
	out := make([]automationRuleResp, 0, len(rules))
	for i := range rules {
		out = append(out, toAutomationRuleResp(&rules[i]))
	}
	response.JSON(w, http.StatusOK, map[string]any{"rules": out})
}

// CreateRule stores a rule (ADMIN+).
func (h *AutomationHandlers) CreateRule(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	var req automationRuleReq
	if !decodeJSON(w, r, &req) {
		return
	}
	rule, err := h.svc.Create(r.Context(), tc.OrgID, automationuc.CreateInput{
		Name: req.Name, Trigger: req.Trigger, TriggerConfig: req.TriggerConfig,
		Action: req.Action, ActionConfig: req.ActionConfig, CreatedBy: tc.UserID,
	})
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusCreated, toAutomationRuleResp(rule))
}

// UpdateRule replaces a rule's definition (ADMIN+).
func (h *AutomationHandlers) UpdateRule(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	var req automationRuleReq
	if !decodeJSON(w, r, &req) {
		return
	}
	rule, err := h.svc.Update(r.Context(), tc.OrgID, chi.URLParam(r, "ruleId"), automationuc.CreateInput{
		Name: req.Name, Trigger: req.Trigger, TriggerConfig: req.TriggerConfig,
		Action: req.Action, ActionConfig: req.ActionConfig, CreatedBy: tc.UserID,
	})
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusOK, toAutomationRuleResp(rule))
}

type automationRunResp struct {
	ID         string    `json:"id"`
	Trigger    string    `json:"trigger"`
	TaskID     string    `json:"task_id"`
	TaskTitle  string    `json:"task_title"`
	TaskNumber int       `json:"task_number"`
	ProjectKey string    `json:"project_key"`
	ActorID    string    `json:"actor_id,omitempty"`
	Status     string    `json:"status"`
	Error      string    `json:"error,omitempty"`
	Depth      int       `json:"depth"`
	CreatedAt  time.Time `json:"created_at"`
}

// ListRuns returns a rule's recent firings, newest first (ADMIN+).
func (h *AutomationHandlers) ListRuns(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	runs, err := h.svc.Runs(r.Context(), tc.OrgID, chi.URLParam(r, "ruleId"), 50)
	if err != nil {
		response.Error(w, err)
		return
	}
	out := make([]automationRunResp, 0, len(runs))
	for _, run := range runs {
		out = append(out, automationRunResp{
			ID: run.ID, Trigger: run.Trigger, TaskID: run.TaskID, TaskTitle: run.TaskTitle,
			TaskNumber: run.TaskNumber, ProjectKey: run.ProjectKey,
			ActorID: run.ActorID, Status: run.Status, Error: run.Error, Depth: run.Depth, CreatedAt: run.CreatedAt,
		})
	}
	response.JSON(w, http.StatusOK, map[string]any{"runs": out})
}

type automationEnabledReq struct {
	Enabled bool `json:"enabled"`
}

// SetRuleEnabled flips a rule without touching its definition (ADMIN+).
func (h *AutomationHandlers) SetRuleEnabled(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	var req automationEnabledReq
	if !decodeJSON(w, r, &req) {
		return
	}
	rule, err := h.svc.SetEnabled(r.Context(), tc.OrgID, chi.URLParam(r, "ruleId"), req.Enabled)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusOK, toAutomationRuleResp(rule))
}

// DeleteRule removes a rule (ADMIN+).
func (h *AutomationHandlers) DeleteRule(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	if err := h.svc.Delete(r.Context(), tc.OrgID, chi.URLParam(r, "ruleId")); err != nil {
		response.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
