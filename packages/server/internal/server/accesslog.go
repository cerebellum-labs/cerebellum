package server

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/cerebellum-labs/cerebellum/packages/server/internal/health"
)

// accessLog writes one record per request through logger, so access lines
// are JSON like every other log line. It logs the route pattern rather than
// the path: the path of an unknown route is whatever the client sent.
func accessLog(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Probes hit /health every few seconds; logging them only buries
		// real traffic.
		if c.Request.URL.Path == health.Path {
			c.Next()
			return
		}

		start := time.Now()
		c.Next()

		logger.LogAttrs(c.Request.Context(), slog.LevelInfo, "http request",
			slog.String("method", c.Request.Method),
			slog.String("route", c.FullPath()),
			slog.Int("status", c.Writer.Status()),
			slog.Float64("latency_ms", float64(time.Since(start).Microseconds())/1000),
			slog.Int("bytes_out", max(c.Writer.Size(), 0)),
		)
	}
}
