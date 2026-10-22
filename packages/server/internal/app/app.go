// Package app wires the server together and runs it: the database, the HTTP
// API and the background work of the same process, all stopped together.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/sync/errgroup"

	"github.com/cerebellum-labs/cerebellum/packages/server/internal/config"
	"github.com/cerebellum-labs/cerebellum/packages/server/internal/db"
	"github.com/cerebellum-labs/cerebellum/packages/server/internal/server"
)

const (
	// Without it an unreachable database would hang the start for as long as
	// the OS takes to give up on the TCP connection.
	connectTimeout  = 30 * time.Second
	shutdownTimeout = 15 * time.Second
)

// Run connects to the database, brings its schema up to date and serves on
// cfg.Port until ctx is done or a component fails, then shuts everything
// down.
func Run(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	return run(ctx, cfg, logger, func(ctx context.Context, addr string) (net.Listener, error) {
		var lc net.ListenConfig
		return lc.Listen(ctx, "tcp", addr)
	})
}

// run is Run with the listener opened by listen, once the database is ready;
// tests hand in one they already hold.
func run(
	ctx context.Context, cfg config.Config, logger *slog.Logger,
	listen func(ctx context.Context, addr string) (net.Listener, error),
) error {
	gin.SetMode(gin.ReleaseMode)

	connectCtx, cancel := context.WithTimeout(ctx, connectTimeout)
	pool, err := db.Open(connectCtx, cfg.DatabaseURL)
	cancel()
	if err != nil {
		return err
	}
	defer pool.Close()

	// Before serving: the handlers expect the current schema.
	if err := db.Migrate(ctx, pool, logger); err != nil {
		return err
	}

	srv, err := server.New(cfg.Port, logger)
	if err != nil {
		return err
	}
	ln, err := listen(ctx, srv.Addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", srv.Addr, err)
	}

	// Every component runs in g with gctx: a signal or the first failure
	// cancels gctx, and the others stop. Background work of this process,
	// like reviewing uncertain decisions, joins the group the same way.
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		logger.InfoContext(gctx, "server starting", slog.String("addr", ln.Addr().String()))
		// Serve closes ln when it returns.
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve http: %w", err)
		}
		return nil
	})
	g.Go(func() error {
		<-gctx.Done()
		logger.InfoContext(gctx, "shutting down")
		// The drain gets time of its own: gctx is already cancelled.
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(gctx), shutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shut down http server: %w", err)
		}
		return nil
	})
	return g.Wait()
}
