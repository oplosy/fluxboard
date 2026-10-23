// Goal endpoints (FR-PROJ-013). Org-level; any org member reads, creation needs
// MEMBER+, changes need the goal owner or ADMIN+ (gated in projectuc).
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

type goalResp struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	OwnerID     string    `json:"owner_id"`
	TargetDate  *string   `json:"target_date"` // YYYY-MM-DD
	Status      string    `json:"status"`
	Total       int       `json:"total"`
	Done        int       `json:"done"`
	Points      int       `json:"points"`
	DonePoints  int       `json:"done_points"`
	CreatedAt   time.Time `json:"created_at"`
}

// goalDetailResp adds the linked items; always present (empty list, never null).
type goalDetailResp struct {
	goalResp
	Items []goalItemResp `json:"items"`
}

type goalItemResp struct {
	Kind       string `json:"kind"`
	ID         string `json:"id"`
	ProjectID  string `json:"project_id"`
	ProjectKey string `json:"project_key"`
	Number     int    `json:"number"`
	Title      string `json:"title"`
	Total      int    `json:"total"`
	Done       int    `json:"done"`
}

func toGoalResp(g *project.Goal) goalResp {
	var day *string
	if g.TargetDate != nil {
		d := g.TargetDate.Format(time.DateOnly)
		day = &d
	}
	return goalResp{
		ID: g.ID, Title: g.Title, Description: g.Description, OwnerID: g.OwnerID, TargetDate: day,
		Status: string(g.Status), Total: g.Total, Done: g.Done, Points: g.Points, DonePoints: g.DonePoints,
		CreatedAt: g.CreatedAt,
	}
}

func toGoalViewResp(v *projectuc.GoalView) goalDetailResp {
	out := goalDetailResp{goalResp: toGoalResp(&v.Goal), Items: make([]goalItemResp, 0, len(v.Items))}
	for _, it := range v.Items {
		out.Items = append(out.Items, goalItemResp{
			Kind: string(it.Kind), ID: it.ID, ProjectID: it.ProjectID, ProjectKey: it.ProjectKey,
			Number: it.Number, Title: it.Title, Total: it.Total, Done: it.Done,
		})
	}
	return out
}

type goalReq struct {
	Title       string  `json:"title"`
	Description string  `json:"description"`
	TargetDate  *string `json:"target_date"` // YYYY-MM-DD or null
}

// input parses the target day; a malformed date is a validation error.
func (req goalReq) input() (projectuc.GoalInput, error) {
	in := projectuc.GoalInput{Title: req.Title, Description: req.Description}
	if req.TargetDate != nil && *req.TargetDate != "" {
		d, err := time.Parse(time.DateOnly, *req.TargetDate)
		if err != nil {
			return in, domain.ErrValidation
		}
		in.TargetDate = &d
	}
	return in, nil
}

// ListGoals returns the org's goals with progress and status.
func (h *ProjectHandlers) ListGoals(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	goals, err := h.svc.ListGoals(r.Context(), tc.OrgID, tc.UserID, tc.Role)
	if err != nil {
		response.Error(w, err)
		return
	}
	out := make([]goalResp, 0, len(goals))
	for i := range goals {
		out = append(out, toGoalResp(&goals[i]))
	}
	response.JSON(w, http.StatusOK, map[string]any{"goals": out})
}

// GetGoal returns one goal with the linked items the caller may see.
func (h *ProjectHandlers) GetGoal(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	v, err := h.svc.GetGoal(r.Context(), tc.OrgID, tc.UserID, chi.URLParam(r, "goalId"), tc.Role)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusOK, toGoalViewResp(v))
}

// CreateGoal stores a goal owned by the caller (MEMBER+).
func (h *ProjectHandlers) CreateGoal(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	var req goalReq
	if !decodeJSON(w, r, &req) {
		return
	}
	in, err := req.input()
	if err != nil {
		response.Error(w, err)
		return
	}
	v, err := h.svc.CreateGoal(r.Context(), tc.OrgID, tc.UserID, in, tc.Role)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusCreated, toGoalViewResp(v))
}

// UpdateGoal edits a goal (owner or ADMIN+).
func (h *ProjectHandlers) UpdateGoal(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	var req goalReq
	if !decodeJSON(w, r, &req) {
		return
	}
	in, err := req.input()
	if err != nil {
		response.Error(w, err)
		return
	}
	v, err := h.svc.UpdateGoal(r.Context(), tc.OrgID, tc.UserID, chi.URLParam(r, "goalId"), in, tc.Role)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusOK, toGoalViewResp(v))
}

// DeleteGoal removes a goal; the linked work stays (owner or ADMIN+).
func (h *ProjectHandlers) DeleteGoal(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	if err := h.svc.DeleteGoal(r.Context(), tc.OrgID, tc.UserID, chi.URLParam(r, "goalId"), tc.Role); err != nil {
		response.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// goalLinker builds the link/unlink handlers for one item kind; idParam names
// the URL parameter holding the item id.
func (h *ProjectHandlers) goalLinker(kind project.GoalItemKind, idParam string, unlink bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tc, ok := mw.TenantFrom(r.Context())
		if !ok {
			response.Error(w, domain.ErrForbidden)
			return
		}
		goalID, itemID := chi.URLParam(r, "goalId"), chi.URLParam(r, idParam)
		var err error
		if unlink {
			err = h.svc.UnlinkGoalItem(r.Context(), tc.OrgID, tc.UserID, goalID, kind, itemID, tc.Role)
		} else {
			err = h.svc.LinkGoalItem(r.Context(), tc.OrgID, tc.UserID, goalID, kind, itemID, tc.Role)
		}
		if err != nil {
			response.Error(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// LinkGoalEpic, UnlinkGoalEpic, LinkGoalTask and UnlinkGoalTask attach or
// detach one epic/task (owner or ADMIN+; the item's project must be visible).
func (h *ProjectHandlers) LinkGoalEpic(w http.ResponseWriter, r *http.Request) {
	h.goalLinker(project.GoalItemEpic, "epicId", false)(w, r)
}
func (h *ProjectHandlers) UnlinkGoalEpic(w http.ResponseWriter, r *http.Request) {
	h.goalLinker(project.GoalItemEpic, "epicId", true)(w, r)
}
func (h *ProjectHandlers) LinkGoalTask(w http.ResponseWriter, r *http.Request) {
	h.goalLinker(project.GoalItemTask, "taskId", false)(w, r)
}
func (h *ProjectHandlers) UnlinkGoalTask(w http.ResponseWriter, r *http.Request) {
	h.goalLinker(project.GoalItemTask, "taskId", true)(w, r)
}
