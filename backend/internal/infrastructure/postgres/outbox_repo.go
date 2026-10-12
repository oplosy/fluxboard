package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/mesutokul/fluxboard/backend/internal/domain/billing"
	"github.com/mesutokul/fluxboard/backend/internal/infrastructure/postgres/gen"
)

// OutboxRepo is the Postgres-backed billing.OutboxRepository. outbox is [T]
// (RLS). ClaimBatch runs through TenantPool.WithTenant; Insert is a package-level
// helper called inside a caller's transaction (the webhook tx).
type OutboxRepo struct{ tp *TenantPool }

// NewOutboxRepo builds an OutboxRepo over the tenant pool.
func NewOutboxRepo(tp *TenantPool) *OutboxRepo { return &OutboxRepo{tp: tp} }

var _ billing.OutboxRepository = (*OutboxRepo)(nil)

func (r *OutboxRepo) Drain(ctx context.Context, orgID string, limit int, handoff func(billing.OutboxItem) error) (int, error) {
	var drained int
	var handoffErr error
	err := r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		drained, handoffErr = 0, nil
		oid, _ := parseUUID(orgID)
		rows, err := q.LockOutboxBatch(ctx, gen.LockOutboxBatchParams{OrgID: oid, Lim: int32(limit)})
		if err != nil {
			return err
		}
		ids := make([]uuid.UUID, 0, len(rows))
		for _, row := range rows {
			if handoffErr = handoff(billing.OutboxItem{
				ID:        row.ID.String(),
				OrgID:     row.OrgID.String(),
				Kind:      billing.OutboxKind(row.Kind),
				Payload:   row.Payload,
				CreatedAt: row.CreatedAt,
				DrainedAt: tsPtr(row.DrainedAt),
			}); handoffErr != nil {
				break // keep this row and the rest undrained for the next drain
			}
			ids = append(ids, row.ID)
		}
		if len(ids) == 0 {
			return nil
		}
		if err := q.MarkOutboxDrained(ctx, gen.MarkOutboxDrainedParams{OrgID: oid, Ids: ids}); err != nil {
			return err
		}
		drained = len(ids)
		return nil
	})
	if err != nil {
		return 0, err
	}
	return drained, handoffErr
}

// InsertEmail enqueues an email:send outbox row in its own tenant tx (satisfies
// notifyuc.OutboxWriter). Notification fan-out uses this after its own writes;
// the email:send worker rechecks the recipient's preference before sending
// (09 §2/§3).
func (r *OutboxRepo) InsertEmail(ctx context.Context, orgID string, payload json.RawMessage) error {
	return r.tp.WithTenant(ctx, orgID, func(q *gen.Queries) error {
		oid, _ := parseUUID(orgID)
		return insertOutbox(ctx, q, oid, billing.OutboxItem{
			ID:      uuid.NewString(),
			OrgID:   orgID,
			Kind:    billing.OutboxEmailSend,
			Payload: payload,
		})
	})
}

// insertOutbox writes an outbox row via q (tx-bound), inside the caller's
// transaction so the enqueue is atomic with the producing write (09 §2).
func insertOutbox(ctx context.Context, q *gen.Queries, oid uuid.UUID, item billing.OutboxItem) error {
	id, err := parseUUID(item.ID)
	if err != nil {
		return fmt.Errorf("outbox insert: id: %w", err)
	}
	return q.InsertOutbox(ctx, gen.InsertOutboxParams{
		ID:      id,
		OrgID:   oid,
		Kind:    string(item.Kind),
		Payload: item.Payload,
	})
}
