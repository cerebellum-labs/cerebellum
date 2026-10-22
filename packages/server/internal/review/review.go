// Package review checks the decisions a robot made unsure: it asks the
// cloud model the same question and, when the answer differs, records a
// correction the robot picks up on its next sync.
package review

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/cerebellum-labs/cerebellum/packages/server/internal/db"
	"github.com/cerebellum-labs/cerebellum/packages/server/internal/model"
)

const (
	// batchSize is how many decisions one pass takes, oldest first.
	batchSize = 20
	// maxFailures is how many failed calls a review gets before it gives
	// up; the failures stay in model_calls.
	maxFailures = 3
	// interval is how often the worker looks without being woken: a
	// safety net for a wake-up lost while it was busy, and for retries.
	interval = 30 * time.Second
	// callTimeout bounds one call to the model.
	callTimeout = 60 * time.Second
)

// Asker asks the cloud model.
type Asker interface {
	Ask(ctx context.Context, q model.Question) (model.Call, error)
}

// Events tells the dashboards what changed.
type Events interface {
	PublishDecision(ctx context.Context, id uuid.UUID)
	PublishCorrection(ctx context.Context, decisionID uuid.UUID)
}

// Worker reviews decisions in the background, one at a time.
type Worker struct {
	q      *db.Queries
	asker  Asker
	events Events
	logger *slog.Logger
	wake   chan struct{}
}

// New returns a worker; Run starts it.
func New(q *db.Queries, asker Asker, events Events, logger *slog.Logger) *Worker {
	return &Worker{
		q:      q,
		asker:  asker,
		events: events,
		logger: logger,
		// One pending wake-up is enough: a pass takes every waiting decision.
		wake: make(chan struct{}, 1),
	}
}

// Wake has the worker look for decisions to review now. It never blocks.
func (w *Worker) Wake() {
	select {
	case w.wake <- struct{}{}:
	default:
	}
}

// Run reviews until ctx is done.
func (w *Worker) Run(ctx context.Context) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		w.drain(ctx)
		select {
		case <-ctx.Done():
			return nil
		case <-w.wake:
		case <-ticker.C:
		}
	}
}

// drain runs passes while they come back full and nothing fails: a failing
// decision waits for the next wake-up or tick instead of being retried at
// once in a loop.
func (w *Worker) drain(ctx context.Context) {
	for ctx.Err() == nil {
		reviewed, failed, err := w.pass(ctx)
		if err != nil {
			// Stopping mid-pass is not a failure worth a log record.
			if ctx.Err() == nil {
				w.logger.ErrorContext(ctx, "review pass failed", slog.Any("error", err))
			}
			return
		}
		if reviewed < batchSize || failed > 0 {
			return
		}
	}
}

// pass reviews one batch and returns how many it took and how many failed.
func (w *Worker) pass(ctx context.Context) (reviewed, failed int, err error) {
	queue, err := w.q.ListReviewQueue(ctx, batchSize)
	if err != nil {
		return 0, 0, fmt.Errorf("list review queue: %w", err)
	}
	for _, d := range queue {
		if err := ctx.Err(); err != nil {
			return reviewed, failed, err
		}
		if !w.reviewLogged(ctx, d) {
			failed++
		}
		reviewed++
	}
	return reviewed, failed, nil
}

// reviewLogged reviews the decision and reports whether it worked. A failed
// review is logged and left for a later pass, which counts its failures.
func (w *Worker) reviewLogged(ctx context.Context, d db.ListReviewQueueRow) bool {
	err := w.review(ctx, d)
	if err != nil {
		w.logger.WarnContext(ctx, "review failed", slog.String("decision_id", d.ID.String()), slog.Any("error", err))
	}
	return err == nil
}

func (w *Worker) review(ctx context.Context, d db.ListReviewQueueRow) error {
	if d.FailedReviews >= maxFailures {
		return w.finish(ctx, d.ID, nil, nil)
	}
	frame, err := w.q.GetFrame(ctx, d.ID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("get frame: %w", err)
	}
	q := model.Question{Options: d.Options, Metadata: d.Metadata, Frame: frame}
	if d.Prompt != nil {
		q.Prompt = *d.Prompt
	}

	callCtx, cancel := context.WithTimeout(ctx, callTimeout)
	call, callErr := w.asker.Ask(callCtx, q)
	cancel()
	if errors.Is(callErr, model.ErrNotConfigured) {
		// Not the decision's fault: don't count it against the review.
		return callErr
	}
	callID, err := model.Record(ctx, w.q, d.ID, model.PurposeReview, call, callErr)
	if err != nil {
		return err
	}
	if callErr != nil {
		return callErr
	}

	var corrected *string
	if d.Action == nil || call.Action != *d.Action {
		corrected = &call.Action
	}
	if err := w.finish(ctx, d.ID, &callID, corrected); err != nil {
		return err
	}
	if corrected != nil {
		w.events.PublishCorrection(ctx, d.ID)
	}
	return nil
}

// finish closes the review and tells the dashboards.
func (w *Worker) finish(ctx context.Context, id uuid.UUID, callID *uuid.UUID, corrected *string) error {
	_, err := w.q.FinishReview(ctx, db.FinishReviewParams{ID: id, ReviewCallID: callID, CorrectedAction: corrected})
	if err != nil {
		return fmt.Errorf("finish review: %w", err)
	}
	w.events.PublishDecision(ctx, id)
	return nil
}
