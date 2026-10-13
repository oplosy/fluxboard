package agent

import (
	"strings"
	"testing"

	"github.com/mesutokul/fluxboard/backend/internal/domain/tenant"
)

func TestValidName(t *testing.T) {
	for _, ok := range []string{"backend-2", "Ajan 1", "qa_bot", "v1.2", "çalışkan", strings.Repeat("a", MaxNameLen)} {
		if !ValidName(ok) {
			t.Errorf("ValidName(%q) = false, want true", ok)
		}
	}
	for _, bad := range []string{"", strings.Repeat("a", MaxNameLen+1), "a/b", "<b>", "bot\n", "@x"} {
		if ValidName(bad) {
			t.Errorf("ValidName(%q) = true, want false", bad)
		}
	}
}

func TestValidRole(t *testing.T) {
	cases := map[tenant.OrgRole]bool{
		tenant.RoleMember: true, tenant.RoleGuest: true,
		tenant.RoleAdmin: false, tenant.RoleOwner: false, "": false, "CONTRIBUTOR": false,
	}
	for role, want := range cases {
		if got := ValidRole(role); got != want {
			t.Errorf("ValidRole(%q) = %v, want %v", role, got, want)
		}
	}
}

func TestEmail(t *testing.T) {
	e := Email("0190-abc")
	if !IsAgentEmail(e) || !IsAgentEmail(strings.ToUpper(e)) {
		t.Fatalf("IsAgentEmail(%q) = false", e)
	}
	if !strings.HasSuffix(e, ".invalid") {
		t.Fatalf("agent email %q must use the reserved .invalid TLD", e)
	}
	if IsAgentEmail("dev@agents.fluxboard.invalid.example.com") || IsAgentEmail("dev@example.com") {
		t.Fatal("IsAgentEmail matched a routable address")
	}
}
