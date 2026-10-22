package agentapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/cerebellum-labs/cerebellum/packages/server/internal/db"
	"github.com/cerebellum-labs/cerebellum/packages/server/internal/httperr"
	"github.com/cerebellum-labs/cerebellum/packages/server/internal/view"
)

// sync takes up to ten of the device's decisions, new or changed, and
// returns them in the server's version with every correction the device
// hasn't carried out yet. It is also the device's heartbeat.
func (h *Handler) sync(c *gin.Context) {
	var req syncRequest
	if !httperr.BindJSON(c, &req) {
		return
	}
	if err := req.validate(); err != nil {
		httperr.BadRequest(c, err.Error())
		return
	}
	ctx := c.Request.Context()
	device := deviceFrom(c)
	if err := h.updateDevice(ctx, device, req); err != nil {
		h.internalError(c, "update device", err)
		return
	}

	resp := syncResponse{Decisions: []view.Decision{}, Errors: []failedItem{}}
	review := false
	for _, item := range req.Items {
		d, err := h.syncItem(ctx, device, item)
		if err != nil {
			resp.Errors = append(resp.Errors, h.failedItemOf(ctx, item.Decision.ID, err))
			continue
		}
		resp.Decisions = append(resp.Decisions, d)
		review = review || (d.NeedsReview && d.ReviewedAt == nil)
	}
	if review {
		h.reviewer.Wake()
	}

	corrections, err := h.pendingCorrections(ctx, device)
	if err != nil {
		h.internalError(c, "list pending corrections", err)
		return
	}
	resp.Corrections = corrections
	resp.ServerTime = view.Time(h.now())
	c.JSON(http.StatusOK, resp)
}

func (r *syncRequest) validate() error {
	switch {
	case len(r.Items) > maxSyncItems:
		return errors.New("send at most 10 items")
	case len(r.AgentVersion) > maxVersionLen:
		return errors.New("agent_version must be at most 64 characters")
	}
	return validObject(r.DeviceMetadata, "device_metadata")
}

// updateDevice records that the device was heard from and what it says
// about itself.
func (h *Handler) updateDevice(ctx context.Context, device uuid.UUID, req syncRequest) error {
	var version *string
	if req.AgentVersion != "" {
		version = &req.AgentVersion
	}
	err := h.q.UpdateDeviceInfo(ctx, db.UpdateDeviceInfoParams{
		ID:           device,
		AgentVersion: version,
		Metadata:     orEmptyObject(req.DeviceMetadata),
	})
	if err != nil {
		return err
	}
	h.events.PublishDevice(ctx, device)
	return nil
}

// syncItem saves one decision and returns it in the server's version.
func (h *Handler) syncItem(ctx context.Context, device uuid.UUID, item itemIn) (view.Decision, error) {
	if err := h.save(ctx, device, item); err != nil {
		return view.Decision{}, err
	}
	d, err := h.decisionOf(ctx, item.Decision.ID)
	if err != nil {
		return view.Decision{}, err
	}
	h.events.PublishDecision(ctx, d.ID)
	return d, nil
}

// pendingCorrections returns the device's corrections not carried out yet.
func (h *Handler) pendingCorrections(ctx context.Context, device uuid.UUID) ([]view.Correction, error) {
	rows, err := h.q.ListPendingCorrections(ctx, device)
	if err != nil {
		return nil, fmt.Errorf("list pending corrections: %w", err)
	}
	corrections := make([]view.Correction, 0, len(rows))
	for _, r := range rows {
		// The query's filter rules these out; so does the schema.
		if r.CorrectedAction == nil || r.ReviewedAt == nil {
			continue
		}
		corrections = append(corrections,
			view.NewCorrection(r.ID, *r.CorrectedAction, r.ReviewReason, r.ReviewModel, *r.ReviewedAt, r.CorrectionAppliedAt))
	}
	return corrections, nil
}
