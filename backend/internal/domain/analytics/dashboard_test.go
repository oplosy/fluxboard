package analytics

import (
	"errors"
	"strings"
	"testing"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
)

func TestNormalizeLayout(t *testing.T) {
	pid := strings.Repeat("a", uuidLen)
	ok, err := NormalizeLayout([]Widget{{ID: " a ", Type: WidgetOverdue}, {ID: "b", Type: WidgetWorkload, ProjectIDs: []string{pid}}})
	if err != nil || ok[0].ID != "a" || ok[0].ProjectIDs == nil {
		t.Fatalf("valid layout: %+v, %v", ok, err)
	}
	many := make([]Widget, MaxWidgets+1)
	for i := range many {
		many[i] = Widget{ID: string(rune('a' + i)), Type: WidgetOverdue}
	}
	bad := map[string][]Widget{
		"unknown type": {{ID: "a", Type: "pie"}},
		"empty id":     {{ID: " ", Type: WidgetOverdue}},
		"duplicate id": {{ID: "a", Type: WidgetOverdue}, {ID: "a", Type: WidgetWorkload}},
		"project id":   {{ID: "a", Type: WidgetOverdue, ProjectIDs: []string{"x"}}},
		"too many":     many,
	}
	for name, ws := range bad {
		if _, err := NormalizeLayout(ws); !errors.Is(err, domain.ErrValidation) {
			t.Errorf("%s: err = %v, want ErrValidation", name, err)
		}
	}
}
