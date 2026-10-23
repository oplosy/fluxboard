// Release endpoints (FR-PROJ-010). Project-scoped; reads need VIEWER,
// writes need LEAD, assignment needs CONTRIBUTOR (gated in projectuc).
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

type releaseResp struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	ReleaseDate *string    `json:"release_date"` // YYYY-MM-DD
	ReleasedAt  *time.Time `json:"released_at"`
	Total       int        `json:"total"`
	Done        int        `json:"done"`
	Points      int        `json:"points"`
	DonePoints  int        `json:"done_points"`
	CreatedAt   time.Time  `json:"created_at"`
}

func toReleaseResp(r *project.Release) releaseResp {
	var day *string
	if r.ReleaseDate != nil {
		d := r.ReleaseDate.Format(time.DateOnly)
		day = &d
	}
	return releaseResp{
		ID: r.ID, Name: r.Name, Description: r.Description, ReleaseDate: day, ReleasedAt: r.ReleasedAt,
		Total: r.Total, Done: r.Done, Points: r.Points, DonePoints: r.DonePoints, CreatedAt: r.CreatedAt,
	}
}

type releaseReq struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	ReleaseDate *string `json:"release_date"` // YYYY-MM-DD or null
	Released    bool    `json:"released"`
}

// input parses the planned day; a malformed date is a validation error.
func (req releaseReq) input() (projectuc.ReleaseInput, error) {
	in := projectuc.ReleaseInput{Name: req.Name, Description: req.Description, Released: req.Released}
	if req.ReleaseDate != nil && *req.ReleaseDate != "" {
		d, err := time.Parse(time.DateOnly, *req.ReleaseDate)
		if err != nil {
			return in, domain.ErrValidation
		}
		in.ReleaseDate = &d
	}
	return in, nil
}

// ListReleases returns a project's releases with progress (VIEWER).
func (h *ProjectHandlers) ListReleases(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	releases, err := h.svc.ListReleases(r.Context(), tc.OrgID, tc.UserID, chi.URLParam(r, "projectId"), tc.Role)
	if err != nil {
		response.Error(w, err)
		return
	}
	out := make([]releaseResp, 0, len(releases))
	for i := range releases {
		out = append(out, toReleaseResp(&releases[i]))
	}
	response.JSON(w, http.StatusOK, map[string]any{"releases": out})
}

// CreateRelease stores an unreleased release (LEAD).
func (h *ProjectHandlers) CreateRelease(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	var req releaseReq
	if !decodeJSON(w, r, &req) {
		return
	}
	in, err := req.input()
	if err != nil {
		response.Error(w, err)
		return
	}
	rel, err := h.svc.CreateRelease(r.Context(), tc.OrgID, tc.UserID, chi.URLParam(r, "projectId"), in, tc.Role)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusCreated, toReleaseResp(rel))
}

// UpdateRelease edits a release and marks it released or reopens it (LEAD).
func (h *ProjectHandlers) UpdateRelease(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	var req releaseReq
	if !decodeJSON(w, r, &req) {
		return
	}
	in, err := req.input()
	if err != nil {
		response.Error(w, err)
		return
	}
	rel, err := h.svc.UpdateRelease(r.Context(), tc.OrgID, tc.UserID, chi.URLParam(r, "projectId"), chi.URLParam(r, "releaseId"), in, tc.Role)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusOK, toReleaseResp(rel))
}

// DeleteRelease removes a release; its tasks stay (LEAD).
func (h *ProjectHandlers) DeleteRelease(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	if err := h.svc.DeleteRelease(r.Context(), tc.OrgID, tc.UserID, chi.URLParam(r, "projectId"), chi.URLParam(r, "releaseId"), tc.Role); err != nil {
		response.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type assignReleaseReq struct {
	TaskIDs   []string `json:"task_ids"`
	ReleaseID *string  `json:"release_id"`
}

// AssignRelease files tasks under a release, null clears it (CONTRIBUTOR).
func (h *ProjectHandlers) AssignRelease(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	var req assignReleaseReq
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := h.svc.AssignRelease(r.Context(), tc.OrgID, tc.UserID, chi.URLParam(r, "projectId"), req.TaskIDs, req.ReleaseID, tc.Role); err != nil {
		response.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
