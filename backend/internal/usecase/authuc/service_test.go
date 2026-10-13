package authuc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/auth"
	"github.com/mesutokul/fluxboard/backend/internal/pkg/argon2x"
	"github.com/mesutokul/fluxboard/backend/internal/pkg/jwtx"
)

// --- fakes ----------------------------------------------------------------

type fakeUsers struct{ byEmail map[string]*auth.User }

func (f *fakeUsers) Create(_ context.Context, u *auth.User) error {
	if _, ok := f.byEmail[u.Email]; ok {
		return domain.ErrConflict
	}
	f.byEmail[u.Email] = u
	return nil
}
func (f *fakeUsers) GetByEmail(_ context.Context, e string) (*auth.User, error) {
	if u, ok := f.byEmail[e]; ok {
		return u, nil
	}
	return nil, domain.ErrNotFound
}
func (f *fakeUsers) GetByID(_ context.Context, id string) (*auth.User, error) {
	for _, u := range f.byEmail {
		if u.ID == id {
			return u, nil
		}
	}
	return nil, domain.ErrNotFound
}
func (f *fakeUsers) UpdatePasswordHash(_ context.Context, id, hash string) error {
	for _, u := range f.byEmail {
		if u.ID == id {
			u.PasswordHash = hash
		}
	}
	return nil
}
func (f *fakeUsers) MarkEmailVerified(_ context.Context, id string) error {
	for _, u := range f.byEmail {
		if u.ID == id {
			u.EmailVerified = true
		}
	}
	return nil
}
func (f *fakeUsers) SetTOTP(_ context.Context, id, enc string, enabled bool) error {
	for _, u := range f.byEmail {
		if u.ID == id {
			u.TOTPSecret = enc
			u.TOTPEnabled = enabled
		}
	}
	return nil
}

type fakeSessions struct {
	created    []*auth.Session
	reuseNext  bool
	revokedAll int
}

func (f *fakeSessions) Create(_ context.Context, s *auth.Session) error {
	f.created = append(f.created, s)
	return nil
}
func (f *fakeSessions) GetByID(context.Context, string) (*auth.Session, error) {
	return nil, domain.ErrNotFound
}
func (f *fakeSessions) Rotate(_ context.Context, _, newHash []byte, newID, ua, ip string, exp time.Time) (*auth.Session, error) {
	if f.reuseNext {
		return nil, auth.ErrRefreshReuse
	}
	return &auth.Session{ID: newID, FamilyID: "fam", UserID: "user-1", TokenHash: newHash, UserAgent: ua, IP: ip, ExpiresAt: exp}, nil
}
func (f *fakeSessions) RevokeByID(context.Context, string, string) error { return nil }
func (f *fakeSessions) RevokeAllForUser(_ context.Context, _, _ string) error {
	f.revokedAll++
	return nil
}
func (f *fakeSessions) ListForUser(context.Context, string) ([]*auth.Session, error) {
	return nil, nil
}
func (f *fakeSessions) RevokeByIDForUser(context.Context, string, string, string) error { return nil }

type fakeTokens struct{}

func (fakeTokens) Create(context.Context, *auth.OneTimeToken) error { return nil }
func (fakeTokens) Consume(context.Context, auth.TokenPurpose, []byte, time.Time) (*auth.OneTimeToken, error) {
	return nil, domain.ErrNotFound
}
func (fakeTokens) Peek(context.Context, auth.TokenPurpose, []byte) (*auth.OneTimeToken, error) {
	return nil, domain.ErrNotFound
}
func (fakeTokens) InvalidateAll(context.Context, string, auth.TokenPurpose) error { return nil }

type fakeCache struct{ active map[string]bool }

func (f *fakeCache) MarkActive(_ context.Context, sid string, _ time.Duration) error {
	f.active[sid] = true
	return nil
}
func (f *fakeCache) Status(_ context.Context, sid string) (bool, bool, error) {
	v, ok := f.active[sid]
	return v, ok, nil
}
func (f *fakeCache) Revoke(_ context.Context, sid string) error { f.active[sid] = false; return nil }

type fakeObs struct{ reuse, success, invalid int }

func (o *fakeObs) LoginAttempt(_ context.Context, r string) {
	switch r {
	case "success":
		o.success++
	case "invalid":
		o.invalid++
	}
}
func (o *fakeObs) RefreshReuse(context.Context) { o.reuse++ }

func newTestService(t *testing.T, sess *fakeSessions, obs *fakeObs) (*Service, *fakeUsers) {
	t.Helper()
	pem, _ := jwtx.GenerateES256PEM()
	priv, _ := jwtx.LoadPrivateKeyPEM(pem)
	signer, _ := jwtx.NewSigner(priv)
	users := &fakeUsers{byEmail: map[string]*auth.User{}}
	svc := New(Deps{
		Users: users, Sessions: sess, Tokens: fakeTokens{},
		Cache: &fakeCache{active: map[string]bool{}}, Signer: signer, Observer: obs,
	})
	return svc, users
}

func seedUser(t *testing.T, users *fakeUsers, email, password string) {
	t.Helper()
	h, err := argon2x.Hash(password)
	if err != nil {
		t.Fatal(err)
	}
	users.byEmail[email] = &auth.User{ID: "user-1", Email: email, PasswordHash: h, PlatformRole: auth.PlatformRoleUser}
}

// --- tests ----------------------------------------------------------------

func TestLoginSuccessIssuesVerifiableToken(t *testing.T) {
	sess := &fakeSessions{}
	obs := &fakeObs{}
	svc, users := newTestService(t, sess, obs)
	seedUser(t, users, "a@b.com", "a-strong-passphrase-1")

	res, err := svc.Login(context.Background(), LoginInput{Email: "A@b.com", Password: "a-strong-passphrase-1"})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if res.Tokens == nil || res.Tokens.AccessToken == "" || res.Tokens.RefreshToken == "" {
		t.Fatal("expected non-empty tokens")
	}
	if len(sess.created) != 1 {
		t.Fatalf("expected 1 session created, got %d", len(sess.created))
	}
	if obs.success != 1 {
		t.Fatalf("expected 1 success metric, got %d", obs.success)
	}
}

func TestLoginWrongPasswordUnauthorized(t *testing.T) {
	sess := &fakeSessions{}
	obs := &fakeObs{}
	svc, users := newTestService(t, sess, obs)
	seedUser(t, users, "a@b.com", "a-strong-passphrase-1")

	_, err := svc.Login(context.Background(), LoginInput{Email: "a@b.com", Password: "wrong-password-here"})
	if !errors.Is(err, domain.ErrUnauthorized) || codeOf(err) != "invalid_credentials" {
		t.Fatalf("want invalid_credentials, got %v", err)
	}
	if obs.invalid != 1 {
		t.Fatalf("expected invalid metric, got %d", obs.invalid)
	}
}

func TestLoginUnknownEmailUniformUnauthorized(t *testing.T) {
	svc, _ := newTestService(t, &fakeSessions{}, &fakeObs{})
	_, err := svc.Login(context.Background(), LoginInput{Email: "nobody@b.com", Password: "whatever-123456"})
	if !errors.Is(err, domain.ErrUnauthorized) || codeOf(err) != "invalid_credentials" {
		t.Fatalf("want the same invalid_credentials as a wrong password, got %v", err)
	}
}

func TestRefreshReuseReportsAndFails(t *testing.T) {
	sess := &fakeSessions{reuseNext: true}
	obs := &fakeObs{}
	svc, _ := newTestService(t, sess, obs)

	_, err := svc.Refresh(context.Background(), "some-raw-token", "ua", "1.2.3.4")
	if err == nil {
		t.Fatal("expected error on reuse")
	}
	if obs.reuse != 1 {
		t.Fatalf("expected reuse metric incremented, got %d", obs.reuse)
	}
}

// codeOf returns the CodedError code carried by err, or "".
func codeOf(err error) string {
	var ce *domain.CodedError
	if errors.As(err, &ce) {
		return ce.Code
	}
	return ""
}

// fieldOf returns the CodedError field carried by err, or "".
func fieldOf(err error) string {
	var ce *domain.CodedError
	if errors.As(err, &ce) {
		return ce.Field
	}
	return ""
}

func TestRegisterFieldErrors(t *testing.T) {
	cases := []struct {
		name            string
		in              RegisterInput
		wantCode, field string
	}{
		{"short password", RegisterInput{Email: "a@b.com", Password: "short"}, "password_too_short", "password"},
		{"long password", RegisterInput{Email: "a@b.com", Password: string(make([]byte, 129))}, "password_too_long", "password"},
		{"breached password", RegisterInput{Email: "a@b.com", Password: "password1234"}, "password_breached", "password"},
		{"password equals email", RegisterInput{Email: "longlocalpart@b.com", Password: "longlocalpart"}, "password_matches_email", "password"},
		{"no at sign", RegisterInput{Email: "not-an-email", Password: "a-strong-passphrase-1"}, "email_invalid", "email"},
		{"no domain dot", RegisterInput{Email: "a@localhost", Password: "a-strong-passphrase-1"}, "email_invalid", "email"},
		{"header injection", RegisterInput{Email: "a@b.com\r\nBcc: x@y.z", Password: "a-strong-passphrase-1"}, "email_invalid", "email"},
		{"display name", RegisterInput{Email: "Eve <a@b.com>", Password: "a-strong-passphrase-1"}, "email_invalid", "email"},
		{"long name", RegisterInput{Email: "a@b.com", Password: "a-strong-passphrase-1", Name: string(make([]rune, 101))}, "name_too_long", "name"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, _ := newTestService(t, &fakeSessions{}, &fakeObs{})
			_, err := svc.Register(context.Background(), c.in)
			if !errors.Is(err, domain.ErrValidation) || codeOf(err) != c.wantCode || fieldOf(err) != c.field {
				t.Fatalf("want %s on %s, got %v (code=%q field=%q)", c.wantCode, c.field, err, codeOf(err), fieldOf(err))
			}
		})
	}
}

func TestRegisterDuplicateReportsEmailTaken(t *testing.T) {
	svc, users := newTestService(t, &fakeSessions{}, &fakeObs{})
	seedUser(t, users, "a@b.com", "a-strong-passphrase-1")
	_, err := svc.Register(context.Background(), RegisterInput{Email: " A@B.com ", Password: "a-strong-passphrase-2"})
	if !errors.Is(err, domain.ErrConflict) || codeOf(err) != "email_taken" || fieldOf(err) != "email" {
		t.Fatalf("want email_taken conflict on email, got %v", err)
	}
}
