// Saved board view endpoints (FR-PROJ-011). Project-scoped; personal views
// need VIEWER, shared views LEAD (gated in projectuc).
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

type savedViewResp struct {
	ID        string              `json:"id"`
	Name      string              `json:"name"`
	Shared    bool                `json:"shared"`
	Mine      bool                `json:"mine"`
	Filters   project.ViewFilters `json:"filters"`
	CreatedAt time.Time           `json:"created_at"`
}

func toSavedViewResp(v *project.SavedView, userID string) savedViewResp {
	return savedViewResp{ID: v.ID, Name: v.Name, Shared: v.Shared, Mine: v.OwnerID == userID, Filters: v.Filters, CreatedAt: v.CreatedAt}
}

type savedViewReq struct {
	Name    string              `json:"name"`
	Shared  bool                `json:"shared"`
	Filters project.ViewFilters `json:"filters"`
}

// ListViews returns the caller's own views and the project's shared ones (VIEWER).
func (h *ProjectHandlers) ListViews(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	views, err := h.svc.ListViews(r.Context(), tc.OrgID, tc.UserID, chi.URLParam(r, "projectId"), tc.Role)
	if err != nil {
		response.Error(w, err)
		return
	}
	out := make([]savedViewResp, 0, len(views))
	for i := range views {
		out = append(out, toSavedViewResp(&views[i], tc.UserID))
	}
	response.JSON(w, http.StatusOK, map[string]any{"views": out})
}

// CreateView saves a view: personal (VIEWER) or shared (LEAD).
func (h *ProjectHandlers) CreateView(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	var req savedViewReq
	if !decodeJSON(w, r, &req) {
		return
	}
	v, err := h.svc.CreateView(r.Context(), tc.OrgID, tc.UserID, chi.URLParam(r, "projectId"), req.Name, req.Shared, req.Filters, tc.Role)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusCreated, toSavedViewResp(v, tc.UserID))
}

// UpdateView renames a view, replaces its filters or changes its sharing.
func (h *ProjectHandlers) UpdateView(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	var req savedViewReq
	if !decodeJSON(w, r, &req) {
		return
	}
	v, err := h.svc.UpdateView(r.Context(), tc.OrgID, tc.UserID, chi.URLParam(r, "projectId"), chi.URLParam(r, "viewId"), req.Name, req.Shared, req.Filters, tc.Role)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusOK, toSavedViewResp(v, tc.UserID))
}

// DeleteView removes a view.
func (h *ProjectHandlers) DeleteView(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	if err := h.svc.DeleteView(r.Context(), tc.OrgID, tc.UserID, chi.URLParam(r, "projectId"), chi.URLParam(r, "viewId"), tc.Role); err != nil {
		response.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
