package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/interface/http/response"
	"github.com/mesutokul/fluxboard/backend/internal/usecase/authuc"
)

func TestCodedErrorEnvelopeCarriesFieldDetails(t *testing.T) {
	rec := httptest.NewRecorder()
	response.Error(rec, domain.NewFieldError("password", "password_too_short", "Password must be at least 12 characters."))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d", rec.Code)
	}
	var env response.Envelope
	if err := json.NewDecoder(rec.Body).Decode(&env); err != nil {
		t.Fatal(err)
	}
	fields, _ := env.Error.Details["fields"].(map[string]any)
	if env.Error.Code != "password_too_short" || fields["password"] != "Password must be at least 12 characters." {
		t.Fatalf("unexpected envelope %+v", env.Error)
	}
}

func TestInternalErrorNeverLeaksCodedMessage(t *testing.T) {
	rec := httptest.NewRecorder()
	response.Error(rec, errors.New("pq: connection refused at 10.0.0.5"))
	if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), "10.0.0.5") {
		t.Fatalf("internal detail leaked: %d %s", rec.Code, rec.Body.String())
	}
}

func TestLoginThrottleExposesRetryAfterInBody(t *testing.T) {
	rec := httptest.NewRecorder()
	writeAuthError(rec, &authuc.RateLimitError{RetryAfter: 90_500_000_000}) // 90.5s
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") != "91" {
		t.Fatalf("status=%d retry-after=%q", rec.Code, rec.Header().Get("Retry-After"))
	}
	var env response.Envelope
	_ = json.NewDecoder(rec.Body).Decode(&env)
	if env.Error.Code != "rate_limited" || env.Error.Details["retry_after"] != float64(91) {
		t.Fatalf("unexpected envelope %+v", env.Error)
	}
}

func oauthTestHandlers() *AuthHandlers {
	return NewAuthHandlers(AuthConfig{
		Service:   authuc.New(authuc.Deps{}),
		WebOrigin: "https://app.example.com",
	})
}

func oauthRequest(target string) *http.Request {
	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, target, nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("provider", "google")
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func TestOAuthCallbackRejectsStateNotPinnedToBrowser(t *testing.T) {
	h := oauthTestHandlers()
	r := oauthRequest("/api/v1/auth/oauth/google/callback?code=c&state=attacker-state")
	r.AddCookie(&http.Cookie{Name: oauthStateCookie, Value: "victim-state"})
	rec := httptest.NewRecorder()
	h.OAuthCallback(rec, r)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "https://app.example.com/login?oauth_error=oauth_state_invalid" {
		t.Fatalf("Location = %q", loc)
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == refreshCookieName && c.Value != "" {
			t.Fatal("no session may be issued on a state mismatch")
		}
	}
}

func TestOAuthCallbackUserCancelled(t *testing.T) {
	h := oauthTestHandlers()
	rec := httptest.NewRecorder()
	h.OAuthCallback(rec, oauthRequest("/api/v1/auth/oauth/google/callback?error=access_denied"))
	if loc := rec.Header().Get("Location"); loc != "https://app.example.com/login?oauth_error=oauth_cancelled" {
		t.Fatalf("Location = %q", loc)
	}
}

func TestOAuthStartUnconfiguredProviderBouncesToLogin(t *testing.T) {
	h := oauthTestHandlers()
	rec := httptest.NewRecorder()
	h.OAuthStart(rec, oauthRequest("/api/v1/auth/oauth/google/start"))
	if loc := rec.Header().Get("Location"); loc != "https://app.example.com/login?oauth_error=oauth_unavailable" {
		t.Fatalf("Location = %q", loc)
	}
}

func TestSanitizeRedirect(t *testing.T) {
	for in, want := range map[string]string{
		"":                    "/app",
		"/app/acme":           "/app/acme",
		"//evil.com":          "/app",
		"/\\evil.com":         "/app",
		"https://evil.com":    "/app",
		"/settings?tab=a#b=c": "/settings?tab=a#b=c",
	} {
		if got := sanitizeRedirect(in); got != want {
			t.Errorf("sanitizeRedirect(%q) = %q, want %q", in, got, want)
		}
	}
}
