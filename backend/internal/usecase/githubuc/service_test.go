package githubuc

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/github"
	"github.com/mesutokul/fluxboard/backend/internal/domain/tenant"
)

const (
	orgA, orgB = "0192a000-0000-7000-8000-00000000000a", "0192a000-0000-7000-8000-00000000000b"
	actor      = "0192a000-0000-7000-8000-0000000000aa"
)

type fakeCipher struct{}

func (fakeCipher) Encrypt(p string) (string, error) { return "enc:" + p, nil }
func (fakeCipher) Decrypt(c string) (string, error) { return strings.TrimPrefix(c, "enc:"), nil }

type fakeOrgs struct{ tenant.OrgRepository }

func (fakeOrgs) GetByID(_ context.Context, id string) (*tenant.Organization, error) {
	if id == orgA || id == orgB {
		return &tenant.Organization{ID: id, Slug: map[string]string{orgA: "acme", orgB: "beta"}[id]}, nil
	}
	return nil, domain.ErrNotFound
}

func (f fakeOrgs) GetBySlug(ctx context.Context, slug string) (*tenant.Organization, error) {
	switch slug {
	case "acme":
		return f.GetByID(ctx, orgA)
	case "beta":
		return f.GetByID(ctx, orgB)
	}
	return nil, domain.ErrNotFound
}

type memRepo struct {
	ints       map[string]*github.Integration
	tasks      map[string]map[string]github.TaskRef // org → "WEB-12" → task
	deliveries map[string]bool
	applied    int
	links      map[string][]string // PR → task ids
}

func newRepo() *memRepo {
	return &memRepo{
		ints: map[string]*github.Integration{}, deliveries: map[string]bool{}, links: map[string][]string{},
		tasks: map[string]map[string]github.TaskRef{
			orgA: {"WEB-12": {TaskID: "t-web-12", ProjectID: "p-web"}, "API-3": {TaskID: "t-api-3", ProjectID: "p-api"}},
			orgB: {"WEB-12": {TaskID: "t-b-web-12", ProjectID: "p-b-web"}, "BETA-1": {TaskID: "t-beta-1", ProjectID: "p-beta"}},
		},
	}
}

func (r *memRepo) Get(_ context.Context, org string) (*github.Integration, error) {
	if in, ok := r.ints[org]; ok {
		return in, nil
	}
	return nil, domain.ErrNotFound
}
func (r *memRepo) SaveSecret(_ context.Context, org, enc, by string) error {
	in := r.ints[org]
	if in == nil {
		in = &github.Integration{OrgID: org}
		r.ints[org] = in
	}
	in.SecretEnc, in.CreatedBy = enc, by
	return nil
}
func (r *memRepo) SaveSettings(_ context.Context, org string, repos, projects []string) error {
	in, ok := r.ints[org]
	if !ok {
		return domain.ErrNotFound
	}
	in.Repos, in.MoveProjects = repos, projects
	return nil
}
func (r *memRepo) Delete(_ context.Context, org string) error { delete(r.ints, org); return nil }
func (r *memRepo) ResolveTasks(_ context.Context, org string, keys []string) ([]github.TaskRef, error) {
	var out []github.TaskRef
	for _, k := range keys {
		if t, ok := r.tasks[org][k]; ok {
			out = append(out, t)
		}
	}
	return out, nil
}
func (r *memRepo) Apply(_ context.Context, org, delivery string, ev *github.PullRequestEvent, ids []string) (bool, error) {
	if r.deliveries[org+delivery] {
		return true, nil
	}
	r.deliveries[org+delivery] = true
	r.applied++
	r.links[org+ev.Repo] = append(r.links[org+ev.Repo], ids...)
	return false, nil
}
func (r *memRepo) ListByTask(context.Context, string, string) ([]github.LinkedPR, error) {
	return nil, nil
}

type fakeTasks struct {
	moved []string
	err   error
}

func (f *fakeTasks) CanViewTask(context.Context, string, string, string, tenant.OrgRole) error {
	return nil
}
func (f *fakeTasks) MoveToDone(_ context.Context, org, by, id string) error {
	if f.err != nil {
		return f.err
	}
	f.moved = append(f.moved, org+"/"+by+"/"+id)
	return nil
}

func setup(t *testing.T) (*Service, *memRepo, *fakeTasks, string) {
	t.Helper()
	repo, tasks := newRepo(), &fakeTasks{}
	svc := New(Deps{Repo: repo, Orgs: fakeOrgs{}, Tasks: tasks, Cipher: fakeCipher{}})
	secret, err := svc.RotateSecret(context.Background(), orgA, actor)
	if err != nil {
		t.Fatal(err)
	}
	return svc, repo, tasks, secret
}

func prBody(action, title, branch string, merged bool) []byte {
	m := "false"
	state := "open"
	if action == "closed" {
		state = "closed"
	}
	if merged {
		m = "true"
	}
	return []byte(`{"action":"` + action + `","pull_request":{"number":7,"title":"` + title + `","html_url":"https://github.com/acme/app/pull/7","state":"` + state + `","merged":` + m + `,"updated_at":"2026-05-01T10:00:00Z","head":{"ref":"` + branch + `"}},"repository":{"full_name":"acme/app"}}`)
}

func deliver(s *Service, secret, org, id string, body []byte) (Outcome, error) {
	return s.HandleWebhook(context.Background(), Delivery{
		OrgRef: org, Event: github.EventPullRequest, DeliveryID: id, Signature: github.Sign(secret, body), Body: body,
	})
}

func TestBadOrMissingSignatureRejectedBeforeParsing(t *testing.T) {
	svc, repo, _, secret := setup(t)
	garbage := []byte("{not json") // would be ErrValidation if it were parsed
	for name, sig := range map[string]string{
		"missing": "", "wrong secret": github.Sign("nope", garbage), "wrong prefix": "sha1=abcd",
		"other body": github.Sign(secret, []byte("other")),
	} {
		_, err := svc.HandleWebhook(context.Background(), Delivery{OrgRef: "acme", Event: github.EventPullRequest, DeliveryID: "d1", Signature: sig, Body: garbage})
		if !errors.Is(err, domain.ErrUnauthorized) {
			t.Errorf("%s: err = %v, want ErrUnauthorized", name, err)
		}
	}
	if repo.applied != 0 {
		t.Fatal("a rejected delivery wrote data")
	}
}

func TestUnknownOrgAndNoIntegrationAreIndistinguishable(t *testing.T) {
	svc, _, _, secret := setup(t)
	body := prBody("opened", "WEB-12", "x", false)
	for _, org := range []string{"ghost", orgB, "beta", "0192a000-0000-7000-8000-0000000000ff"} { // beta has no integration
		if _, err := deliver(svc, secret, org, "d", body); !errors.Is(err, domain.ErrUnauthorized) {
			t.Errorf("org %q: err = %v, want ErrUnauthorized", org, err)
		}
	}
}

func TestOrgByIDAndSlug(t *testing.T) {
	svc, _, _, secret := setup(t)
	for i, ref := range []string{"acme", orgA} {
		out, err := deliver(svc, secret, ref, "d"+string(rune('a'+i)), prBody("opened", "WEB-12", "x", false))
		if err != nil || out != OutcomeProcessed {
			t.Fatalf("ref %q: %v %v", ref, out, err)
		}
	}
}

func TestReplayedDeliveryIgnored(t *testing.T) {
	svc, repo, _, secret := setup(t)
	body := prBody("opened", "WEB-12", "x", false)
	if out, err := deliver(svc, secret, "acme", "d1", body); err != nil || out != OutcomeProcessed {
		t.Fatalf("first: %v %v", out, err)
	}
	if out, err := deliver(svc, secret, "acme", "d1", body); err != nil || out != OutcomeDuplicate {
		t.Fatalf("replay: %v %v", out, err)
	}
	if repo.applied != 1 {
		t.Fatalf("applied = %d, want 1", repo.applied)
	}
}

func TestKeyExtractionScopedToOrg(t *testing.T) {
	svc, repo, _, secret := setup(t)
	// title has WEB-12 (org A) and BETA-1 / API-3: BETA-1 only exists in org B, GHOST-9 nowhere.
	body := prBody("opened", "web-12 and BETA-1 ghost-9", "feature/api-3-thing", false)
	if _, err := deliver(svc, secret, "acme", "d1", body); err != nil {
		t.Fatal(err)
	}
	got := repo.links[orgA+"acme/app"]
	if len(got) != 2 || got[0] != "t-web-12" || got[1] != "t-api-3" {
		t.Fatalf("linked %v, want [t-web-12 t-api-3] and nothing from org B", got)
	}
	if len(repo.links[orgB+"acme/app"]) != 0 {
		t.Fatal("org B was written")
	}
}

func TestMergeMovesOnlyWhenEnabled(t *testing.T) {
	svc, _, tasks, secret := setup(t)
	merged := prBody("closed", "WEB-12 API-3", "x", true)

	// Option off: linked but not moved.
	if _, err := deliver(svc, secret, "acme", "d1", merged); err != nil {
		t.Fatal(err)
	}
	if len(tasks.moved) != 0 {
		t.Fatalf("moved %v with the option off", tasks.moved)
	}

	// Option on for the WEB project only; closed-without-merge and opened never move.
	if _, err := svc.UpdateSettings(context.Background(), orgA, nil, []string{"p-web"}); err != nil {
		t.Fatal(err)
	}
	for i, body := range [][]byte{prBody("closed", "WEB-12", "x", false), prBody("opened", "WEB-12", "x", false)} {
		if _, err := deliver(svc, secret, "acme", "n"+string(rune('0'+i)), body); err != nil {
			t.Fatal(err)
		}
	}
	if len(tasks.moved) != 0 {
		t.Fatalf("moved %v without a merge", tasks.moved)
	}
	if _, err := deliver(svc, secret, "acme", "d2", merged); err != nil {
		t.Fatal(err)
	}
	if len(tasks.moved) != 1 || tasks.moved[0] != orgA+"/"+actor+"/t-web-12" {
		t.Fatalf("moved %v, want only t-web-12 attributed to the integration owner", tasks.moved)
	}
}

func TestMergeMoveFailureDoesNotFailDelivery(t *testing.T) {
	svc, _, tasks, secret := setup(t)
	if _, err := svc.UpdateSettings(context.Background(), orgA, nil, []string{"p-web"}); err != nil {
		t.Fatal(err)
	}
	tasks.err = domain.ErrConflict // open blockers
	out, err := deliver(svc, secret, "acme", "d1", prBody("closed", "WEB-12", "x", true))
	if err != nil || out != OutcomeProcessed {
		t.Fatalf("got %v %v, want processed", out, err)
	}
}

func TestEventsAndRepoAllowList(t *testing.T) {
	svc, repo, _, secret := setup(t)
	body := []byte(`{"zen":"hi"}`)
	if out, err := svc.HandleWebhook(context.Background(), Delivery{OrgRef: "acme", Event: github.EventPing, Signature: github.Sign(secret, body), Body: body}); err != nil || out != OutcomePong {
		t.Fatalf("ping: %v %v", out, err)
	}
	if _, err := svc.HandleWebhook(context.Background(), Delivery{OrgRef: "acme", Event: "push", DeliveryID: "d", Signature: github.Sign(secret, body), Body: body}); !errors.Is(err, ErrUnsupportedEvent) {
		t.Fatalf("push: %v", err)
	}
	if out, _ := deliver(svc, secret, "acme", "d1", prBody("labeled", "WEB-12", "x", false)); out != OutcomeIgnored {
		t.Fatalf("labeled: %v", out)
	}
	if _, err := svc.UpdateSettings(context.Background(), orgA, []string{" Acme/Other "}, nil); err != nil {
		t.Fatal(err)
	}
	if out, _ := deliver(svc, secret, "acme", "d2", prBody("opened", "WEB-12", "x", false)); out != OutcomeIgnored {
		t.Fatalf("repo outside allow-list: %v", out)
	}
	if repo.applied != 0 {
		t.Fatal("ignored deliveries wrote data")
	}
}

func TestRotateReplacesSecretAndNeverStoresPlaintext(t *testing.T) {
	svc, repo, _, old := setup(t)
	if repo.ints[orgA].SecretEnc == old {
		t.Fatal("secret stored in plaintext")
	}
	fresh, err := svc.RotateSecret(context.Background(), orgA, actor)
	if err != nil || fresh == old || len(fresh) != 64 {
		t.Fatalf("rotate: %q %v", fresh, err)
	}
	if _, err := deliver(svc, old, "acme", "d1", prBody("opened", "WEB-12", "x", false)); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("old secret still works: %v", err)
	}
	if _, err := deliver(svc, fresh, "acme", "d1", prBody("opened", "WEB-12", "x", false)); err != nil {
		t.Fatalf("new secret rejected: %v", err)
	}
}

func TestDisabledWithoutCipher(t *testing.T) {
	svc := New(Deps{Repo: newRepo(), Orgs: fakeOrgs{}, Tasks: &fakeTasks{}})
	if _, err := svc.RotateSecret(context.Background(), orgA, actor); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("rotate: %v", err)
	}
	if _, err := svc.HandleWebhook(context.Background(), Delivery{OrgRef: "acme"}); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("webhook: %v", err)
	}
}

func TestUpdateSettingsValidation(t *testing.T) {
	svc, _, _, _ := setup(t)
	for _, repos := range [][]string{{"nope"}, {"a/b/c"}, {"a b/c"}, make([]string, github.MaxRepos+1)} {
		if _, err := svc.UpdateSettings(context.Background(), orgA, repos, nil); !errors.Is(err, domain.ErrValidation) {
			t.Errorf("%v: err = %v, want ErrValidation", repos, err)
		}
	}
	got, err := svc.UpdateSettings(context.Background(), orgA, []string{"Acme/App", "acme/app"}, nil)
	if err != nil || len(got.Repos) != 1 || got.Repos[0] != "acme/app" {
		t.Fatalf("normalise: %+v %v", got, err)
	}
}
