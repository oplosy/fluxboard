package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/billing"
	"github.com/mesutokul/fluxboard/backend/internal/domain/chat"
	"github.com/mesutokul/fluxboard/backend/internal/infrastructure/postgres/gen"
)

// ChatRepo is the Postgres-backed chat.Repository ([T], RLS).
type ChatRepo struct{ tp *TenantPool }

// NewChatRepo builds a ChatRepo over the tenant pool.
func NewChatRepo(tp *TenantPool) *ChatRepo { return &ChatRepo{tp: tp} }

var _ chat.Repository = (*ChatRepo)(nil)

// ChatSendPayload is the chat:send outbox payload; the worker loads the rest
// from the delivery row so the secret never travels through Redis.
type ChatSendPayload struct {
	OrgID      string `json:"org_id"`
	DeliveryID string `json:"delivery_id"`
}

func mapChatChannel(row gen.ChatChannel) chat.Channel {
	cats := row.Categories
	if cats == nil {
		cats = []string{}
	}
	return chat.Channel{
		ID: row.ID.String(), OrgID: row.OrgID.String(), Provider: chat.Provider(row.Provider),
		Name: row.Name, URLEnc: row.UrlEnc, URLHint: row.UrlHint, Categories: cats,
		Enabled: row.Enabled, CreatedBy: row.CreatedBy.String(), CreatedAt: row.CreatedAt,
	}
}

func (r *ChatRepo) Create(ctx context.Context, orgID string, c *chat.Channel) error {
	id, err := parseUUID(c.ID)
	if err != nil {
		return fmt.Errorf("chat channel create: %w", err)
	}
	by, err := parseUUID(c.CreatedBy)
	if err != nil {
		return fmt.Errorf("chat channel create: by: %w", err)
	}
	return r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		oid, _ := parseUUID(orgID)
		return q.CreateChatChannel(ctx, gen.CreateChatChannelParams{
			ID: id, OrgID: oid, Provider: string(c.Provider), Name: c.Name, UrlEnc: c.URLEnc,
			UrlHint: c.URLHint, Categories: c.Categories, Enabled: c.Enabled, CreatedBy: by,
		})
	})
}

func (r *ChatRepo) List(ctx context.Context, orgID string) ([]chat.Channel, error) {
	out := make([]chat.Channel, 0)
	err := r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		oid, _ := parseUUID(orgID)
		rows, err := q.ListChatChannels(ctx, oid)
		if err != nil {
			return err
		}
		for _, row := range rows {
			out = append(out, mapChatChannel(row))
		}
		return nil
	})
	return out, err
}

func (r *ChatRepo) Get(ctx context.Context, orgID, id string) (*chat.Channel, error) {
	cid, err := parseUUID(id)
	if err != nil {
		return nil, domain.ErrNotFound
	}
	var out *chat.Channel
	err = r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		oid, _ := parseUUID(orgID)
		row, err := q.GetChatChannel(ctx, gen.GetChatChannelParams{OrgID: oid, ID: cid})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		c := mapChatChannel(row)
		out = &c
		return nil
	})
	return out, err
}

func (r *ChatRepo) Update(ctx context.Context, orgID string, c *chat.Channel) error {
	cid, err := parseUUID(c.ID)
	if err != nil {
		return domain.ErrNotFound
	}
	return r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		oid, _ := parseUUID(orgID)
		n, err := q.UpdateChatChannel(ctx, gen.UpdateChatChannelParams{
			Name: c.Name, Categories: c.Categories, Enabled: c.Enabled, OrgID: oid, ID: cid,
		})
		if err != nil {
			return err
		}
		if n == 0 {
			return domain.ErrNotFound
		}
		return nil
	})
}

func (r *ChatRepo) Delete(ctx context.Context, orgID, id string) error {
	cid, err := parseUUID(id)
	if err != nil {
		return domain.ErrNotFound
	}
	return r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		oid, _ := parseUUID(orgID)
		n, err := q.DeleteChatChannel(ctx, gen.DeleteChatChannelParams{OrgID: oid, ID: cid})
		if err != nil {
			return err
		}
		if n == 0 {
			return domain.ErrNotFound
		}
		return nil
	})
}

// Enqueue writes the delivery and its chat:send outbox row in one tenant tx, so
// a delivery is never recorded without being queued (or the reverse).
func (r *ChatRepo) Enqueue(ctx context.Context, orgID string, d *chat.Delivery) error {
	id, err := parseUUID(d.ID)
	if err != nil {
		return fmt.Errorf("chat enqueue: %w", err)
	}
	cid, err := parseUUID(d.ChannelID)
	if err != nil {
		return fmt.Errorf("chat enqueue: channel: %w", err)
	}
	payload, err := json.Marshal(ChatSendPayload{OrgID: orgID, DeliveryID: d.ID})
	if err != nil {
		return err
	}
	return r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		oid, _ := parseUUID(orgID)
		if err := q.CreateChatDelivery(ctx, gen.CreateChatDeliveryParams{
			ID: id, OrgID: oid, ChannelID: cid, Category: d.Category,
			Title: d.Title, Body: d.Body, Link: d.Link,
		}); err != nil {
			return err
		}
		return insertOutbox(ctx, q, oid, billing.OutboxItem{
			ID: uuid.NewString(), OrgID: orgID, Kind: billing.OutboxChatSend, Payload: payload,
		})
	})
}

func (r *ChatRepo) ListDeliveries(ctx context.Context, orgID string, limit int) ([]chat.Delivery, error) {
	out := make([]chat.Delivery, 0)
	err := r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		oid, _ := parseUUID(orgID)
		rows, err := q.ListChatDeliveries(ctx, gen.ListChatDeliveriesParams{OrgID: oid, Lim: int32(limit)})
		if err != nil {
			return err
		}
		for _, row := range rows {
			out = append(out, chat.Delivery{
				ID: row.ID.String(), OrgID: row.OrgID.String(), ChannelID: row.ChannelID.String(),
				ChannelName: row.ChannelName, Category: row.Category, Title: row.Title, Body: row.Body,
				Link: row.Link, Status: row.Status, Attempts: int(row.Attempts), LastError: row.LastError,
				CreatedAt: row.CreatedAt, SentAt: tsPtr(row.SentAt),
			})
		}
		return nil
	})
	return out, err
}

func (r *ChatRepo) GetDelivery(ctx context.Context, orgID, id string) (*chat.Delivery, error) {
	did, err := parseUUID(id)
	if err != nil {
		return nil, domain.ErrNotFound
	}
	var out *chat.Delivery
	err = r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		oid, _ := parseUUID(orgID)
		row, err := q.GetChatDelivery(ctx, gen.GetChatDeliveryParams{OrgID: oid, ID: did})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		out = &chat.Delivery{
			ID: row.ID.String(), OrgID: row.OrgID.String(), ChannelID: row.ChannelID.String(),
			Category: row.Category, Title: row.Title, Body: row.Body, Link: row.Link,
			Status: row.Status, Attempts: int(row.Attempts), LastError: row.LastError,
			CreatedAt: row.CreatedAt, SentAt: tsPtr(row.SentAt),
		}
		return nil
	})
	return out, err
}

func (r *ChatRepo) RecordAttempt(ctx context.Context, orgID, id, status, lastError string) error {
	did, err := parseUUID(id)
	if err != nil {
		return domain.ErrNotFound
	}
	return r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		oid, _ := parseUUID(orgID)
		n, err := q.RecordChatAttempt(ctx, gen.RecordChatAttemptParams{
			Status: status, LastError: lastError, OrgID: oid, ID: did,
		})
		if err != nil {
			return err
		}
		if n == 0 {
			return domain.ErrNotFound
		}
		return nil
	})
}

func (r *ChatRepo) TaskPath(ctx context.Context, orgID, taskID string) (string, string, int, error) {
	tid, err := parseUUID(taskID)
	if err != nil {
		return "", "", 0, domain.ErrNotFound
	}
	var row gen.GetTaskPathRow
	err = r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		oid, _ := parseUUID(orgID)
		var err error
		row, err = q.GetTaskPath(ctx, gen.GetTaskPathParams{OrgID: oid, ID: tid})
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNotFound
		}
		return err
	})
	return row.Slug, row.Key, int(row.Number), err
}
