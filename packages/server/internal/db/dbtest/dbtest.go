//go:build integration

// Package dbtest gives each integration test a PostgreSQL database of its
// own on the server TEST_DATABASE_URL points to (make test-integration uses
// the one from make db). The URL must be in postgres:// form, and its user
// must be allowed to create databases.
package dbtest

import (
	"context"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// maxConns keeps a test's pool small: tests and packages run in parallel,
// and together they must stay under the server's max_connections (100).
const maxConns = 4

// NewDatabase creates an empty database, drops it when the test ends, and
// returns its URL.
func NewDatabase(t *testing.T) string {
	t.Helper()

	raw := os.Getenv("TEST_DATABASE_URL")
	if raw == "" {
		t.Fatal("TEST_DATABASE_URL is not set")
	}
	dbURL, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse TEST_DATABASE_URL: %v", err)
	}

	ctx := t.Context()
	// Cleanups run after t.Context is cancelled.
	cleanupCtx := context.WithoutCancel(ctx)

	admin, err := pgx.Connect(ctx, raw)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() {
		if err := admin.Close(cleanupCtx); err != nil {
			t.Errorf("close admin connection: %v", err)
		}
	})

	name := "test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	ident := pgx.Identifier{name}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+ident); err != nil {
		t.Fatalf("create database: %v", err)
	}
	// Registered after the close above, so it runs before it.
	t.Cleanup(func() {
		if _, err := admin.Exec(cleanupCtx, "DROP DATABASE "+ident+" WITH (FORCE)"); err != nil {
			t.Errorf("drop database: %v", err)
		}
	})

	dbURL.Path = "/" + name
	return dbURL.String()
}

// NewPool opens a small pool on the database at dbURL, closed when the test
// ends.
func NewPool(t *testing.T, dbURL string) *pgxpool.Pool {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	cfg.MaxConns = maxConns
	pool, err := pgxpool.NewWithConfig(t.Context(), cfg)
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}
