// Task approval endpoints (ADR-037). Fine-grained gates live in taskuc.
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

type approverResp struct {
	UserID    string     `json:"user_id"`
	Decision  string     `json:"decision"` // "" while waiting
	Comment   string     `json:"comment"`
	DecidedAt *time.Time `json:"decided_at"`
}

type approvalResp struct {
	ID          string         `json:"id"`
	TaskID      string         `json:"task_id"`
	RequestedBy string         `json:"requested_by"`
	Note        string         `json:"note"`
	Deadline    *time.Time     `json:"deadline"`
	State       string         `json:"state"`
	Stale       bool           `json:"stale"`
	DecidedAt   *time.Time     `json:"decided_at"`
	CreatedAt   time.Time      `json:"created_at"`
	Approvers   []approverResp `json:"approvers"`
}

func toApprovalResp(a *project.Approval) approvalResp {
	r := approvalResp{
		ID: a.ID, TaskID: a.TaskID, RequestedBy: a.RequestedBy, Note: a.Note, Deadline: a.Deadline,
		State: string(a.State), Stale: a.Stale(), DecidedAt: a.DecidedAt, CreatedAt: a.CreatedAt,
		Approvers: make([]approverResp, 0, len(a.Approvers)),
	}
	for _, p := range a.Approvers {
		r.Approvers = append(r.Approvers, approverResp{
			UserID: p.UserID, Decision: string(p.Decision), Comment: p.Comment, DecidedAt: p.DecidedAt,
		})
	}
	return r
}

// ListTaskApprovals returns the task's approval requests, newest first.
func (h *TaskHandlers) ListTaskApprovals(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	as, err := h.svc.ListApprovals(r.Context(), tc.OrgID, tc.UserID, chi.URLParam(r, "taskId"), tc.Role)
	if err != nil {
		response.Error(w, err)
		return
	}
	out := make([]approvalResp, 0, len(as))
	for i := range as {
		out = append(out, toApprovalResp(&as[i]))
	}
	response.JSON(w, http.StatusOK, map[string]any{"approvals": out})
}

type requestApprovalReq struct {
	ApproverIDs []string   `json:"approver_ids"`
	Deadline    *time.Time `json:"deadline"`
	Note        string     `json:"note"`
}

// RequestTaskApproval asks approvers to sign off the task.
func (h *TaskHandlers) RequestTaskApproval(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	var req requestApprovalReq
	if !decodeJSON(w, r, &req) {
		return
	}
	a, err := h.svc.RequestApproval(r.Context(), tc.OrgID, tc.UserID, chi.URLParam(r, "taskId"),
		taskuc.RequestApprovalInput{ApproverIDs: req.ApproverIDs, Deadline: req.Deadline, Note: req.Note}, tc.Role)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusCreated, toApprovalResp(a))
}

type decideApprovalReq struct {
	Decision string `json:"decision"` // approved | changes_requested
	Comment  string `json:"comment"`
}

// DecideTaskApproval records the caller's decision on a request.
func (h *TaskHandlers) DecideTaskApproval(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	var req decideApprovalReq
	if !decodeJSON(w, r, &req) {
		return
	}
	a, err := h.svc.DecideApproval(r.Context(), tc.OrgID, tc.UserID, chi.URLParam(r, "taskId"),
		chi.URLParam(r, "approvalId"), project.ApprovalDecision(req.Decision), req.Comment, tc.Role)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusOK, toApprovalResp(a))
}

// CancelTaskApproval withdraws an open request.
func (h *TaskHandlers) CancelTaskApproval(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	if err := h.svc.CancelApproval(r.Context(), tc.OrgID, tc.UserID, chi.URLParam(r, "taskId"),
		chi.URLParam(r, "approvalId"), tc.Role); err != nil {
		response.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type waitingApprovalResp struct {
	ApprovalID  string     `json:"approval_id"`
	TaskID      string     `json:"task_id"`
	ProjectID   string     `json:"project_id"`
	TaskNumber  int        `json:"task_number"`
	TaskTitle   string     `json:"task_title"`
	RequestedBy string     `json:"requested_by"`
	Deadline    *time.Time `json:"deadline"`
	CreatedAt   time.Time  `json:"created_at"`
}

// ListWaitingApprovals returns the open requests awaiting the caller.
func (h *TaskHandlers) ListWaitingApprovals(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	ws, err := h.svc.ListWaitingApprovals(r.Context(), tc.OrgID, tc.UserID)
	if err != nil {
		response.Error(w, err)
		return
	}
	out := make([]waitingApprovalResp, 0, len(ws))
	for _, x := range ws {
		out = append(out, waitingApprovalResp{
			ApprovalID: x.ApprovalID, TaskID: x.TaskID, ProjectID: x.ProjectID, TaskNumber: x.TaskNumber, TaskTitle: x.TaskTitle,
			RequestedBy: x.RequestedBy, Deadline: x.Deadline, CreatedAt: x.CreatedAt,
		})
	}
	response.JSON(w, http.StatusOK, map[string]any{"approvals": out})
}
