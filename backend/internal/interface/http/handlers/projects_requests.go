// Request-inbox endpoints: public form submissions awaiting triage. All gated
// at CONTRIBUTOR in projectuc (rows carry submitter contact details).
package handlers

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/project"
	mw "github.com/mesutokul/fluxboard/backend/internal/interface/http/middleware"
	"github.com/mesutokul/fluxboard/backend/internal/interface/http/response"
	"github.com/mesutokul/fluxboard/backend/internal/usecase/projectuc"
)

type requestResp struct {
	ID                string  `json:"id"`
	FormID            string  `json:"form_id"`
	FormSchemaVersion int     `json:"form_schema_version"`
	Title             string  `json:"title"`
	Description       string  `json:"description"`
	Priority          string  `json:"priority"`
	SubmitterName     string  `json:"submitter_name"`
	SubmitterEmail    string  `json:"submitter_email"`
	Status            string  `json:"status"`
	OwnerID           string  `json:"owner_id"`
	DeclineReason     string  `json:"decline_reason"`
	TaskID            string  `json:"task_id"`
	ReceivedAt        string  `json:"received_at"`
	DecidedAt         *string `json:"decided_at"`
}

func toRequestResp(r *project.IntakeRequest) requestResp {
	out := requestResp{
		ID: r.ID, FormID: r.FormID, FormSchemaVersion: r.FormSchemaVersion, Title: r.Title,
		Description: r.Description, Priority: string(r.Priority), SubmitterName: r.SubmitterName,
		SubmitterEmail: r.SubmitterEmail, Status: string(r.Status), OwnerID: r.OwnerID,
		DeclineReason: r.DeclineReason, TaskID: r.TaskID,
		ReceivedAt: r.ReceivedAt.UTC().Format(time.RFC3339),
	}
	if r.DecidedAt != nil {
		d := r.DecidedAt.UTC().Format(time.RFC3339)
		out.DecidedAt = &d
	}
	return out
}

// ListRequests returns a project's inbox (?status= new|accepted|declined|done).
func (h *ProjectHandlers) ListRequests(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	rows, err := h.svc.ListRequests(r.Context(), tc.OrgID, tc.UserID, chi.URLParam(r, "projectId"), r.URL.Query().Get("status"), tc.Role)
	if err != nil {
		response.Error(w, err)
		return
	}
	out := make([]requestResp, 0, len(rows))
	for i := range rows {
		out = append(out, toRequestResp(&rows[i]))
	}
	response.JSON(w, http.StatusOK, map[string]any{"requests": out})
}

type triageRequestReq struct {
	OwnerID  string `json:"owner_id"`
	Priority string `json:"priority"`
}

// TriageRequest sets owner/priority on a new request.
func (h *ProjectHandlers) TriageRequest(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	var req triageRequestReq
	if !decodeJSON(w, r, &req) {
		return
	}
	got, err := h.svc.TriageRequest(r.Context(), tc.OrgID, tc.UserID, chi.URLParam(r, "projectId"), chi.URLParam(r, "requestId"),
		projectuc.TriageRequestInput{OwnerID: req.OwnerID, Priority: req.Priority}, tc.Role)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusOK, toRequestResp(got))
}

// AcceptRequest creates the task for a new request.
func (h *ProjectHandlers) AcceptRequest(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	t, err := h.svc.AcceptRequest(r.Context(), tc.OrgID, tc.UserID, chi.URLParam(r, "projectId"), chi.URLParam(r, "requestId"), tc.Role)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"task_id": t.ID, "number": t.Number})
}

type declineRequestReq struct {
	Reason string `json:"reason"`
}

// DeclineRequest closes a new request with a reason mailed to the submitter.
func (h *ProjectHandlers) DeclineRequest(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	var req declineRequestReq
	if !decodeJSON(w, r, &req) {
		return
	}
	got, err := h.svc.DeclineRequest(r.Context(), tc.OrgID, tc.UserID, chi.URLParam(r, "projectId"), chi.URLParam(r, "requestId"), req.Reason, tc.Role)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusOK, toRequestResp(got))
}
