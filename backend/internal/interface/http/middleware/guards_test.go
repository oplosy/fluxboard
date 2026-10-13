package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/agent"
	"github.com/mesutokul/fluxboard/backend/internal/domain/apikey"
	"github.com/mesutokul/fluxboard/backend/internal/domain/tenant"
)

func runGuard(guard func(http.Handler) http.Handler, r *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	guard(next).ServeHTTP(rec, r)
	return rec
}

// FR-ADM-003: an impersonation token may read but never write.
func TestImpersonationReadOnly(t *testing.T) {
	tests := []struct {
		name         string
		method       string
		impersonated bool
		want         int
	}{
		{"read allowed", http.MethodGet, true, http.StatusOK},
		{"write blocked", http.MethodPost, true, http.StatusForbidden},
		{"patch blocked", http.MethodPatch, true, http.StatusForbidden},
		{"delete blocked", http.MethodDelete, true, http.StatusForbidden},
		{"non-impersonated write allowed", http.MethodPost, false, http.StatusOK},
		{"no tenant context passes", http.MethodPost, false, http.StatusOK},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequestWithContext(context.Background(), tc.method, "/orgs/org1/x", nil)
			if tc.name != "no tenant context passes" {
				ctx := context.WithValue(r.Context(), ctxKeyTenant,
					TenantContext{OrgID: "org1", Impersonated: tc.impersonated})
				r = r.WithContext(ctx)
			}
			rec := runGuard(ImpersonationReadOnly, r)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d", rec.Code, tc.want)
			}
		})
	}
}

// FR-ADM-003: an impersonation token is read-only everywhere, keyed off the
// Principal's `imp` claim — so it also guards the org-independent secured routes.
func TestRejectImpersonationWrite(t *testing.T) {
	tests := []struct {
		name   string
		method string
		imp    string
		want   int
	}{
		{"imp GET allowed", http.MethodGet, "org1", http.StatusOK},
		{"imp POST blocked", http.MethodPost, "org1", http.StatusForbidden},
		{"imp PATCH blocked", http.MethodPatch, "org1", http.StatusForbidden},
		{"imp DELETE blocked", http.MethodDelete, "org1", http.StatusForbidden},
		{"non-imp POST allowed", http.MethodPost, "", http.StatusOK},
		{"no principal passes", http.MethodPost, "\x00none", http.StatusOK},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequestWithContext(context.Background(), tc.method, "/me", nil)
			if tc.imp != "\x00none" {
				r = r.WithContext(WithPrincipal(r.Context(), Principal{UserID: "admin", ImpersonatedOrg: tc.imp}))
			}
			rec := runGuard(RejectImpersonationWrite, r)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d", rec.Code, tc.want)
			}
		})
	}
}

// FR-API-002: a read-scoped API key may read but never write; a write key may do both.
func TestAPIKeyScopeGuard(t *testing.T) {
	readKey := APIKeyInfo{OrgID: "org1", Scopes: []apikey.Scope{apikey.ScopeRead}}
	writeKey := APIKeyInfo{OrgID: "org1", Scopes: []apikey.Scope{apikey.ScopeWrite}}
	tests := []struct {
		name   string
		method string
		info   *APIKeyInfo
		want   int
	}{
		{"read key GET ok", http.MethodGet, &readKey, http.StatusOK},
		{"read key POST 403", http.MethodPost, &readKey, http.StatusForbidden},
		{"write key POST ok", http.MethodPost, &writeKey, http.StatusOK},
		{"no api-key context passes", http.MethodPost, nil, http.StatusOK},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequestWithContext(context.Background(), tc.method, "/orgs/org1/x", nil)
			if tc.info != nil {
				r = r.WithContext(WithAPIKey(r.Context(), *tc.info))
			}
			rec := runGuard(APIKeyScopeGuard, r)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d", rec.Code, tc.want)
			}
		})
	}
}

// ADR-033: an API key — an agent's included — never reaches user-scoped routes.
func TestSessionOnly(t *testing.T) {
	r := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/me", nil)
	if rec := runGuard(SessionOnly, r); rec.Code != http.StatusOK {
		t.Fatalf("session request: status = %d, want 200", rec.Code)
	}
	ctx := WithAPIKey(r.Context(), APIKeyInfo{OrgID: "org1", KeyID: "k1", AgentID: "a1", AgentRole: tenant.RoleMember})
	if rec := runGuard(SessionOnly, r.WithContext(ctx)); rec.Code != http.StatusForbidden {
		t.Fatalf("API-key request: status = %d, want 403", rec.Code)
	}
}

func TestAPIKeyInfoRole(t *testing.T) {
	cases := []struct {
		info APIKeyInfo
		want tenant.OrgRole
	}{
		{APIKeyInfo{Scopes: []apikey.Scope{apikey.ScopeWrite}}, tenant.RoleAdmin},
		{APIKeyInfo{Scopes: []apikey.Scope{apikey.ScopeRead}}, tenant.RoleMember},
		// An agent acts with its own role, whatever its key's scope.
		{APIKeyInfo{Scopes: []apikey.Scope{apikey.ScopeWrite}, AgentID: "a1", AgentRole: tenant.RoleGuest}, tenant.RoleGuest},
	}
	for _, c := range cases {
		if got := c.info.Role(); got != c.want {
			t.Errorf("Role(%+v) = %s, want %s", c.info, got, c.want)
		}
	}
}

type stubKeys struct {
	k   apikey.APIKey
	err error
}

func (s stubKeys) ResolveByKey(context.Context, string) (apikey.APIKey, error) { return s.k, s.err }

// ADR-033: an agent's key authenticates as the agent; a paused agent is 403
// agent_paused (not 401, so a client does not ask for a new key).
func TestHybridAuthAgentKey(t *testing.T) {
	run := func(keys APIKeyResolver) (*httptest.ResponseRecorder, Principal, APIKeyInfo) {
		var p Principal
		var info APIKeyInfo
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, _ = PrincipalFrom(r.Context())
			info, _ = APIKeyFrom(r.Context())
			w.WriteHeader(http.StatusOK)
		})
		r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil)
		r.Header.Set("Authorization", "Bearer "+apikey.Prefix+"abc")
		rec := httptest.NewRecorder()
		HybridAuth(&Authenticator{}, keys)(next).ServeHTTP(rec, r)
		return rec, p, info
	}

	rec, p, info := run(stubKeys{k: apikey.APIKey{ID: "k1", OrgID: "org1", AgentID: "a1", AgentRole: "MEMBER"}})
	if rec.Code != http.StatusOK || p.UserID != "a1" || info.AgentID != "a1" || info.Role() != tenant.RoleMember {
		t.Fatalf("agent key: %d %+v %+v", rec.Code, p, info)
	}
	rec, p, _ = run(stubKeys{k: apikey.APIKey{ID: "k2", OrgID: "org1"}})
	if rec.Code != http.StatusOK || p.UserID != "apikey:k2" {
		t.Fatalf("org key: %d %+v", rec.Code, p)
	}
	rec, _, _ = run(stubKeys{err: agent.ErrPaused})
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "agent_paused") {
		t.Fatalf("paused agent: %d %s", rec.Code, rec.Body)
	}
	rec, _, _ = run(stubKeys{err: domain.ErrUnauthorized})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("bad key: %d", rec.Code)
	}
}
