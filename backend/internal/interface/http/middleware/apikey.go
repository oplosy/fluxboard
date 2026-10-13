package middleware

import (
	"context"
	"errors"
	"net/http"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/agent"
	"github.com/mesutokul/fluxboard/backend/internal/domain/apikey"
	"github.com/mesutokul/fluxboard/backend/internal/domain/tenant"
	"github.com/mesutokul/fluxboard/backend/internal/interface/http/response"
	"github.com/mesutokul/fluxboard/backend/internal/pkg/reqmeta"
)

// ctxKeyAPIKey stores APIKeyInfo when a request authenticated via an API key.
const ctxKeyAPIKey ctxKey = 102

// APIKeyInfo is the resolved API-key context for a request (no user session).
// AgentID is set when the key belongs to an agent (ADR-033); the request then
// acts as that agent with AgentRole.
type APIKeyInfo struct {
	OrgID     string
	KeyID     string
	Scopes    []apikey.Scope
	AgentID   string
	AgentRole tenant.OrgRole
}

// Role is the org role the key acts with: an agent's own role, else the
// synthetic scope role of an org key (write⇒ADMIN, read⇒MEMBER) so the existing
// route gates apply; APIKeyScopeGuard enforces the write scope.
func (i APIKeyInfo) Role() tenant.OrgRole {
	if i.AgentID != "" {
		return i.AgentRole
	}
	if i.HasWrite() {
		return tenant.RoleAdmin
	}
	return tenant.RoleMember
}

// HasWrite reports whether the key carries the write scope.
func (i APIKeyInfo) HasWrite() bool {
	for _, s := range i.Scopes {
		if s == apikey.ScopeWrite {
			return true
		}
	}
	return false
}

// WithAPIKey stores info on ctx.
func WithAPIKey(ctx context.Context, info APIKeyInfo) context.Context {
	return context.WithValue(ctx, ctxKeyAPIKey, info)
}

// APIKeyFrom returns the APIKeyInfo and whether the request is API-key authed.
func APIKeyFrom(ctx context.Context) (APIKeyInfo, bool) {
	info, ok := ctx.Value(ctxKeyAPIKey).(APIKeyInfo)
	return info, ok
}

// APIKeyResolver authenticates an inbound key secret (hash → org + scopes).
// Implemented by apikeyuc.Service; declared here to keep middleware off the usecase.
type APIKeyResolver interface {
	ResolveByKey(ctx context.Context, plaintext string) (apikey.APIKey, error)
}

// HybridAuth resolves either an API key OR a user session on the /api/v1 surface
// (FR-API-002). An `Authorization: Bearer fbk_…` credential is resolved to its org
// + scopes (no user session); anything else falls through to the session
// authenticator. API-key principals are marked email-verified so the RequireVerified
// gate is a no-op for them (a key has no email to verify). An agent's key acts as
// the agent (its users row id); an org key as the synthetic "apikey:<id>".
// A paused agent is refused with 403 agent_paused, every other failure is 401.
func HybridAuth(a *Authenticator, keys APIKeyResolver) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		session := a.Authenticate(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, ok := bearerToken(r)
			if !ok || !apikey.LooksLikeKey(raw) {
				session.ServeHTTP(w, r)
				return
			}
			k, err := keys.ResolveByKey(r.Context(), raw)
			if err != nil {
				if errors.Is(err, agent.ErrPaused) {
					response.Error(w, agent.ErrPaused)
					return
				}
				response.Error(w, domain.ErrUnauthorized)
				return
			}
			id := "apikey:" + k.ID
			if k.AgentID != "" {
				id = k.AgentID
			}
			ctx := WithPrincipal(r.Context(), Principal{UserID: id, EmailVerified: true})
			ctx = WithAPIKey(ctx, APIKeyInfo{
				OrgID: k.OrgID, KeyID: k.ID, Scopes: k.Scopes,
				AgentID: k.AgentID, AgentRole: tenant.OrgRole(k.AgentRole),
			})
			ctx = reqmeta.WithActor(ctx, id)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// SessionOnly refuses API keys on user-scoped routes (/me, POST /orgs,
// invitation accept, …): a key — an agent's included — acts inside its org only
// and must never edit, delete or extend a user account (ADR-033).
func SessionOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := APIKeyFrom(r.Context()); ok {
			response.Error(w, domain.ErrForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// APIKeyScopeGuard enforces the key's scope on write requests: an API-key request
// that mutates (non GET/HEAD/OPTIONS) must carry the write scope, else 403. It runs
// after TenantGuard.Resolve. Session requests pass through untouched.
func APIKeyScopeGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info, ok := APIKeyFrom(r.Context())
		if ok {
			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				// read: any scope
			default:
				if !info.HasWrite() {
					response.Error(w, domain.ErrForbidden)
					return
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}
