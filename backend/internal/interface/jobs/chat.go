package jobs

// chat:send delivers one Slack / Teams channel message (FR-NTF-005, ADR-038).
// The outbox payload carries only ids; the handler loads the delivery, decrypts
// the channel's webhook URL and posts. Outcomes are written to the delivery row
// (the UI's status source). Error strings stored or logged here are fixed
// classifications — a net/http error embeds the URL, which is a secret.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/hibiken/asynq"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/chat"
)

// TypeChatSend is the chat message delivery task kind.
const TypeChatSend = "chat:send"

// chatSendTimeout bounds one webhook POST.
const chatSendTimeout = 10 * time.Second

// ChatPayload is the chat:send task payload (postgres.ChatSendPayload).
type ChatPayload struct {
	OrgID      string `json:"org_id"`
	DeliveryID string `json:"delivery_id"`
}

// URLDecrypter reverses the at-rest encryption of a channel URL.
type URLDecrypter interface {
	Decrypt(encoded string) (string, error)
}

// ChatDeps wires the chat:send handler.
type ChatDeps struct {
	Repo   chat.Repository
	Cipher URLDecrypter
	// HTTP is the client used for the POST; nil builds one that never follows
	// redirects. Validate re-checks the URL right before sending (defense in
	// depth); nil uses the strict provider allow-list. Only tests override it.
	HTTP     *http.Client
	Validate func(p chat.Provider, rawURL string) error
	Logger   *slog.Logger
}

// Chat runs chat:send.
type Chat struct {
	d ChatDeps
	// retry reports (retries used, retry budget) of the running task; a field so
	// tests can stand in for Asynq's private context keys.
	retry func(ctx context.Context) (int, int)
}

// NewChat builds the handler.
func NewChat(d ChatDeps) *Chat {
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
	if d.HTTP == nil {
		d.HTTP = &http.Client{
			Timeout:       chatSendTimeout,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		}
	}
	if d.Validate == nil {
		d.Validate = chat.ValidateWebhookURL
	}
	return &Chat{d: d, retry: func(ctx context.Context) (int, int) {
		used, _ := asynq.GetRetryCount(ctx)
		budget, _ := asynq.GetMaxRetry(ctx)
		return used, budget
	}}
}

// Register wires the handler onto an Asynq mux.
func (c *Chat) Register(mux *asynq.ServeMux) { mux.HandleFunc(TypeChatSend, c.Handle) }

// errChatPermanent marks a failure retrying cannot fix.
var errChatPermanent = errors.New("permanent")

// Handle posts one delivery. A retryable failure returns an error so Asynq
// backs off; once the retry budget is spent (or the failure is permanent) the
// delivery is marked failed and the task ends without an error.
func (c *Chat) Handle(ctx context.Context, t *asynq.Task) error {
	var p ChatPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil || p.OrgID == "" || p.DeliveryID == "" {
		return fmt.Errorf("chat:send: bad payload: %w", asynq.SkipRetry)
	}
	d, err := c.d.Repo.GetDelivery(ctx, p.OrgID, p.DeliveryID)
	if errors.Is(err, domain.ErrNotFound) {
		return nil // channel (and its deliveries) deleted meanwhile
	}
	if err != nil {
		return fmt.Errorf("chat:send: load delivery: %w", err)
	}
	if d.Status != chat.StatusPending {
		return nil // already settled; a duplicate task is a no-op
	}
	ch, err := c.d.Repo.Get(ctx, p.OrgID, d.ChannelID)
	if errors.Is(err, domain.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("chat:send: load channel: %w", err)
	}

	reason, err := c.post(ctx, ch, d)
	if err == nil {
		if rerr := c.d.Repo.RecordAttempt(ctx, p.OrgID, d.ID, chat.StatusSent, ""); rerr != nil {
			return fmt.Errorf("chat:send: record: %w", rerr)
		}
		return nil
	}
	retried, maxRetry := c.retry(ctx)
	final := errors.Is(err, errChatPermanent) || retried >= maxRetry
	status := chat.StatusPending
	if final {
		status = chat.StatusFailed
	}
	c.d.Logger.Warn("chat:send failed", "org", p.OrgID, "delivery", d.ID, "channel", ch.ID, "reason", reason, "final", final)
	if rerr := c.d.Repo.RecordAttempt(ctx, p.OrgID, d.ID, status, reason); rerr != nil {
		return fmt.Errorf("chat:send: record: %w", rerr)
	}
	if final {
		return nil
	}
	return fmt.Errorf("chat:send: %s", reason)
}

// post sends the message. It returns a URL-free reason for the delivery row.
func (c *Chat) post(ctx context.Context, ch *chat.Channel, d *chat.Delivery) (string, error) {
	rawURL, err := c.d.Cipher.Decrypt(ch.URLEnc)
	if err != nil {
		return "could not decrypt webhook URL", errChatPermanent
	}
	if err := c.d.Validate(ch.Provider, rawURL); err != nil {
		return "webhook URL is not allowed", errChatPermanent
	}
	body, err := chat.RenderPayload(ch.Provider, d.Title, d.Body, d.Link)
	if err != nil {
		return "could not build message", errChatPermanent
	}
	ctx, cancel := context.WithTimeout(ctx, chatSendTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, bytes.NewReader(body))
	if err != nil {
		return "could not build request", errChatPermanent
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.d.HTTP.Do(req)
	if err != nil {
		return "request failed (network error or timeout)", err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return "", nil
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
		return fmt.Sprintf("HTTP %d from %s", resp.StatusCode, ch.Provider), fmt.Errorf("status %d", resp.StatusCode)
	default: // 3xx (redirects are never followed) and other 4xx: the URL itself is wrong
		return fmt.Sprintf("HTTP %d from %s", resp.StatusCode, ch.Provider), errChatPermanent
	}
}
