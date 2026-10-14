package taskuc

import (
	"encoding/csv"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/project"
)

func TestParseCSV(t *testing.T) {
	// Turkish Excel: BOM + ';' delimiter + a quoted multi-line cell + a blank row.
	h, recs, err := parseCSV("\uFEFFBaşlık;Açıklama\nA;\"line1\nline2\"\n;\nB;x\n")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(h, []string{"Başlık", "Açıklama"}) || len(recs) != 2 || recs[0][1] != "line1\nline2" {
		t.Fatalf("headers %q records %q", h, recs)
	}
	// Comma CSV whose header holds a ';' inside quotes stays comma-delimited.
	if h, _, _ := parseCSV("Title,\"a;b\"\nx,y\n"); len(h) != 2 {
		t.Fatalf("comma sniff: %q", h)
	}
	for name, in := range map[string]string{"empty": "", "not utf-8": "Title\n\xff\n"} {
		if _, _, err := parseCSV(in); !errors.Is(err, domain.ErrValidation) {
			t.Errorf("%s: err %v, want ErrValidation", name, err)
		}
	}
	tooMany := "Title\n" + strings.Repeat("x\n", MaxImportRows+1)
	if _, _, err := parseCSV(tooMany); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("row cap: err %v", err)
	}
}

func TestSuggestMapping(t *testing.T) {
	// Trello's CSV export headers.
	got := SuggestMapping([]string{"Card ID", "Card Name", "Card Description", "Labels", "Members", "Due Date", "List Name"})
	want := ImportMapping{
		FieldTitle: "Card Name", FieldDescription: "Card Description", FieldLabels: "Labels",
		FieldAssignee: "Members", FieldDueDate: "Due Date", FieldStatus: "List Name",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	// Our own CSV export maps back fully.
	if m := SuggestMapping(exportCSVHeaders); len(m) != 7 {
		t.Fatalf("export headers map %d fields: %v", len(m), m)
	}
}

func TestCheckMapping(t *testing.T) {
	h := []string{"Name", "Due"}
	if _, err := checkMapping(ImportMapping{FieldTitle: "Name", FieldDueDate: "Due"}, h); err != nil {
		t.Fatal(err)
	}
	for name, m := range map[string]ImportMapping{
		"no title":       {FieldDueDate: "Due"},
		"unknown header": {FieldTitle: "Nope"},
		"unknown field":  {FieldTitle: "Name", "sprint": "Due"},
	} {
		if _, err := checkMapping(m, h); !errors.Is(err, domain.ErrValidation) {
			t.Errorf("%s: err %v, want ErrValidation", name, err)
		}
	}
}

func TestMapRows(t *testing.T) {
	todo := project.Column{ID: "c1", Name: "To Do"}
	done := project.Column{ID: "c2", Name: "Done"}
	lk := importLookup{
		columns:       map[string]project.Column{"to do": todo, "done": done},
		defaultColumn: todo,
		people:        map[string]string{"ada@example.com": "u1"},
		labels:        map[string]bool{"bug": true},
	}
	headers := []string{"Title", "Status", "Assignee", "Due", "Priority", "Labels"}
	idx, err := checkMapping(ImportMapping{
		FieldTitle: "Title", FieldStatus: "Status", FieldAssignee: "Assignee",
		FieldDueDate: "Due", FieldPriority: "Priority", FieldLabels: "Labels",
	}, headers)
	if err != nil {
		t.Fatal(err)
	}
	recs, err := csv.NewReader(strings.NewReader(strings.Join([]string{
		`Ship it,done,Ada@Example.com,2026-03-01,High,"Bug (red), bug"`,
		`'=SUM(A1),,,15.03.2026,,`,
		`,Nowhere,bob@example.com,someday,p0,Feature`,
		`Short,,not-an-email,,,`,
	}, "\n"))).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	rows := mapRows(recs, idx, lk)

	r := rows[0]
	due := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	if r.Line != 2 || r.Title != "Ship it" || r.ColumnID != "c2" || r.AssigneeID == nil || *r.AssigneeID != "u1" ||
		r.DueDate == nil || !r.DueDate.Equal(due) || r.Priority != project.PriorityHigh ||
		!reflect.DeepEqual(r.Labels, []string{"Bug"}) || len(r.Errors) != 0 {
		t.Fatalf("row 1: %+v", r)
	}
	r = rows[1] // empty status → first column; formula guard undone; dd.mm.yyyy
	if r.Title != "=SUM(A1)" || r.ColumnID != "c1" || r.Priority != project.PriorityNone ||
		r.DueDate == nil || r.DueDate.Day() != 15 || len(r.Errors) != 0 {
		t.Fatalf("row 2: %+v", r)
	}
	// Every problem is reported, not just the first. Labels: no create right.
	if r := rows[2]; len(r.Errors) != 6 {
		t.Fatalf("row 3 errors = %q, want 6 (title, column, assignee, date, priority, label)", r.Errors)
	}
	if r := rows[3]; len(r.Errors) != 1 || !strings.Contains(r.Errors[0], "not an email") {
		t.Fatalf("row 4 errors = %q", r.Errors)
	}

	lk.createLabels = true
	if r := mapRows(recs[2:3], idx, lk)[0]; len(r.Errors) != 5 || !reflect.DeepEqual(r.Labels, []string{"Feature"}) {
		t.Fatalf("with label create right: %+v", r)
	}
}

func TestExportCSVRoundTrip(t *testing.T) {
	due := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	out := string(ExportCSV(&ExportProject{Tasks: []ExportedTask{{
		Number: 7, Title: "=HYPERLINK(\"x\")", Description: "- a\n- b", Status: "Done",
		AssigneeEmail: "ada@example.com", Priority: "low", DueDate: &due, Labels: []string{"Bug", "UI"},
	}}}))
	h, recs, err := parseCSV(out)
	if err != nil {
		t.Fatal(err)
	}
	if recs[0][0] != `'=HYPERLINK("x")` {
		t.Fatalf("formula not neutralised: %q", recs[0][0])
	}
	idx, err := checkMapping(SuggestMapping(h), h)
	if err != nil {
		t.Fatal(err)
	}
	lk := importLookup{
		columns: map[string]project.Column{"done": {ID: "c2", Name: "Done"}}, defaultColumn: project.Column{ID: "c1"},
		people: map[string]string{"ada@example.com": "u1"}, labels: map[string]bool{"bug": true, "ui": true},
	}
	r := mapRows(recs, idx, lk)[0]
	if len(r.Errors) != 0 || r.Title != `=HYPERLINK("x")` || r.Description != "- a\n- b" || r.ColumnID != "c2" ||
		!r.DueDate.Equal(due) || r.Priority != project.PriorityLow || !reflect.DeepEqual(r.Labels, []string{"Bug", "UI"}) {
		t.Fatalf("round trip: %+v", r)
	}
}
