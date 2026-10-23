// Org dashboard endpoints (FR-AN-005, ADR-038). Any org member; the data is
// limited to the projects the caller can see (gated in analyticsuc).
package handlers

import (
	"net/http"
	"time"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/analytics"
	mw "github.com/mesutokul/fluxboard/backend/internal/interface/http/middleware"
	"github.com/mesutokul/fluxboard/backend/internal/interface/http/response"
)

type widgetJSON struct {
	ID         string   `json:"id"`
	Type       string   `json:"type"`
	ProjectIDs []string `json:"project_ids"`
}

func toWidgetsResp(ws []analytics.Widget) []widgetJSON {
	out := make([]widgetJSON, 0, len(ws))
	for _, w := range ws {
		ids := w.ProjectIDs
		if ids == nil {
			ids = []string{}
		}
		out = append(out, widgetJSON{ID: w.ID, Type: string(w.Type), ProjectIDs: ids})
	}
	return out
}

// Dashboard returns per-project counts, per-assignee open work and weekly
// completions for the caller's visible projects.
func (h *AnalyticsHandlers) Dashboard(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	d, err := h.svc.Dashboard(r.Context(), tc.OrgID, tc.UserID, tc.Role)
	if err != nil {
		response.Error(w, err)
		return
	}
	projects := make([]map[string]any, 0, len(d.Projects))
	for _, p := range d.Projects {
		projects = append(projects, map[string]any{
			"project_id": p.ProjectID, "key": p.Key, "name": p.Name, "color": p.Color,
			"open": p.Open, "done": p.Done, "overdue": p.Overdue,
		})
	}
	assignees := make([]map[string]any, 0, len(d.Assignees))
	for _, a := range d.Assignees {
		var user *string // null = unassigned
		if a.UserID != "" {
			u := a.UserID
			user = &u
		}
		assignees = append(assignees, map[string]any{
			"project_id": a.ProjectID, "user_id": user, "open": a.Open, "overdue": a.Overdue,
		})
	}
	weekly := make([]map[string]any, 0, len(d.Weekly))
	for _, wk := range d.Weekly {
		weekly = append(weekly, map[string]any{
			"project_id": wk.ProjectID, "week": wk.Week.Format(time.DateOnly), "completed": wk.Completed,
		})
	}
	response.JSON(w, http.StatusOK, map[string]any{
		"week_from": d.WeekFrom.Format(time.DateOnly), "weeks": analytics.DashboardWeeks,
		"projects": projects, "assignees": assignees, "weekly": weekly,
	})
}

// DashboardLayout returns the caller's saved widgets (default when none).
func (h *AnalyticsHandlers) DashboardLayout(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	ws, err := h.svc.DashboardLayout(r.Context(), tc.OrgID, tc.UserID)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"widgets": toWidgetsResp(ws)})
}

// SaveDashboardLayout replaces the caller's saved widgets.
func (h *AnalyticsHandlers) SaveDashboardLayout(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	var req struct {
		Widgets []widgetJSON `json:"widgets"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	in := make([]analytics.Widget, 0, len(req.Widgets))
	for _, x := range req.Widgets {
		in = append(in, analytics.Widget{ID: x.ID, Type: analytics.WidgetType(x.Type), ProjectIDs: x.ProjectIDs})
	}
	ws, err := h.svc.SaveDashboardLayout(r.Context(), tc.OrgID, tc.UserID, in)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"widgets": toWidgetsResp(ws)})
}
