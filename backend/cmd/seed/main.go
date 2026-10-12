// Command seed loads the deterministic demo fixture of docs/12-TESTING.md §5
// into a local database (`make seed`, which runs stripeseed first so the plan
// rows exist).
//
// Usage:
//
//	seed [-clock YYYY-MM-DD]
//
// Every ID is fixed and every row is derived from the clock (default: today,
// UTC midnight), so the same clock always yields the same data. Re-running
// resets the seed orgs to the fixture. It runs on the owner pool
// (DATABASE_URL_MIGRATE) and refuses APP_ENV=prod: the demo users share a
// published password.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mesutokul/fluxboard/backend/internal/config"
	"github.com/mesutokul/fluxboard/backend/internal/pkg/aesgcm"
	"github.com/mesutokul/fluxboard/backend/internal/pkg/argon2x"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if err := run(logger, os.Args[1:]); err != nil {
		logger.Error("seed failed", "err", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger, args []string) error {
	fs := flag.NewFlagSet("seed", flag.ContinueOnError)
	clockFlag := fs.String("clock", "", "anchor date YYYY-MM-DD (UTC); default today")
	if err := fs.Parse(args); err != nil {
		return err
	}
	clock, err := parseClock(*clockFlag, time.Now())
	if err != nil {
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.IsProd() {
		return errors.New("seed: refusing to run with APP_ENV=prod (demo users share a published password)")
	}
	if cfg.DatabaseURLMigrate == "" {
		return errors.New("seed: DATABASE_URL_MIGRATE (owner role) is required; the fixture writes across tenants")
	}

	passwordHash, err := argon2x.Hash(DemoPassword)
	if err != nil {
		return fmt.Errorf("hash demo password: %w", err)
	}
	totpSecret, err := encryptAdminTOTP(cfg.TOTPEncKey)
	if err != nil {
		return err
	}
	if totpSecret == "" {
		logger.Warn("TOTP_ENC_KEY not set; padmin@demo.dev seeded without 2FA, so the admin console stays locked")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURLMigrate)
	if err != nil {
		return err
	}
	defer pool.Close()

	f := Build(clock)
	if err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		return write(ctx, tx, f, passwordHash, totpSecret)
	}); err != nil {
		return err
	}

	logger.Info("seed complete",
		"clock", clock.Format(time.DateOnly), "users", len(f.Users), "orgs", len(f.Orgs),
		"projects", len(f.Projects), "tasks", len(f.Tasks), "usage_rows", len(f.Usage),
		"password", "see docs/12-TESTING.md §5")
	if totpSecret != "" {
		logger.Info("platform admin 2FA (dev only)", "email", "padmin@demo.dev", "totp_secret", PlatformAdminTOTPSecret)
	}
	return nil
}

// parseClock returns the fixture anchor: the given date, or today, at UTC midnight.
func parseClock(s string, now time.Time) (time.Time, error) {
	if s == "" {
		y, m, d := now.UTC().Date()
		return time.Date(y, m, d, 0, 0, 0, 0, time.UTC), nil
	}
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("seed: -clock must be YYYY-MM-DD: %w", err)
	}
	return t, nil
}

// encryptAdminTOTP encrypts the fixed admin TOTP secret the same way 2FA
// enrollment stores it (AES-GCM with TOTP_ENC_KEY). Empty key ⇒ no 2FA.
func encryptAdminTOTP(key string) (string, error) {
	if key == "" {
		return "", nil
	}
	k, err := aesgcm.ParseKey(key)
	if err != nil {
		return "", fmt.Errorf("totp enc key: %w", err)
	}
	c, err := aesgcm.New(k)
	if err != nil {
		return "", err
	}
	return c.Encrypt(PlatformAdminTOTPSecret)
}
