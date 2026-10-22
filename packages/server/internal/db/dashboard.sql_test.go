//go:build integration

package db

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNotifyDashboard(t *testing.T) {
	t.Parallel()
	pool := migratedPool(t)
	ctx := t.Context()

	listener, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Release()
	if _, err := listener.Exec(ctx, "LISTEN dashboard"); err != nil {
		t.Fatal(err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// On a failed step the connection must go back to the pool, or closing
	// the pool at cleanup blocks; after Commit this does nothing.
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	id := uuid.Must(uuid.NewV7())
	// Sent twice in one transaction, delivered once.
	for range 2 {
		if err := New(tx).NotifyDashboard(ctx, NotifyDashboardParams{TableName: "decisions", ID: id}); err != nil {
			t.Fatalf("notify: %v", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}

	waitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	n, err := listener.Conn().WaitForNotification(waitCtx)
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	var payload struct {
		Table string    `json:"table"`
		ID    uuid.UUID `json:"id"`
	}
	if err := json.Unmarshal([]byte(n.Payload), &payload); err != nil {
		t.Fatalf("payload %q: %v", n.Payload, err)
	}
	if payload.Table != "decisions" || payload.ID != id {
		t.Errorf("got %+v, want decisions %s", payload, id)
	}

	shortCtx, cancelShort := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancelShort()
	n, err = listener.Conn().WaitForNotification(shortCtx)
	switch {
	case err == nil:
		t.Errorf("got a second notification %q, want one", n.Payload)
	case !errors.Is(err, context.DeadlineExceeded):
		t.Errorf("waiting for a second notification: %v", err)
	}
}
