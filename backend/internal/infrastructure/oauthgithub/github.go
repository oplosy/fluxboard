// Package oauthgithub implements auth.OAuthProvider for GitHub using
// golang.org/x/oauth2 with PKCE (docs/04-AUTH.md §4). GitHub has no OpenID
// userinfo, so the profile comes from GET /user and the verified primary
// address from GET /user/emails.
package oauthgithub

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"

	"github.com/mesutokul/fluxboard/backend/internal/domain/auth"
)

const (
	apiBase     = "https://api.github.com"
	httpTimeout = 10 * time.Second
)

// Provider is a configured GitHub OAuth client.
type Provider struct {
	cfg     *oauth2.Config
	apiBase string
	http    *http.Client
}

// New builds a Provider from OAuth App credentials and the registered callback URL.
func New(clientID, clientSecret, redirectURL string) *Provider {
	return &Provider{
		cfg: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Endpoint:     github.Endpoint,
			Scopes:       []string{"read:user", "user:email"},
		},
		apiBase: apiBase,
		http:    &http.Client{Timeout: httpTimeout},
	}
}

var _ auth.OAuthProvider = (*Provider)(nil)

// AuthCodeURL builds the consent URL carrying the PKCE S256 challenge.
func (p *Provider) AuthCodeURL(state, codeChallenge string) string {
	return p.cfg.AuthCodeURL(state,
		oauth2.SetAuthURLParam("code_challenge", codeChallenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
		oauth2.SetAuthURLParam("allow_signup", "true"),
	)
}

// Exchange swaps code+verifier for a token and fetches the profile plus the
// verified primary email. EmailVerified is false when GitHub reports no
// verified primary address.
func (p *Provider) Exchange(ctx context.Context, code, codeVerifier string) (auth.OAuthUser, error) {
	ctx = context.WithValue(ctx, oauth2.HTTPClient, p.http)
	tok, err := p.cfg.Exchange(ctx, code, oauth2.SetAuthURLParam("code_verifier", codeVerifier))
	if err != nil {
		return auth.OAuthUser{}, fmt.Errorf("github exchange: %w", err)
	}
	client := p.cfg.Client(ctx, tok)

	var user struct {
		ID    int64  `json:"id"`
		Login string `json:"login"`
		Name  string `json:"name"`
	}
	if err := p.get(ctx, client, "/user", &user); err != nil {
		return auth.OAuthUser{}, err
	}
	if user.ID == 0 {
		return auth.OAuthUser{}, fmt.Errorf("github user: missing id")
	}
	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := p.get(ctx, client, "/user/emails", &emails); err != nil {
		return auth.OAuthUser{}, err
	}
	out := auth.OAuthUser{Sub: strconv.FormatInt(user.ID, 10), Name: user.Name}
	if out.Name == "" {
		out.Name = user.Login
	}
	for _, e := range emails {
		if e.Primary && e.Verified {
			out.Email, out.EmailVerified = e.Email, true
			break
		}
	}
	return out, nil
}

func (p *Provider) get(ctx context.Context, client *http.Client, path string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.apiBase+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("github %s: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("github %s: status %d", path, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		return fmt.Errorf("github %s decode: %w", path, err)
	}
	return nil
}
