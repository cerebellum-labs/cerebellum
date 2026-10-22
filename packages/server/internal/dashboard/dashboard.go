// Package dashboard serves the API of the web dashboard: devices, the
// journal of decisions with their frames, statistics, and a WebSocket
// stream of changes. Reading is open to everyone; changing devices takes
// the admin password.
package dashboard

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"

	"github.com/cerebellum-labs/cerebellum/packages/server/internal/db"
	"github.com/cerebellum-labs/cerebellum/packages/server/internal/events"
	"github.com/cerebellum-labs/cerebellum/packages/server/internal/httperr"
)

// Config is what the dashboard API needs beyond its dependencies.
type Config struct {
	// AdminPassword unlocks changing devices.
	AdminPassword string
	// SessionSecret signs the admin cookie.
	SessionSecret []byte
	// AllowedOrigin is the dashboard's origin when it runs on a host of its
	// own; empty when it shares the API's origin.
	AllowedOrigin string
}

// Handler serves the dashboard API.
type Handler struct {
	q             *db.Queries
	hub           *events.Hub
	logger        *slog.Logger
	sessions      sessions
	adminPassword string
	allowedOrigin string
	// wsOrigins are the hosts, besides the API's own, whose pages may open
	// the event stream.
	wsOrigins    []string
	loginLimiter *rate.Limiter
	now          func() time.Time
}

// New returns the dashboard API. It publishes its own changes to hub and
// streams everything published there to the connected dashboards.
func New(q *db.Queries, hub *events.Hub, cfg Config, logger *slog.Logger) *Handler {
	h := &Handler{
		q:             q,
		hub:           hub,
		logger:        logger,
		sessions:      sessions{secret: cfg.SessionSecret},
		adminPassword: cfg.AdminPassword,
		allowedOrigin: cfg.AllowedOrigin,
		loginLimiter:  newLoginLimiter(),
		now:           time.Now,
	}
	if u, err := url.Parse(cfg.AllowedOrigin); err == nil && u.Host != "" {
		h.wsOrigins = []string{u.Host}
	}
	return h
}

// Register mounts the API under /api/dashboard.
func (h *Handler) Register(r gin.IRouter) {
	g := r.Group("/api/dashboard", h.checkOrigin)

	g.POST("/login", h.login)
	g.POST("/logout", h.logout)

	g.GET("/devices", h.listDevices)
	g.POST("/devices", h.requireAdmin, h.createDevice)
	g.POST("/devices/:id/rotate-token", h.requireAdmin, h.rotateToken)
	g.DELETE("/devices/:id", h.requireAdmin, h.deleteDevice)

	g.GET("/decisions", h.listDecisions)
	g.GET("/decisions/:id", h.getDecision)
	g.GET("/stats", h.stats)
	g.GET("/frames/:id", h.frame)

	g.GET("/ws", h.stream)
}

// bindJSON decodes the request body into v. On failure it answers 413 or
// 400 and returns false.
func bindJSON(c *gin.Context, v any) bool {
	err := c.ShouldBindJSON(v)
	if err == nil {
		return true
	}
	if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
		httperr.PayloadTooLarge(c)
		return false
	}
	httperr.BadRequest(c, "the request body is not valid JSON")
	return false
}

// internalError logs err with the request's context and answers 500,
// keeping the details out of the response.
func (h *Handler) internalError(c *gin.Context, msg string, err error) {
	h.logger.LogAttrs(c.Request.Context(), slog.LevelError, msg, slog.Any("error", err))
	httperr.InternalError(c)
}
