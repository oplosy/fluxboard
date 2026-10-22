// Watcher endpoints (docs/08, FR-TASK-014). Callers watch or unwatch a task
// for themselves only; every call needs VIEWER (gated in taskuc).
package handlers

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	mw "github.com/mesutokul/fluxboard/backend/internal/interface/http/middleware"
	"github.com/mesutokul/fluxboard/backend/internal/interface/http/response"
)

type watcherResp struct {
	UserID    string    `json:"user_id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

// ListTaskWatchers returns who watches the task.
func (h *TaskHandlers) ListTaskWatchers(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	ws, err := h.svc.ListWatchers(r.Context(), tc.OrgID, tc.UserID, chi.URLParam(r, "taskId"), tc.Role)
	if err != nil {
		response.Error(w, err)
		return
	}
	out := make([]watcherResp, 0, len(ws))
	for _, wt := range ws {
		out = append(out, watcherResp{UserID: wt.UserID, Name: wt.Name, Email: wt.Email, CreatedAt: wt.CreatedAt})
	}
	response.JSON(w, http.StatusOK, map[string]any{"watchers": out})
}

// WatchTask makes the caller watch the task (idempotent).
func (h *TaskHandlers) WatchTask(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	if err := h.svc.Watch(r.Context(), tc.OrgID, tc.UserID, chi.URLParam(r, "taskId"), tc.Role); err != nil {
		response.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// UnwatchTask stops the caller watching the task (idempotent).
func (h *TaskHandlers) UnwatchTask(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	if err := h.svc.Unwatch(r.Context(), tc.OrgID, tc.UserID, chi.URLParam(r, "taskId"), tc.Role); err != nil {
		response.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
