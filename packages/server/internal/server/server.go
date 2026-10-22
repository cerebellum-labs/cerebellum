package server

import (
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Option configures the router built by New and NewRouter.
type Option func(*options)

type options struct {
	allowedOrigin string
	routes        []func(gin.IRouter)
}

// WithAllowedOrigin lets the page at origin, like
// https://demo.cerebellumlabs.dev, call the API from the browser with its
// cookies. Without it only pages of the API's own origin can.
func WithAllowedOrigin(origin string) Option {
	return func(o *options) { o.allowedOrigin = origin }
}

// WithRoutes adds a feature's routes, like the dashboard API's.
func WithRoutes(register func(gin.IRouter)) Option {
	return func(o *options) { o.routes = append(o.routes, register) }
}

// New returns the HTTP server of the API, listening on port.
func New(port string, logger *slog.Logger, opts ...Option) (*http.Server, error) {
	router, err := NewRouter(logger, opts...)
	if err != nil {
		return nil, err
	}
	return &http.Server{
		Addr:              net.JoinHostPort("", port),
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       60 * time.Second,
		// net/http's own errors (bad TLS handshakes, broken connections)
		// go to the JSON log too, not as plain text to stderr.
		ErrorLog: slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}, nil
}
