package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// cors lets the page at origin call the API with its cookies, and nobody
// else: a request from any other origin gets no CORS headers, so the
// browser keeps the response from the page. It answers preflights itself,
// since no route handles OPTIONS.
func cors(origin string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Responses differ by Origin, so caches must keep them apart.
		c.Header("Vary", "Origin")
		if origin == "" || c.GetHeader("Origin") != origin {
			c.Next()
			return
		}

		h := c.Writer.Header()
		h.Set("Access-Control-Allow-Origin", origin)
		h.Set("Access-Control-Allow-Credentials", "true")
		h.Set("Access-Control-Expose-Headers", HeaderRequestID)

		if c.Request.Method == http.MethodOptions && c.GetHeader("Access-Control-Request-Method") != "" {
			h.Set("Access-Control-Allow-Methods", "GET, POST, DELETE")
			h.Set("Access-Control-Allow-Headers", "Content-Type, "+HeaderRequestID)
			h.Set("Access-Control-Max-Age", "600")
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// noSniff keeps browsers from second-guessing Content-Type, so a JSON or
// JPEG response is never run as a script or a page.
func noSniff(c *gin.Context) {
	c.Header("X-Content-Type-Options", "nosniff")
	c.Next()
}
