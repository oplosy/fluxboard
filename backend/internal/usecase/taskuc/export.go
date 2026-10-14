package taskuc

import (
	"bytes"
	"context"
	"encoding/csv"
	"strconv"
	"strings"
	"time"

	"github.com/mesutokul/fluxboard/backend/internal/domain/project"
	"github.com/mesutokul/fluxboard/backend/internal/domain/tenant"
)

// Project export (FR-TASK-013): CSV of tasks (re-importable — its headers map
// back through SuggestMapping) and a JSON document with comments, labels and
// attachment metadata. VIEWER+ on the project; a hidden project is NotFound.

// ExportProject is the JSON export document.
type ExportProject struct {
	FormatVersion int             `json:"format_version"`
	ExportedAt    time.Time       `json:"exported_at"`
	Project       ExportedProject `json:"project"`
	Columns       []ExportedRef   `json:"columns"`
	Labels        []ExportedLabel `json:"labels"`
	Tasks         []ExportedTask  `json:"tasks"`
}

// ExportedProject identifies the exported project.
type ExportedProject struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
}

// ExportedRef is an id + name pair (board column).
type ExportedRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ExportedLabel is a label used by at least one exported task.
type ExportedLabel struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

// ExportedTask is one live task.
type ExportedTask struct {
	ID            string               `json:"id"`
	Number        int                  `json:"number"`
	Title         string               `json:"title"`
	Description   string               `json:"description"`
	ColumnID      string               `json:"column_id"`
	Status        string               `json:"status"`
	AssigneeID    *string              `json:"assignee_id"`
	AssigneeEmail string               `json:"assignee_email,omitempty"`
	Priority      string               `json:"priority"`
	StartDate     *time.Time           `json:"start_date"`
	DueDate       *time.Time           `json:"due_date"`
	Labels        []string             `json:"labels"`
	CreatedBy     string               `json:"created_by"`
	CreatedAt     time.Time            `json:"created_at"`
	UpdatedAt     time.Time            `json:"updated_at"`
	Comments      []ExportedComment    `json:"comments,omitempty"`
	Attachments   []ExportedAttachment `json:"attachments,omitempty"`
}

// ExportedComment is a live (not deleted) comment.
type ExportedComment struct {
	ID        string    `json:"id"`
	AuthorID  string    `json:"author_id"`
	Body      string    `json:"body"`
	Edited    bool      `json:"edited"`
	CreatedAt time.Time `json:"created_at"`
}

// ExportedAttachment is attachment metadata only; bytes stay in the object
// store behind the authorised download endpoint.
type ExportedAttachment struct {
	ID          string    `json:"id"`
	Filename    string    `json:"filename"`
	ContentType string    `json:"content_type"`
	SizeBytes   int64     `json:"size_bytes"`
	UploaderID  string    `json:"uploader_id"`
	CreatedAt   time.Time `json:"created_at"`
}

// ExportProject builds the export. details adds comments and attachment
// metadata (the JSON export); the CSV export skips those reads.
func (s *Service) ExportProject(ctx context.Context, orgID, userID, projectID string, orgRole tenant.OrgRole, details bool) (*ExportProject, error) {
	p, _, err := project.Access(ctx, s.projects, s.members, orgID, projectID, userID, orgRole, project.RoleViewer)
	if err != nil {
		return nil, err
	}
	b, err := s.boards.GetByProject(ctx, orgID, projectID)
	if err != nil {
		return nil, err
	}
	cols, err := s.columns.ListByBoard(ctx, orgID, b.ID)
	if err != nil {
		return nil, err
	}
	tasks, err := s.tasks.ListByProject(ctx, orgID, projectID)
	if err != nil {
		return nil, err
	}
	taskLabels, err := s.labels.ListForProject(ctx, orgID, projectID)
	if err != nil {
		return nil, err
	}
	emails := map[string]string{}
	if s.people != nil {
		ms, err := s.people.List(ctx, orgID, tenant.MemberQuery{Limit: maxPeople})
		if err != nil {
			return nil, err
		}
		for _, m := range ms {
			emails[m.UserID] = m.Email
		}
	}

	out := &ExportProject{
		FormatVersion: 1, ExportedAt: s.now().UTC(),
		Project: ExportedProject{ID: p.ID, Key: p.Key, Name: p.Name},
		Columns: make([]ExportedRef, 0, len(cols)), Labels: []ExportedLabel{}, Tasks: make([]ExportedTask, 0, len(tasks)),
	}
	colName := map[string]string{}
	for _, c := range cols {
		colName[c.ID] = c.Name
		out.Columns = append(out.Columns, ExportedRef{ID: c.ID, Name: c.Name})
	}
	seenLabel := map[string]bool{}
	for _, t := range tasks {
		et := ExportedTask{
			ID: t.ID, Number: t.Number, Title: t.Title, Description: t.Description,
			ColumnID: t.ColumnID, Status: colName[t.ColumnID], AssigneeID: t.AssigneeID,
			Priority: string(t.Priority), StartDate: t.StartDate, DueDate: t.DueDate, Labels: []string{},
			CreatedBy: t.CreatedBy, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt,
		}
		if t.AssigneeID != nil {
			et.AssigneeEmail = emails[*t.AssigneeID]
		}
		for _, l := range taskLabels[t.ID] {
			et.Labels = append(et.Labels, l.Name)
			if !seenLabel[l.ID] {
				seenLabel[l.ID] = true
				out.Labels = append(out.Labels, ExportedLabel{ID: l.ID, Name: l.Name, Color: l.Color})
			}
		}
		if details {
			// shortcut: two reads per task; add project-wide comment/attachment queries if exports of big projects get slow.
			if err := s.exportDetails(ctx, orgID, &et); err != nil {
				return nil, err
			}
		}
		out.Tasks = append(out.Tasks, et)
	}
	return out, nil
}

func (s *Service) exportDetails(ctx context.Context, orgID string, et *ExportedTask) error {
	cs, err := s.comments.ListByTask(ctx, orgID, et.ID)
	if err != nil {
		return err
	}
	for _, c := range cs {
		if c.DeletedAt == nil {
			et.Comments = append(et.Comments, ExportedComment{ID: c.ID, AuthorID: c.AuthorID, Body: c.Body, Edited: c.Edited, CreatedAt: c.CreatedAt})
		}
	}
	as, err := s.attachments.ListByTask(ctx, orgID, et.ID)
	if err != nil {
		return err
	}
	for _, a := range as {
		if a.Status == project.AttachmentCommitted {
			et.Attachments = append(et.Attachments, ExportedAttachment{
				ID: a.ID, Filename: a.Filename, ContentType: a.ContentType, SizeBytes: a.SizeBytes, UploaderID: a.UploaderID, CreatedAt: a.CreatedAt,
			})
		}
	}
	return nil
}

// exportCSVHeaders: the first seven map back on import (SuggestMapping).
var exportCSVHeaders = []string{"Title", "Description", "Status", "Assignee", "Due Date", "Priority", "Labels", "Number", "Start Date", "Created At"}

// ExportCSV renders the export's tasks as CSV (UTF-8 with BOM so Excel opens
// it as UTF-8). Cells starting with = + - @ are prefixed with ' so a
// spreadsheet never evaluates task text as a formula (CSV injection).
func ExportCSV(e *ExportProject) []byte {
	var buf bytes.Buffer
	buf.WriteString("\uFEFF")
	w := csv.NewWriter(&buf)
	_ = w.Write(exportCSVHeaders)
	for _, t := range e.Tasks {
		_ = w.Write([]string{
			safeCell(t.Title), safeCell(t.Description), safeCell(t.Status), safeCell(t.AssigneeEmail),
			day(t.DueDate), t.Priority, safeCell(strings.Join(t.Labels, ", ")),
			strconv.Itoa(t.Number), day(t.StartDate), t.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	w.Flush()
	return buf.Bytes()
}

func day(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format("2006-01-02")
}

func safeCell(v string) string {
	if v != "" && strings.ContainsRune("=+-@\t\r", rune(v[0])) {
		return "'" + v
	}
	return v
}
