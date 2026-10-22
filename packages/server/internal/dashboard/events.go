package dashboard

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/cerebellum-labs/cerebellum/packages/server/internal/db"
	"github.com/cerebellum-labs/cerebellum/packages/server/internal/view"
)

type decisionEvent struct {
	Type     string        `json:"type"`
	Decision view.Decision `json:"decision"`
}

type correctionEvent struct {
	Type       string          `json:"type"`
	Correction view.Correction `json:"correction"`
}

type deviceEvent struct {
	Type   string      `json:"type"`
	Device view.Device `json:"device"`
}

// PublishDecision tells the connected dashboards that the decision changed.
// Like the other Publish methods, it reads the current row, does nothing
// while no dashboard listens, and only logs what goes wrong: a missed event
// must not fail the write it reports.
func (h *Handler) PublishDecision(ctx context.Context, id uuid.UUID) {
	if !h.hub.Listening() {
		return
	}
	row, err := h.q.GetDecision(ctx, id)
	if err != nil {
		h.logEventError(ctx, "decision", err)
		return
	}
	h.publish(ctx, decisionEvent{Type: "decision.upserted", Decision: view.NewDecision(db.ListDecisionsRow(row))})
}

// PublishCorrection tells the connected dashboards that the review
// corrected the decision.
func (h *Handler) PublishCorrection(ctx context.Context, decisionID uuid.UUID) {
	if !h.hub.Listening() {
		return
	}
	row, err := h.q.GetDecision(ctx, decisionID)
	if err != nil {
		h.logEventError(ctx, "correction", err)
		return
	}
	for _, c := range view.NewCorrections(db.ListDecisionsRow(row)) {
		h.publish(ctx, correctionEvent{Type: "correction.created", Correction: c})
	}
}

// PublishDevice tells the connected dashboards that the device changed,
// like being heard from.
func (h *Handler) PublishDevice(ctx context.Context, id uuid.UUID) {
	if !h.hub.Listening() {
		return
	}
	device, err := h.q.GetDevice(ctx, id)
	if err != nil {
		h.logEventError(ctx, "device", err)
		return
	}
	h.publishDevice(ctx, view.NewDevice(device, time.Now()))
}

// publishDevice sends a device the handler already has.
func (h *Handler) publishDevice(ctx context.Context, d view.Device) {
	h.publish(ctx, deviceEvent{Type: "device.updated", Device: d})
}

func (h *Handler) publish(ctx context.Context, event any) {
	msg, err := json.Marshal(event)
	if err != nil {
		// The views are plain structs; this would be a bug, not bad input.
		h.logger.LogAttrs(ctx, slog.LevelError, "marshal dashboard event", slog.Any("error", err))
		return
	}
	h.hub.Publish(msg)
}

func (h *Handler) logEventError(ctx context.Context, kind string, err error) {
	h.logger.LogAttrs(ctx, slog.LevelWarn, "dashboard event not sent", slog.String("kind", kind), slog.Any("error", err))
}
