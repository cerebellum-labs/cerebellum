// Package server wires the HTTP router and its middleware.
package server

import (
	"github.com/gin-gonic/gin"

	"github.com/cerebellum-labs/cerebellum/packages/server/internal/health"
	"github.com/cerebellum-labs/cerebellum/packages/server/internal/httperr"
)

// NewRouter builds the gin engine with production defaults. gin.SetMode must
// be called before this, since gin reads the mode when the engine is created.
func NewRouter() (*gin.Engine, error) {
	r := gin.New()

	// Trust no proxy headers by default: otherwise any client can spoof its
	// IP through X-Forwarded-For. List the load balancer CIDRs here once the
	// service runs behind one.
	if err := r.SetTrustedProxies(nil); err != nil {
		return nil, err
	}

	// Answer a wrong method on a known path with 405 and an Allow header
	// instead of a misleading 404.
	r.HandleMethodNotAllowed = true

	// The request id comes first so every later step can log it; recovery
	// sits inside the access log, so a panicked request is still logged
	// with its 500.
	r.Use(
		requestID,
		accessLog,
		recovery,
		bodyLimit,
	)

	r.NoRoute(httperr.NotFound)
	r.NoMethod(httperr.MethodNotAllowed)

	health.Register(r)

	return r, nil
}
