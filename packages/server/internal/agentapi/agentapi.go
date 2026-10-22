// Package agentapi serves the API the robots' agents call: /decide asks the
// cloud model for an irreversible decision, /sync hands over everything
// else and brings back corrections. Every request starts on the device;
// the server never calls it, so a robot behind NAT or a phone hotspot works.
package agentapi

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/singleflight"

	"github.com/cerebellum-labs/cerebellum/packages/server/internal/db"
	"github.com/cerebellum-labs/cerebellum/packages/server/internal/httperr"
	"github.com/cerebellum-labs/cerebellum/packages/server/internal/model"
	"github.com/cerebellum-labs/cerebellum/packages/server/internal/view"
)

// checkViolation is PostgreSQL's SQLSTATE for a failed CHECK constraint.
const checkViolation = "23514"

const deviceKey = "agentapi.device"

// Asker asks the cloud model.
type Asker interface {
	Ask(ctx context.Context, q model.Question) (model.Call, error)
}

// Events tells the dashboards what changed.
type Events interface {
	PublishDecision(ctx context.Context, id uuid.UUID)
	PublishDevice(ctx context.Context, id uuid.UUID)
}

// Reviewer reviews uncertain decisions; Wake has it look at once.
type Reviewer interface {
	Wake()
}

// Handler serves the agent API.
type Handler struct {
	pool     *pgxpool.Pool
	q        *db.Queries
	asker    Asker
	events   Events
	reviewer Reviewer
	logger   *slog.Logger
	// One model call per decision at a time: an agent that times out and
	// retries waits for the call already running instead of paying twice.
	calls singleflight.Group
	now   func() time.Time
}

// New returns the agent API.
func New(pool *pgxpool.Pool, asker Asker, events Events, reviewer Reviewer, logger *slog.Logger) *Handler {
	return &Handler{
		pool:     pool,
		q:        db.New(pool),
		asker:    asker,
		events:   events,
		reviewer: reviewer,
		logger:   logger,
		now:      time.Now,
	}
}

// Register mounts the API under /api/v1.
func (h *Handler) Register(r gin.IRouter) {
	g := r.Group("/api/v1", h.authenticate)
	g.POST("/decide", h.decide)
	g.POST("/sync", h.sync)
}

// authenticate finds the device by its token. The device a request acts
// for always comes from the token, never from the body: one device can't
// write into another's journal.
func (h *Handler) authenticate(c *gin.Context) {
	token, ok := strings.CutPrefix(c.GetHeader("Authorization"), "Bearer ")
	if !ok || token == "" {
		httperr.Unauthorized(c, "send the device token as Authorization: Bearer <token>")
		return
	}
	hash := sha256.Sum256([]byte(token))
	device, err := h.q.GetDeviceByTokenHash(c.Request.Context(), hash[:])
	if errors.Is(err, pgx.ErrNoRows) {
		httperr.Unauthorized(c, "the device token is wrong, was rotated, or its device was deleted")
		return
	}
	if err != nil {
		h.internalError(c, "authenticate device", err)
		return
	}
	c.Set(deviceKey, device.ID)
	c.Next()
}

func deviceFrom(c *gin.Context) uuid.UUID {
	id, _ := c.MustGet(deviceKey).(uuid.UUID)
	return id
}

// itemError is an item's failure with a reason the agent may read, and
// whether sending the item again may work.
type itemError struct {
	reason string
	retry  bool
}

func (e itemError) Error() string { return e.reason }

// reject fails an item the server will never accept as it is: a broken rule.
func reject(format string, args ...any) error {
	return itemError{reason: fmt.Sprintf(format, args...)}
}

// unavailable fails an item for now: the cloud model gave no answer.
func unavailable(format string, args ...any) error {
	return itemError{reason: fmt.Sprintf(format, args...), retry: true}
}

// save writes one decision of the device, with its frame, in a single
// transaction: the frame is kept only when the decision is the device's.
func (h *Handler) save(ctx context.Context, device uuid.UUID, item itemIn) error {
	d := item.Decision
	if err := d.validate(); err != nil {
		return reject("%s", err)
	}
	frame, err := decodeFrame(item.Frame)
	if err != nil {
		return reject("%s", err)
	}

	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }() // Does nothing after Commit.
	q := h.q.WithTx(tx)

	_, err = q.UpsertDecision(ctx, db.UpsertDecisionParams{
		ID:                  d.ID,
		DeviceID:            device,
		Revision:            d.Revision,
		Task:                d.Task,
		Options:             d.Options,
		ActionClass:         d.ActionClass,
		ProposedAction:      d.ProposedAction,
		Prompt:              d.Prompt,
		Metadata:            orEmptyObject(d.Metadata),
		LocalModel:          d.LocalModel,
		LocalConfidence:     d.LocalConfidence,
		LocalLatencyMs:      d.LocalLatencyMs,
		NeedsReview:         d.NeedsReview,
		DecidedAt:           d.DecidedAt,
		Action:              d.Action,
		ExecutedAt:          d.ExecutedAt,
		CancelledAt:         d.CancelledAt,
		CancelReason:        d.CancelReason,
		CorrectionAppliedAt: d.CorrectionAppliedAt,
	})
	switch pgErr, isPg := errors.AsType[*pgconn.PgError](err); {
	case isPg && pgErr.Code == checkViolation:
		return reject("the decision breaks the rule %s", pgErr.ConstraintName)
	case errors.Is(err, pgx.ErrNoRows):
		// Nothing written: an older revision, or another device's id.
		owner, err := q.DecisionOwner(ctx, d.ID)
		if err != nil {
			return fmt.Errorf("decision owner: %w", err)
		}
		if owner != device {
			return reject("the id belongs to another device's decision")
		}
	case err != nil:
		return fmt.Errorf("upsert decision: %w", err)
	}

	if frame != nil {
		if err := q.InsertFrame(ctx, db.InsertFrameParams{DecisionID: d.ID, Jpeg: frame}); err != nil {
			return fmt.Errorf("insert frame: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// decisionOf reads the decision back in the server's version.
func (h *Handler) decisionOf(ctx context.Context, id uuid.UUID) (view.Decision, error) {
	row, err := h.q.GetDecision(ctx, id)
	if err != nil {
		return view.Decision{}, fmt.Errorf("get decision: %w", err)
	}
	return view.NewDecision(db.ListDecisionsRow(row)), nil
}

// failedItemOf turns an item's failure into what the agent sees: the reason
// of an itemError, a generic message (and a log record) for the rest, which
// may work on a second try.
func (h *Handler) failedItemOf(ctx context.Context, id uuid.UUID, err error) failedItem {
	if e, ok := errors.AsType[itemError](err); ok {
		return failedItem{DecisionID: id, Error: e.reason, Retry: e.retry}
	}
	h.logger.LogAttrs(ctx, slog.LevelError, "agent item failed", slog.String("decision_id", id.String()), slog.Any("error", err))
	return failedItem{DecisionID: id, Error: "internal error, try again", Retry: true}
}

func (h *Handler) internalError(c *gin.Context, msg string, err error) {
	h.logger.LogAttrs(c.Request.Context(), slog.LevelError, msg, slog.Any("error", err))
	httperr.InternalError(c)
}
