// GitHub pull-request linking (FR-TASK-016, ADR-039). The webhook is the one
// route here mounted OUTSIDE session auth: GitHub carries no bearer token, so
// the usecase authenticates the request by verifying the HMAC signature of the
// raw body. The settings routes are org ADMIN+ (write:integrations).
package handlers

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	mw "github.com/mesutokul/fluxboard/backend/internal/interface/http/middleware"
	"github.com/mesutokul/fluxboard/backend/internal/interface/http/response"
	"github.com/mesutokul/fluxboard/backend/internal/usecase/githubuc"
)

// maxGithubBody caps the raw webhook body (GitHub itself caps payloads at 25 MB;
// pull_request events are far smaller).
const maxGithubBody = 1 << 20 // 1 MiB

// GithubHandlers serves the GitHub integration surface.
type GithubHandlers struct {
	svc    *githubuc.Service
	logger *slog.Logger
}

// NewGithubHandlers builds GithubHandlers.
func NewGithubHandlers(svc *githubuc.Service, logger *slog.Logger) *GithubHandlers {
	if logger == nil {
		logger = slog.Default()
	}
	return &GithubHandlers{svc: svc, logger: logger}
}

// Webhook receives a GitHub delivery. Status contract: 200 on any accepted
// delivery (processed, duplicate, ignored, ping), 401 for an unknown org or a
// bad/absent signature (indistinguishable), 422 for an unsupported event or
// malformed payload, 413 for an oversized body, 500 to make GitHub redeliver.
func (h *GithubHandlers) Webhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxGithubBody))
	if err != nil {
		status := http.StatusBadRequest
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			status = http.StatusRequestEntityTooLarge
		}
		w.WriteHeader(status)
		return
	}
	out, err := h.svc.HandleWebhook(r.Context(), githubuc.Delivery{
		OrgRef:     chi.URLParam(r, "orgRef"),
		Event:      r.Header.Get("X-GitHub-Event"),
		DeliveryID: r.Header.Get("X-GitHub-Delivery"),
		Signature:  r.Header.Get("X-Hub-Signature-256"),
		Body:       body,
	})
	switch {
	case err == nil:
		response.JSON(w, http.StatusOK, map[string]string{"status": string(out)})
	case errors.Is(err, githubuc.ErrUnsupportedEvent):
		response.Error(w, domain.NewCodedError(domain.ErrValidation, "unsupported_event", "unsupported event"))
	default:
		if !errors.Is(err, domain.ErrUnauthorized) && !errors.Is(err, domain.ErrValidation) {
			h.logger.ErrorContext(r.Context(), "github webhook failed", "err", err)
		}
		response.Error(w, err)
	}
}

type githubSettingsResp struct {
	Configured          bool     `json:"configured"`
	WebhookPath         string   `json:"webhook_path"`
	Repos               []string `json:"repos"`
	MoveOnMergeProjects []string `json:"move_on_merge_project_ids"`
}

func githubSettingsOut(orgID string, s *githubuc.Settings) githubSettingsResp {
	return githubSettingsResp{
		Configured: s.Configured, WebhookPath: "/api/v1/integrations/github/" + orgID + "/webhook",
		Repos: s.Repos, MoveOnMergeProjects: s.MoveProjects,
	}
}

// GetSettings returns the org's GitHub settings (ADMIN). The secret is never returned.
func (h *GithubHandlers) GetSettings(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	s, err := h.svc.GetSettings(r.Context(), tc.OrgID)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusOK, githubSettingsOut(tc.OrgID, s))
}

// RotateSecret creates or replaces the webhook secret and returns it once (ADMIN).
func (h *GithubHandlers) RotateSecret(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	secret, err := h.svc.RotateSecret(r.Context(), tc.OrgID, tc.UserID)
	if err != nil {
		response.Error(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	response.JSON(w, http.StatusOK, map[string]string{"secret": secret})
}

type githubSettingsReq struct {
	Repos               []string `json:"repos"`
	MoveOnMergeProjects []string `json:"move_on_merge_project_ids"`
}

// UpdateSettings replaces the allow-list and move-on-merge projects (ADMIN).
func (h *GithubHandlers) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	var req githubSettingsReq
	if !decodeJSON(w, r, &req) {
		return
	}
	s, err := h.svc.UpdateSettings(r.Context(), tc.OrgID, req.Repos, req.MoveOnMergeProjects)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusOK, githubSettingsOut(tc.OrgID, s))
}

// Disconnect removes the integration (ADMIN).
func (h *GithubHandlers) Disconnect(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	if err := h.svc.Disconnect(r.Context(), tc.OrgID); err != nil {
		response.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type pullRequestResp struct {
	Repo      string    `json:"repo"`
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	State     string    `json:"state"`
	URL       string    `json:"url"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ListTaskPullRequests returns the pull requests linked to a task (VIEWER+).
func (h *GithubHandlers) ListTaskPullRequests(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	prs, err := h.svc.ListForTask(r.Context(), tc.OrgID, tc.UserID, chi.URLParam(r, "taskId"), tc.Role)
	if err != nil {
		response.Error(w, err)
		return
	}
	out := make([]pullRequestResp, 0, len(prs))
	for _, p := range prs {
		out = append(out, pullRequestResp{Repo: p.Repo, Number: p.Number, Title: p.Title, State: p.State, URL: p.URL, UpdatedAt: p.UpdatedAt})
	}
	response.JSON(w, http.StatusOK, map[string]any{"pull_requests": out})
}
