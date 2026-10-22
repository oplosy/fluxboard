// Epic endpoints (docs/08, FR-PROJ-008). Project-scoped; reads need VIEWER,
// managing epics needs LEAD and assigning tasks CONTRIBUTOR (gated in projectuc).
package handlers

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/project"
	mw "github.com/mesutokul/fluxboard/backend/internal/interface/http/middleware"
	"github.com/mesutokul/fluxboard/backend/internal/interface/http/response"
)

type epicResp struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Color     string    `json:"color"`
	Total     int       `json:"total"`
	Done      int       `json:"done"`
	CreatedAt time.Time `json:"created_at"`
}

func toEpicResp(e *project.Epic) epicResp {
	return epicResp{ID: e.ID, Name: e.Name, Color: e.Color, Total: e.Total, Done: e.Done, CreatedAt: e.CreatedAt}
}

type epicReq struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

// ListEpics returns a project's epics with progress (VIEWER).
func (h *ProjectHandlers) ListEpics(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	epics, err := h.svc.ListEpics(r.Context(), tc.OrgID, tc.UserID, chi.URLParam(r, "projectId"), tc.Role)
	if err != nil {
		response.Error(w, err)
		return
	}
	out := make([]epicResp, 0, len(epics))
	for i := range epics {
		out = append(out, toEpicResp(&epics[i]))
	}
	response.JSON(w, http.StatusOK, map[string]any{"epics": out})
}

// CreateEpic stores an epic (LEAD).
func (h *ProjectHandlers) CreateEpic(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	var req epicReq
	if !decodeJSON(w, r, &req) {
		return
	}
	e, err := h.svc.CreateEpic(r.Context(), tc.OrgID, tc.UserID, chi.URLParam(r, "projectId"), req.Name, req.Color, tc.Role)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusCreated, toEpicResp(e))
}

// UpdateEpic renames or recolors an epic (LEAD).
func (h *ProjectHandlers) UpdateEpic(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	var req epicReq
	if !decodeJSON(w, r, &req) {
		return
	}
	e, err := h.svc.UpdateEpic(r.Context(), tc.OrgID, tc.UserID, chi.URLParam(r, "projectId"), chi.URLParam(r, "epicId"), req.Name, req.Color, tc.Role)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusOK, toEpicResp(e))
}

// DeleteEpic removes an epic; its tasks stay (LEAD).
func (h *ProjectHandlers) DeleteEpic(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	if err := h.svc.DeleteEpic(r.Context(), tc.OrgID, tc.UserID, chi.URLParam(r, "projectId"), chi.URLParam(r, "epicId"), tc.Role); err != nil {
		response.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type assignEpicReq struct {
	TaskIDs []string `json:"task_ids"`
	EpicID  *string  `json:"epic_id"`
}

// AssignEpic files tasks under an epic, null clears it (CONTRIBUTOR).
func (h *ProjectHandlers) AssignEpic(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	var req assignEpicReq
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := h.svc.AssignEpic(r.Context(), tc.OrgID, tc.UserID, chi.URLParam(r, "projectId"), req.TaskIDs, req.EpicID, tc.Role); err != nil {
		response.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
