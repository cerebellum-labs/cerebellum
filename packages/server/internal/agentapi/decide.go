package agentapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"golang.org/x/sync/errgroup"

	"github.com/cerebellum-labs/cerebellum/packages/server/internal/db"
	"github.com/cerebellum-labs/cerebellum/packages/server/internal/httperr"
	"github.com/cerebellum-labs/cerebellum/packages/server/internal/model"
	"github.com/cerebellum-labs/cerebellum/packages/server/internal/view"
)

const (
	// How long one decision may take with the model, retry included. The
	// agent stops waiting after 10 s and asks again; the call goes on, and
	// the second request gets its answer.
	modelTimeout = 60 * time.Second
	// A reply that isn't one of the options is asked again once.
	modelAttempts = 2
)

// decide asks the cloud model for irreversible decisions, up to five at
// once, in parallel. Each comes back answered or with its error.
func (h *Handler) decide(c *gin.Context) {
	var req decideRequest
	if !httperr.BindJSON(c, &req) {
		return
	}
	if len(req.Items) == 0 || len(req.Items) > maxDecideItems {
		httperr.BadRequest(c, "send 1 to 5 items")
		return
	}
	ctx := c.Request.Context()
	device := deviceFrom(c)
	if err := h.q.TouchDevice(ctx, device); err != nil {
		h.internalError(c, "touch device", err)
		return
	}
	h.events.PublishDevice(ctx, device)

	decisions := make([]*view.Decision, len(req.Items))
	errs := make([]error, len(req.Items))
	var g errgroup.Group
	for i, item := range req.Items {
		g.Go(func() error {
			d, err := h.decideItem(ctx, device, item)
			decisions[i], errs[i] = d, err
			return nil
		})
	}
	_ = g.Wait() // The items report their own errors.

	resp := decideResponse{Decisions: []view.Decision{}, Errors: []failedItem{}}
	for i, item := range req.Items {
		if errs[i] != nil {
			resp.Errors = append(resp.Errors, h.failedItemOf(ctx, item.Decision.ID, errs[i]))
			continue
		}
		resp.Decisions = append(resp.Decisions, *decisions[i])
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) decideItem(ctx context.Context, device uuid.UUID, item itemIn) (*view.Decision, error) {
	if item.Decision.ActionClass != "irreversible" {
		return nil, reject("/decide takes irreversible decisions; send the others to /sync")
	}
	id := item.Decision.ID
	if err := h.save(ctx, device, item); err != nil {
		return nil, err
	}
	h.events.PublishDecision(ctx, id)

	ch := h.calls.DoChan(id.String(), func() (any, error) {
		// Detached from the request, so an agent that gives up doesn't
		// throw away an answer it will ask for again.
		callCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), modelTimeout)
		defer cancel()
		return h.answer(callCtx, id)
	})
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-ch:
		if res.Err != nil {
			return nil, res.Err
		}
		d, ok := res.Val.(view.Decision)
		if !ok {
			return nil, fmt.Errorf("answer returned %T", res.Val)
		}
		return &d, nil
	}
}

// answer has the model decide, unless it already has, and returns the
// decision in the server's version.
func (h *Handler) answer(ctx context.Context, id uuid.UUID) (view.Decision, error) {
	row, err := h.q.GetDecision(ctx, id)
	if err != nil {
		return view.Decision{}, fmt.Errorf("get decision: %w", err)
	}
	if row.Decision.ModelCallID != nil {
		return view.NewDecision(db.ListDecisionsRow(row)), nil
	}
	q, err := h.question(ctx, row.Decision)
	if err != nil {
		return view.Decision{}, err
	}

	var lastErr error
	for range modelAttempts {
		call, callErr := h.asker.Ask(ctx, q)
		if errors.Is(callErr, model.ErrNotConfigured) {
			return view.Decision{}, unavailable("the server has no cloud model configured")
		}
		callID, err := model.Record(ctx, h.q, id, model.PurposeDecide, call, callErr)
		if err != nil {
			return view.Decision{}, err
		}
		if callErr == nil {
			// Another instance may have answered first; then its answer stays.
			if _, err := h.q.SetServerDecision(ctx, db.SetServerDecisionParams{ID: id, Action: &call.Action, ModelCallID: &callID}); err != nil {
				return view.Decision{}, fmt.Errorf("set server decision: %w", err)
			}
			h.events.PublishDecision(ctx, id)
			return h.decisionOf(ctx, id)
		}
		lastErr = callErr
		if !errors.Is(callErr, model.ErrInvalidAnswer) {
			break
		}
	}
	h.logger.WarnContext(ctx, "cloud model gave no decision", "decision_id", id.String(), "error", lastErr)
	return view.Decision{}, unavailable("the cloud model gave no usable answer, try again")
}

// question is what the model is asked about the decision: its prompt,
// options, the robot's context and frame.
func (h *Handler) question(ctx context.Context, d db.Decision) (model.Question, error) {
	frame, err := h.q.GetFrame(ctx, d.ID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return model.Question{}, fmt.Errorf("get frame: %w", err)
	}
	q := model.Question{Options: d.Options, Metadata: d.Metadata, Frame: frame}
	if d.Prompt != nil {
		q.Prompt = *d.Prompt
	}
	return q, nil
}
