package authuc

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/auth"
	"github.com/mesutokul/fluxboard/backend/internal/pkg/uuidv7"
)

// oauthStateTTL bounds how long a started OAuth flow may take to call back
// (docs/04-AUTH.md §4: 10 min).
const oauthStateTTL = 10 * time.Minute

// Supported identity providers. The values are stored in
// oauth_identities.provider (CHECK constraint, migration 0033).
const (
	ProviderGoogle = "google"
	ProviderGitHub = "github"
)

// OAuth failure codes. The HTTP layer redirects the browser back to the web
// app with ?oauth_error=<code>, so each one maps to a specific message there.
var (
	errOAuthUnavailable = domain.NewCodedError(domain.ErrNotFound,
		"oauth_unavailable", "This sign-in provider is not available.")
	errOAuthState = domain.NewCodedError(domain.ErrUnauthorized,
		"oauth_state_invalid", "The sign-in attempt expired or was started in another browser. Try again.")
	errOAuthExchange = domain.NewCodedError(domain.ErrUnauthorized,
		"oauth_failed", "The provider did not confirm your sign-in. Try again.")
	errOAuthEmailUnverified = domain.NewCodedError(domain.ErrForbidden,
		"oauth_email_unverified", "Your provider account has no verified email address.")
	errOAuthVerifyFirst = domain.NewCodedError(domain.ErrConflict,
		"oauth_verify_first", "An unverified account already uses this email. Verify it first, then connect.")
)

// Providers lists the configured provider names, sorted.
func (s *Service) Providers() []string {
	out := make([]string, 0, len(s.providers))
	for name := range s.providers {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// OAuthStart is the outcome of StartOAuth: where to send the browser, and the
// state value the HTTP layer pins to the browser (cookie) for the callback.
type OAuthStart struct {
	URL   string
	State string
}

// StartOAuth generates PKCE + state, stores the verifier, and returns the
// provider consent URL to redirect the browser to (docs/04-AUTH.md §4).
func (s *Service) StartOAuth(ctx context.Context, states auth.OAuthStateStore, provider, redirectAfter string) (OAuthStart, error) {
	p, ok := s.providers[provider]
	if !ok || states == nil {
		return OAuthStart{}, errOAuthUnavailable
	}
	state, err := randB64(32)
	if err != nil {
		return OAuthStart{}, err
	}
	verifier, err := randB64(48) // 64 base64url chars, within 43–128 (RFC 7636)
	if err != nil {
		return OAuthStart{}, err
	}
	st := auth.OAuthState{Verifier: verifier, RedirectAfter: redirectAfter, Provider: provider}
	if err := states.Save(ctx, state, st, oauthStateTTL); err != nil {
		return OAuthStart{}, err
	}
	return OAuthStart{URL: p.AuthCodeURL(state, pkceChallenge(verifier)), State: state}, nil
}

// OAuthCallbackResult is the outcome of a successful callback: either a session
// (Tokens) or, for a TOTP-protected account, a pending second factor.
type OAuthCallbackResult struct {
	Tokens        Tokens
	TwoFactor     bool
	PendingToken  string
	RedirectAfter string
}

// CompleteOAuth validates state, exchanges the code, applies the identity
// linking rules (docs/04-AUTH.md §4), and issues a session — or a pending-2FA
// token when the account has TOTP on, so a provider login never skips the
// second factor.
func (s *Service) CompleteOAuth(ctx context.Context, states auth.OAuthStateStore, provider, code, state, userAgent, ip string) (OAuthCallbackResult, error) {
	p, ok := s.providers[provider]
	if !ok || states == nil {
		return OAuthCallbackResult{}, errOAuthUnavailable
	}
	if state == "" {
		return OAuthCallbackResult{}, errOAuthState
	}
	st, err := states.Take(ctx, state)
	if err != nil {
		return OAuthCallbackResult{}, errOAuthState // unknown/expired/replayed state
	}
	// A state minted for one provider must not complete another's flow. Empty
	// Provider = state saved before this field existed (Google only).
	if st.Provider != provider && (st.Provider != "" || provider != ProviderGoogle) {
		return OAuthCallbackResult{}, errOAuthState
	}
	if code == "" {
		return OAuthCallbackResult{}, errOAuthExchange
	}
	profile, err := p.Exchange(ctx, code, st.Verifier)
	if err != nil {
		slog.Default().WarnContext(ctx, "oauth exchange failed", "provider", provider, "err", err)
		return OAuthCallbackResult{}, errOAuthExchange
	}
	if profile.Sub == "" {
		return OAuthCallbackResult{}, errOAuthExchange
	}
	if !profile.EmailVerified || !validEmail(normalizeEmail(profile.Email)) {
		return OAuthCallbackResult{}, errOAuthEmailUnverified
	}

	u, err := s.resolveOAuthUser(ctx, provider, profile)
	if err != nil {
		return OAuthCallbackResult{}, err
	}
	if u.Locked() {
		s.observe(ctx).LoginAttempt(ctx, "locked")
		return OAuthCallbackResult{}, errAccountLocked
	}
	if u.TOTPEnabled {
		pending, err := s.signer.SignPending2FA(u.ID, s.clock.Now(), pending2FATTL)
		if err != nil {
			return OAuthCallbackResult{}, err
		}
		s.observe(ctx).LoginAttempt(ctx, "2fa_required")
		return OAuthCallbackResult{TwoFactor: true, PendingToken: pending, RedirectAfter: st.RedirectAfter}, nil
	}
	tokens, err := s.issueSession(ctx, u.ID, userAgent, ip)
	if err != nil {
		return OAuthCallbackResult{}, err
	}
	s.observe(ctx).LoginAttempt(ctx, "success")
	return OAuthCallbackResult{Tokens: tokens, RedirectAfter: st.RedirectAfter}, nil
}

// resolveOAuthUser applies the four-way linking decision and returns the user
// to log in.
func (s *Service) resolveOAuthUser(ctx context.Context, provider string, p auth.OAuthUser) (*auth.User, error) {
	// 1. Known identity → that user.
	id, err := s.oauth.GetByProviderSub(ctx, provider, p.Sub)
	if err == nil {
		return s.users.GetByID(ctx, id.UserID)
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}

	email := normalizeEmail(p.Email)
	u, err := s.users.GetByEmail(ctx, email)
	switch {
	case err == nil && u.EmailVerified:
		// 2. Existing verified account → link identity, login.
		if err := s.linkIdentity(ctx, u.ID, provider, p.Sub); err != nil {
			return nil, err
		}
		return u, nil
	case err == nil && !u.EmailVerified:
		// 3. Unverified local account → refuse silent takeover.
		return nil, errOAuthVerifyFirst
	case errors.Is(err, domain.ErrNotFound):
		// 4. New user: create verified, passwordless, link identity.
		return s.createOAuthUser(ctx, email, provider, p)
	default:
		return nil, err
	}
}

func (s *Service) createOAuthUser(ctx context.Context, email, provider string, p auth.OAuthUser) (*auth.User, error) {
	now := s.clock.Now()
	name := strings.TrimSpace(p.Name)
	if r := []rune(name); len(r) > maxNameLen {
		name = string(r[:maxNameLen])
	}
	u := &auth.User{
		ID:            uuidv7.New().String(),
		Email:         email,
		Name:          name,
		EmailVerified: true,
		PlatformRole:  auth.PlatformRoleUser,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := s.users.Create(ctx, u); err != nil {
		return nil, err
	}
	if err := s.linkIdentity(ctx, u.ID, provider, p.Sub); err != nil {
		return nil, err
	}
	return u, nil
}

func (s *Service) linkIdentity(ctx context.Context, userID, provider, sub string) error {
	return s.oauth.Create(ctx, &auth.OAuthIdentity{
		ID:          uuidv7.New().String(),
		UserID:      userID,
		Provider:    provider,
		ProviderSub: sub,
	})
}

// pkceChallenge is the RFC 7636 S256 transform: base64url(sha256(verifier)).
func pkceChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// randB64 returns n random bytes base64url-encoded (unpadded).
func randB64(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("oauth rand: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
