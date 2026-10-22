// Package logging builds the service's JSON logger and ties log records to
// the request they belong to: a record logged with a request's context
// carries its request_id, wherever in the code it is written.
package logging

import (
	"context"
	"io"
	"log/slog"
)

type requestIDKey struct{}

// New returns a JSON logger writing to w that adds the request id found in
// the context of each record.
func New(w io.Writer) *slog.Logger {
	return slog.New(NewHandler(slog.NewJSONHandler(w, nil)))
}

// NewHandler wraps next so that records logged with a context from
// WithRequestID carry a request_id attribute.
func NewHandler(next slog.Handler) slog.Handler {
	return handler{next}
}

// WithRequestID returns a copy of ctx that carries the request id.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}

// RequestID returns the request id in ctx, or "" when there is none.
func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}

type handler struct {
	slog.Handler
}

func (h handler) Handle(ctx context.Context, r slog.Record) error {
	if id := RequestID(ctx); id != "" {
		r.AddAttrs(slog.String("request_id", id))
	}
	return h.Handler.Handle(ctx, r)
}

func (h handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return handler{h.Handler.WithAttrs(attrs)}
}

func (h handler) WithGroup(name string) slog.Handler {
	return handler{h.Handler.WithGroup(name)}
}
