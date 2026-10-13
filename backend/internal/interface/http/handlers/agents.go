// This file is the org settings agent surface (ADR-033, FR-AGT-002/003). Any
// member may list agents (they are teammates on the board); creating, editing,
// deleting and key management are ADMIN+ (write:agents). A key's secret is
// revealed exactly once, in the response that creates it.
package handlers

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/agent"
	"github.com/mesutokul/fluxboard/backend/internal/domain/tenant"
	mw "github.com/mesutokul/fluxboard/backend/internal/interface/http/middleware"
	"github.com/mesutokul/fluxboard/backend/internal/interface/http/response"
	"github.com/mesutokul/fluxboard/backend/internal/usecase/agentuc"
)

// AgentHandlers serves the org-scoped agent management endpoints.
type AgentHandlers struct {
	svc *agentuc.Service
	log *slog.Logger
}

// NewAgentHandlers builds AgentHandlers.
func NewAgentHandlers(svc *agentuc.Service, log *slog.Logger) *AgentHandlers {
	if log == nil {
		log = slog.Default()
	}
	return &AgentHandlers{svc: svc, log: log}
}

type agentKeyResp struct {
	ID         string     `json:"id"`
	Prefix     string     `json:"prefix"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

type agentResp struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Role      string         `json:"role"`
	Status    string         `json:"status"`
	CreatedBy string         `json:"created_by,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	Keys      []agentKeyResp `json:"keys"`
}

func toAgentResp(a agent.Agent) agentResp {
	keys := make([]agentKeyResp, 0, len(a.Keys))
	for _, k := range a.Keys {
		keys = append(keys, agentKeyResp{ID: k.ID, Prefix: k.Prefix, LastUsedAt: k.LastUsedAt, CreatedAt: k.CreatedAt})
	}
	return agentResp{
		ID: a.ID, Name: a.Name, Role: string(a.Role), Status: string(a.Status),
		CreatedBy: a.CreatedBy, CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt, Keys: keys,
	}
}

// actorID is the acting user, or "" for an org API key (no user to attribute).
func actorID(tc mw.TenantContext) string {
	if strings.HasPrefix(tc.UserID, "apikey:") {
		return ""
	}
	return tc.UserID
}

// List: GET /orgs/{orgId}/agents — the org's agents and its max_agents.
func (h *AgentHandlers) List(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	v, err := h.svc.List(r.Context(), tc.OrgID)
	if err != nil {
		response.Error(w, err)
		return
	}
	out := make([]agentResp, 0, len(v.Agents))
	for _, a := range v.Agents {
		out = append(out, toAgentResp(a))
	}
	response.JSON(w, http.StatusOK, map[string]any{
		"agents": out,
		"limit":  map[string]int{"current": len(v.Agents), "max": v.Max},
	})
}

type createAgentReq struct {
	Name string `json:"name"`
	Role string `json:"role"`
}

// Create: POST /orgs/{orgId}/agents — the agent plus its first key's secret, once.
func (h *AgentHandlers) Create(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	var req createAgentReq
	if !decodeJSON(w, r, &req) {
		return
	}
	c, err := h.svc.Create(r.Context(), tc.OrgID, actorID(tc), agentuc.CreateInput{
		Name: req.Name, Role: tenant.OrgRole(strings.ToUpper(strings.TrimSpace(req.Role))),
	})
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusCreated, map[string]any{
		"agent":  toAgentResp(c.Agent),
		"key_id": c.Key.ID,
		"secret": c.Plaintext, // shown once; never retrievable again
	})
}

type updateAgentReq struct {
	Name   *string `json:"name"`
	Role   *string `json:"role"`
	Status *string `json:"status"`
}

// Update: PATCH /orgs/{orgId}/agents/{agentId} — rename, re-role, pause/resume.
func (h *AgentHandlers) Update(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	var req updateAgentReq
	if !decodeJSON(w, r, &req) {
		return
	}
	in := agentuc.UpdateInput{Name: req.Name}
	if req.Role != nil {
		role := tenant.OrgRole(strings.ToUpper(strings.TrimSpace(*req.Role)))
		in.Role = &role
	}
	if req.Status != nil {
		st := agent.Status(strings.ToLower(strings.TrimSpace(*req.Status)))
		in.Status = &st
	}
	a, err := h.svc.Update(r.Context(), tc.OrgID, actorID(tc), chi.URLParam(r, "agentId"), in)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"agent": toAgentResp(a)})
}

// Delete: DELETE /orgs/{orgId}/agents/{agentId} — revokes its keys, frees the slot.
func (h *AgentHandlers) Delete(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	if err := h.svc.Delete(r.Context(), tc.OrgID, actorID(tc), chi.URLParam(r, "agentId")); err != nil {
		response.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// CreateKey: POST /orgs/{orgId}/agents/{agentId}/keys — another key's secret, once.
func (h *AgentHandlers) CreateKey(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	c, err := h.svc.CreateKey(r.Context(), tc.OrgID, actorID(tc), chi.URLParam(r, "agentId"))
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusCreated, map[string]any{
		"agent":  toAgentResp(c.Agent),
		"key_id": c.Key.ID,
		"secret": c.Plaintext,
	})
}

// RevokeKey: DELETE /orgs/{orgId}/agents/{agentId}/keys/{keyId}
func (h *AgentHandlers) RevokeKey(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	err := h.svc.RevokeKey(r.Context(), tc.OrgID, actorID(tc), chi.URLParam(r, "agentId"), chi.URLParam(r, "keyId"))
	if err != nil {
		response.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
