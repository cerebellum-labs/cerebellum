package server

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/cerebellum-labs/cerebellum/packages/server/internal/httperr"
)

// maxBodyBytes caps every request body. A device sends at most one camera
// frame per request; 5 MB fits a JPEG up to 4K after base64 (+33%) and
// metadata, while one request still can't exhaust memory.
const maxBodyBytes = 5 << 20

// bodyLimit rejects a declared Content-Length over the cap before anything is
// read, and caps the rest (chunked bodies, lying clients) while it is read: a
// handler then gets *http.MaxBytesError and answers httperr.PayloadTooLarge.
func bodyLimit(c *gin.Context) {
	if c.Request.ContentLength > maxBodyBytes {
		httperr.PayloadTooLarge(c)
		return
	}
	if c.Request.Body != nil {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBodyBytes)
	}
	c.Next()
}
