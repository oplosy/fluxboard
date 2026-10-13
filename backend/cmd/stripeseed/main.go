// Command stripeseed idempotently populates the plans table (docs/06-BILLING.md
// §1, FR-BILL-002). It is the local-dev equivalent of `make stripe-seed`.
//
// MODE=stub (default): upsert the three static plan rows with null Stripe price
// IDs — no external calls. This is enough for entitlement derivation, quota
// enforcement, and the stub checkout flow.
//
// MODE=live: additionally create (or find) the Stripe catalog — Billing Meters,
// one product per paid plan, the licensed seat price and the metered usage
// prices — and write the returned ids into the plans table (ADR-029). Only
// test-mode keys are accepted until the plan catalog is final (report B02).
//
// Re-running is safe: plans are upserted on plans.code, and the Stripe objects
// are found by fixed product id / price lookup key before anything is created.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	stripe "github.com/stripe/stripe-go/v86"

	"github.com/mesutokul/fluxboard/backend/internal/config"
	"github.com/mesutokul/fluxboard/backend/internal/infrastructure/postgres/planseed"
	stripex "github.com/mesutokul/fluxboard/backend/internal/infrastructure/stripe"
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
	live := cfg.StripeMode == "live"
	if live {
		if err := requireTestKey(cfg.StripeSecretKey); err != nil {
			return err
		}
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
			"max_projects", p.MaxProjects, "max_agents", p.MaxAgents, "metered", p.Metered)
	}
	if !live {
		logger.Info("stripeseed complete (stub)", "plans", len(planseed.Plans))
		return nil
	}

	var paid []stripex.CatalogPlan
	for _, p := range planseed.Plans {
		if p.MonthlyPrice > 0 {
			paid = append(paid, stripex.CatalogPlan{Code: p.Code, Name: p.Name, SeatUnitAmount: p.MonthlyPrice, Metered: p.Metered})
		}
	}
	ids, err := stripex.SeedCatalog(ctx, stripe.NewClient(cfg.StripeSecretKey), planseed.Currency, paid)
	if err != nil {
		return err
	}
	for _, p := range paid {
		id := ids[p.Code]
		if err := planseed.SetStripeIDs(ctx, pool, p.Code, id.ProductID, id.SeatPriceID, id.StoragePriceID, id.APIPriceID); err != nil {
			return err
		}
		logger.Info("stripe catalog", "plan", p.Code, "product", id.ProductID, "seat_price", id.SeatPriceID,
			"storage_price", id.StoragePriceID, "api_price", id.APIPriceID)
	}
	logger.Info("stripeseed complete (live, test mode)", "plans", len(planseed.Plans), "paid", len(paid))
	return nil
}

// requireTestKey keeps live-mode keys out until the catalog's prices are final
// (report B02); the usage prices are still $0 placeholders.
func requireTestKey(key string) error {
	switch {
	case strings.HasPrefix(key, "sk_test_"), strings.HasPrefix(key, "rk_test_"):
		return nil
	case strings.HasPrefix(key, "sk_live_"), strings.HasPrefix(key, "rk_live_"):
		return errors.New("stripeseed: refusing a live-mode key: the plan catalog is not final (report B02); seed with a test-mode key")
	default:
		return fmt.Errorf("stripeseed: STRIPE_SECRET_KEY must be a Stripe test-mode secret key (sk_test_…) in MODE=live")
	}
}
