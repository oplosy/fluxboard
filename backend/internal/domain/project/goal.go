package project

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
)

// GoalStatus is derived on read from progress and the target date (FR-PROJ-013).
type GoalStatus string

const (
	GoalOnTrack GoalStatus = "on_track"
	GoalAtRisk  GoalStatus = "at_risk" // target date passed, not done
	GoalDone    GoalStatus = "done"    // has tasks, all in the done column
)

// Goal ties epics and tasks of any project of the org into one objective
// (FR-PROJ-013). Counters and Status are derived from the linked work the
// viewer may see, never stored.
type Goal struct {
	ID          string
	OrgID       string
	Title       string
	Description string
	OwnerID     string
	TargetDate  *time.Time // calendar day (midnight UTC); nil = no date
	CreatedAt   time.Time
	Total       int // distinct visible tasks: linked directly or via a linked epic
	Done        int // of those, tasks in the done column
	Points      int // summed estimates
	DonePoints  int // of those, points in the done column
	Status      GoalStatus
}

// GoalItemKind says what a link points at.
type GoalItemKind string

const (
	GoalItemEpic GoalItemKind = "epic"
	GoalItemTask GoalItemKind = "task"
)

// Valid reports whether k is a known kind.
func (k GoalItemKind) Valid() bool { return k == GoalItemEpic || k == GoalItemTask }

// GoalLink is a stored link; ProjectID is the project of the linked item.
type GoalLink struct {
	GoalID    string
	Kind      GoalItemKind
	ItemID    string
	ProjectID string
}

// GoalItem is a linked epic or task shown on a goal, only for projects the
// viewer can see. For an epic Total/Done count its tasks; for a task Total is 1.
type GoalItem struct {
	Kind       GoalItemKind
	ID         string
	ProjectID  string
	ProjectKey string
	Number     int // task number within the project; 0 for an epic
	Title      string
	Total      int
	Done       int
}

const (
	// MaxGoalTitleLen caps a goal title (runes).
	MaxGoalTitleLen = 200
	// MaxGoalDescriptionLen caps a goal description (runes).
	MaxGoalDescriptionLen = 2000
)

// NormalizeGoal trims and validates a goal's title and description.
func NormalizeGoal(title, description string) (string, string, error) {
	title = strings.TrimSpace(title)
	description = strings.TrimSpace(description)
	if title == "" || utf8.RuneCountInString(title) > MaxGoalTitleLen {
		return "", "", domain.ErrValidation
	}
	if utf8.RuneCountInString(description) > MaxGoalDescriptionLen {
		return "", "", domain.ErrValidation
	}
	return title, description, nil
}

// DeriveGoalStatus: done when it has tasks and all are done; at risk when the
// target day is before today (UTC) and it is not done; otherwise on track.
func DeriveGoalStatus(total, done int, target *time.Time, now time.Time) GoalStatus {
	if total > 0 && done == total {
		return GoalDone
	}
	if target != nil {
		n := now.UTC()
		today := time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
		if target.Before(today) {
			return GoalAtRisk
		}
	}
	return GoalOnTrack
}

// GoalRepository persists goals and their links (FR-PROJ-013).
type GoalRepository interface {
	Create(ctx context.Context, orgID string, g *Goal) error
	Get(ctx context.Context, orgID, id string) (*Goal, error)
	// List returns the org's goals oldest-first.
	List(ctx context.Context, orgID string) ([]Goal, error)
	// Update sets title, description and target date.
	Update(ctx context.Context, orgID string, g *Goal) error
	// Delete removes a goal and its links; the linked work stays.
	Delete(ctx context.Context, orgID, id string) error
	// Link adds a link; linking twice is a no-op.
	Link(ctx context.Context, orgID, goalID string, kind GoalItemKind, itemID string) error
	// Unlink removes a link. ErrNotFound if it does not exist.
	Unlink(ctx context.Context, orgID, goalID string, kind GoalItemKind, itemID string) error
	// Links returns the links of one goal, or of every goal when goalID is "",
	// oldest-first, with the project of each linked item.
	Links(ctx context.Context, orgID, goalID string) ([]GoalLink, error)
}
