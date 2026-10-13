package argon2x

import (
	"errors"
	"strings"
	"testing"
)

// TestParams is the regression guard from docs/04-AUTH.md §6 checklist: a silent
// downgrade of Argon2id parameters is a security incident, so pin them here.
func TestParams(t *testing.T) {
	if Params.Memory != 64*1024 {
		t.Errorf("Memory = %d, want %d", Params.Memory, 64*1024)
	}
	if Params.Iterations != 3 {
		t.Errorf("Iterations = %d, want 3", Params.Iterations)
	}
	if Params.Parallelism != 2 {
		t.Errorf("Parallelism = %d, want 2", Params.Parallelism)
	}
	if Params.SaltLength != 16 {
		t.Errorf("SaltLength = %d, want 16", Params.SaltLength)
	}
	if Params.KeyLength != 32 {
		t.Errorf("KeyLength = %d, want 32", Params.KeyLength)
	}
}

func TestHashVerifyRoundTrip(t *testing.T) {
	h, err := Hash("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	ok, err := Verify("correct horse battery staple", h)
	if err != nil || !ok {
		t.Fatalf("verify good password: ok=%v err=%v", ok, err)
	}
	ok, _ = Verify("wrong", h)
	if ok {
		t.Fatal("verify wrong password returned true")
	}
}

func TestCheckPolicy(t *testing.T) {
	cases := []struct {
		name, pw, email string
		wantCode        string // "" => accepted
	}{
		{"ok", "a-strong-passphrase-123", "user@example.com", ""},
		{"too short", "short", "user@example.com", CodeTooShort},
		{"too long", strings.Repeat("a", 129), "user@example.com", CodeTooLong},
		{"length counts runes not bytes", "şifreğüçöıŞİ", "user@example.com", ""},
		{"breached", "password1234", "user@example.com", CodeBreached},
		{"equals local part", "longusername", "longusername@example.com", CodeMatchesEmail},
		{"equals whole email", "me@example.com", "me@example.com", CodeMatchesEmail},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := CheckPolicy(c.pw, c.email)
			if c.wantCode == "" {
				if err != nil {
					t.Fatalf("CheckPolicy(%q) = %v, want nil", c.pw, err)
				}
				return
			}
			var pe PolicyError
			if !errors.As(err, &pe) {
				t.Fatalf("expected PolicyError, got %T (%v)", err, err)
			}
			if pe.Code != c.wantCode {
				t.Fatalf("code = %q, want %q", pe.Code, c.wantCode)
			}
		})
	}
}
