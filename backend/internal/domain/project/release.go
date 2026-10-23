package project

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
)

// Release groups the tasks of one project shipping together (FR-PROJ-010).
// Tasks join via Task.ReleaseID. Total/Done/Points/DonePoints are derived from
// the tasks when listing, never stored.
type Release struct {
	ID          string
	OrgID       string
	ProjectID   string
	Name        string
	Description string
	ReleaseDate *time.Time // planned day (midnight UTC); nil = no date
	ReleasedAt  *time.Time // nil = not released yet
	CreatedBy   string
	CreatedAt   time.Time
	Total       int // tasks in the release
	Done        int // of those, tasks in the done column
	Points      int // summed estimates of the release's tasks
	DonePoints  int // of those, points in the done column
}

const (
	// MaxReleaseNameLen caps a release name (runes).
	MaxReleaseNameLen = 100
	// MaxReleaseDescriptionLen caps a release description (runes).
	MaxReleaseDescriptionLen = 2000
)

// NormalizeRelease trims and validates a release's name and description.
func NormalizeRelease(name, description string) (string, string, error) {
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	if name == "" || utf8.RuneCountInString(name) > MaxReleaseNameLen {
		return "", "", domain.ErrValidation
	}
	if utf8.RuneCountInString(description) > MaxReleaseDescriptionLen {
		return "", "", domain.ErrValidation
	}
	return name, description, nil
}
