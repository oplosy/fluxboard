// Package planseed holds the plan catalog and its idempotent upsert into the
// plans table (docs/06-BILLING.md §1, FR-BILL-002). cmd/stripeseed (stub mode)
// and the integration suites load the catalog through it, so the tier numbers
// live in one place.
package planseed

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
)

// Plan mirrors the frozen tier table (docs/build/PHASE-4-BILLING.md §4.0.3).
// Money-independent limits only; -1 = unlimited.
type Plan struct {
	Code               string
	Name               string
	MaxMembers         int
	MaxProjects        int
	MaxStorageBytes    int64
	APIRatePerMin      int
	AuditRetentionDays int
	Metered            bool
	MonthlyPrice       int64 // recurring price in minor units (cents); backs admin MRR (FR-ADM-002)
}

// Plans is the catalog.
var Plans = []Plan{
	{"free", "Free", 5, 3, 2 << 30, 60, 7, false, 0},
	{"pro", "Pro", 25, 50, 50 << 30, 300, 30, false, 1200},
	{"business", "Business", -1, -1, 500 << 30, 1200, 365, true, 4900},
}

// Execer is satisfied by *pgxpool.Pool, *pgx.Conn and pgx.Tx.
type Execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

const upsert = `
INSERT INTO plans (code, name, max_members, max_projects, max_storage_bytes,
                   api_rate_per_min, audit_retention_days, metered, monthly_price)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
ON CONFLICT (code) DO UPDATE SET
  name                 = EXCLUDED.name,
  max_members          = EXCLUDED.max_members,
  max_projects         = EXCLUDED.max_projects,
  max_storage_bytes    = EXCLUDED.max_storage_bytes,
  api_rate_per_min     = EXCLUDED.api_rate_per_min,
  audit_retention_days = EXCLUDED.audit_retention_days,
  metered              = EXCLUDED.metered,
  monthly_price        = EXCLUDED.monthly_price`

// Currency of every catalog price (minor units in MonthlyPrice).
const Currency = "usd"

// Upsert writes every plan, keyed on plans.code, so re-running keeps the table
// in sync with the catalog. Stripe ids are left untouched (SetStripeIDs).
func Upsert(ctx context.Context, db Execer) error {
	for _, p := range Plans {
		if _, err := db.Exec(ctx, upsert, p.Code, p.Name, p.MaxMembers, p.MaxProjects,
			p.MaxStorageBytes, p.APIRatePerMin, p.AuditRetentionDays, p.Metered, p.MonthlyPrice); err != nil {
			return fmt.Errorf("seed plan %q: %w", p.Code, err)
		}
	}
	return nil
}

const setStripeIDs = `
UPDATE plans
SET stripe_product_id        = $2,
    seat_price_id            = $3,
    metered_storage_price_id = nullif($4, ''),
    metered_api_price_id     = nullif($5, '')
WHERE code = $1`

// SetStripeIDs records the Stripe product and price ids stripe-seed created
// for a plan (MODE=live). The plan row must already exist (Upsert first).
func SetStripeIDs(ctx context.Context, db Execer, code, productID, seatPriceID, storagePriceID, apiPriceID string) error {
	tag, err := db.Exec(ctx, setStripeIDs, code, productID, seatPriceID, storagePriceID, apiPriceID)
	if err != nil {
		return fmt.Errorf("set stripe ids for plan %q: %w", code, err)
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("set stripe ids: plan %q not found", code)
	}
	return nil
}
