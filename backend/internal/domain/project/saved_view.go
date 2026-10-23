package project

import (
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
)

// SavedView is a named set of board filters and grouping (FR-PROJ-011).
// A personal view (Shared false) is visible to its owner only.
type SavedView struct {
	ID        string
	OrgID     string
	ProjectID string
	OwnerID   string
	Name      string
	Shared    bool
	Filters   ViewFilters
	CreatedAt time.Time
}

// ViewFilters mirrors the web board's filter state. Empty strings mean "any".
type ViewFilters struct {
	Text     string `json:"text"`
	Priority string `json:"priority"` // "" or a Priority
	Assignee string `json:"assignee"` // any | me | unassigned
	Sprint   string `json:"sprint"`   // "" | backlog | sprint id
	Epic     string `json:"epic"`     // "" | none | epic id
	GroupBy  string `json:"group_by"` // none | assignee | epic | priority
}

const (
	// MaxViewNameLen caps a saved view name (runes).
	MaxViewNameLen = 60
	// MaxViewTextLen caps the saved text filter (runes).
	MaxViewTextLen = 200
)

// NormalizeSavedView trims and validates a view's name and filters. Ids in
// Sprint/Epic are only checked for shape: a view outlives a deleted sprint or
// epic, and the board then simply matches nothing.
func NormalizeSavedView(name string, f ViewFilters) (string, ViewFilters, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > MaxViewNameLen {
		return "", f, domain.ErrValidation
	}
	if utf8.RuneCountInString(f.Text) > MaxViewTextLen {
		return "", f, domain.ErrValidation
	}
	if f.Priority != "" && !Priority(f.Priority).Valid() {
		return "", f, domain.ErrValidation
	}
	if f.Assignee == "" {
		f.Assignee = "any"
	}
	if f.GroupBy == "" {
		f.GroupBy = "none"
	}
	if !slices.Contains([]string{"any", "me", "unassigned"}, f.Assignee) ||
		!slices.Contains([]string{"none", "assignee", "epic", "priority"}, f.GroupBy) {
		return "", f, domain.ErrValidation
	}
	if !idOrKeyword(f.Sprint, "backlog") || !idOrKeyword(f.Epic, "none") {
		return "", f, domain.ErrValidation
	}
	return name, f, nil
}

// uuidRE matches a canonical UUID; the domain stays stdlib-only.
var uuidRE = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func idOrKeyword(v, keyword string) bool {
	return v == "" || v == keyword || uuidRE.MatchString(v)
}
