// Sprint endpoints (docs/08, FR-SPRINT). Project-scoped; reads need VIEWER,
// writes need LEAD on the project (gated in projectuc).
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

type sprintResp struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	Goal           string     `json:"goal"`
	Status         string     `json:"status"`
	StartedAt      *time.Time `json:"started_at,omitempty"`
	EndedAt        *time.Time `json:"ended_at,omitempty"`
	CompletedTotal int        `json:"completed_total"`
	CompletedDone  int        `json:"completed_done"`
	// Story points snapshot at completion (FR-AN-003).
	CompletedPointsTotal int       `json:"completed_points_total"`
	CompletedPointsDone  int       `json:"completed_points_done"`
	CreatedAt            time.Time `json:"created_at"`
}

func toSprintResp(s *project.Sprint) sprintResp {
	return sprintResp{
		ID: s.ID, Name: s.Name, Goal: s.Goal, Status: string(s.Status),
		StartedAt: s.StartedAt, EndedAt: s.EndedAt,
		CompletedTotal: s.CompletedTotal, CompletedDone: s.CompletedDone,
		CompletedPointsTotal: s.CompletedPointsTotal, CompletedPointsDone: s.CompletedPointsDone,
		CreatedAt: s.CreatedAt,
	}
}

type createSprintReq struct {
	Name      string     `json:"name"`
	Goal      string     `json:"goal"`
	StartedAt *time.Time `json:"started_at"`
	EndedAt   *time.Time `json:"ended_at"`
}

// ListSprints returns a project's sprints (VIEWER).
func (h *ProjectHandlers) ListSprints(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	sprints, err := h.svc.ListSprints(r.Context(), tc.OrgID, tc.UserID, chi.URLParam(r, "projectId"), tc.Role)
	if err != nil {
		response.Error(w, err)
		return
	}
	out := make([]sprintResp, 0, len(sprints))
	for i := range sprints {
		out = append(out, toSprintResp(&sprints[i]))
	}
	response.JSON(w, http.StatusOK, map[string]any{"sprints": out})
}

// CreateSprint stores a planned sprint (LEAD).
func (h *ProjectHandlers) CreateSprint(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	var req createSprintReq
	if !decodeJSON(w, r, &req) {
		return
	}
	sp, err := h.svc.CreateSprint(r.Context(), tc.OrgID, tc.UserID, chi.URLParam(r, "projectId"), projectuc.CreateSprintInput{
		Name: req.Name, Goal: req.Goal, StartedAt: req.StartedAt, EndedAt: req.EndedAt,
	}, tc.Role)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusCreated, toSprintResp(sp))
}

// StartSprint activates a planned sprint (LEAD).
func (h *ProjectHandlers) StartSprint(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	sp, err := h.svc.StartSprint(r.Context(), tc.OrgID, tc.UserID, chi.URLParam(r, "projectId"), chi.URLParam(r, "sprintId"), tc.Role)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusOK, toSprintResp(sp))
}

// CompleteSprint closes the active sprint (LEAD).
func (h *ProjectHandlers) CompleteSprint(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	sp, err := h.svc.CompleteSprint(r.Context(), tc.OrgID, tc.UserID, chi.URLParam(r, "projectId"), chi.URLParam(r, "sprintId"), tc.Role)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusOK, toSprintResp(sp))
}

type burndownDayResp struct {
	Date            string `json:"date"` // YYYY-MM-DD, UTC
	RemainingPoints int    `json:"remaining_points"`
	RemainingTasks  int    `json:"remaining_tasks"`
}

// SprintBurndown returns remaining work per day of a started sprint (VIEWER).
func (h *ProjectHandlers) SprintBurndown(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	b, err := h.svc.SprintBurndown(r.Context(), tc.OrgID, tc.UserID, chi.URLParam(r, "projectId"), chi.URLParam(r, "sprintId"), tc.Role)
	if err != nil {
		response.Error(w, err)
		return
	}
	days := make([]burndownDayResp, 0, len(b.Days))
	for _, d := range b.Days {
		days = append(days, burndownDayResp{Date: d.Date.Format(time.DateOnly), RemainingPoints: d.RemainingPoints, RemainingTasks: d.RemainingTasks})
	}
	var end *string
	if b.End != nil {
		e := b.End.UTC().Format(time.DateOnly)
		end = &e
	}
	response.JSON(w, http.StatusOK, map[string]any{
		"start": b.Start.Format(time.DateOnly), "end": end,
		"total_points": b.TotalPoints, "total_tasks": b.TotalTasks, "days": days,
	})
}

// DeleteSprint removes a planned sprint (LEAD).
func (h *ProjectHandlers) DeleteSprint(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	if err := h.svc.DeleteSprint(r.Context(), tc.OrgID, tc.UserID, chi.URLParam(r, "projectId"), chi.URLParam(r, "sprintId"), tc.Role); err != nil {
		response.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type assignSprintReq struct {
	TaskIDs  []string `json:"task_ids"`
	SprintID *string  `json:"sprint_id"`
}

// AssignSprint moves tasks into a sprint (nil = backlog) (LEAD).
func (h *ProjectHandlers) AssignSprint(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	var req assignSprintReq
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := h.svc.AssignSprint(r.Context(), tc.OrgID, tc.UserID, chi.URLParam(r, "projectId"), req.TaskIDs, req.SprintID, tc.Role); err != nil {
		response.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
