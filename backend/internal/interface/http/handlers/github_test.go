package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/mesutokul/fluxboard/backend/internal/domain/github"
	"github.com/mesutokul/fluxboard/backend/internal/domain/tenant"
	"github.com/mesutokul/fluxboard/backend/internal/usecase/githubuc"
)

const ghOrg = "0192a000-0000-7000-8000-00000000000a"

type ghCipher struct{}

func (ghCipher) Encrypt(p string) (string, error) { return p, nil }
func (ghCipher) Decrypt(c string) (string, error) { return c, nil }

type ghOrgs struct{ tenant.OrgRepository }

func (ghOrgs) GetByID(_ context.Context, id string) (*tenant.Organization, error) {
	return &tenant.Organization{ID: id}, nil
}

type ghRepo struct{ github.Repository }

func (ghRepo) Get(context.Context, string) (*github.Integration, error) {
	return &github.Integration{OrgID: ghOrg, SecretEnc: "topsecret"}, nil
}

func ghRouter() http.Handler {
	h := NewGithubHandlers(githubuc.New(githubuc.Deps{Repo: ghRepo{}, Orgs: ghOrgs{}, Cipher: ghCipher{}}), nil)
	r := chi.NewRouter()
	r.Post("/integrations/github/{orgRef}/webhook", h.Webhook)
	return r
}

func ghPost(body string, hdr map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/integrations/github/"+ghOrg+"/webhook", strings.NewReader(body))
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	ghRouter().ServeHTTP(rec, req)
	return rec
}

func TestGithubWebhookStatuses(t *testing.T) {
	ping := `{"zen":"x"}`
	signed := github.Sign("topsecret", []byte(ping))
	cases := []struct {
		name string
		body string
		hdr  map[string]string
		want int
	}{
		{"no signature", ping, map[string]string{"X-GitHub-Event": "ping"}, http.StatusUnauthorized},
		{"bad signature before parsing", "{not json", map[string]string{"X-GitHub-Event": "pull_request", "X-GitHub-Delivery": "d", "X-Hub-Signature-256": "sha256=00"}, http.StatusUnauthorized},
		{"valid ping", ping, map[string]string{"X-GitHub-Event": "ping", "X-Hub-Signature-256": signed}, http.StatusOK},
		{"unsupported event", ping, map[string]string{"X-GitHub-Event": "push", "X-Hub-Signature-256": signed}, http.StatusUnprocessableEntity},
		{"oversized body", strings.Repeat("a", maxGithubBody+1), map[string]string{"X-GitHub-Event": "ping"}, http.StatusRequestEntityTooLarge},
	}
	for _, c := range cases {
		if got := ghPost(c.body, c.hdr).Code; got != c.want {
			t.Errorf("%s: status %d, want %d", c.name, got, c.want)
		}
	}
}
