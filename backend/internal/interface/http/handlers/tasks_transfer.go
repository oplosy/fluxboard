package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	mw "github.com/mesutokul/fluxboard/backend/internal/interface/http/middleware"
	"github.com/mesutokul/fluxboard/backend/internal/interface/http/response"
	"github.com/mesutokul/fluxboard/backend/internal/usecase/taskuc"
)

// maxImportBodyBytes bounds an import request (CSV text inside JSON).
const maxImportBodyBytes = 4 << 20

type importReq struct {
	CSV     string               `json:"csv"`
	Mapping taskuc.ImportMapping `json:"mapping"`
}

func decodeImport(w http.ResponseWriter, r *http.Request) (importReq, bool) {
	var req importReq
	r.Body = http.MaxBytesReader(w, r.Body, maxImportBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		response.Error(w, errors.Join(domain.ErrValidation, err))
		return req, false
	}
	return req, true
}

// PreviewImport parses and validates a CSV against the project without
// writing (FR-TASK-012). Omit mapping to get a suggested one.
func (h *TaskHandlers) PreviewImport(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	req, ok := decodeImport(w, r)
	if !ok {
		return
	}
	p, err := h.svc.PreviewImport(r.Context(), tc.OrgID, tc.UserID, tc.Role, taskuc.ImportInput{
		ProjectID: chi.URLParam(r, "projectId"), CSV: req.CSV, Mapping: req.Mapping,
	})
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusOK, p)
}

// ImportTasks creates the CSV's tasks (FR-TASK-012). Idempotency-Key is
// required; a retry with the same key and payload never duplicates rows.
func (h *TaskHandlers) ImportTasks(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	req, ok := decodeImport(w, r)
	if !ok {
		return
	}
	res, err := h.svc.ImportTasks(r.Context(), tc.OrgID, tc.UserID, tc.Role, taskuc.ImportInput{
		ProjectID: chi.URLParam(r, "projectId"), CSV: req.CSV, Mapping: req.Mapping,
		IdemKey: r.Header.Get("Idempotency-Key"),
	})
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusOK, res)
}

// ExportProject downloads a project's tasks (FR-TASK-013): ?format=csv (default)
// or ?format=json (with comments, labels and attachment metadata).
func (h *TaskHandlers) ExportProject(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "csv"
	}
	if format != "csv" && format != "json" {
		response.Error(w, fmt.Errorf("%w: format must be csv or json", domain.ErrValidation))
		return
	}
	e, err := h.svc.ExportProject(r.Context(), tc.OrgID, tc.UserID, chi.URLParam(r, "projectId"), tc.Role, format == "json")
	if err != nil {
		response.Error(w, err)
		return
	}
	name := fmt.Sprintf("%s-tasks-%s.%s", e.Project.Key, e.ExportedAt.Format("2006-01-02"), format)
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	w.Header().Set("Cache-Control", "no-store")
	if format == "json" {
		response.JSON(w, http.StatusOK, e)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	_, _ = w.Write(taskuc.ExportCSV(e))
}
