package chat

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/notify"
)

func TestValidateWebhookURL(t *testing.T) {
	cases := []struct {
		name string
		p    Provider
		url  string
		ok   bool
	}{
		{"slack ok", ProviderSlack, "https://hooks.slack.com/services/T0/B0/xyz", true},
		{"teams legacy ok", ProviderTeams, "https://contoso.webhook.office.com/webhookb2/abc@def/IncomingWebhook/x/y", true},
		{"teams workflows ok", ProviderTeams, "https://prod-12.westeurope.logic.azure.com/workflows/abc?sig=x", true},
		{"explicit port", ProviderTeams, "https://prod-12.westeurope.logic.azure.com:443/workflows/abc", false},
		{"http rejected", ProviderSlack, "http://hooks.slack.com/services/T0/B0/xyz", false},
		{"other host", ProviderSlack, "https://evil.example.com/services/T0/B0/xyz", false},
		{"suffix trick", ProviderSlack, "https://hooks.slack.com.evil.com/services/T0/B0/xyz", false},
		{"prefix trick teams", ProviderTeams, "https://webhook.office.com.evil.io/x", false},
		{"userinfo", ProviderSlack, "https://hooks.slack.com@evil.com/services/T0/B0/xyz", false},
		{"ipv4", ProviderSlack, "https://127.0.0.1/services/T0/B0/xyz", false},
		{"ipv6", ProviderTeams, "https://[::1]/x", false},
		{"metadata ip", ProviderTeams, "https://169.254.169.254/latest", false},
		{"slack wrong path", ProviderSlack, "https://hooks.slack.com/other", false},
		{"slack host for teams", ProviderTeams, "https://hooks.slack.com/services/T0/B0/xyz", false},
		{"teams host for slack", ProviderSlack, "https://outlook.office.com/webhook/abc", false},
		{"empty", ProviderSlack, "", false},
		{"bad provider", Provider("x"), "https://hooks.slack.com/services/a", false},
	}
	for _, c := range cases {
		err := ValidateWebhookURL(c.p, c.url)
		if c.ok && err != nil {
			t.Errorf("%s: unexpected error %v", c.name, err)
		}
		if !c.ok && !errors.Is(err, domain.ErrValidation) {
			t.Errorf("%s: want ErrValidation, got %v", c.name, err)
		}
	}
}

func TestHintHidesToken(t *testing.T) {
	h := Hint("https://hooks.slack.com/services/T0/B0/SECRETTOKEN1234")
	if !strings.HasSuffix(h, "1234") || strings.Contains(h, "SECRET") || strings.Contains(h, "T0/B0") {
		t.Fatalf("hint leaks the token: %q", h)
	}
}

func TestNormalizeChannel(t *testing.T) {
	name, cats, err := NormalizeChannel("  #dev  ", []string{"mention", "mention", "comment"})
	if err != nil || name != "#dev" || len(cats) != 2 {
		t.Fatalf("got %q %v %v", name, cats, err)
	}
	for _, bad := range [][]string{{"bogus"}, {"auth_reset"}} {
		if _, _, err := NormalizeChannel("x", bad); !errors.Is(err, domain.ErrValidation) {
			t.Errorf("categories %v: want ErrValidation, got %v", bad, err)
		}
	}
	if _, _, err := NormalizeChannel("  ", nil); !errors.Is(err, domain.ErrValidation) {
		t.Error("blank name must be rejected")
	}
}

func TestWants(t *testing.T) {
	c := &Channel{Enabled: true, Categories: []string{"mention"}}
	if !c.Wants(notify.CategoryMention) || c.Wants(notify.CategoryComment) {
		t.Error("category filter wrong")
	}
	c.Enabled = false
	if c.Wants(notify.CategoryMention) {
		t.Error("disabled channel must not receive")
	}
}

func TestRenderPayload(t *testing.T) {
	s, err := RenderPayload(ProviderSlack, "A <b> & c", "body", "https://app/x")
	var got map[string]string
	if err != nil || json.Unmarshal(s, &got) != nil ||
		!strings.Contains(got["text"], "*A &lt;b&gt; &amp; c*") || !strings.Contains(got["text"], "<https://app/x|Open in Fluxboard>") {
		t.Fatalf("slack: %s %v", s, err)
	}
	m, err := RenderPayload(ProviderTeams, "T", "", "https://app/x")
	if err != nil || !strings.Contains(string(m), "AdaptiveCard") || !strings.Contains(string(m), "Action.OpenUrl") {
		t.Fatalf("teams: %s %v", m, err)
	}
}
