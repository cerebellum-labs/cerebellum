// Package health exposes the service liveness endpoint.
package health

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Path is the route the liveness probe is served on.
const Path = "/health"

type response struct {
	Status string `json:"status"`
}

// Register mounts the liveness probe on r for both GET and HEAD, so load
// balancers that probe with HEAD get the same answer without a body.
func Register(r gin.IRoutes) {
	r.GET(Path, handle)
	r.HEAD(Path, handle)
}

// handle reports that the process is up and serving HTTP. It deliberately
// checks no dependencies: a liveness probe that fails when the database is
// down makes the orchestrator restart healthy instances and turns an outage
// into a restart loop.
func handle(c *gin.Context) {
	// Probes must always see the live state, never a cached answer.
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, response{Status: "ok"})
}
