package project

import (
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
)

// Epic groups related tasks of one project (FR-PROJ-008). Tasks join via
// Task.EpicID. Total/Done are derived from the tasks when listing, never stored.
type Epic struct {
	ID        string
	OrgID     string
	ProjectID string
	Name      string
	Color     string // "" or #RRGGBB
	CreatedBy string
	CreatedAt time.Time
	Total     int // tasks in the epic
	Done      int // of those, tasks in the done column
}

// MaxEpicNameLen caps an epic name (runes).
const MaxEpicNameLen = 100

var epicColorRE = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// NormalizeEpic trims and validates an epic's name and color.
func NormalizeEpic(name, color string) (string, string, error) {
	name = strings.TrimSpace(name)
	color = strings.TrimSpace(color)
	if name == "" || utf8.RuneCountInString(name) > MaxEpicNameLen {
		return "", "", domain.ErrValidation
	}
	if color != "" && !epicColorRE.MatchString(color) {
		return "", "", domain.ErrValidation
	}
	return name, strings.ToLower(color), nil
}
