package jobs

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/hibiken/asynq"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/chat"
)

type chatRepo struct {
	ch        *chat.Channel
	d         *chat.Delivery
	attempts  int
	lastError string
}

func (r *chatRepo) Create(context.Context, string, *chat.Channel) error  { return nil }
func (r *chatRepo) List(context.Context, string) ([]chat.Channel, error) { return nil, nil }
func (r *chatRepo) Get(context.Context, string, string) (*chat.Channel, error) {
	return r.ch, nil
}
func (r *chatRepo) Update(context.Context, string, *chat.Channel) error   { return nil }
func (r *chatRepo) Delete(context.Context, string, string) error          { return nil }
func (r *chatRepo) Enqueue(context.Context, string, *chat.Delivery) error { return nil }
func (r *chatRepo) ListDeliveries(context.Context, string, int) ([]chat.Delivery, error) {
	return nil, nil
}
func (r *chatRepo) GetDelivery(context.Context, string, string) (*chat.Delivery, error) {
	if r.d == nil {
		return nil, domain.ErrNotFound
	}
	cp := *r.d
	return &cp, nil
}
func (r *chatRepo) RecordAttempt(_ context.Context, _, _, status, lastError string) error {
	r.d.Status, r.lastError = status, lastError
	r.attempts++
	return nil
}
func (r *chatRepo) TaskPath(context.Context, string, string) (string, string, int, error) {
	return "", "", 0, domain.ErrNotFound
}

type plainCipher struct{}

func (plainCipher) Decrypt(s string) (string, error) { return s, nil }

// newChat returns a handler wired to a repo whose channel points at srvURL. The
// URL allow-list is relaxed ONLY here (the test server is plain http on
// localhost); production wiring never sets Validate.
func newChat(srvURL string, provider chat.Provider) (*Chat, *chatRepo) {
	repo := &chatRepo{
		ch: &chat.Channel{ID: "c1", Provider: provider, URLEnc: srvURL + "/services/TOPSECRET"},
		d:  &chat.Delivery{ID: "d1", ChannelID: "c1", Title: "Hello", Body: "World", Link: "https://app/x", Status: chat.StatusPending},
	}
	h := NewChat(ChatDeps{Repo: repo, Cipher: plainCipher{}, Validate: func(chat.Provider, string) error { return nil }})
	return h, repo
}

func chatTask() *asynq.Task {
	return asynq.NewTask(TypeChatSend, []byte(`{"org_id":"o1","delivery_id":"d1"}`))
}

func TestChatSend_PostsSlackPayload(t *testing.T) {
	var gotBody, gotType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody, gotType = string(b), r.Header.Get("Content-Type")
		w.WriteHeader(200)
	}))
	defer srv.Close()
	h, repo := newChat(srv.URL, chat.ProviderSlack)

	if err := h.Handle(context.Background(), chatTask()); err != nil {
		t.Fatal(err)
	}
	if repo.d.Status != chat.StatusSent || gotType != "application/json" || !strings.Contains(gotBody, `"text"`) || !strings.Contains(gotBody, "Hello") {
		t.Fatalf("status=%s type=%s body=%s", repo.d.Status, gotType, gotBody)
	}
}

func TestChatSend_RetriesOn5xxThenGivesUp(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(502)
	}))
	defer srv.Close()
	h, repo := newChat(srv.URL, chat.ProviderTeams)

	// Not the last try: error returned (Asynq retries), delivery stays pending.
	h.retry = func(context.Context) (int, int) { return 0, 5 }
	ctx := context.Background()
	if err := h.Handle(ctx, chatTask()); err == nil {
		t.Fatal("5xx must return an error so Asynq retries")
	}
	if repo.d.Status != chat.StatusPending || repo.lastError != "HTTP 502 from teams" {
		t.Fatalf("after first failure: status=%s err=%q", repo.d.Status, repo.lastError)
	}

	// Retry budget spent: marked failed, no error (task ends).
	h.retry = func(context.Context) (int, int) { return 5, 5 }
	if err := h.Handle(ctx, chatTask()); err != nil {
		t.Fatalf("final failure must not retry again: %v", err)
	}
	if repo.d.Status != chat.StatusFailed || hits.Load() != 2 {
		t.Fatalf("status=%s hits=%d", repo.d.Status, hits.Load())
	}
	if strings.Contains(repo.lastError, "TOPSECRET") || strings.Contains(repo.lastError, srv.URL) {
		t.Fatalf("stored error leaks the webhook URL: %q", repo.lastError)
	}
}

func TestChatSend_4xxIsPermanentAndRedirectsNotFollowed(t *testing.T) {
	var followed atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/elsewhere" {
			followed.Store(true)
		}
		http.Redirect(w, r, "/elsewhere", http.StatusFound)
	}))
	defer srv.Close()
	h, repo := newChat(srv.URL, chat.ProviderSlack)

	h.retry = func(context.Context) (int, int) { return 0, 5 }
	if err := h.Handle(context.Background(), chatTask()); err != nil {
		t.Fatalf("a redirect is permanent, not retried: %v", err)
	}
	if repo.d.Status != chat.StatusFailed || followed.Load() {
		t.Fatalf("status=%s redirectFollowed=%v", repo.d.Status, followed.Load())
	}
}

func TestChatSend_RejectsDisallowedURLAtSendTime(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits.Add(1) }))
	defer srv.Close()
	h, repo := newChat(srv.URL, chat.ProviderSlack)
	h.d.Validate = chat.ValidateWebhookURL // strict, as in production

	if err := h.Handle(context.Background(), chatTask()); err != nil {
		t.Fatal(err)
	}
	if hits.Load() != 0 || repo.d.Status != chat.StatusFailed {
		t.Fatalf("a non-allow-listed URL must never be requested: hits=%d status=%s", hits.Load(), repo.d.Status)
	}
}

func TestChatSend_SettledDeliveryIsNoop(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits.Add(1) }))
	defer srv.Close()
	h, repo := newChat(srv.URL, chat.ProviderSlack)
	repo.d.Status = chat.StatusSent
	if err := h.Handle(context.Background(), chatTask()); err != nil || hits.Load() != 0 {
		t.Fatalf("err=%v hits=%d", err, hits.Load())
	}
	repo.d = nil // channel deleted meanwhile
	if err := h.Handle(context.Background(), chatTask()); err != nil {
		t.Fatalf("missing delivery must end quietly: %v", err)
	}
}
