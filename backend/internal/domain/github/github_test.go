package github

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestExtractKeys(t *testing.T) {
	cases := []struct {
		name  string
		texts []string
		want  []string
	}{
		{"title", []string{"Fix login (WEB-12)"}, []string{"WEB-12"}},
		{"case insensitive", []string{"feature/web-12-login"}, []string{"WEB-12"}},
		{"multiple and dedup", []string{"WEB-1, api-2 and WEB-1", "web-1 pay-30"}, []string{"WEB-1", "API-2", "PAY-30"}},
		{"underscore separator", []string{"feat_WEB-7_thing"}, []string{"WEB-7"}},
		{"no partial inside a longer word", []string{"abcdefghi-12 x86-64"}, nil},
		{"zero is not a task", []string{"WEB-0 WEB-000"}, nil},
		{"leading zeros normalised", []string{"WEB-007"}, []string{"WEB-7"}},
		{"none", []string{"just a title", ""}, nil},
	}
	for _, c := range cases {
		if got := ExtractKeys(c.texts...); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}

func TestExtractKeysCap(t *testing.T) {
	var b strings.Builder
	for i := 1; i <= 50; i++ {
		fmt.Fprintf(&b, "WEB-%d ", i)
	}
	if got := ExtractKeys(b.String()); len(got) != MaxKeysPerPR {
		t.Fatalf("len = %d, want %d", len(got), MaxKeysPerPR)
	}
}

func TestVerifySignature(t *testing.T) {
	body := []byte(`{"a":1}`)
	good := Sign("s3cret", body)
	if !VerifySignature("s3cret", body, good) {
		t.Fatal("valid signature rejected")
	}
	for name, hdr := range map[string]string{
		"empty": "", "no prefix": good[len("sha256="):], "bad hex": "sha256=zz",
		"wrong secret": Sign("other", body), "other body": Sign("s3cret", []byte("x")),
	} {
		if VerifySignature("s3cret", body, hdr) {
			t.Errorf("%s: accepted", name)
		}
	}
	if VerifySignature("", body, Sign("", body)) {
		t.Error("empty secret accepted")
	}
}

func TestParsePullRequest(t *testing.T) {
	ev, err := ParsePullRequest([]byte(`{"action":"closed","pull_request":{"number":5,"title":"T","html_url":"https://github.com/Acme/App/pull/5","state":"closed","merged":true,"updated_at":"2026-01-02T03:04:05Z","head":{"ref":"web-1"}},"repository":{"full_name":"Acme/App"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if ev.Repo != "acme/app" || ev.Number != 5 || !ev.Merged() || ev.HeadRef != "web-1" {
		t.Fatalf("unexpected event %+v", ev)
	}
	for _, bad := range []string{`{`, `{"action":"opened"}`, `{"pull_request":{"number":1},"repository":{"full_name":"no-slash"}}`} {
		if _, err := ParsePullRequest([]byte(bad)); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
}

func TestIntegrationAllowsRepo(t *testing.T) {
	i := &Integration{}
	if !i.AllowsRepo("a/b") {
		t.Error("empty list should allow any repo")
	}
	i.Repos = []string{"acme/app"}
	if !i.AllowsRepo("Acme/App") || i.AllowsRepo("acme/other") {
		t.Error("allow-list mismatch")
	}
}
