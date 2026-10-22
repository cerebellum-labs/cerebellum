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

	"github.com/cerebellum-labs/cerebellum/packages/server/internal/agentapi"
	"github.com/cerebellum-labs/cerebellum/packages/server/internal/config"
	"github.com/cerebellum-labs/cerebellum/packages/server/internal/dashboard"
	"github.com/cerebellum-labs/cerebellum/packages/server/internal/db"
	"github.com/cerebellum-labs/cerebellum/packages/server/internal/events"
	"github.com/cerebellum-labs/cerebellum/packages/server/internal/model"
	"github.com/cerebellum-labs/cerebellum/packages/server/internal/review"
	"github.com/cerebellum-labs/cerebellum/packages/server/internal/server"
)

const (
	// Without it an unreachable database would hang the start for as long as
	// the OS takes to give up on the TCP connection.
	connectTimeout  = 30 * time.Second
	shutdownTimeout = 15 * time.Second
	// A last bound on a request to the cloud model, above the per-call
	// timeouts of /decide and the review.
	modelHTTPTimeout = 3 * time.Minute
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

	q := db.New(pool)
	hub := &events.Hub{}
	dash := dashboard.New(q, hub, dashboard.Config{
		AdminPassword: cfg.AdminPassword,
		SessionSecret: cfg.SessionSecret,
		AllowedOrigin: cfg.DashboardOrigin,
	}, logger)

	modelCfg := model.Config{
		BaseURL:         cfg.ModelBaseURL,
		APIKey:          cfg.ModelAPIKey,
		Model:           cfg.ModelID,
		PriceInputPerM:  cfg.ModelPriceInputPerM,
		PriceOutputPerM: cfg.ModelPriceOutputPerM,
	}
	modelHTTP := &http.Client{Timeout: modelHTTPTimeout}
	// The robot waits on /decide, so it gets the fast answer. Nobody waits
	// on a review, whose point is a closer look at the frame: it reasons.
	decider := model.New(modelCfg, modelHTTP)
	modelCfg.Reasoning = true
	reviewerModel := model.New(modelCfg, modelHTTP)
	if !decider.Configured() {
		logger.WarnContext(ctx, "NEBIUS_API_KEY is not set: decisions that need the cloud model fail and nothing is reviewed")
	}
	reviewer := review.New(q, reviewerModel, dash, logger)
	agents := agentapi.New(pool, decider, dash, reviewer, logger)

	srv, err := server.New(cfg.Port, logger,
		server.WithAllowedOrigin(cfg.DashboardOrigin),
		server.WithRoutes(dash.Register),
		server.WithRoutes(agents.Register),
	)
	if err != nil {
		return err
	}
	// Shutdown waits for requests but not for hijacked connections: closing
	// the hub ends the event streams, so their WebSockets close too.
	srv.RegisterOnShutdown(hub.Close)
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
	if reviewerModel.Configured() {
		g.Go(func() error { return reviewer.Run(gctx) })
	}
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
