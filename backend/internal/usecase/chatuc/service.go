// Package chatuc is the application service for Slack / Teams notification
// channels (FR-NTF-005, ADR-038): channel management for org ADMIN+, test
// sends and the fan-out that queues a delivery per matching channel. Delivery
// itself runs in the worker (outbox chat:send). Domain-only deps; it never
// imports another usecase package.
package chatuc

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/chat"
	"github.com/mesutokul/fluxboard/backend/internal/domain/notify"
	"github.com/mesutokul/fluxboard/backend/internal/pkg/uuidv7"
)

// Encrypter seals a webhook URL for storage (aesgcm.Cipher).
type Encrypter interface {
	Encrypt(plaintext string) (string, error)
}

// Deps wires the service.
type Deps struct {
	Repo    chat.Repository
	Cipher  Encrypter
	BaseURL string // public web origin, for task deep links
	Logger  *slog.Logger
}

// Service implements the chat channel logic.
type Service struct {
	repo    chat.Repository
	cipher  Encrypter
	baseURL string
	logger  *slog.Logger
}

// New builds the service.
func New(d Deps) *Service {
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
	return &Service{repo: d.Repo, cipher: d.Cipher, baseURL: strings.TrimRight(d.BaseURL, "/"), logger: d.Logger}
}

// deliveryListLimit is how many recent deliveries the settings page shows.
const deliveryListLimit = 30

func newID() string { return uuidv7.New().String() }

// CreateInput is a new channel. URL is the plaintext webhook URL; it is
// validated, encrypted and never stored or returned in the clear.
type CreateInput struct {
	Provider   chat.Provider
	Name       string
	URL        string
	Categories []string
}

// Create validates and stores a channel.
func (s *Service) Create(ctx context.Context, orgID, userID string, in CreateInput) (*chat.Channel, error) {
	name, cats, err := chat.NormalizeChannel(in.Name, in.Categories)
	if err != nil {
		return nil, err
	}
	if err := chat.ValidateWebhookURL(in.Provider, in.URL); err != nil {
		return nil, domain.NewFieldError("url", "webhook_url_invalid",
			"Use the https incoming-webhook URL from Slack (hooks.slack.com) or Teams (webhook.office.com / logic.azure.com).")
	}
	existing, err := s.repo.List(ctx, orgID)
	if err != nil {
		return nil, err
	}
	if len(existing) >= chat.MaxChannelsPerOrg {
		return nil, domain.NewFieldError("name", "chat_channel_limit", fmt.Sprintf("An organization can have at most %d chat channels.", chat.MaxChannelsPerOrg))
	}
	enc, err := s.cipher.Encrypt(in.URL)
	if err != nil {
		return nil, fmt.Errorf("chat: encrypt url: %w", err)
	}
	c := &chat.Channel{
		ID: newID(), OrgID: orgID, Provider: in.Provider, Name: name, URLEnc: enc,
		URLHint: chat.Hint(in.URL), Categories: cats, Enabled: true, CreatedBy: userID,
	}
	if err := s.repo.Create(ctx, orgID, c); err != nil {
		return nil, err
	}
	return s.repo.Get(ctx, orgID, c.ID)
}

// List returns the org's channels (URLEnc is populated; callers must not expose it).
func (s *Service) List(ctx context.Context, orgID string) ([]chat.Channel, error) {
	return s.repo.List(ctx, orgID)
}

// UpdateInput changes a channel's name, categories and on/off switch.
type UpdateInput struct {
	Name       string
	Categories []string
	Enabled    bool
}

// Update rewrites the editable fields (the URL is replaced by delete + create).
func (s *Service) Update(ctx context.Context, orgID, id string, in UpdateInput) (*chat.Channel, error) {
	name, cats, err := chat.NormalizeChannel(in.Name, in.Categories)
	if err != nil {
		return nil, err
	}
	c, err := s.repo.Get(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	c.Name, c.Categories, c.Enabled = name, cats, in.Enabled
	if err := s.repo.Update(ctx, orgID, c); err != nil {
		return nil, err
	}
	return c, nil
}

// Delete removes a channel and its delivery log.
func (s *Service) Delete(ctx context.Context, orgID, id string) error {
	return s.repo.Delete(ctx, orgID, id)
}

// Test queues a test message to one channel, whatever its categories or
// on/off state, and returns the pending delivery.
func (s *Service) Test(ctx context.Context, orgID, id string) (*chat.Delivery, error) {
	c, err := s.repo.Get(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	d := &chat.Delivery{
		ID: newID(), OrgID: orgID, ChannelID: c.ID, Category: chat.CategoryTest,
		Title: "Fluxboard test message", Body: "Chat notifications are connected to " + c.Name + ".",
		Link: s.baseURL, Status: chat.StatusPending,
	}
	if err := s.repo.Enqueue(ctx, orgID, d); err != nil {
		return nil, err
	}
	return d, nil
}

// Deliveries returns the newest deliveries across the org's channels.
func (s *Service) Deliveries(ctx context.Context, orgID string) ([]chat.Delivery, error) {
	return s.repo.ListDeliveries(ctx, orgID, deliveryListLimit)
}

// Notify queues one delivery per enabled channel subscribed to cat. It is
// best-effort (called from notification fan-out): failures are logged, never
// returned. entityType/entityID resolve a task deep link when set.
func (s *Service) Notify(ctx context.Context, orgID string, cat notify.Category, title, body, entityType, entityID string) {
	channels, err := s.repo.List(ctx, orgID)
	if err != nil {
		s.logger.Error("chat: list channels failed", "err", err, "org", orgID)
		return
	}
	var link string
	linked := false
	for i := range channels {
		if !channels[i].Wants(cat) {
			continue
		}
		if !linked {
			link, linked = s.link(ctx, orgID, entityType, entityID), true
		}
		d := &chat.Delivery{
			ID: newID(), OrgID: orgID, ChannelID: channels[i].ID, Category: string(cat),
			Title: title, Body: body, Link: link, Status: chat.StatusPending,
		}
		if err := s.repo.Enqueue(ctx, orgID, d); err != nil {
			s.logger.Error("chat: enqueue failed", "err", err, "org", orgID, "channel", channels[i].ID)
		}
	}
}

// link builds the task deep link, or the app root when the task is not resolvable.
func (s *Service) link(ctx context.Context, orgID, entityType, entityID string) string {
	if entityType != "task" || entityID == "" {
		return s.baseURL
	}
	slug, key, num, err := s.repo.TaskPath(ctx, orgID, entityID)
	if err != nil {
		return s.baseURL
	}
	return fmt.Sprintf("%s/app/%s/projects/%s/tasks/%d", s.baseURL, slug, key, num)
}
