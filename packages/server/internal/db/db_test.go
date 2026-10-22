//go:build integration

package db

import (
	"crypto/sha256"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"

	"github.com/cerebellum-labs/cerebellum/packages/server/internal/db/dbtest"
)

var discardLogger = slog.New(slog.DiscardHandler)

// testPool returns a pool on a new, empty database of the test's own.
func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	return dbtest.NewPool(t, dbtest.NewDatabase(t))
}

// migratedPool returns a pool on a new database with the schema applied.
func migratedPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool := testPool(t)
	if err := Migrate(t.Context(), pool, discardLogger); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return pool
}

// insertDevice adds a device under a key of its own and returns its id and
// token hash.
func insertDevice(t *testing.T, pool *pgxpool.Pool) (id uuid.UUID, tokenHash []byte) {
	t.Helper()
	hash := sha256.Sum256([]byte(uuid.NewString()))
	key := "device-" + uuid.NewString()[:8]
	err := pool.QueryRow(t.Context(),
		`INSERT INTO devices (key, name, token_hash) VALUES ($1, 'Test device', $2) RETURNING id`,
		key, hash[:]).Scan(&id)
	if err != nil {
		t.Fatalf("insert device: %v", err)
	}
	return id, hash[:]
}

func TestMigrate(t *testing.T) {
	t.Parallel()
	pool := testPool(t)
	ctx := t.Context()

	if err := Migrate(ctx, pool, discardLogger); err != nil {
		t.Fatalf("first run: %v", err)
	}
	// A restart finds nothing to apply.
	if err := Migrate(ctx, pool, discardLogger); err != nil {
		t.Fatalf("second run: %v", err)
	}

	// Every Down section undoes its Up, so the schema can be rebuilt.
	sqlDB := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Errorf("close: %v", err)
		}
	})
	provider, err := newProvider(sqlDB)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.DownTo(ctx, 0); err != nil {
		t.Fatalf("down: %v", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("up after down: %v", err)
	}
}
