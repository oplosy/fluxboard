package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/auth"
	"github.com/mesutokul/fluxboard/backend/internal/domain/tenant"
	mw "github.com/mesutokul/fluxboard/backend/internal/interface/http/middleware"
	"github.com/mesutokul/fluxboard/backend/internal/usecase/useruc"
)

type exportUsers struct{ u *auth.User }

func (e exportUsers) GetByID(_ context.Context, id string) (*auth.User, error) {
	if e.u == nil || e.u.ID != id {
		return nil, domain.ErrNotFound
	}
	return e.u, nil
}
func (exportUsers) UpdateName(context.Context, string, string) error      { return nil }
func (exportUsers) UpdateAvatarKey(context.Context, string, string) error { return nil }
func (exportUsers) Delete(context.Context, string) error                  { return nil }

type exportIdentities []*auth.OAuthIdentity

func (e exportIdentities) ListByUser(context.Context, string) ([]*auth.OAuthIdentity, error) {
	return e, nil
}

func newExportHandlers() *UserHandlers {
	created := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	user := &auth.User{
		ID: "u1", Email: "ada@example.com", Name: "Ada", EmailVerified: true, TOTPEnabled: true,
		PasswordHash: "$argon2id$secret-hash", TOTPSecret: "encrypted-totp-secret", AvatarKey: "avatars/u1/x",
		CreatedAt: created,
	}
	svc := useruc.New(useruc.Deps{
		Users:      exportUsers{u: user},
		Identities: exportIdentities{{Provider: "google", ProviderSub: "g-123", CreatedAt: created}},
		Logger:     slog.New(slog.DiscardHandler),
	})
	h := NewUserHandlers(svc, slog.New(slog.DiscardHandler))
	h.Export = ExportSources{
		Sessions: func(context.Context, string) ([]*auth.Session, error) {
			return []*auth.Session{{ID: "s1", TokenHash: []byte("refresh-token-hash"), IP: "203.0.113.7", UserAgent: "Firefox", CreatedAt: created, LastUsedAt: created, ExpiresAt: created.Add(720 * time.Hour)}}, nil
		},
		Orgs: func(context.Context, string) ([]tenant.OrgMembership, error) {
			return []tenant.OrgMembership{{OrgID: "o1", Name: "Acme", Slug: "acme", Role: tenant.RoleAdmin, CreatedAt: created}}, nil
		},
	}
	return h
}

// GET /me/export returns the caller's own data as a download, never secrets
// (report B09; KVKK art. 11, GDPR art. 15/20).
func TestExportMe(t *testing.T) {
	h := newExportHandlers()
	req := httptest.NewRequestWithContext(mw.WithPrincipal(t.Context(), mw.Principal{UserID: "u1", SID: "s1"}),
		http.MethodGet, "/api/v1/me/export", nil)
	rec := httptest.NewRecorder()
	h.ExportMe(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if cd := rec.Header().Get("Content-Disposition"); !strings.HasPrefix(cd, `attachment; filename="fluxboard-personal-data-`) ||
		rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("headers: disposition %q cache %q", cd, rec.Header().Get("Cache-Control"))
	}
	body := rec.Body.String()
	for _, secret := range []string{"secret-hash", "encrypted-totp-secret", "refresh-token-hash", "avatars/u1"} {
		if strings.Contains(body, secret) {
			t.Errorf("export leaks %q", secret)
		}
	}
	var got exportResp
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.FormatVersion != 1 || got.Account.Email != "ada@example.com" || !got.Account.TwoFactorEnabled || !got.Account.HasAvatar {
		t.Errorf("account = %+v", got.Account)
	}
	if len(got.Identities) != 1 || got.Identities[0].Provider != "google" || got.Identities[0].Subject != "g-123" {
		t.Errorf("identities = %+v", got.Identities)
	}
	if len(got.Organizations) != 1 || got.Organizations[0].Slug != "acme" || got.Organizations[0].Role != "ADMIN" {
		t.Errorf("organizations = %+v", got.Organizations)
	}
	if len(got.Sessions) != 1 || got.Sessions[0].IP != "203.0.113.7" || got.Sessions[0].UserAgent != "Firefox" {
		t.Errorf("sessions = %+v", got.Sessions)
	}
	if got.ContentNote == "" {
		t.Error("export must say where content data lives")
	}
}

func TestExportMeRefusesImpersonation(t *testing.T) {
	h := newExportHandlers()
	req := httptest.NewRequestWithContext(
		mw.WithPrincipal(t.Context(), mw.Principal{UserID: "u1", ImpersonatedOrg: "o1"}),
		http.MethodGet, "/api/v1/me/export", nil)
	rec := httptest.NewRecorder()
	h.ExportMe(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("impersonation export = %d, want 403", rec.Code)
	}
}
