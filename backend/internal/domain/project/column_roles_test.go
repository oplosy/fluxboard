package project

import "testing"

func cols(names ...string) []Column {
	out := make([]Column, len(names))
	for i, n := range names {
		out[i] = Column{ID: n, Name: n}
	}
	return out
}

func TestEffectiveRolesInferredFromNames(t *testing.T) {
	roles := EffectiveRoles(cols("Backlog", "To Do", "In-Progress", "Code Review", "Done"))
	want := map[string]ColumnRole{
		"Backlog": ColumnBacklog, "To Do": ColumnTodo, "In-Progress": ColumnInProgress,
		"Code Review": ColumnReview, "Done": ColumnDone,
	}
	for id, r := range want {
		if roles[id] != r {
			t.Errorf("%s = %q, want %q", id, roles[id], r)
		}
	}
}

func TestEffectiveRolesExplicitWinsAndIsUnique(t *testing.T) {
	cs := cols("Todo", "Doing", "QA", "Done", "Done (old)")
	cs[2].Role = ColumnReview // a lead marked "QA" as review
	cs[4].Role = ColumnDone   // and the archive column as done
	roles := EffectiveRoles(cs)
	if roles["QA"] != ColumnReview || roles["Done (old)"] != ColumnDone {
		t.Fatalf("explicit roles lost: %v", roles)
	}
	if r, ok := roles["Done"]; ok {
		t.Fatalf("an explicit done must stop name inference for done, got Done=%q", r)
	}
	if roles["Doing"] != ColumnInProgress || roles["Todo"] != ColumnTodo {
		t.Fatalf("inferred roles: %v", roles)
	}
	// Two columns that both look like todo: the left-most wins.
	roles = EffectiveRoles(cols("todo", "To-do"))
	if roles["todo"] != ColumnTodo || roles["To-do"] != "" {
		t.Fatalf("duplicate inference: %v", roles)
	}
}

func TestDoneColumnID(t *testing.T) {
	if got := DoneColumnID(cols("Todo", "Shipped", "Archive")); got != "Archive" {
		t.Fatalf("no done role: want the last column, got %q", got)
	}
	cs := cols("Todo", "Done", "Archive")
	if got := DoneColumnID(cs); got != "Done" {
		t.Fatalf("done by name: got %q", got)
	}
	if got := DoneColumnID(nil); got != "" {
		t.Fatalf("empty board: got %q", got)
	}
	if ColumnForRole(cs, ColumnReview) != nil {
		t.Fatal("no review column expected")
	}
}

func TestColumnRoleValid(t *testing.T) {
	for _, r := range ColumnRoles {
		if !r.Valid() {
			t.Errorf("%q should be valid", r)
		}
	}
	if ColumnRole("archive").Valid() || ColumnRole("").Valid() {
		t.Fatal("unknown roles must be invalid")
	}
}
