// Package chat is the domain of Slack / Teams notification channels
// (FR-NTF-005, ADR-038): the provider webhook-URL allow-list, channel and
// delivery models, the message payload formats and the repository port.
// Stdlib-only apart from sibling domain packages.
package chat

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/notify"
)

// Provider is a supported chat service.
type Provider string

const (
	ProviderSlack Provider = "slack"
	ProviderTeams Provider = "teams"
)

// Delivery statuses.
const (
	StatusPending = "pending"
	StatusSent    = "sent"
	StatusFailed  = "failed"
)

// CategoryTest marks the test-send delivery; it is never category-filtered.
const CategoryTest = "test"

const (
	// MaxChannelsPerOrg caps channels per org.
	MaxChannelsPerOrg = 10
	// MaxNameLen caps a channel name (runes).
	MaxNameLen = 60
	maxURLLen  = 500
	// Message fields are cut so a payload stays far below provider limits.
	maxTitleLen = 200
	maxBodyLen  = 1000
)

// MaxRetries is the Asynq retry budget of a chat:send task; the worker marks
// a delivery failed once that many retries are used up.
const MaxRetries = 5

// Channel is one org-level webhook destination. URLEnc is the encrypted
// webhook URL; it is never exposed through the API (only URLHint is).
type Channel struct {
	ID         string
	OrgID      string
	Provider   Provider
	Name       string
	URLEnc     string
	URLHint    string
	Categories []string
	Enabled    bool
	CreatedBy  string
	CreatedAt  time.Time
}

// Wants reports whether the channel is on and subscribed to cat.
func (c *Channel) Wants(cat notify.Category) bool {
	return c.Enabled && slices.Contains(c.Categories, string(cat))
}

// Delivery is one message queued for a channel; Status tracks the outbox run.
type Delivery struct {
	ID          string
	OrgID       string
	ChannelID   string
	ChannelName string // joined on list only
	Category    string
	Title       string
	Body        string
	Link        string
	Status      string
	Attempts    int
	LastError   string
	CreatedAt   time.Time
	SentAt      *time.Time
}

// ValidProvider reports whether p is supported.
func ValidProvider(p Provider) bool { return p == ProviderSlack || p == ProviderTeams }

// NormalizeChannel trims the name and validates it and the category list
// (duplicates dropped; every entry must be a notification-center category).
func NormalizeChannel(name string, categories []string) (string, []string, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > MaxNameLen {
		return "", nil, domain.ErrValidation
	}
	out := make([]string, 0, len(categories))
	for _, c := range categories {
		if !notify.Category(c).Valid() {
			return "", nil, domain.ErrValidation
		}
		if !slices.Contains(out, c) {
			out = append(out, c)
		}
	}
	return name, out, nil
}

// ValidateWebhookURL accepts only https incoming-webhook URLs on the
// provider's own hosts: no credentials, no explicit port, no IP literals.
// Because the host is allow-listed, the URL can never point into our own
// network (SSRF); only the path is chosen by the admin.
func ValidateWebhookURL(p Provider, raw string) error {
	if !ValidProvider(p) || len(raw) > maxURLLen {
		return domain.ErrValidation
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" || u.Fragment != "" {
		return domain.ErrValidation
	}
	host := strings.ToLower(u.Hostname())
	if host == "" || net.ParseIP(host) != nil {
		return domain.ErrValidation
	}
	ok := false
	switch p {
	case ProviderSlack:
		ok = host == "hooks.slack.com" && strings.HasPrefix(u.Path, "/services/") && len(u.Path) > len("/services/")
	case ProviderTeams:
		ok = (host == "outlook.office.com" || strings.HasSuffix(host, ".webhook.office.com") ||
			strings.HasSuffix(host, ".logic.azure.com")) && len(u.Path) > 1
	}
	if !ok {
		return domain.ErrValidation
	}
	return nil
}

// Hint renders the display-only form of a validated URL: host plus the last
// four characters of the path, enough to tell channels apart without
// revealing the secret token.
func Hint(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	tail := u.Path
	if len(tail) > 4 {
		tail = tail[len(tail)-4:]
	}
	return u.Hostname() + "/…" + tail
}

// RenderPayload builds the provider-specific JSON body: Slack mrkdwn text, or
// a Teams Adaptive Card message (accepted by Workflows webhooks).
func RenderPayload(p Provider, title, body, link string) ([]byte, error) {
	title, body = clip(title, maxTitleLen), clip(body, maxBodyLen)
	switch p {
	case ProviderSlack:
		text := "*" + slackEscape(title) + "*"
		if body != "" {
			text += "\n" + slackEscape(body)
		}
		if link != "" {
			text += "\n<" + link + "|Open in Fluxboard>"
		}
		return json.Marshal(map[string]string{"text": text})
	case ProviderTeams:
		blocks := []map[string]any{{"type": "TextBlock", "text": title, "weight": "Bolder", "wrap": true}}
		if body != "" {
			blocks = append(blocks, map[string]any{"type": "TextBlock", "text": body, "wrap": true})
		}
		card := map[string]any{
			"$schema": "http://adaptivecards.io/schemas/adaptive-card.json",
			"type":    "AdaptiveCard", "version": "1.4", "body": blocks,
		}
		if link != "" {
			card["actions"] = []map[string]any{{"type": "Action.OpenUrl", "title": "Open in Fluxboard", "url": link}}
		}
		return json.Marshal(map[string]any{
			"type": "message",
			"attachments": []map[string]any{{
				"contentType": "application/vnd.microsoft.card.adaptive", "content": card,
			}},
		})
	}
	return nil, fmt.Errorf("chat: unknown provider %q", p)
}

// slackEscape neutralizes the three characters Slack treats as control syntax.
func slackEscape(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(s)
}

func clip(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	return string([]rune(s)[:max-1]) + "…"
}
