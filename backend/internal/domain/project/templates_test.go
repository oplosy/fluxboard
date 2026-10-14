package project

import "testing"

// The catalog is data: every reference inside a template must resolve.
func TestTemplatesConsistent(t *testing.T) {
	keys := map[string]bool{}
	for _, tpl := range Templates {
		if tpl.Key == "" || keys[tpl.Key] {
			t.Fatalf("empty or duplicate key %q", tpl.Key)
		}
		keys[tpl.Key] = true
		if got, ok := TemplateByKey(tpl.Key); !ok || got.Key != tpl.Key {
			t.Fatalf("TemplateByKey(%q) failed", tpl.Key)
		}
		if len(tpl.Columns) == 0 {
			t.Fatalf("%s: no columns", tpl.Key)
		}
		roles := map[ColumnRole]bool{}
		for _, c := range tpl.Columns {
			if c.Role != "" && (!c.Role.Valid() || roles[c.Role]) {
				t.Fatalf("%s: bad or repeated role %q", tpl.Key, c.Role)
			}
			roles[c.Role] = true
		}
		labels := map[string]bool{}
		for _, l := range tpl.Labels {
			if l.Name == "" || len(l.Name) > 30 || labels[l.Name] {
				t.Fatalf("%s: bad label %q", tpl.Key, l.Name)
			}
			labels[l.Name] = true
		}
		for _, task := range tpl.Tasks {
			if task.Title == "" || task.Column < 0 || task.Column >= len(tpl.Columns) || !task.Priority.Valid() {
				t.Fatalf("%s: bad task %+v", tpl.Key, task)
			}
			for _, l := range task.Labels {
				if !labels[l] {
					t.Fatalf("%s: task %q uses unknown label %q", tpl.Key, task.Title, l)
				}
			}
		}
	}
	if _, ok := TemplateByKey("nope"); ok {
		t.Fatal("unknown key found")
	}
}
