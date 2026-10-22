//go:build integration

package app

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/cerebellum-labs/cerebellum/packages/server/internal/config"
	"github.com/cerebellum-labs/cerebellum/packages/server/internal/db/dbtest"
)

// TestRun starts the whole server on a database of its own, waits for it to
// answer, and checks it stops cleanly once its context is cancelled.
func TestRun(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	// The test holds the listener from the start, so no other process can
	// take the port between picking it and serving on it.
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	// For when Run fails before serving; otherwise Serve has closed it and
	// this second close only reports that.
	t.Cleanup(func() { _ = ln.Close() })
	listen := func(context.Context, string) (net.Listener, error) { return ln, nil }

	cfg := config.Config{Port: "0", DatabaseURL: dbtest.NewDatabase(t)}
	done := make(chan error, 1)
	go func() { done <- run(ctx, cfg, slog.New(slog.DiscardHandler), listen) }()

	healthURL := "http://" + ln.Addr().String() + "/health"
	// Long enough for a slow connect and the migrations.
	deadline := time.Now().Add(connectTimeout + 10*time.Second)
	for {
		status, err := get(ctx, healthURL)
		if err == nil && status == http.StatusOK {
			break
		}
		select {
		case err := <-done:
			t.Fatalf("Run returned before serving: %v", err)
		case <-time.After(50 * time.Millisecond):
		}
		if time.Now().After(deadline) {
			t.Fatalf("server did not answer %s: status %d, error %v", healthURL, status, err)
		}
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Run returned %v, want nil after cancel", err)
		}
	case <-time.After(shutdownTimeout + 5*time.Second):
		t.Fatal("Run did not return after cancel")
	}
}

// get requests url with a timeout of its own and returns the status code.
func get(ctx context.Context, url string) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	if err := resp.Body.Close(); err != nil {
		return 0, fmt.Errorf("close body: %w", err)
	}
	return resp.StatusCode, nil
}
