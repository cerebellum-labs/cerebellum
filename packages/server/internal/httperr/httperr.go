// Package httperr defines the single JSON error shape every endpoint returns,
// so clients can parse failures without special-casing plain-text bodies.
package httperr

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Error codes are part of the API contract: never rename one.
const (
	CodeInternalError    = "internal_error"
	CodeNotFound         = "not_found"
	CodeMethodNotAllowed = "method_not_allowed"
	CodePayloadTooLarge  = "payload_too_large"
	CodeBadRequest       = "bad_request"
	CodeUnauthorized     = "unauthorized"
	CodeForbidden        = "forbidden"
	CodeConflict         = "conflict"
	CodeTooManyRequests  = "too_many_requests"
)

// Response is the error envelope: {"error":{"code":"...","message":"..."}}.
// Code is a stable machine-readable identifier. Message is for logs and
// debugging.
type Response struct {
	Error Body `json:"error"`
}

// Body is the error object inside Response.
type Body struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Abort stops the handler chain and writes the error response.
func Abort(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, Response{Error: Body{Code: code, Message: message}})
}

// NotFound handles requests that match no route.
func NotFound(c *gin.Context) {
	Abort(c, http.StatusNotFound, CodeNotFound, "route not found")
}

// MethodNotAllowed handles requests to a known path with the wrong method.
// gin sets the Allow header before calling it.
func MethodNotAllowed(c *gin.Context) {
	Abort(c, http.StatusMethodNotAllowed, CodeMethodNotAllowed, "method not allowed")
}

// PayloadTooLarge answers 413 for a body over the server's limit. Handlers
// call it when reading the body fails with *http.MaxBytesError.
func PayloadTooLarge(c *gin.Context) {
	Abort(c, http.StatusRequestEntityTooLarge, CodePayloadTooLarge, "request body too large")
}

// InternalError answers 500 with a generic message so internals never leak.
func InternalError(c *gin.Context) {
	Abort(c, http.StatusInternalServerError, CodeInternalError, "internal server error")
}

// BadRequest answers 400 for a request the server can't accept; message
// tells the client what to fix.
func BadRequest(c *gin.Context, message string) {
	Abort(c, http.StatusBadRequest, CodeBadRequest, message)
}

// Unauthorized answers 401 for a request that needs a session it doesn't
// have.
func Unauthorized(c *gin.Context, message string) {
	Abort(c, http.StatusUnauthorized, CodeUnauthorized, message)
}

// Forbidden answers 403 for a request the server refuses whoever sends it.
func Forbidden(c *gin.Context, message string) {
	Abort(c, http.StatusForbidden, CodeForbidden, message)
}

// Missing answers 404 for a resource that doesn't exist.
func Missing(c *gin.Context, message string) {
	Abort(c, http.StatusNotFound, CodeNotFound, message)
}

// Conflict answers 409 for a request that clashes with what already exists.
func Conflict(c *gin.Context, message string) {
	Abort(c, http.StatusConflict, CodeConflict, message)
}

// TooManyRequests answers 429 when a client has to slow down.
func TooManyRequests(c *gin.Context, message string) {
	Abort(c, http.StatusTooManyRequests, CodeTooManyRequests, message)
}
