// Recurring-task endpoints (research report P1). Reads need VIEWER, writes
// CONTRIBUTOR (gated in taskuc).
package handlers

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/project"
	mw "github.com/mesutokul/fluxboard/backend/internal/interface/http/middleware"
	"github.com/mesutokul/fluxboard/backend/internal/interface/http/response"
	"github.com/mesutokul/fluxboard/backend/internal/usecase/taskuc"
)

type recurrenceResp struct {
	ID       string `json:"id"`
	Freq     string `json:"freq"`
	Weekdays []int  `json:"weekdays"` // 0 = Sunday … 6
	MonthDay *int   `json:"month_day"`
	Timezone string `json:"timezone"`
	Mode     string `json:"mode"`
	Paused   bool   `json:"paused"`
	LastOn   string `json:"last_on"` // YYYY-MM-DD in the rule's timezone
}

func toRecurrenceResp(r *project.Recurrence) *recurrenceResp {
	if r == nil {
		return nil
	}
	days := make([]int, 0, len(r.Weekdays))
	for _, d := range r.Weekdays {
		days = append(days, int(d))
	}
	var monthDay *int
	if r.Freq == project.FreqMonthly {
		monthDay = &r.MonthDay
	}
	return &recurrenceResp{
		ID: r.ID, Freq: string(r.Freq), Weekdays: days, MonthDay: monthDay, Timezone: r.Timezone,
		Mode: string(r.Mode), Paused: r.Paused, LastOn: r.LastOn.Format(time.DateOnly),
	}
}

// GetTaskRecurrence returns the task's rule, or null when it does not repeat.
func (h *TaskHandlers) GetTaskRecurrence(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	rec, err := h.svc.GetRecurrence(r.Context(), tc.OrgID, tc.UserID, chi.URLParam(r, "taskId"), tc.Role)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusOK, toRecurrenceResp(rec))
}

type setRecurrenceReq struct {
	Freq     string `json:"freq"`
	Weekdays []int  `json:"weekdays"`
	MonthDay int    `json:"month_day"`
	Timezone string `json:"timezone"`
	Mode     string `json:"mode"`
	Paused   bool   `json:"paused"`
}

// SetTaskRecurrence creates, edits, pauses or resumes the task's rule.
func (h *TaskHandlers) SetTaskRecurrence(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	var req setRecurrenceReq
	if !decodeJSON(w, r, &req) {
		return
	}
	days := make([]time.Weekday, 0, len(req.Weekdays))
	for _, d := range req.Weekdays {
		days = append(days, time.Weekday(d)) // range-checked by Recurrence.Validate
	}
	rec, err := h.svc.SetRecurrence(r.Context(), tc.OrgID, tc.UserID, chi.URLParam(r, "taskId"), taskuc.RecurrenceInput{
		Freq: project.RecurrenceFreq(req.Freq), Weekdays: days, MonthDay: req.MonthDay,
		Timezone: req.Timezone, Mode: project.RecurrenceMode(req.Mode), Paused: req.Paused,
	}, tc.Role)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusOK, toRecurrenceResp(rec))
}

// StopTaskRecurrence removes the task's rule; generated occurrences stay.
func (h *TaskHandlers) StopTaskRecurrence(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	if err := h.svc.StopRecurrence(r.Context(), tc.OrgID, tc.UserID, chi.URLParam(r, "taskId"), tc.Role); err != nil {
		response.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
