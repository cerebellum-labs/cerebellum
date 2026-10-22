package server

import (
	"crypto/rand"

	"github.com/gin-gonic/gin"

	"github.com/cerebellum-labs/cerebellum/packages/server/internal/logging"
)

// HeaderRequestID carries the id that ties a request to its log records.
const HeaderRequestID = "X-Request-ID"

const maxRequestIDLen = 64

// requestID reuses the caller's X-Request-ID, so a device can match its own
// logs with ours, or generates one. It echoes the id back on the response
// and puts it in the request context, where the logger picks it up for every
// record logged while handling the request.
func requestID(c *gin.Context) {
	id := c.GetHeader(HeaderRequestID)
	if !validRequestID(id) {
		id = rand.Text()
	}
	c.Request = c.Request.WithContext(logging.WithRequestID(c.Request.Context(), id))
	c.Header(HeaderRequestID, id)
	c.Next()
}

// validRequestID accepts short ids of letters, digits, '-', '_' and '.'.
// Anything else is replaced: the id is written to every log record, and a
// caller must not be able to forge log lines or bloat them through it.
func validRequestID(id string) bool {
	if id == "" || len(id) > maxRequestIDLen {
		return false
	}
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '-', r == '_', r == '.':
		default:
			return false
		}
	}
	return true
}
