//go:build integration

package review

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cerebellum-labs/cerebellum/packages/server/internal/db"
	"github.com/cerebellum-labs/cerebellum/packages/server/internal/db/dbtest"
	"github.com/cerebellum-labs/cerebellum/packages/server/internal/model"
)

// fakeAsker answers every question with action, or fails when it is "".
type fakeAsker struct {
	action string
	mu     sync.Mutex
	asked  int
}

func (f *fakeAsker) Ask(context.Context, model.Question) (model.Call, error) {
	f.mu.Lock()
	f.asked++
	f.mu.Unlock()
	call := model.Call{Model: "fake", Latency: time.Second, CostUSD: 0.0003}
	if f.action == "" {
		return call, fmt.Errorf("%w: no JSON", model.ErrInvalidAnswer)
	}
	call.Action, call.Reason = f.action, "Not a fruit."
	return call, nil
}

func (f *fakeAsker) calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.asked
}

type fakeEvents struct{ decisions, corrections atomic.Int32 }

func (f *fakeEvents) PublishDecision(context.Context, uuid.UUID)   { f.decisions.Add(1) }
func (f *fakeEvents) PublishCorrection(context.Context, uuid.UUID) { f.corrections.Add(1) }

// uncertain adds a device with a reversible decision that needs review: the
// robot put the item in the basket.
func uncertain(t *testing.T) (*pgxpool.Pool, uuid.UUID) {
	t.Helper()
	pool := dbtest.NewPool(t, dbtest.NewDatabase(t))
	if err := db.Migrate(t.Context(), pool, slog.New(slog.DiscardHandler)); err != nil {
		t.Fatal(err)
	}
	var device uuid.UUID
	if err := pool.QueryRow(t.Context(), `INSERT INTO devices (key, name, token_hash) VALUES ('d', 'd', sha256('t')) RETURNING id`).
		Scan(&device); err != nil {
		t.Fatal(err)
	}
	id := uuid.Must(uuid.NewV7())
	if _, err := pool.Exec(t.Context(),
		`INSERT INTO decisions (id, device_id, revision, task, options, action_class, action, prompt, needs_review, local_confidence, decided_at)
		 VALUES ($1, $2, 1, 'sort_item', '{basket,discard}', 'reversible', 'basket', 'Is it a fruit?', true, 0.6, now())`,
		id, device); err != nil {
		t.Fatal(err)
	}
	return pool, id
}

type reviewed struct {
	done      bool
	corrected *string
	byCall    bool
}

func state(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) reviewed {
	t.Helper()
	var r reviewed
	if err := pool.QueryRow(t.Context(),
		`SELECT reviewed_at IS NOT NULL, corrected_action, review_call_id IS NOT NULL FROM decisions WHERE id = $1`, id).
		Scan(&r.done, &r.corrected, &r.byCall); err != nil {
		t.Fatal(err)
	}
	return r
}

func TestReview(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		answer        string
		wantCorrected bool
	}{
		{"the model disagrees", "discard", true},
		{"the model agrees", "basket", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			pool, id := uncertain(t)
			events := &fakeEvents{}
			w := New(db.New(pool), &fakeAsker{action: tt.answer}, events, slog.New(slog.DiscardHandler))

			if _, failed, err := w.pass(t.Context()); err != nil || failed != 0 {
				t.Fatalf("pass: %d failed, %v", failed, err)
			}
			got := state(t, pool, id)
			if !got.done || !got.byCall || (got.corrected != nil) != tt.wantCorrected {
				t.Errorf("after review: %+v", got)
			}
			if tt.wantCorrected && (*got.corrected != "discard" || events.corrections.Load() != 1) {
				t.Errorf("corrected to %v with %d correction events", *got.corrected, events.corrections.Load())
			}
		})
	}
}

func TestReviewGivesUp(t *testing.T) {
	t.Parallel()
	pool, id := uncertain(t)
	asker := &fakeAsker{}
	w := New(db.New(pool), asker, &fakeEvents{}, slog.New(slog.DiscardHandler))

	for i := range maxFailures {
		if _, failed, err := w.pass(t.Context()); err != nil || failed != 1 {
			t.Fatalf("pass %d: %d failed, %v", i, failed, err)
		}
	}
	if state(t, pool, id).done {
		t.Fatal("gave up before running out of attempts")
	}
	// The next pass gives up without asking again.
	if _, _, err := w.pass(t.Context()); err != nil {
		t.Fatal(err)
	}
	if got := state(t, pool, id); !got.done || got.byCall || got.corrected != nil || asker.calls() != maxFailures {
		t.Errorf("after giving up: %+v, %d calls", got, asker.calls())
	}
}

func TestReviewNotConfigured(t *testing.T) {
	t.Parallel()
	pool, id := uncertain(t)
	w := New(db.New(pool), model.New(model.Config{Model: "m"}, nil), &fakeEvents{}, slog.New(slog.DiscardHandler))

	_, failed, err := w.pass(t.Context())
	if err != nil || failed != 1 {
		t.Fatalf("pass: %d failed, %v", failed, err)
	}
	var calls int
	if err := pool.QueryRow(t.Context(), `SELECT count(*) FROM model_calls WHERE decision_id = $1`, id).Scan(&calls); err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Errorf("a call that never happened was recorded %d times", calls)
	}
}

func TestRun(t *testing.T) {
	t.Parallel()
	pool, id := uncertain(t)
	w := New(db.New(pool), &fakeAsker{action: "discard"}, &fakeEvents{}, slog.New(slog.DiscardHandler))

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- w.Run(ctx) }()
	w.Wake()

	deadline := time.Now().Add(5 * time.Second)
	for !state(t, pool, id).done {
		if time.Now().After(deadline) {
			t.Fatal("the worker did not review the decision")
		}
		time.Sleep(20 * time.Millisecond)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Errorf("Run returned %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after cancel")
	}
}
