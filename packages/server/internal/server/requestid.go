package server

import (
	"crypto/rand"

	"github.com/gin-gonic/gin"
)

// HeaderRequestID carries the id that ties a request to its log records.
const HeaderRequestID = "X-Request-ID"

const (
	contextKeyRequestID = "request_id"
	maxRequestIDLen     = 64
)

// requestID reuses the caller's X-Request-ID, so a device can match its own
// logs with ours, or generates one. It echoes the id back on the response.
func requestID(c *gin.Context) {
	id := c.GetHeader(HeaderRequestID)
	if !validRequestID(id) {
		id = rand.Text()
	}
	c.Set(contextKeyRequestID, id)
	c.Header(HeaderRequestID, id)
	c.Next()
}

func requestIDFrom(c *gin.Context) string {
	return c.GetString(contextKeyRequestID)
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
