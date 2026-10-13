package handlers

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	mw "github.com/mesutokul/fluxboard/backend/internal/interface/http/middleware"
	"github.com/mesutokul/fluxboard/backend/internal/interface/http/response"
)

// --- Password: forgot / reset / change ------------------------------------

type forgotReq struct {
	Email string `json:"email"`
}

// PasswordForgot always returns 202 (no enumeration); the email is sent by the
// usecase when the account exists.
func (h *AuthHandlers) PasswordForgot(w http.ResponseWriter, r *http.Request) {
	var req forgotReq
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := h.svc.ForgotPassword(r.Context(), req.Email); err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusAccepted, map[string]string{
		"message": "If the email is valid, a reset link has been sent.",
	})
}

type resetReq struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

// PasswordResetValidate reports whether a reset link is still usable (204) or
// not (422 token_invalid) without consuming it, so the page can say so upfront.
func (h *AuthHandlers) PasswordResetValidate(w http.ResponseWriter, r *http.Request) {
	var req verifyEmailReq // reuse {token}
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := h.svc.ValidateResetToken(r.Context(), req.Token); err != nil {
		response.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// PasswordReset consumes a reset token and sets a new password (revokes all
// sessions). Clears the refresh cookie since every session is now dead.
func (h *AuthHandlers) PasswordReset(w http.ResponseWriter, r *http.Request) {
	var req resetReq
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := h.svc.ResetPassword(r.Context(), req.Token, req.Password); err != nil {
		response.Error(w, err)
		return
	}
	h.clearRefreshCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

type changeReq struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// PasswordChange updates the caller's password (requires auth) and revokes every
// session — the client must re-authenticate afterwards.
func (h *AuthHandlers) PasswordChange(w http.ResponseWriter, r *http.Request) {
	var req changeReq
	if !decodeJSON(w, r, &req) {
		return
	}
	p, _ := mw.PrincipalFrom(r.Context())
	if err := h.svc.ChangePassword(r.Context(), p.UserID, req.CurrentPassword, req.NewPassword, p.SID); err != nil {
		response.Error(w, err)
		return
	}
	h.clearRefreshCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

// --- Email verification (resend) ------------------------------------------

// VerifyEmailRequest re-sends the verification link. Always 202 (no enumeration).
func (h *AuthHandlers) VerifyEmailRequest(w http.ResponseWriter, r *http.Request) {
	var req forgotReq // reuse {email}
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := h.svc.ResendEmailVerify(r.Context(), req.Email); err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusAccepted, map[string]string{
		"message": "If the email is valid and unverified, a verification link has been sent.",
	})
}

// VerifyEmailResendSelf re-sends the verification link to the signed-in
// caller's own address (requires auth, not a verified email).
func (h *AuthHandlers) VerifyEmailResendSelf(w http.ResponseWriter, r *http.Request) {
	p, _ := mw.PrincipalFrom(r.Context())
	if err := h.svc.ResendEmailVerifyForUser(r.Context(), p.UserID); err != nil {
		response.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

type authMeResp struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	Name          string `json:"name"`
	EmailVerified bool   `json:"email_verified"`
	HasPassword   bool   `json:"has_password"`
}

// AuthMe is the minimal account view available before email verification
// (GET /me sits behind the verified-email gate).
func (h *AuthHandlers) AuthMe(w http.ResponseWriter, r *http.Request) {
	p, _ := mw.PrincipalFrom(r.Context())
	u, err := h.svc.Account(r.Context(), p.UserID)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusOK, authMeResp{
		ID: u.ID, Email: u.Email, Name: u.Name,
		EmailVerified: u.EmailVerified, HasPassword: u.HasPassword(),
	})
}

// --- Sessions -------------------------------------------------------------

type sessionResp struct {
	ID         string    `json:"id"`
	UserAgent  string    `json:"user_agent"`
	IP         string    `json:"ip,omitempty"`
	Current    bool      `json:"current"`
	CreatedAt  time.Time `json:"created_at"`
	LastUsedAt time.Time `json:"last_used_at"`
	ExpiresAt  time.Time `json:"expires_at"`
}

// Sessions lists the caller's live sessions, flagging the one making the request.
func (h *AuthHandlers) Sessions(w http.ResponseWriter, r *http.Request) {
	p, _ := mw.PrincipalFrom(r.Context())
	sessions, err := h.svc.ListSessions(r.Context(), p.UserID)
	if err != nil {
		response.Error(w, err)
		return
	}
	items := make([]sessionResp, 0, len(sessions))
	for _, s := range sessions {
		items = append(items, sessionResp{
			ID: s.ID, UserAgent: s.UserAgent, IP: s.IP,
			Current: s.ID == p.SID, CreatedAt: s.CreatedAt,
			LastUsedAt: s.LastUsedAt, ExpiresAt: s.ExpiresAt,
		})
	}
	response.JSON(w, http.StatusOK, map[string]any{"items": items})
}

// RevokeSession revokes one of the caller's sessions by id.
func (h *AuthHandlers) RevokeSession(w http.ResponseWriter, r *http.Request) {
	p, _ := mw.PrincipalFrom(r.Context())
	id := chi.URLParam(r, "id")
	if err := h.svc.RevokeSession(r.Context(), p.UserID, id); err != nil {
		response.Error(w, err)
		return
	}
	// Revoking the current session invalidates the refresh cookie too.
	if id == p.SID {
		h.clearRefreshCookie(w)
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- TOTP 2FA -------------------------------------------------------------

type enroll2FAResp struct {
	Secret          string `json:"secret"`
	ProvisioningURI string `json:"provisioning_uri"`
}

// Enroll2FA generates a TOTP secret for the caller (not yet active).
func (h *AuthHandlers) Enroll2FA(w http.ResponseWriter, r *http.Request) {
	p, _ := mw.PrincipalFrom(r.Context())
	res, err := h.svc.Enroll2FA(r.Context(), p.UserID)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusOK, enroll2FAResp{Secret: res.Secret, ProvisioningURI: res.ProvisioningURI})
}

type codeReq struct {
	Code string `json:"code"`
}

// Activate2FA confirms a code and turns on 2FA, returning one-time recovery codes.
func (h *AuthHandlers) Activate2FA(w http.ResponseWriter, r *http.Request) {
	var req codeReq
	if !decodeJSON(w, r, &req) {
		return
	}
	p, _ := mw.PrincipalFrom(r.Context())
	codes, err := h.svc.Activate2FA(r.Context(), p.UserID, req.Code)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"recovery_codes": codes})
}

// Disable2FA turns off 2FA after re-verifying a current code/recovery code.
func (h *AuthHandlers) Disable2FA(w http.ResponseWriter, r *http.Request) {
	var req codeReq
	if !decodeJSON(w, r, &req) {
		return
	}
	p, _ := mw.PrincipalFrom(r.Context())
	if err := h.svc.Disable2FA(r.Context(), p.UserID, req.Code); err != nil {
		response.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type verify2FAReq struct {
	PendingToken string `json:"pending_token"`
	Code         string `json:"code"`
}

// Verify2FA exchanges a pending-2FA token + code for a session.
func (h *AuthHandlers) Verify2FA(w http.ResponseWriter, r *http.Request) {
	var req verify2FAReq
	if !decodeJSON(w, r, &req) {
		return
	}
	tokens, err := h.svc.Verify2FA(r.Context(), req.PendingToken, req.Code, r.UserAgent(), clientIP(r))
	if err != nil {
		response.Error(w, err)
		return
	}
	h.writeSession(w, &tokens)
}

// --- OAuth: Google + GitHub (PKCE) ------------------------------------------

const (
	// oauthStateCookie pins a started flow to the browser that started it, so a
	// callback URL lured onto another browser cannot log that browser into the
	// attacker's account (login CSRF). Lax: the callback is a top-level GET
	// navigation coming back from the provider.
	oauthStateCookie     = "fluxboard_oauth_state"
	oauthStateCookiePath = "/api/v1/auth/oauth"
	oauthStateCookieTTL  = 10 * time.Minute
)

// OAuthProviders lists the configured sign-in providers so the web app only
// renders buttons that work.
func (h *AuthHandlers) OAuthProviders(w http.ResponseWriter, _ *http.Request) {
	providers := []string{}
	if h.states != nil {
		providers = h.svc.Providers()
	}
	response.JSON(w, http.StatusOK, map[string]any{"providers": providers})
}

// OAuthStart redirects the browser to the provider's consent screen. Failures
// are browser navigations too, so they redirect back to the login page with a
// code instead of rendering JSON.
func (h *AuthHandlers) OAuthStart(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	redirectAfter := sanitizeRedirect(r.URL.Query().Get("redirect"))
	start, err := h.svc.StartOAuth(r.Context(), h.states, provider, redirectAfter)
	if err != nil {
		h.oauthFail(w, r, provider, err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookie,
		Value:    start.State,
		Path:     oauthStateCookiePath,
		MaxAge:   int(oauthStateCookieTTL.Seconds()),
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, start.URL, http.StatusFound)
}

// OAuthCallback completes the flow: checks the state against the browser
// cookie, exchanges the code, sets the refresh cookie, and redirects to the web
// app, which mints its in-memory access token via /auth/refresh. No token ever
// travels in the URL. A TOTP account instead receives a short-lived pending
// token in the URL fragment (never sent to servers) to finish at /login/2fa.
func (h *AuthHandlers) OAuthCallback(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	q := r.URL.Query()
	h.clearOAuthStateCookie(w)
	if e := q.Get("error"); e != "" {
		code := "oauth_failed"
		if e == "access_denied" {
			code = "oauth_cancelled"
		}
		h.redirectLogin(w, r, code)
		return
	}
	state := q.Get("state")
	c, err := r.Cookie(oauthStateCookie)
	if err != nil || state == "" || subtle.ConstantTimeCompare([]byte(c.Value), []byte(state)) != 1 {
		h.redirectLogin(w, r, "oauth_state_invalid")
		return
	}
	res, err := h.svc.CompleteOAuth(r.Context(), h.states, provider, q.Get("code"), state, r.UserAgent(), clientIP(r))
	if err != nil {
		h.oauthFail(w, r, provider, err)
		return
	}
	dest := strings.TrimRight(h.webOrigin, "/") + "/oauth/callback?next=" + url.QueryEscape(sanitizeRedirect(res.RedirectAfter))
	if res.TwoFactor {
		http.Redirect(w, r, dest+"#pending_token="+url.QueryEscape(res.PendingToken), http.StatusFound)
		return
	}
	h.setRefreshCookie(w, res.Tokens.RefreshToken, res.Tokens.RefreshExpiresAt)
	http.Redirect(w, r, dest, http.StatusFound)
}

// oauthFail logs unexpected failures and sends the browser back to /login with
// the failure's code (coded domain errors) or a generic one.
func (h *AuthHandlers) oauthFail(w http.ResponseWriter, r *http.Request, provider string, err error) {
	code := "oauth_failed"
	var ce *domain.CodedError
	if errors.As(err, &ce) && ce.Code != "" {
		code = ce.Code
	} else {
		h.logger.ErrorContext(r.Context(), "oauth flow failed", "provider", provider, "err", err)
	}
	h.redirectLogin(w, r, code)
}

func (h *AuthHandlers) redirectLogin(w http.ResponseWriter, r *http.Request, code string) {
	http.Redirect(w, r, strings.TrimRight(h.webOrigin, "/")+"/login?oauth_error="+url.QueryEscape(code), http.StatusFound)
}

func (h *AuthHandlers) clearOAuthStateCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookie,
		Value:    "",
		Path:     oauthStateCookiePath,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

// sanitizeRedirect confines post-login redirects to same-origin paths so the
// flow cannot be turned into an open redirect. Anything not starting with a
// single "/" (or starting with "//" or "/\") collapses to "/app".
func sanitizeRedirect(p string) string {
	if p == "" || !strings.HasPrefix(p, "/") || strings.HasPrefix(p, "//") || strings.HasPrefix(p, "/\\") {
		return "/app"
	}
	return p
}
