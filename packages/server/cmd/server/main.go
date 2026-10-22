// Command server runs the Cerebellum server: the HTTP API for devices and
// the dashboard, and its background work.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/cerebellum-labs/cerebellum/packages/server/internal/app"
	"github.com/cerebellum-labs/cerebellum/packages/server/internal/config"
	"github.com/cerebellum-labs/cerebellum/packages/server/internal/logging"
)

// version is set at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	logger := logging.New(os.Stdout).With(slog.String("version", version))
	// For libraries that log through slog's default logger.
	slog.SetDefault(logger)

	if err := run(logger); err != nil {
		logger.Error("server stopped", slog.Any("error", err))
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	// After the first signal, restore default handling, so a second Ctrl+C
	// kills the process instead of waiting out a stuck drain.
	context.AfterFunc(ctx, stop)

	return app.Run(ctx, cfg, logger)
}
