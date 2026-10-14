package project

import "context"

// Project templates (FR-PROJ-007, ADR-035): a fixed server-side catalog of
// boards a project can start from — columns with workflow roles, org labels and
// a few starter tasks. A template is applied in one transaction (ProjectSeeder),
// so a failure never leaves a half-built project.

// TemplateColumn is one board column of a template; Role may be "".
type TemplateColumn struct {
	Name string
	Role ColumnRole
}

// TemplateLabel is an org label a template ensures exists (reused by name).
type TemplateLabel struct {
	Name  string
	Color string
}

// TemplateTask is a starter task; Column indexes Template.Columns and Labels
// name entries of Template.Labels.
type TemplateTask struct {
	Title       string
	Description string
	Column      int
	Priority    Priority
	Labels      []string
}

// Template is one catalog entry.
type Template struct {
	Key         string
	Name        string
	Description string
	Columns     []TemplateColumn
	Labels      []TemplateLabel
	Tasks       []TemplateTask
}

// Templates is the catalog, in display order.
var Templates = []Template{
	{
		Key:         "customer-requests",
		Name:        "Customer requests",
		Description: "Triage incoming customer requests from intake to resolved.",
		Columns: []TemplateColumn{
			{"New", ColumnBacklog}, {"Triaged", ColumnTodo}, {"In Progress", ColumnInProgress},
			{"Waiting on Customer", ColumnReview}, {"Resolved", ColumnDone},
		},
		Labels: []TemplateLabel{
			{"bug", "#ef4444"}, {"feature-request", "#3b82f6"}, {"question", "#8b5cf6"}, {"urgent", "#dc2626"},
		},
		Tasks: []TemplateTask{
			{Title: "Read me: how requests flow through this board", Column: 0, Priority: PriorityNone,
				Description: "New requests land in **New**. Triage them into **Triaged** with a label and priority, then work them left to right."},
			{Title: "Example: customer cannot reset their password", Column: 1, Priority: PriorityHigh, Labels: []string{"bug", "urgent"}},
			{Title: "Example: export invoices as CSV", Column: 0, Priority: PriorityMedium, Labels: []string{"feature-request"}},
		},
	},
	{
		Key:         "marketing-campaign",
		Name:        "Marketing campaign",
		Description: "Content pipeline from idea to published, with a review step.",
		Columns: []TemplateColumn{
			{"Ideas", ColumnBacklog}, {"Planned", ColumnTodo}, {"In Production", ColumnInProgress},
			{"Review", ColumnReview}, {"Published", ColumnDone},
		},
		Labels: []TemplateLabel{
			{"blog", "#8b5cf6"}, {"social", "#06b6d4"}, {"email", "#f59e0b"}, {"ads", "#10b981"},
		},
		Tasks: []TemplateTask{
			{Title: "Define campaign goals and KPIs", Column: 1, Priority: PriorityHigh},
			{Title: "Draft the launch blog post", Column: 0, Priority: PriorityMedium, Labels: []string{"blog"}},
			{Title: "Schedule launch-week social posts", Column: 0, Priority: PriorityMedium, Labels: []string{"social"}},
			{Title: "Write the announcement email", Column: 0, Priority: PriorityMedium, Labels: []string{"email"}},
		},
	},
	{
		Key:         "employee-onboarding",
		Name:        "Employee onboarding",
		Description: "Everything a new hire needs, from accounts to first-week training.",
		Columns: []TemplateColumn{
			{"To Prepare", ColumnBacklog}, {"This Week", ColumnTodo}, {"In Progress", ColumnInProgress},
			{"Waiting for Sign-off", ColumnReview}, {"Done", ColumnDone},
		},
		Labels: []TemplateLabel{
			{"it-setup", "#3b82f6"}, {"hr", "#ec4899"}, {"training", "#10b981"},
		},
		Tasks: []TemplateTask{
			{Title: "Prepare laptop and accounts", Column: 0, Priority: PriorityHigh, Labels: []string{"it-setup"}},
			{Title: "Sign contract and company policies", Column: 0, Priority: PriorityHigh, Labels: []string{"hr"}},
			{Title: "Schedule intro meetings with the team", Column: 1, Priority: PriorityMedium},
			{Title: "Complete security awareness training", Column: 1, Priority: PriorityMedium, Labels: []string{"training"}},
		},
	},
	{
		Key:         "operations-checklist",
		Name:        "Operations checklist",
		Description: "Recurring operational work with a verification step before done.",
		Columns: []TemplateColumn{
			{"Backlog", ColumnBacklog}, {"To Do", ColumnTodo}, {"Doing", ColumnInProgress},
			{"Verify", ColumnReview}, {"Done", ColumnDone},
		},
		Labels: []TemplateLabel{
			{"recurring", "#6b7280"}, {"compliance", "#f59e0b"}, {"maintenance", "#06b6d4"},
		},
		Tasks: []TemplateTask{
			{Title: "Check that last night's backups restored", Column: 1, Priority: PriorityHigh, Labels: []string{"recurring"}},
			{Title: "Review expiring certificates and domains", Column: 0, Priority: PriorityMedium, Labels: []string{"maintenance"}},
			{Title: "Update the vendor contact list", Column: 0, Priority: PriorityLow, Labels: []string{"compliance"}},
		},
	},
}

// TemplateByKey returns the catalog entry with key.
func TemplateByKey(key string) (Template, bool) {
	for _, t := range Templates {
		if t.Key == key {
			return t, true
		}
	}
	return Template{}, false
}

// ProjectSeed is a fully built project (ids and ranks assigned) for a seeder.
type ProjectSeed struct {
	Project Project
	LeadID  string // made project LEAD
	Board   Board
	Columns []Column // Role is applied too
	Labels  []Label  // an org label with the same name is reused, not duplicated
	Tasks   []Task   // Number is assigned by the seeder
	// TaskLabels maps a task ID to the names of its labels.
	TaskLabels map[string][]string
}

// ProjectSeeder writes a ProjectSeed in a single tenant-scoped transaction:
// all or nothing. domain.ErrConflict on a project key clash.
type ProjectSeeder interface {
	Seed(ctx context.Context, orgID string, s *ProjectSeed) error
}
