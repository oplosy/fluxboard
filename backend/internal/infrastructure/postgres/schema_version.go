package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// SchemaVersion is the readiness check for migrations (docs/10-INFRA-DEVOPS.md
// §6): Ping fails unless golang-migrate's schema_migrations row is clean and at
// least Want, the newest migration this binary was built with. A newer schema
// passes, so pods of the previous release stay ready while expand-only
// migrations for the next one are applied ahead of the rollout.
//
// schema_migrations belongs to golang-migrate, not to the sqlc schema, hence
// the inline query.
type SchemaVersion struct {
	Pool *pgxpool.Pool
	Want uint
}

// Ping implements the readiness Pinger.
func (s SchemaVersion) Ping(ctx context.Context) error {
	var version int64
	var dirty bool
	if err := s.Pool.QueryRow(ctx, `SELECT version, dirty FROM schema_migrations`).Scan(&version, &dirty); err != nil {
		return fmt.Errorf("read schema_migrations: %w", err)
	}
	return checkSchemaVersion(version, dirty, s.Want)
}

func checkSchemaVersion(version int64, dirty bool, want uint) error {
	if dirty {
		return fmt.Errorf("migration %d is dirty", version)
	}
	if version < 0 || uint64(version) < uint64(want) {
		return fmt.Errorf("schema at %d, binary needs %d", version, want)
	}
	return nil
}
