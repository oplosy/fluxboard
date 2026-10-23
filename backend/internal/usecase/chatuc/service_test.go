package chatuc

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/chat"
	"github.com/mesutokul/fluxboard/backend/internal/domain/notify"
)

// fakeRepo keeps channels per org so cross-org access can be asserted.
type fakeRepo struct {
	channels   map[string]map[string]*chat.Channel // org → id → channel
	deliveries []*chat.Delivery
}

func newFakeRepo() *fakeRepo { return &fakeRepo{channels: map[string]map[string]*chat.Channel{}} }

func (f *fakeRepo) Create(_ context.Context, org string, c *chat.Channel) error {
	if f.channels[org] == nil {
		f.channels[org] = map[string]*chat.Channel{}
	}
	cp := *c
	f.channels[org][c.ID] = &cp
	return nil
}
func (f *fakeRepo) List(_ context.Context, org string) ([]chat.Channel, error) {
	var out []chat.Channel
	for _, c := range f.channels[org] {
		out = append(out, *c)
	}
	return out, nil
}
func (f *fakeRepo) Get(_ context.Context, org, id string) (*chat.Channel, error) {
	c, ok := f.channels[org][id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	cp := *c
	return &cp, nil
}
func (f *fakeRepo) Update(_ context.Context, org string, c *chat.Channel) error {
	if _, ok := f.channels[org][c.ID]; !ok {
		return domain.ErrNotFound
	}
	cp := *c
	f.channels[org][c.ID] = &cp
	return nil
}
func (f *fakeRepo) Delete(_ context.Context, org, id string) error {
	if _, ok := f.channels[org][id]; !ok {
		return domain.ErrNotFound
	}
	delete(f.channels[org], id)
	return nil
}
func (f *fakeRepo) Enqueue(_ context.Context, _ string, d *chat.Delivery) error {
	f.deliveries = append(f.deliveries, d)
	return nil
}
func (f *fakeRepo) ListDeliveries(context.Context, string, int) ([]chat.Delivery, error) {
	return nil, nil
}
func (f *fakeRepo) GetDelivery(context.Context, string, string) (*chat.Delivery, error) {
	return nil, domain.ErrNotFound
}
func (f *fakeRepo) RecordAttempt(context.Context, string, string, string, string) error { return nil }
func (f *fakeRepo) TaskPath(_ context.Context, _, taskID string) (string, string, int, error) {
	if taskID == "gone" {
		return "", "", 0, domain.ErrNotFound
	}
	return "acme", "WEB", 12, nil
}

type fakeCipher struct{}

func (fakeCipher) Encrypt(s string) (string, error) { return "enc:" + strings.ToUpper(s), nil }

const slackURL = "https://hooks.slack.com/services/T0/B0/TOPSECRETTOKEN"

func newSvc() (*Service, *fakeRepo) {
	r := newFakeRepo()
	return New(Deps{Repo: r, Cipher: fakeCipher{}, BaseURL: "https://app.example.com/"}), r
}

func TestCreate_RejectsBadURLs(t *testing.T) {
	s, _ := newSvc()
	for _, u := range []string{
		"http://hooks.slack.com/services/T0/B0/x", // not https
		"https://example.com/services/T0/B0/x",    // other host
		"https://10.0.0.5/services/T0/B0/x",       // IP
		"https://hooks.slack.com@evil.test/services/T0/B0/x",
		"",
	} {
		_, err := s.Create(context.Background(), "org1", "u1", CreateInput{Provider: chat.ProviderSlack, Name: "dev", URL: u})
		if !errors.Is(err, domain.ErrValidation) {
			t.Errorf("%q: want ErrValidation, got %v", u, err)
		}
	}
}

func TestCreate_StoresEncryptedAndNeverEchoesSecret(t *testing.T) {
	s, repo := newSvc()
	c, err := s.Create(context.Background(), "org1", "u1", CreateInput{
		Provider: chat.ProviderSlack, Name: "dev", URL: slackURL, Categories: []string{"mention"},
	})
	if err != nil {
		t.Fatal(err)
	}
	stored := repo.channels["org1"][c.ID]
	if stored.URLEnc == slackURL || !strings.HasPrefix(stored.URLEnc, "enc:") {
		t.Fatalf("url not encrypted: %q", stored.URLEnc)
	}
	if strings.Contains(c.URLHint, "TOPSECRET") || !strings.HasSuffix(c.URLHint, "OKEN") {
		t.Fatalf("hint leaks the token or is wrong: %q", c.URLHint)
	}
}

func TestCreate_ChannelLimit(t *testing.T) {
	s, _ := newSvc()
	for i := 0; i <= chat.MaxChannelsPerOrg; i++ {
		_, err := s.Create(context.Background(), "org1", "u1", CreateInput{Provider: chat.ProviderSlack, Name: "c", URL: slackURL})
		if i < chat.MaxChannelsPerOrg && err != nil {
			t.Fatalf("channel %d: %v", i, err)
		}
		if i == chat.MaxChannelsPerOrg && !errors.Is(err, domain.ErrValidation) {
			t.Fatalf("over the cap: want ErrValidation, got %v", err)
		}
	}
}

func TestNotify_FiltersByCategoryAndEnabled(t *testing.T) {
	s, repo := newSvc()
	ctx := context.Background()
	mk := func(name string, cats []string) *chat.Channel {
		c, err := s.Create(ctx, "org1", "u1", CreateInput{Provider: chat.ProviderSlack, Name: name, URL: slackURL, Categories: cats})
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	mention := mk("mentions", []string{"mention"})
	mk("comments", []string{"comment"})
	off := mk("off", []string{"mention"})
	if _, err := s.Update(ctx, "org1", off.ID, UpdateInput{Name: "off", Categories: []string{"mention"}, Enabled: false}); err != nil {
		t.Fatal(err)
	}

	s.Notify(ctx, "org1", notify.CategoryMention, "You were mentioned", "For Bob: hi", "task", "t1")

	if len(repo.deliveries) != 1 || repo.deliveries[0].ChannelID != mention.ID {
		t.Fatalf("deliveries = %+v; want exactly one to the mention channel", repo.deliveries)
	}
	if got := repo.deliveries[0].Link; got != "https://app.example.com/app/acme/projects/WEB/tasks/12" {
		t.Fatalf("link = %q", got)
	}

	// A category nobody subscribed to queues nothing; a vanished task falls back to the app root.
	repo.deliveries = nil
	s.Notify(ctx, "org1", notify.CategoryBilling, "t", "b", "", "")
	if len(repo.deliveries) != 0 {
		t.Fatalf("unsubscribed category queued %d deliveries", len(repo.deliveries))
	}
	s.Notify(ctx, "org1", notify.CategoryMention, "t", "b", "task", "gone")
	if len(repo.deliveries) != 1 || repo.deliveries[0].Link != "https://app.example.com" {
		t.Fatalf("fallback link wrong: %+v", repo.deliveries)
	}
}

func TestTest_QueuesRegardlessOfCategoriesAndState(t *testing.T) {
	s, repo := newSvc()
	ctx := context.Background()
	c, _ := s.Create(ctx, "org1", "u1", CreateInput{Provider: chat.ProviderTeams, Name: "t", URL: "https://x.webhook.office.com/webhookb2/abc"})
	if _, err := s.Update(ctx, "org1", c.ID, UpdateInput{Name: "t", Enabled: false}); err != nil {
		t.Fatal(err)
	}
	d, err := s.Test(ctx, "org1", c.ID)
	if err != nil || d.Category != chat.CategoryTest || len(repo.deliveries) != 1 {
		t.Fatalf("test send: %+v %v", d, err)
	}
}

func TestCrossOrgIsolation(t *testing.T) {
	s, _ := newSvc()
	ctx := context.Background()
	c, _ := s.Create(ctx, "orgA", "u1", CreateInput{Provider: chat.ProviderSlack, Name: "a", URL: slackURL})

	if _, err := s.Test(ctx, "orgB", c.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Test across orgs: %v", err)
	}
	if _, err := s.Update(ctx, "orgB", c.ID, UpdateInput{Name: "x"}); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Update across orgs: %v", err)
	}
	if err := s.Delete(ctx, "orgB", c.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Delete across orgs: %v", err)
	}
	if list, _ := s.List(ctx, "orgB"); len(list) != 0 {
		t.Errorf("List leaked %d channels", len(list))
	}
}
