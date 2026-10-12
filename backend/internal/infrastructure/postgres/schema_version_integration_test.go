//go:build integration

package postgres

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mesutokul/fluxboard/backend/migrations"
)

// The readiness check must read schema_migrations as the app role (granted by
// the owner's default privileges) and accept exactly the migrated version.
func TestSchemaVersionAgainstMigratedDB(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("TEST_DATABASE_URL not set; integration tests need a migrated database (see make test-integration)")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	latest, err := migrations.Latest()
	if err != nil {
		t.Fatal(err)
	}
	if err := (SchemaVersion{Pool: pool, Want: latest}).Ping(ctx); err != nil {
		t.Fatalf("fully migrated DB not ready: %v", err)
	}
	if err := (SchemaVersion{Pool: pool, Want: latest + 1}).Ping(ctx); err == nil {
		t.Fatal("a binary newer than the schema must not be ready")
	}
}
