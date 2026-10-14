package taskuc

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/mail"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/project"
	"github.com/mesutokul/fluxboard/backend/internal/domain/tenant"
)

// CSV task import (FR-TASK-012). Preview parses + maps + validates without
// writing; Import re-runs the same preview and creates every row through
// CreateTask, so numbering, ranks, events, automation and notifications are
// the normal ones. Durability comes from deterministic task ids: row i of an
// import keyed (org, project, Idempotency-Key, payload hash) always gets the
// same id, so a retry — after a timeout, a crash mid-way or a double click —
// hits the primary key for rows already written and only creates the rest.

// Import fields a CSV column can map to.
const (
	FieldTitle       = "title"
	FieldDescription = "description"
	FieldStatus      = "status"
	FieldAssignee    = "assignee"
	FieldDueDate     = "due_date"
	FieldPriority    = "priority"
	FieldLabels      = "labels"
)

// MaxImportRows caps one import; the request runs synchronously.
const MaxImportRows = 1000

// importFieldAliases are the lower-cased headers SuggestMapping recognises:
// our own export, Trello's CSV and common spreadsheet names (EN + TR).
var importFieldAliases = map[string][]string{
	FieldTitle:       {"title", "name", "card name", "task", "task name", "summary", "başlık", "görev"},
	FieldDescription: {"description", "card description", "desc", "details", "notes", "açıklama"},
	FieldStatus:      {"status", "list name", "list", "column", "state", "durum", "sütun"},
	FieldAssignee:    {"assignee", "assignee email", "email", "owner", "members", "atanan"},
	FieldDueDate:     {"due date", "due", "deadline", "due_date", "bitiş", "son tarih"},
	FieldPriority:    {"priority", "öncelik"},
	FieldLabels:      {"labels", "label", "tags", "etiketler"},
}

// ImportMapping maps an import field to a CSV header.
type ImportMapping map[string]string

// ImportRow is one mapped CSV row. Line is the 1-based line in the file
// (header = 1). Errors empty ⇒ the row will import.
type ImportRow struct {
	Line          int              `json:"line"`
	Title         string           `json:"title"`
	Description   string           `json:"description,omitempty"`
	ColumnID      string           `json:"column_id"`
	ColumnName    string           `json:"column_name"`
	AssigneeID    *string          `json:"assignee_id,omitempty"`
	AssigneeEmail string           `json:"assignee_email,omitempty"`
	DueDate       *time.Time       `json:"due_date,omitempty"`
	Priority      project.Priority `json:"priority"`
	Labels        []string         `json:"labels"`
	Errors        []string         `json:"errors"`
}

// ImportPreview is the dry-run result.
type ImportPreview struct {
	Headers []string      `json:"headers"`
	Mapping ImportMapping `json:"mapping"`
	Rows    []ImportRow   `json:"rows"`
	Valid   int           `json:"valid"`
	Invalid int           `json:"invalid"`
}

// ImportInput is the payload for PreviewImport / ImportTasks. A nil Mapping
// is replaced by SuggestMapping(headers).
type ImportInput struct {
	ProjectID string
	CSV       string
	Mapping   ImportMapping
	IdemKey   string
}

// ImportedTask is one task an import wrote (or found from an earlier attempt).
type ImportedTask struct {
	ID     string `json:"id"`
	Number int    `json:"number"`
	Title  string `json:"title"`
}

// ImportResult reports an import. Existing counts rows a previous attempt with
// the same key already wrote.
type ImportResult struct {
	Created  int            `json:"created"`
	Existing int            `json:"existing"`
	Tasks    []ImportedTask `json:"tasks"`
}

// importLookup is what row mapping resolves names against.
type importLookup struct {
	columns       map[string]project.Column // lower(name)
	defaultColumn project.Column            // first column; used for an empty status
	people        map[string]string         // lower(email) → user id; nil ⇒ unavailable
	labels        map[string]bool           // lower(name) of existing labels
	createLabels  bool                      // caller may create missing labels
}

// parseCSV reads an Excel/Trello CSV: optional UTF-8 BOM, delimiter sniffed
// from the header line (',' ';' or tab — Turkish Excel writes ';').
func parseCSV(data string) (headers []string, records [][]string, err error) {
	data = strings.TrimPrefix(data, "\uFEFF")
	if !utf8.ValidString(data) {
		return nil, nil, fmt.Errorf("%w: CSV must be UTF-8", domain.ErrValidation)
	}
	first, _, _ := strings.Cut(data, "\n")
	delim := ','
	for _, d := range []rune{';', '\t'} {
		if strings.Count(first, string(d)) > strings.Count(first, string(delim)) {
			delim = d
		}
	}
	r := csv.NewReader(strings.NewReader(data))
	r.Comma = delim
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	headers, err = r.Read()
	if errors.Is(err, io.EOF) {
		return nil, nil, fmt.Errorf("%w: CSV is empty", domain.ErrValidation)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("%w: CSV: %v", domain.ErrValidation, err)
	}
	for i := range headers {
		headers[i] = strings.TrimSpace(headers[i])
	}
	for {
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, nil, fmt.Errorf("%w: CSV: %v", domain.ErrValidation, err)
		}
		if blank(rec) {
			continue
		}
		if len(records) == MaxImportRows {
			return nil, nil, fmt.Errorf("%w: at most %d rows per import", domain.ErrValidation, MaxImportRows)
		}
		records = append(records, rec)
	}
	return headers, records, nil
}

func blank(rec []string) bool {
	for _, v := range rec {
		if strings.TrimSpace(v) != "" {
			return false
		}
	}
	return true
}

// SuggestMapping guesses a mapping from header names (first match wins).
func SuggestMapping(headers []string) ImportMapping {
	m := ImportMapping{}
	for field, aliases := range importFieldAliases {
		for _, a := range aliases {
			for _, h := range headers {
				if strings.EqualFold(h, a) {
					m[field] = h
					break
				}
			}
			if m[field] != "" {
				break
			}
		}
	}
	return m
}

// checkMapping rejects unknown fields, unknown headers and a missing title.
func checkMapping(m ImportMapping, headers []string) (map[string]int, error) {
	idx := map[string]int{}
	for field, h := range m {
		if _, ok := importFieldAliases[field]; !ok {
			return nil, fmt.Errorf("%w: unknown field %q", domain.ErrValidation, field)
		}
		if h == "" {
			continue
		}
		i := indexOf(headers, h)
		if i < 0 {
			return nil, fmt.Errorf("%w: no CSV column %q", domain.ErrValidation, h)
		}
		idx[field] = i
	}
	if _, ok := idx[FieldTitle]; !ok {
		return nil, fmt.Errorf("%w: map a column to title", domain.ErrValidation)
	}
	return idx, nil
}

func indexOf(headers []string, h string) int {
	for i, v := range headers {
		if v == h {
			return i
		}
	}
	return -1
}

// mapRows turns records into validated rows. Line numbers assume one physical
// line per record (multi-line quoted cells shift them; good enough to locate).
func mapRows(records [][]string, idx map[string]int, lk importLookup) []ImportRow {
	rows := make([]ImportRow, 0, len(records))
	for n, rec := range records {
		cell := func(field string) string {
			i, ok := idx[field]
			if !ok || i >= len(rec) {
				return ""
			}
			return unescapeCell(strings.TrimSpace(rec[i]))
		}
		row := ImportRow{Line: n + 2, Title: cell(FieldTitle), Description: cell(FieldDescription), Labels: []string{}, Errors: []string{}}
		fail := func(format string, a ...any) { row.Errors = append(row.Errors, fmt.Sprintf(format, a...)) }

		if row.Title == "" {
			fail("title is empty")
		} else if utf8.RuneCountInString(row.Title) > project.MaxTitleLen {
			fail("title is longer than %d characters", project.MaxTitleLen)
		}

		if status := cell(FieldStatus); status == "" {
			row.ColumnID, row.ColumnName = lk.defaultColumn.ID, lk.defaultColumn.Name
		} else if c, ok := lk.columns[strings.ToLower(status)]; ok {
			row.ColumnID, row.ColumnName = c.ID, c.Name
		} else {
			fail("no board column named %q", status)
		}

		if email := cell(FieldAssignee); email != "" {
			row.AssigneeEmail = email
			if addr, err := mail.ParseAddress(email); err != nil {
				fail("assignee %q is not an email", email)
			} else if lk.people == nil {
				fail("assignee lookup is unavailable")
			} else if id, ok := lk.people[strings.ToLower(addr.Address)]; ok {
				row.AssigneeID = &id
			} else {
				fail("no org member with email %q", email)
			}
		}

		if due := cell(FieldDueDate); due != "" {
			d, err := parseImportDate(due)
			if err != nil {
				fail("due date %q: use YYYY-MM-DD", due)
			}
			row.DueDate = d
		}

		row.Priority = project.Priority(strings.ToLower(cell(FieldPriority)))
		if row.Priority == "" {
			row.Priority = project.PriorityNone
		}
		if !row.Priority.Valid() {
			fail("priority %q: use none, low, medium, high or urgent", cell(FieldPriority))
		}

		for _, l := range splitLabels(cell(FieldLabels)) {
			switch {
			case utf8.RuneCountInString(l) > project.MaxLabelLen:
				fail("label %q is longer than %d characters", l, project.MaxLabelLen)
			case !lk.labels[strings.ToLower(l)] && !lk.createLabels:
				fail("no label named %q", l)
			default:
				row.Labels = append(row.Labels, l)
			}
		}
		rows = append(rows, row)
	}
	return rows
}

// unescapeCell undoes ExportCSV's formula guard ('=x → =x) so exports
// round-trip.
func unescapeCell(v string) string {
	if len(v) > 1 && v[0] == '\'' && strings.ContainsRune("=+-@", rune(v[1])) {
		return v[1:]
	}
	return v
}

// importDateLayouts: ISO first (our export, Trello), then Turkish and US
// spreadsheet dates. RFC3339 also accepts fractional seconds.
var importDateLayouts = []string{
	time.RFC3339, "2006-01-02", "2006-01-02 15:04:05", "2006-01-02T15:04:05", "02.01.2006", "2.1.2006", "01/02/2006", "1/2/2006",
}

func parseImportDate(s string) (*time.Time, error) {
	for _, l := range importDateLayouts {
		if t, err := time.Parse(l, s); err == nil {
			t = t.UTC()
			return &t, nil
		}
	}
	return nil, domain.ErrValidation
}

// trelloLabelColor strips Trello's " (color)" suffix: "Bug (red)" → "Bug".
var trelloLabelColor = regexp.MustCompile(`\s*\([a-z_]+\)$`)

// splitLabels splits "a, b; c" into distinct trimmed names.
func splitLabels(s string) []string {
	var out []string
	seen := map[string]bool{}
	for _, p := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ';' }) {
		p = strings.TrimSpace(trelloLabelColor.ReplaceAllString(strings.TrimSpace(p), ""))
		if p != "" && !seen[strings.ToLower(p)] {
			seen[strings.ToLower(p)] = true
			out = append(out, p)
		}
	}
	return out
}

// PreviewImport parses, maps and validates a CSV without writing anything
// (CONTRIBUTOR+ on the project, the same gate as CreateTask).
func (s *Service) PreviewImport(ctx context.Context, orgID, userID string, orgRole tenant.OrgRole, in ImportInput) (*ImportPreview, error) {
	if _, err := s.access(ctx, orgID, in.ProjectID, userID, orgRole, project.RoleContributor); err != nil {
		return nil, err
	}
	headers, records, err := parseCSV(in.CSV)
	if err != nil {
		return nil, err
	}
	m := in.Mapping
	if m == nil {
		m = SuggestMapping(headers)
	}
	idx, err := checkMapping(m, headers)
	if err != nil {
		return nil, err
	}
	lk, err := s.importLookup(ctx, orgID, in.ProjectID, orgRole, idx)
	if err != nil {
		return nil, err
	}
	p := &ImportPreview{Headers: headers, Mapping: m, Rows: mapRows(records, idx, lk)}
	for _, r := range p.Rows {
		if len(r.Errors) == 0 {
			p.Valid++
		} else {
			p.Invalid++
		}
	}
	return p, nil
}

func (s *Service) importLookup(ctx context.Context, orgID, projectID string, orgRole tenant.OrgRole, idx map[string]int) (importLookup, error) {
	lk := importLookup{columns: map[string]project.Column{}, labels: map[string]bool{}, createLabels: orgRole.AtLeast(tenant.RoleMember)}
	b, err := s.boards.GetByProject(ctx, orgID, projectID)
	if err != nil {
		return lk, err
	}
	cols, err := s.columns.ListByBoard(ctx, orgID, b.ID)
	if err != nil {
		return lk, err
	}
	if len(cols) == 0 {
		return lk, fmt.Errorf("%w: the board has no columns", domain.ErrValidation)
	}
	lk.defaultColumn = cols[0]
	for _, c := range cols {
		lk.columns[strings.ToLower(c.Name)] = c
	}
	if _, ok := idx[FieldAssignee]; ok && s.people != nil {
		ms, err := s.people.List(ctx, orgID, tenant.MemberQuery{Limit: maxPeople})
		if err != nil {
			return lk, err
		}
		lk.people = make(map[string]string, len(ms))
		for _, m := range ms {
			lk.people[strings.ToLower(m.Email)] = m.UserID
		}
	}
	if _, ok := idx[FieldLabels]; ok {
		ls, err := s.labels.List(ctx, orgID)
		if err != nil {
			return lk, err
		}
		for _, l := range ls {
			lk.labels[strings.ToLower(l.Name)] = true
		}
	}
	return lk, nil
}

// importNamespace roots the deterministic task ids (uuid v5) of imports.
var importNamespace = uuid.MustParse("6f1d3f4e-0b5a-5c7e-9a43-6d2f1e8b7c10")

// ImportTasks creates every row of a fully valid CSV (FR-TASK-012). Any row
// error rejects the whole import before writing. A failure mid-way returns the
// error; retrying with the same Idempotency-Key and payload resumes without
// duplicating rows already written.
func (s *Service) ImportTasks(ctx context.Context, orgID, userID string, orgRole tenant.OrgRole, in ImportInput) (*ImportResult, error) {
	key := strings.TrimSpace(in.IdemKey)
	if key == "" || len(key) > 200 {
		return nil, fmt.Errorf("%w: Idempotency-Key required (max 200 chars)", domain.ErrValidation)
	}
	p, err := s.PreviewImport(ctx, orgID, userID, orgRole, in)
	if err != nil {
		return nil, err
	}
	if p.Invalid > 0 {
		return nil, fmt.Errorf("%w: %d of %d rows have errors; fix them and preview again", domain.ErrValidation, p.Invalid, len(p.Rows))
	}
	labelIDs, err := s.ensureLabels(ctx, orgID, p.Rows)
	if err != nil {
		return nil, err
	}
	mapping, _ := json.Marshal(p.Mapping) // map keys marshal sorted: stable
	sum := sha256.Sum256(append([]byte(in.CSV), mapping...))
	ns := uuid.NewSHA1(importNamespace, bytes.Join([][]byte{[]byte(orgID), []byte(in.ProjectID), []byte(key), sum[:]}, []byte{0}))

	res := &ImportResult{Tasks: make([]ImportedTask, 0, len(p.Rows))}
	for _, row := range p.Rows {
		id := uuid.NewSHA1(ns, []byte(strconv.Itoa(row.Line))).String()
		t, err := s.CreateTask(ctx, orgID, userID, CreateTaskInput{
			ID: id, ProjectID: in.ProjectID, ColumnID: row.ColumnID, Title: row.Title,
			Description: row.Description, AssigneeID: row.AssigneeID, Priority: row.Priority, DueDate: row.DueDate,
		}, orgRole)
		trashed := false
		switch {
		case err == nil:
			res.Created++
		case errors.Is(err, domain.ErrConflict):
			// An earlier attempt wrote this row (the id is taken).
			t, trashed, err = s.importedTask(ctx, orgID, id, err)
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", row.Line, err)
			}
			res.Existing++
		default:
			return nil, fmt.Errorf("line %d: %w", row.Line, err)
		}
		if !trashed {
			for _, l := range row.Labels {
				if err := s.labels.Attach(ctx, orgID, t.ID, labelIDs[strings.ToLower(l)]); err != nil {
					return nil, fmt.Errorf("line %d: label: %w", row.Line, err)
				}
			}
		}
		res.Tasks = append(res.Tasks, ImportedTask{ID: t.ID, Number: t.Number, Title: t.Title})
	}
	return res, nil
}

// importedTask loads the task an earlier attempt created under id (it may
// since have been trashed). Neither found ⇒ the conflict was something else.
func (s *Service) importedTask(ctx context.Context, orgID, id string, conflict error) (*project.Task, bool, error) {
	if t, err := s.tasks.Get(ctx, orgID, id); err == nil {
		return t, false, nil
	}
	if t, err := s.tasks.GetTrashed(ctx, orgID, id); err == nil {
		return t, true, nil
	}
	return nil, false, conflict
}

// ensureLabels resolves every label name the rows use to an id, creating the
// missing ones (mapRows only lets them through when the caller may).
func (s *Service) ensureLabels(ctx context.Context, orgID string, rows []ImportRow) (map[string]string, error) {
	ids := map[string]string{}
	need := false
	for _, r := range rows {
		need = need || len(r.Labels) > 0
	}
	if !need {
		return ids, nil
	}
	ls, err := s.labels.List(ctx, orgID)
	if err != nil {
		return nil, err
	}
	for _, l := range ls {
		ids[strings.ToLower(l.Name)] = l.ID
	}
	for _, r := range rows {
		for _, name := range r.Labels {
			if ids[strings.ToLower(name)] != "" {
				continue
			}
			l, err := s.CreateLabel(ctx, orgID, name, importLabelColor)
			if errors.Is(err, domain.ErrConflict) { // created concurrently: find it
				l, err = s.labelByName(ctx, orgID, name)
			}
			if err != nil {
				return nil, err
			}
			ids[strings.ToLower(name)] = l.ID
		}
	}
	return ids, nil
}

func (s *Service) labelByName(ctx context.Context, orgID, name string) (*project.Label, error) {
	ls, err := s.labels.List(ctx, orgID)
	if err != nil {
		return nil, err
	}
	for i := range ls {
		if strings.EqualFold(ls[i].Name, name) {
			return &ls[i], nil
		}
	}
	return nil, domain.ErrConflict
}

const (
	importLabelColor = "#64748b"
	// maxPeople bounds the member list read for email matching.
	// shortcut: one page of 5000 members, page through ListMembers if an org outgrows it.
	maxPeople = 5000
)

// People reads org members (tenant.MembershipRepository).
type People interface {
	List(ctx context.Context, orgID string, q tenant.MemberQuery) ([]tenant.Member, error)
}
