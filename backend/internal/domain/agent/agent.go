// Package agent is the domain for agents: org-owned automated workers that
// connect over MCP with their own keys (ADR-033, FR-AGT-002/003). An agent is
// team-shared (any ADMIN+ manages it), acts with a non-admin org role, never
// counts as a seat, and is capped per plan by max_agents.
//
// Its identity is a users row with the same id, so tasks, comments and
// activity can reference it like a person. That row has no password, an
// unroutable address (Email) and no membership, so it can never sign in.
package agent

import (
	"context"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/apikey"
	"github.com/mesutokul/fluxboard/backend/internal/domain/tenant"
)

// Status is whether an agent may act. A paused agent's keys are refused.
type Status string

const (
	StatusActive Status = "active"
	StatusPaused Status = "paused"
)

// Valid reports whether s is a known status.
func (s Status) Valid() bool { return s == StatusActive || s == StatusPaused }

// ErrPaused refuses a paused agent's key (403, not 401: the key is valid, the
// agent is on hold, so a client should not prompt for a new key).
var ErrPaused = domain.NewCodedError(domain.ErrForbidden, "agent_paused",
	"This agent is paused. An org admin can resume it in Settings → Agents.")

// MaxNameLen bounds an agent name (runes).
const MaxNameLen = 40

// MaxKeys bounds an agent's active keys: enough to rotate without downtime.
const MaxKeys = 3

// emailDomain is RFC 2606 reserved: mail to it can never be delivered.
const emailDomain = "agents.fluxboard.invalid"

// Email is the address of an agent's backing users row.
func Email(id string) string { return "agent-" + id + "@" + emailDomain }

// IsAgentEmail reports whether an address belongs to an agent (never mailed).
func IsAgentEmail(email string) bool {
	return strings.HasSuffix(strings.ToLower(email), "@"+emailDomain)
}

// ValidName reports whether name is usable: 1..MaxNameLen runes of letters,
// digits, space, '-', '_' or '.', not blank. Callers trim first.
func ValidName(name string) bool {
	if name == "" || utf8.RuneCountInString(name) > MaxNameLen {
		return false
	}
	for _, r := range name {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && !strings.ContainsRune(" -_.", r) {
			return false
		}
	}
	return true
}

// ValidRole reports whether an agent may hold role: never OWNER/ADMIN, so an
// agent cannot manage members, billing, keys or other agents.
func ValidRole(role tenant.OrgRole) bool {
	return role == tenant.RoleMember || role == tenant.RoleGuest
}

// Agent is an org's agent with its active keys (secrets never included).
type Agent struct {
	ID        string
	OrgID     string
	Name      string
	Role      tenant.OrgRole
	Status    Status
	CreatedBy string
	CreatedAt time.Time
	UpdatedAt time.Time
	Keys      []apikey.APIKey
}

// Repository persists agents. agents is tenant-owned ([T]); every method runs
// under a tenant tx. Deleted agents are invisible to every method.
type Repository interface {
	// Create inserts the backing users row, the agent and its first key atomically.
	Create(ctx context.Context, a Agent, key apikey.APIKey) error
	// List returns the org's agents with their active keys, oldest first.
	List(ctx context.Context, orgID string) ([]Agent, error)
	// Get returns one agent with its active keys; domain.ErrNotFound if absent.
	Get(ctx context.Context, orgID, id string) (Agent, error)
	// Count returns the org's agents, paused included (the plan cap counts them).
	Count(ctx context.Context, orgID string) (int64, error)
	// Update saves name, role and status (and renames the backing user).
	Update(ctx context.Context, a Agent) error
	// Delete soft-deletes the agent, revokes its keys and releases its task
	// claims (and the assignments they made); ErrNotFound if absent.
	Delete(ctx context.Context, orgID, id string) error
	// AddKey stores another key for the agent.
	AddKey(ctx context.Context, orgID, agentID string, key apikey.APIKey) error
	// RevokeKey revokes one of the agent's keys; ErrNotFound if absent.
	RevokeKey(ctx context.Context, orgID, agentID, keyID string) error
}
