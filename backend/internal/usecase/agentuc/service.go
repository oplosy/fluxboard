// Package agentuc is the application service for agents (ADR-033,
// FR-AGT-002/003): create, rename, pause/resume, delete, and rotate their keys.
// The plan cap (max_agents) is enforced at the route by the entitlement gate;
// this service reports it alongside the list. Domain-only deps.
package agentuc

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/agent"
	"github.com/mesutokul/fluxboard/backend/internal/domain/apikey"
	"github.com/mesutokul/fluxboard/backend/internal/domain/audit"
	"github.com/mesutokul/fluxboard/backend/internal/domain/billing"
	"github.com/mesutokul/fluxboard/backend/internal/domain/tenant"
	"github.com/mesutokul/fluxboard/backend/internal/pkg/uuidv7"
)

// Stable error codes the web client maps to messages.
var (
	errNameInvalid = domain.NewFieldError("name", "agent_name_invalid",
		"Use 1–40 letters, digits, spaces, dots, dashes or underscores.")
	errRoleInvalid = domain.NewFieldError("role", "agent_role_invalid",
		"An agent can be a Member or a Guest.")
	errStatusInvalid = domain.NewFieldError("status", "agent_status_invalid",
		"Status must be active or paused.")
	errNameTaken = &domain.CodedError{Kind: domain.ErrConflict, Code: "agent_name_taken",
		Field: "name", Message: "An agent with this name already exists."}
	errKeyLimit = domain.NewCodedError(domain.ErrConflict, "agent_key_limit",
		"An agent can have at most 3 active keys. Revoke one first.")
)

// Limits resolves the org's plan entitlements (billinguc.Service).
type Limits interface {
	Resolve(ctx context.Context, orgID string) (billing.Entitlements, error)
}

// Deps wires the service.
type Deps struct {
	Agents agent.Repository
	Limits Limits
	Audit  audit.Writer // nil ⇒ no audit (tests)
	Logger *slog.Logger
	Now    func() time.Time
}

// Service implements agent management.
type Service struct {
	agents agent.Repository
	limits Limits
	audit  audit.Writer
	logger *slog.Logger
	now    func() time.Time
}

// New builds the service.
func New(d Deps) *Service {
	if d.Now == nil {
		d.Now = time.Now
	}
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
	return &Service{agents: d.Agents, limits: d.Limits, audit: d.Audit, logger: d.Logger, now: d.Now}
}

// ListView is the org's agents plus the plan cap (-1 = unlimited).
type ListView struct {
	Agents []agent.Agent
	Max    int
}

// List returns the org's agents and its max_agents.
func (s *Service) List(ctx context.Context, orgID string) (ListView, error) {
	as, err := s.agents.List(ctx, orgID)
	if err != nil {
		return ListView{}, err
	}
	ent, err := s.limits.Resolve(ctx, orgID)
	if err != nil {
		return ListView{}, err
	}
	return ListView{Agents: as, Max: ent.MaxAgents}, nil
}

// Count returns the org's agents, paused included (the entitlement gate's counter).
func (s *Service) Count(ctx context.Context, orgID string) (int64, error) {
	return s.agents.Count(ctx, orgID)
}

// CreateInput is a new agent. Role defaults to MEMBER.
type CreateInput struct {
	Name string
	Role tenant.OrgRole
}

// Created is a new agent or key plus the key's one-time plaintext.
type Created struct {
	Agent     agent.Agent
	Key       apikey.APIKey
	Plaintext string // shown once; never retrievable again
}

// Create makes an agent with its first key. actor is the creating user ("" for
// an org API key).
func (s *Service) Create(ctx context.Context, orgID, actor string, in CreateInput) (Created, error) {
	name := strings.TrimSpace(in.Name)
	if !agent.ValidName(name) {
		return Created{}, errNameInvalid
	}
	if in.Role == "" {
		in.Role = tenant.RoleMember
	}
	if !agent.ValidRole(in.Role) {
		return Created{}, errRoleInvalid
	}
	now := s.now().UTC()
	a := agent.Agent{
		ID: uuidv7.New().String(), OrgID: orgID, Name: name, Role: in.Role,
		Status: agent.StatusActive, CreatedBy: actor, CreatedAt: now, UpdatedAt: now,
	}
	key, plaintext, err := newKey(a, actor, now)
	if err != nil {
		return Created{}, err
	}
	if err := s.agents.Create(ctx, a, key); err != nil {
		if errors.Is(err, domain.ErrConflict) {
			return Created{}, errNameTaken
		}
		return Created{}, err
	}
	a.Keys = []apikey.APIKey{key}
	s.append(ctx, orgID, actor, audit.ActionAgentCreate, a.ID, map[string]any{"name": name, "role": string(in.Role)})
	return Created{Agent: a, Key: key, Plaintext: plaintext}, nil
}

// UpdateInput changes any subset of an agent's name, role and status.
type UpdateInput struct {
	Name   *string
	Role   *tenant.OrgRole
	Status *agent.Status
}

// Update renames, re-roles, pauses or resumes an agent. A paused agent's keys
// are refused at authentication.
func (s *Service) Update(ctx context.Context, orgID, actor, id string, in UpdateInput) (agent.Agent, error) {
	a, err := s.agents.Get(ctx, orgID, id)
	if err != nil {
		return agent.Agent{}, err
	}
	changes := map[string]any{}
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if !agent.ValidName(name) {
			return agent.Agent{}, errNameInvalid
		}
		if name != a.Name {
			changes["name"] = name
			a.Name = name
		}
	}
	if in.Role != nil {
		if !agent.ValidRole(*in.Role) {
			return agent.Agent{}, errRoleInvalid
		}
		if *in.Role != a.Role {
			changes["role"] = string(*in.Role)
			a.Role = *in.Role
		}
	}
	if in.Status != nil {
		if !in.Status.Valid() {
			return agent.Agent{}, errStatusInvalid
		}
		if *in.Status != a.Status {
			changes["status"] = string(*in.Status)
			a.Status = *in.Status
		}
	}
	if len(changes) == 0 {
		return a, nil
	}
	if err := s.agents.Update(ctx, a); err != nil {
		if errors.Is(err, domain.ErrConflict) {
			return agent.Agent{}, errNameTaken
		}
		return agent.Agent{}, err
	}
	a.UpdatedAt = s.now().UTC()
	s.append(ctx, orgID, actor, audit.ActionAgentUpdate, a.ID, changes)
	return a, nil
}

// Delete removes an agent: its keys stop working and its plan slot frees up.
// Its past work keeps its name.
func (s *Service) Delete(ctx context.Context, orgID, actor, id string) error {
	if err := s.agents.Delete(ctx, orgID, id); err != nil {
		return err
	}
	s.append(ctx, orgID, actor, audit.ActionAgentDelete, id, nil)
	return nil
}

// CreateKey adds a key to an agent (rotation: add, switch, revoke the old one).
func (s *Service) CreateKey(ctx context.Context, orgID, actor, id string) (Created, error) {
	a, err := s.agents.Get(ctx, orgID, id)
	if err != nil {
		return Created{}, err
	}
	if len(a.Keys) >= agent.MaxKeys {
		return Created{}, errKeyLimit
	}
	key, plaintext, err := newKey(a, actor, s.now().UTC())
	if err != nil {
		return Created{}, err
	}
	if err := s.agents.AddKey(ctx, orgID, a.ID, key); err != nil {
		return Created{}, err
	}
	a.Keys = append(a.Keys, key)
	s.append(ctx, orgID, actor, audit.ActionAgentKeyCreate, a.ID, map[string]any{"prefix": key.Prefix})
	return Created{Agent: a, Key: key, Plaintext: plaintext}, nil
}

// RevokeKey revokes one of an agent's keys.
func (s *Service) RevokeKey(ctx context.Context, orgID, actor, id, keyID string) error {
	if err := s.agents.RevokeKey(ctx, orgID, id, keyID); err != nil {
		return err
	}
	s.append(ctx, orgID, actor, audit.ActionAgentKeyRevoke, id, map[string]any{"key_id": keyID})
	return nil
}

// newKey mints a read+write key for the agent; the agent's role, not the key
// scope, bounds what it may do.
func newKey(a agent.Agent, actor string, now time.Time) (apikey.APIKey, string, error) {
	g, err := apikey.Generate()
	if err != nil {
		return apikey.APIKey{}, "", err
	}
	return apikey.APIKey{
		ID: uuidv7.New().String(), OrgID: a.OrgID, Prefix: g.Prefix, KeyHash: g.KeyHash,
		Name: a.Name, Scopes: []apikey.Scope{apikey.ScopeRead, apikey.ScopeWrite},
		CreatedBy: actor, CreatedAt: now, AgentID: a.ID,
	}, g.Plaintext, nil
}

// append writes a best-effort audit row (never surfaced to the caller).
func (s *Service) append(ctx context.Context, orgID, actor string, action audit.Action, targetID string, meta map[string]any) {
	if s.audit == nil {
		return
	}
	if err := s.audit.Append(ctx, audit.Entry{
		OrgID: orgID, ActorUserID: actor, Action: action,
		TargetType: "agent", TargetID: targetID, Metadata: meta, Severity: audit.SeverityInfo,
	}); err != nil {
		s.logger.Warn("agent audit append failed", "action", action, "err", err)
	}
}
