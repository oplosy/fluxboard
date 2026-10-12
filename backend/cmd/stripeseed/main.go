// Command stripeseed idempotently populates the plans table (docs/06-BILLING.md
// §1, FR-BILL-002). It is the local-dev equivalent of `make stripe-seed`.
//
// MODE=stub (default): upsert the three static plan rows with null Stripe price
// IDs — no external calls. This is enough for entitlement derivation, quota
// enforcement, and the stub checkout flow.
//
// MODE=live (future): create Stripe products + licensed seat prices + metered
// prices, then upsert the returned price IDs. Left as a documented branch; the
// real stripe-go adapter (Phase 4 §4) fills it in.
//
// Re-running is safe: the upsert is keyed on plans.code (ON CONFLICT DO UPDATE),
// so limits stay in sync with this file on every run.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mesutokul/fluxboard/backend/internal/config"
	"github.com/mesutokul/fluxboard/backend/internal/infrastructure/postgres/planseed"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if err := run(logger); err != nil {
		logger.Error("stripeseed failed", "err", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.StripeMode == "live" {
		return fmt.Errorf("stripeseed: MODE=live not implemented yet (stub only); wire the stripe-go adapter first")
	}

	ctx := context.Background()
	// Prefer the owner DSN (DDL/seed role) when set; plans has no RLS so the app
	// role also works, but the owner keeps seed writes off the runtime role.
	dsn := cfg.DatabaseURLMigrate
	if dsn == "" {
		dsn = cfg.DatabaseURL
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := planseed.Upsert(ctx, pool); err != nil {
		return err
	}
	for _, p := range planseed.Plans {
		logger.Info("seeded plan", "code", p.Code, "max_members", p.MaxMembers,
			"max_projects", p.MaxProjects, "metered", p.Metered)
	}
	logger.Info("stripeseed complete (stub)", "plans", len(planseed.Plans))
	return nil
}
