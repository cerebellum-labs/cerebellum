//go:build integration

package db

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestGetDeviceByTokenHash(t *testing.T) {
	t.Parallel()
	pool := migratedPool(t)
	ctx := t.Context()
	q := New(pool)
	id, hash := insertDevice(t, pool)

	device, err := q.GetDeviceByTokenHash(ctx, hash)
	if err != nil {
		t.Fatalf("get device: %v", err)
	}
	if device.ID != id {
		t.Errorf("got device %s, want %s", device.ID, id)
	}

	if _, err := pool.Exec(ctx, `UPDATE devices SET deleted_at = now() WHERE id = $1`, id); err != nil {
		t.Fatalf("delete device: %v", err)
	}
	if _, err := q.GetDeviceByTokenHash(ctx, hash); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("deleted device: got error %v, want %v", err, pgx.ErrNoRows)
	}
}
