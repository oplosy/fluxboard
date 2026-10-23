// Wiki endpoints (FR-PROJ-013). Project-scoped; reads need VIEWER, writes
// CONTRIBUTOR, delete a LEAD or the author (gated in projectuc).
package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/project"
	mw "github.com/mesutokul/fluxboard/backend/internal/interface/http/middleware"
	"github.com/mesutokul/fluxboard/backend/internal/interface/http/response"
	"github.com/mesutokul/fluxboard/backend/internal/usecase/projectuc"
)

type wikiTaskResp struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
}

type wikiPageResp struct {
	ID        string         `json:"id"`
	Title     string         `json:"title"`
	Body      string         `json:"body,omitempty"` // absent in listings
	Version   int            `json:"version"`
	CreatedBy string         `json:"created_by"`
	UpdatedBy string         `json:"updated_by"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	Tasks     []wikiTaskResp `json:"tasks,omitempty"` // live tasks the body names
}

func toWikiPageResp(p *project.WikiPage) wikiPageResp {
	return wikiPageResp{
		ID: p.ID, Title: p.Title, Body: p.Body, Version: p.Version,
		CreatedBy: p.CreatedBy, UpdatedBy: p.UpdatedBy, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

func toWikiViewResp(v *projectuc.WikiPageView) wikiPageResp {
	out := toWikiPageResp(v.Page)
	for _, t := range v.Tasks {
		out.Tasks = append(out.Tasks, wikiTaskResp{Number: t.Number, Title: t.Title})
	}
	return out
}

func wikiListResp(pages []project.WikiPage) map[string]any {
	out := make([]wikiPageResp, 0, len(pages))
	for i := range pages {
		out = append(out, toWikiPageResp(&pages[i]))
	}
	return map[string]any{"pages": out}
}

type wikiPageReq struct {
	Title   string `json:"title"`
	Body    string `json:"body"`
	Version int    `json:"version"` // update only: the version the editor read
}

// ListWikiPages returns a project's pages by title; ?q= filters titles (VIEWER).
func (h *ProjectHandlers) ListWikiPages(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	pages, err := h.svc.ListWikiPages(r.Context(), tc.OrgID, tc.UserID, chi.URLParam(r, "projectId"), r.URL.Query().Get("q"), tc.Role)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusOK, wikiListResp(pages))
}

// WikiPagesMentioningTask lists the pages naming ?task_number= (VIEWER).
func (h *ProjectHandlers) WikiPagesMentioningTask(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	n, err := strconv.Atoi(r.URL.Query().Get("task_number"))
	if err != nil || n < 1 {
		response.Error(w, domain.ErrValidation)
		return
	}
	pages, err := h.svc.WikiPagesMentioningTask(r.Context(), tc.OrgID, tc.UserID, chi.URLParam(r, "projectId"), n, tc.Role)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusOK, wikiListResp(pages))
}

// CreateWikiPage stores a page (CONTRIBUTOR).
func (h *ProjectHandlers) CreateWikiPage(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	var req wikiPageReq
	if !decodeJSON(w, r, &req) {
		return
	}
	v, err := h.svc.CreateWikiPage(r.Context(), tc.OrgID, tc.UserID, chi.URLParam(r, "projectId"), req.Title, req.Body, tc.Role)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusCreated, toWikiViewResp(v))
}

// GetWikiPage returns one page with its body and mentioned tasks (VIEWER).
func (h *ProjectHandlers) GetWikiPage(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	v, err := h.svc.GetWikiPage(r.Context(), tc.OrgID, tc.UserID, chi.URLParam(r, "projectId"), chi.URLParam(r, "wikiId"), tc.Role)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusOK, toWikiViewResp(v))
}

// UpdateWikiPage edits a page; a stale version is a 409 (CONTRIBUTOR).
func (h *ProjectHandlers) UpdateWikiPage(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	var req wikiPageReq
	if !decodeJSON(w, r, &req) {
		return
	}
	v, err := h.svc.UpdateWikiPage(r.Context(), tc.OrgID, tc.UserID, chi.URLParam(r, "projectId"), chi.URLParam(r, "wikiId"), req.Title, req.Body, req.Version, tc.Role)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusOK, toWikiViewResp(v))
}

// DeleteWikiPage removes a page (LEAD or author).
func (h *ProjectHandlers) DeleteWikiPage(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	if err := h.svc.DeleteWikiPage(r.Context(), tc.OrgID, tc.UserID, chi.URLParam(r, "projectId"), chi.URLParam(r, "wikiId"), tc.Role); err != nil {
		response.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
