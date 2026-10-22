// Package db connects to PostgreSQL and keeps its schema up to date.
// Migrations live in migrations/ and are built into the binary; the queries
// in queries/ are compiled into Go by sqlc (make generate).
package db

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

// maxConns caps the pool unless the URL sets pool_max_conns. pgxpool's own
// cap is max(4, CPUs), too few when /decide holds a connection for the
// length of a model call. pgx's defaults for connection lifetime (1 h),
// idle time (30 min) and health checks (1 min) are kept.
const maxConns = 20

//go:embed migrations/*.sql
var migrations embed.FS

// Open connects to the database at url and checks the connection.
func Open(ctx context.Context, url string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	if !strings.Contains(url, "pool_max_conns") {
		cfg.MaxConns = maxConns
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create database pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("connect to database: %w", err)
	}
	return pool, nil
}

// Migrate applies the migrations the database doesn't have yet and logs
// each one through logger.
func Migrate(ctx context.Context, pool *pgxpool.Pool, logger *slog.Logger) (err error) {
	// goose works on database/sql. Closing this handle leaves the pool open.
	sqlDB := stdlib.OpenDBFromPool(pool)
	defer func() {
		if closeErr := sqlDB.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close migration handle: %w", closeErr))
		}
	}()

	provider, err := newProvider(sqlDB)
	if err != nil {
		return err
	}
	results, err := provider.Up(ctx)
	if err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}
	for _, r := range results {
		logger.LogAttrs(ctx, slog.LevelInfo, "migration applied",
			slog.Int64("version", r.Source.Version),
			slog.Float64("duration_ms", float64(r.Duration.Microseconds())/1000),
		)
	}
	return nil
}

func newProvider(sqlDB *sql.DB) (*goose.Provider, error) {
	fsys, err := fs.Sub(migrations, "migrations")
	if err != nil {
		return nil, fmt.Errorf("open migrations: %w", err)
	}
	// Several instances may start at once; an advisory lock lets one of them
	// migrate while the others wait.
	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return nil, fmt.Errorf("create migration lock: %w", err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, fsys, goose.WithSessionLocker(locker))
	if err != nil {
		return nil, fmt.Errorf("create migration provider: %w", err)
	}
	return provider, nil
}
