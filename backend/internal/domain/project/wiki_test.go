package project

import (
	"reflect"
	"strings"
	"testing"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
)

func TestExtractTaskNumbers(t *testing.T) {
	cases := []struct {
		name, body string
		want       []int
	}{
		{"none", "no keys here", []int{}},
		{"single", "see WEB-12 for details", []int{12}},
		{"multiple sorted and distinct", "WEB-3, WEB-1 and WEB-3 again; (WEB-20).", []int{1, 3, 20}},
		{"other project ignored", "OPS-4 and WEB-5", []int{5}},
		{"lowercase ignored", "web-7 Web-8", []int{}},
		{"embedded in a word ignored", "XWEB-9 WEB-10x WEBB-11 WEB-12_", []int{}},
		{"adjacent keys", "WEB-1 WEB-2\nWEB-3", []int{1, 2, 3}},
		{"zero ignored", "WEB-0", []int{}},
		{"huge number ignored", "WEB-99999999999", []int{}},
	}
	for _, c := range cases {
		if got := ExtractTaskNumbers(c.body, "WEB"); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestNormalizeWikiPage(t *testing.T) {
	title, body, err := NormalizeWikiPage("  Plan  ", "  keep body whitespace ")
	if err != nil || title != "Plan" || body != "  keep body whitespace " {
		t.Fatalf("got %q %q %v", title, body, err)
	}
	for name, in := range map[string][2]string{
		"blank title": {"  ", "x"},
		"long title":  {strings.Repeat("a", MaxWikiTitleLen+1), ""},
		"long body":   {"t", strings.Repeat("a", MaxWikiBodyLen+1)},
	} {
		if _, _, err := NormalizeWikiPage(in[0], in[1]); err != domain.ErrValidation {
			t.Errorf("%s: err %v, want ErrValidation", name, err)
		}
	}
}
