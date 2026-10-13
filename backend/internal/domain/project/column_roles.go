package project

import (
	"strings"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
)

// Column roles (ADR-033 decision 5, FR-AGT-006): fixed workflow stages a
// board's columns map onto, so agents move work by meaning ("in review"), not
// by a column's free-text name. A column's role is explicit when a lead set it,
// otherwise inferred from its name; each role belongs to at most one column.

// ColumnRole is a workflow stage.
type ColumnRole string

const (
	ColumnBacklog    ColumnRole = "backlog"
	ColumnTodo       ColumnRole = "todo"
	ColumnInProgress ColumnRole = "in_progress"
	ColumnReview     ColumnRole = "review"
	ColumnDone       ColumnRole = "done"
)

// ColumnRoles lists every role in workflow order.
var ColumnRoles = []ColumnRole{ColumnBacklog, ColumnTodo, ColumnInProgress, ColumnReview, ColumnDone}

// Valid reports whether r is a known role.
func (r ColumnRole) Valid() bool {
	for _, x := range ColumnRoles {
		if r == x {
			return true
		}
	}
	return false
}

// ErrHumanApprovalRequired refuses an agent's move into a done column: only a
// person can accept work as done (ADR-033 decision 6, FR-AGT-007).
var ErrHumanApprovalRequired = domain.NewCodedError(domain.ErrForbidden, "human_approval_required",
	"Only a person can mark a task done. Submit it for review instead.")

// ErrNoColumnForRole: the board has no column for the requested stage.
var ErrNoColumnForRole = domain.NewCodedError(domain.ErrValidation, "column_role_missing",
	"This board has no column for that stage. A project lead can assign column roles in project settings.")

// inferRole guesses a role from a column name ("In Progress", "to-do", …).
func inferRole(name string) ColumnRole {
	n := strings.ToLower(name)
	n = strings.NewReplacer(" ", "", "-", "", "_", "").Replace(n)
	switch n {
	case "backlog":
		return ColumnBacklog
	case "todo", "ready":
		return ColumnTodo
	case "inprogress", "doing", "wip":
		return ColumnInProgress
	case "review", "inreview", "codereview":
		return ColumnReview
	case "done", "completed", "complete":
		return ColumnDone
	}
	return ""
}

// EffectiveRoles maps column id → role for a board's rank-ordered columns:
// explicit roles first, then name inference for roles nobody holds (the
// left-most matching column wins).
func EffectiveRoles(cols []Column) map[string]ColumnRole {
	out := make(map[string]ColumnRole, len(cols))
	taken := make(map[ColumnRole]bool, len(ColumnRoles))
	for _, c := range cols {
		if c.Role != "" {
			out[c.ID] = c.Role
			taken[c.Role] = true
		}
	}
	for _, c := range cols {
		if c.Role != "" {
			continue
		}
		if r := inferRole(c.Name); r != "" && !taken[r] {
			out[c.ID] = r
			taken[r] = true
		}
	}
	return out
}

// ColumnForRole returns the column holding role, or nil.
func ColumnForRole(cols []Column, role ColumnRole) *Column {
	roles := EffectiveRoles(cols)
	for i := range cols {
		if roles[cols[i].ID] == role {
			return &cols[i]
		}
	}
	return nil
}

// DoneColumnID is the column that means "done": the done role, else the last
// column (the board's historical convention). "" for an empty board.
func DoneColumnID(cols []Column) string {
	if c := ColumnForRole(cols, ColumnDone); c != nil {
		return c.ID
	}
	if len(cols) == 0 {
		return ""
	}
	return cols[len(cols)-1].ID
}
