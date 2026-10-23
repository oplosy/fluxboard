// Workload endpoint (FR-AN-004). Project-scoped, VIEWER (gated in projectuc).
package handlers

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	mw "github.com/mesutokul/fluxboard/backend/internal/interface/http/middleware"
	"github.com/mesutokul/fluxboard/backend/internal/interface/http/response"
)

type workloadRowResp struct {
	UserID               *string `json:"user_id"` // null = unassigned
	OpenTasks            int     `json:"open_tasks"`
	OpenPoints           int     `json:"open_points"`
	ScheduledTasks       int     `json:"scheduled_tasks"`
	ScheduledPoints      int     `json:"scheduled_points"`
	ScheduledUnestimated int     `json:"scheduled_unestimated"`
	OverdueTasks         int     `json:"overdue_tasks"`
	LoggedMinutes        int64   `json:"logged_minutes"`
}

// Workload returns per-assignee load for ?from=YYYY-MM-DD&to=YYYY-MM-DD (VIEWER).
func (h *ProjectHandlers) Workload(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	from, errFrom := time.Parse(time.DateOnly, r.URL.Query().Get("from"))
	to, errTo := time.Parse(time.DateOnly, r.URL.Query().Get("to"))
	if errFrom != nil || errTo != nil {
		response.Error(w, domain.ErrValidation)
		return
	}
	rows, err := h.svc.Workload(r.Context(), tc.OrgID, tc.UserID, chi.URLParam(r, "projectId"), from, to, tc.Role)
	if err != nil {
		response.Error(w, err)
		return
	}
	out := make([]workloadRowResp, 0, len(rows))
	for _, row := range rows {
		var user *string
		if row.UserID != "" {
			u := row.UserID
			user = &u
		}
		out = append(out, workloadRowResp{
			UserID: user, OpenTasks: row.OpenTasks, OpenPoints: row.OpenPoints,
			ScheduledTasks: row.ScheduledTasks, ScheduledPoints: row.ScheduledPoints,
			ScheduledUnestimated: row.ScheduledUnestimated, OverdueTasks: row.OverdueTasks,
			LoggedMinutes: row.LoggedSeconds / 60,
		})
	}
	response.JSON(w, http.StatusOK, map[string]any{
		"from": from.Format(time.DateOnly), "to": to.Format(time.DateOnly), "rows": out,
	})
}
