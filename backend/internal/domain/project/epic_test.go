package project

import (
	"errors"
	"strings"
	"testing"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
)

func TestNormalizeEpic(t *testing.T) {
	name, color, err := NormalizeEpic("  Checkout  ", " #A1B2C3 ")
	if err != nil || name != "Checkout" || color != "#a1b2c3" {
		t.Fatalf("got (%q, %q, %v)", name, color, err)
	}
	if _, color, err := NormalizeEpic("Checkout", ""); err != nil || color != "" {
		t.Fatalf("empty color: (%q, %v)", color, err)
	}
	for _, tc := range []struct{ name, color string }{
		{"", ""},
		{"   ", ""},
		{strings.Repeat("é", MaxEpicNameLen+1), ""},
		{"Checkout", "red"},
		{"Checkout", "#abc"},
	} {
		if _, _, err := NormalizeEpic(tc.name, tc.color); !errors.Is(err, domain.ErrValidation) {
			t.Errorf("NormalizeEpic(%q, %q) err = %v, want ErrValidation", tc.name, tc.color, err)
		}
	}
}
