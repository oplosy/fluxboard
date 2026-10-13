package oauthgithub

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/oauth2"
)

func testProvider(t *testing.T, emails string) *Provider {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/login/oauth/access_token", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil || r.Form.Get("code_verifier") != "verifier" {
			http.Error(w, "missing verifier", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"tok","token_type":"bearer"}`))
	})
	mux.HandleFunc("/user", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":42,"login":"octo","name":""}`))
	})
	mux.HandleFunc("/user/emails", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(emails))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	p := New("id", "secret", "http://localhost/cb")
	p.cfg.Endpoint = oauth2.Endpoint{AuthURL: srv.URL + "/login/oauth/authorize", TokenURL: srv.URL + "/login/oauth/access_token"}
	p.apiBase = srv.URL
	return p
}

func TestExchangePicksVerifiedPrimaryEmail(t *testing.T) {
	p := testProvider(t, `[{"email":"old@x.io","primary":false,"verified":true},{"email":"me@x.io","primary":true,"verified":true}]`)
	u, err := p.Exchange(context.Background(), "code", "verifier")
	if err != nil {
		t.Fatal(err)
	}
	if u.Sub != "42" || u.Email != "me@x.io" || !u.EmailVerified || u.Name != "octo" {
		t.Fatalf("unexpected profile %+v", u)
	}
}

func TestExchangeUnverifiedPrimaryIsNotVerified(t *testing.T) {
	p := testProvider(t, `[{"email":"me@x.io","primary":true,"verified":false}]`)
	u, err := p.Exchange(context.Background(), "code", "verifier")
	if err != nil {
		t.Fatal(err)
	}
	if u.EmailVerified {
		t.Fatalf("unverified primary must not be reported verified: %+v", u)
	}
}
