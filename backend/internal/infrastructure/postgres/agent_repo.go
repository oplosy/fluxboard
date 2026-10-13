package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/agent"
	"github.com/mesutokul/fluxboard/backend/internal/domain/apikey"
	"github.com/mesutokul/fluxboard/backend/internal/domain/tenant"
	"github.com/mesutokul/fluxboard/backend/internal/infrastructure/postgres/gen"
)

// AgentRepo is the Postgres-backed agent.Repository. agents and api_keys are
// [T], so every method runs through TenantPool; the backing users row (global)
// is written in the same tx so an agent never exists without its identity.
type AgentRepo struct{ tp *TenantPool }

// NewAgentRepo builds the repo over the tenant pool.
func NewAgentRepo(tp *TenantPool) *AgentRepo { return &AgentRepo{tp: tp} }

var (
	_ agent.Repository = (*AgentRepo)(nil)
	_ agent.Lookup     = (*AgentRepo)(nil)
)

// IsAgent reports whether userID is (or was) one of the org's agents. A
// non-uuid principal (an org key, "apikey:<id>") is not.
func (r *AgentRepo) IsAgent(ctx context.Context, orgID, userID string) (bool, error) {
	id, err := uuid.Parse(userID)
	if err != nil {
		return false, nil
	}
	var ok bool
	err = r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		oid, _ := parseUUID(orgID)
		var err error
		ok, err = q.IsAgent(ctx, gen.IsAgentParams{OrgID: oid, ID: id})
		return err
	})
	return ok, err
}

// Create inserts the users row, the agent and its first key. A live agent with
// the same name (case-insensitive) ⇒ domain.ErrConflict.
func (r *AgentRepo) Create(ctx context.Context, a agent.Agent, key apikey.APIKey) error {
	err := r.tp.WithTenant(ctx, a.OrgID, func(q *gen.Queries) error {
		id, err := parseUUID(a.ID)
		if err != nil {
			return err
		}
		oid, _ := parseUUID(a.OrgID)
		createdBy, err := nullableUUID(ptrOrNil(a.CreatedBy))
		if err != nil {
			return err
		}
		if err := q.CreateAgentUser(ctx, gen.CreateAgentUserParams{
			ID: id, Email: agent.Email(a.ID), Name: a.Name,
		}); err != nil {
			return err
		}
		if err := q.CreateAgent(ctx, gen.CreateAgentParams{
			ID: id, OrgID: oid, Name: a.Name, Role: string(a.Role), CreatedBy: createdBy,
		}); err != nil {
			return err
		}
		return addAgentKey(ctx, q, oid, id, key)
	})
	if isUnique(err) {
		return domain.ErrConflict
	}
	return err
}

// List returns the org's live agents with their active keys.
func (r *AgentRepo) List(ctx context.Context, orgID string) ([]agent.Agent, error) {
	var out []agent.Agent
	err := r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		oid, _ := parseUUID(orgID)
		rows, err := q.ListAgents(ctx, oid)
		if err != nil {
			return err
		}
		keys, err := agentKeys(ctx, q, oid)
		if err != nil {
			return err
		}
		out = make([]agent.Agent, 0, len(rows))
		for _, row := range rows {
			a := agentFromRow(row)
			a.Keys = keys[a.ID]
			out = append(out, a)
		}
		return nil
	})
	return out, err
}

// Get returns one live agent with its active keys.
func (r *AgentRepo) Get(ctx context.Context, orgID, id string) (agent.Agent, error) {
	var out agent.Agent
	err := r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		oid, _ := parseUUID(orgID)
		aid, err := parseUUID(id)
		if err != nil {
			return domain.ErrNotFound
		}
		row, err := q.GetAgent(ctx, gen.GetAgentParams{OrgID: oid, ID: aid})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		keys, err := agentKeys(ctx, q, oid)
		if err != nil {
			return err
		}
		out = agentFromRow(row)
		out.Keys = keys[out.ID]
		return nil
	})
	return out, err
}

// Count returns the org's live agents (paused included).
func (r *AgentRepo) Count(ctx context.Context, orgID string) (int64, error) {
	var n int64
	err := r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		oid, _ := parseUUID(orgID)
		c, err := q.CountAgents(ctx, oid)
		n = c
		return err
	})
	return n, err
}

// Update saves name, role and status, and keeps the users row's name in step
// so the agent shows under its new name everywhere.
func (r *AgentRepo) Update(ctx context.Context, a agent.Agent) error {
	err := r.tp.WithTenant(ctx, a.OrgID, func(q *gen.Queries) error {
		oid, _ := parseUUID(a.OrgID)
		id, err := parseUUID(a.ID)
		if err != nil {
			return domain.ErrNotFound
		}
		n, err := q.UpdateAgent(ctx, gen.UpdateAgentParams{
			OrgID: oid, ID: id, Name: a.Name, Role: string(a.Role), Status: string(a.Status),
		})
		if err != nil {
			return err
		}
		if n == 0 {
			return domain.ErrNotFound
		}
		return q.UpdateUserName(ctx, gen.UpdateUserNameParams{ID: id, Name: a.Name})
	})
	if isUnique(err) {
		return domain.ErrConflict
	}
	return err
}

// Delete soft-deletes the agent and revokes every key it holds.
func (r *AgentRepo) Delete(ctx context.Context, orgID, id string) error {
	return r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		oid, _ := parseUUID(orgID)
		aid, err := parseUUID(id)
		if err != nil {
			return domain.ErrNotFound
		}
		n, err := q.DeleteAgent(ctx, gen.DeleteAgentParams{OrgID: oid, ID: aid})
		if err != nil {
			return err
		}
		if n == 0 {
			return domain.ErrNotFound
		}
		if err := q.RevokeAgentAPIKeys(ctx, gen.RevokeAgentAPIKeysParams{OrgID: oid, AgentID: pgUUIDOf(aid)}); err != nil {
			return err
		}
		return q.ReleaseAgentClaims(ctx, gen.ReleaseAgentClaimsParams{OrgID: oid, AgentID: pgUUIDOf(aid)})
	})
}

// AddKey stores another key for a live agent.
func (r *AgentRepo) AddKey(ctx context.Context, orgID, agentID string, key apikey.APIKey) error {
	return r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		oid, _ := parseUUID(orgID)
		aid, err := parseUUID(agentID)
		if err != nil {
			return domain.ErrNotFound
		}
		if _, err := q.GetAgent(ctx, gen.GetAgentParams{OrgID: oid, ID: aid}); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		return addAgentKey(ctx, q, oid, aid, key)
	})
}

// RevokeKey revokes one active key of the agent.
func (r *AgentRepo) RevokeKey(ctx context.Context, orgID, agentID, keyID string) error {
	return r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		oid, _ := parseUUID(orgID)
		aid, err := parseUUID(agentID)
		if err != nil {
			return domain.ErrNotFound
		}
		kid, err := parseUUID(keyID)
		if err != nil {
			return domain.ErrNotFound
		}
		n, err := q.RevokeAgentAPIKey(ctx, gen.RevokeAgentAPIKeyParams{OrgID: oid, AgentID: pgUUIDOf(aid), ID: kid})
		if err != nil {
			return err
		}
		if n == 0 {
			return domain.ErrNotFound
		}
		return nil
	})
}

// agentKeys returns the org's active agent keys grouped by agent id.
func agentKeys(ctx context.Context, q *gen.Queries, orgID uuid.UUID) (map[string][]apikey.APIKey, error) {
	rows, err := q.ListAgentAPIKeys(ctx, orgID)
	if err != nil {
		return nil, err
	}
	out := make(map[string][]apikey.APIKey)
	for _, row := range rows {
		k := apiKeyFromRow(row)
		out[k.AgentID] = append(out[k.AgentID], k)
	}
	return out, nil
}

func addAgentKey(ctx context.Context, q *gen.Queries, orgID, agentID uuid.UUID, key apikey.APIKey) error {
	kid, err := parseUUID(key.ID)
	if err != nil {
		return err
	}
	createdBy, err := nullableUUID(ptrOrNil(key.CreatedBy))
	if err != nil {
		return err
	}
	return q.CreateAgentAPIKey(ctx, gen.CreateAgentAPIKeyParams{
		ID: kid, OrgID: orgID, Prefix: key.Prefix, KeyHash: key.KeyHash, Name: key.Name,
		Scopes: scopesToStrings(key.Scopes), CreatedBy: createdBy, AgentID: pgUUIDOf(agentID),
	})
}

func pgUUIDOf(u uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: u, Valid: true} }

func agentFromRow(row gen.Agent) agent.Agent {
	return agent.Agent{
		ID:        row.ID.String(),
		OrgID:     row.OrgID.String(),
		Name:      row.Name,
		Role:      tenant.OrgRole(row.Role),
		Status:    agent.Status(row.Status),
		CreatedBy: derefStr(uuidStrPtr(row.CreatedBy)),
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
}
