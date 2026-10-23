package project

import (
	"strings"
	"testing"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
)

func TestNormalizeRelease(t *testing.T) {
	name, desc, err := NormalizeRelease("  v1.2  ", "  Search and export  ")
	if err != nil || name != "v1.2" || desc != "Search and export" {
		t.Fatalf("got %q %q %v", name, desc, err)
	}
	for _, c := range []struct{ name, desc string }{
		{"   ", ""},
		{strings.Repeat("é", MaxReleaseNameLen+1), ""},
		{"v2", strings.Repeat("x", MaxReleaseDescriptionLen+1)},
	} {
		if _, _, err := NormalizeRelease(c.name, c.desc); err != domain.ErrValidation {
			t.Errorf("NormalizeRelease(len %d, len %d) err = %v, want ErrValidation", len(c.name), len(c.desc), err)
		}
	}
}
