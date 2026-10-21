package server

import (
	"errors"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"

	"github.com/cerebellum-labs/cerebellum/packages/server/internal/httperr"
)

// recovery turns a panic into a 500 and logs it as one slog record with the
// stack in a single field. gin's own recovery prints the stack as plain text
// to stderr, which a log pipeline splits into unrelated lines.
func recovery(c *gin.Context) {
	defer func() {
		rec := recover()
		if rec == nil {
			return
		}
		// net/http uses this panic to abort a response on purpose and stays
		// quiet about it; hand it back instead of reporting a crash.
		if err, ok := rec.(error); ok && errors.Is(err, http.ErrAbortHandler) {
			panic(rec)
		}

		slog.LogAttrs(c.Request.Context(), slog.LevelError, "panic recovered",
			slog.Any("panic", rec),
			slog.String("stack", string(debug.Stack())),
			slog.String("method", c.Request.Method),
			slog.String("route", c.FullPath()),
			slog.String("request_id", requestIDFrom(c)),
		)

		// A handler that already sent headers can't change the status;
		// stopping the chain is all that is left.
		if c.Writer.Written() {
			c.Abort()
			return
		}
		httperr.InternalError(c)
	}()
	c.Next()
}
