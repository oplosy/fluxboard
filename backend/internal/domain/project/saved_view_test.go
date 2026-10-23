package project

import (
	"strings"
	"testing"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
)

func TestNormalizeSavedView(t *testing.T) {
	sprint := "0190f0b8-8a2c-7c1e-9d3e-4a5b6c7d8e9f"
	name, f, err := NormalizeSavedView("  My bugs ", ViewFilters{Text: "login", Priority: "high", Sprint: sprint, Epic: "none"})
	if err != nil || name != "My bugs" {
		t.Fatalf("got %q, %v", name, err)
	}
	if f.Assignee != "any" || f.GroupBy != "none" {
		t.Errorf("defaults = %+v", f)
	}

	for label, c := range map[string]struct {
		name string
		f    ViewFilters
	}{
		"blank name":    {"  ", ViewFilters{}},
		"long name":     {strings.Repeat("v", MaxViewNameLen+1), ViewFilters{}},
		"long text":     {"v", ViewFilters{Text: strings.Repeat("t", MaxViewTextLen+1)}},
		"bad priority":  {"v", ViewFilters{Priority: "critical"}},
		"bad assignee":  {"v", ViewFilters{Assignee: "someone"}},
		"bad group":     {"v", ViewFilters{GroupBy: "label"}},
		"bad sprint id": {"v", ViewFilters{Sprint: "not-a-uuid"}},
		"bad epic":      {"v", ViewFilters{Epic: "backlog"}},
	} {
		if _, _, err := NormalizeSavedView(c.name, c.f); err != domain.ErrValidation {
			t.Errorf("%s: err %v, want ErrValidation", label, err)
		}
	}
}
