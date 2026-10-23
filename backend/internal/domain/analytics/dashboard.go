package analytics

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
)

// WidgetType names one dashboard widget kind (FR-AN-005).
type WidgetType string

const (
	WidgetProjectStatus WidgetType = "project_status" // open vs done per project
	WidgetOverdue       WidgetType = "overdue"        // overdue open tasks per project
	WidgetThroughput    WidgetType = "throughput"     // completed tasks per week
	WidgetWorkload      WidgetType = "workload"       // open tasks per assignee
)

// Valid reports whether t is a known widget type.
func (t WidgetType) Valid() bool {
	switch t {
	case WidgetProjectStatus, WidgetOverdue, WidgetThroughput, WidgetWorkload:
		return true
	}
	return false
}

// Layout limits keep the stored JSON small and the page readable.
const (
	MaxWidgets        = 12
	MaxWidgetProjects = 50
	maxWidgetIDLen    = 40
	uuidLen           = 36
	DashboardWeeks    = 8
)

// Widget is one entry of a user's dashboard layout. ProjectIDs empty means
// every project the caller can see. The ids only select what to show; they never
// grant access (the data is already limited to the caller's visible projects).
type Widget struct {
	ID         string     `json:"id"`
	Type       WidgetType `json:"type"`
	ProjectIDs []string   `json:"project_ids"`
}

// NormalizeLayout validates a widget list and returns it with non-nil project
// id slices. domain.ErrValidation on an unknown type, a duplicate or empty
// widget id, or a list over the limits.
func NormalizeLayout(ws []Widget) ([]Widget, error) {
	if len(ws) > MaxWidgets {
		return nil, fmt.Errorf("%w: at most %d widgets", domain.ErrValidation, MaxWidgets)
	}
	seen := make(map[string]bool, len(ws))
	out := make([]Widget, 0, len(ws))
	for _, w := range ws {
		w.ID = strings.TrimSpace(w.ID)
		if w.ID == "" || len(w.ID) > maxWidgetIDLen || seen[w.ID] {
			return nil, fmt.Errorf("%w: widget id", domain.ErrValidation)
		}
		seen[w.ID] = true
		if !w.Type.Valid() {
			return nil, fmt.Errorf("%w: widget type", domain.ErrValidation)
		}
		if len(w.ProjectIDs) > MaxWidgetProjects {
			return nil, fmt.Errorf("%w: at most %d projects per widget", domain.ErrValidation, MaxWidgetProjects)
		}
		ids := make([]string, 0, len(w.ProjectIDs))
		for _, id := range w.ProjectIDs {
			if len(id) != uuidLen {
				return nil, fmt.Errorf("%w: project id", domain.ErrValidation)
			}
			ids = append(ids, id)
		}
		w.ProjectIDs = ids
		out = append(out, w)
	}
	return out, nil
}

// DefaultLayout is what a user sees before saving one: every widget, all projects.
func DefaultLayout() []Widget {
	return []Widget{
		{ID: "status", Type: WidgetProjectStatus, ProjectIDs: []string{}},
		{ID: "overdue", Type: WidgetOverdue, ProjectIDs: []string{}},
		{ID: "throughput", Type: WidgetThroughput, ProjectIDs: []string{}},
		{ID: "workload", Type: WidgetWorkload, ProjectIDs: []string{}},
	}
}

// ProjectCounts is one visible project's current task counts (done = in the
// done column, open = everything else not trashed, overdue = open and past due).
type ProjectCounts struct {
	ProjectID string
	Key       string
	Name      string
	Color     string
	Open      int
	Done      int
	Overdue   int
}

// AssigneeCount is the open work one person (UserID "" = unassigned) holds in
// one project.
type AssigneeCount struct {
	ProjectID string
	UserID    string
	Open      int
	Overdue   int
}

// WeeklyCompleted is a project's completed tasks in the ISO week starting Week.
type WeeklyCompleted struct {
	ProjectID string
	Week      time.Time
	Completed int
}

// DashboardData is everything the org dashboard needs, per project, limited to
// the projects the caller can see. Widgets sum the projects they cover.
type DashboardData struct {
	Projects  []ProjectCounts
	Assignees []AssigneeCount
	Weekly    []WeeklyCompleted
	WeekFrom  time.Time // Monday of the first week shown
}

// DashboardRepository reads the live task aggregates and stores layouts.
// [T]; tenant tx (RLS). Every aggregate is scoped to projectIDs.
type DashboardRepository interface {
	// Counts fills the three aggregate lists for projectIDs; weekly rows start at weekFrom.
	Counts(ctx context.Context, orgID string, projectIDs []string, now, weekFrom time.Time) (projects []ProjectCounts, assignees []AssigneeCount, weekly []WeeklyCompleted, err error)
	// GetLayout returns the user's saved widgets, or domain.ErrNotFound.
	GetLayout(ctx context.Context, orgID, userID string) ([]Widget, error)
	// SaveLayout replaces the user's widgets.
	SaveLayout(ctx context.Context, orgID, userID string, widgets []Widget) error
}
