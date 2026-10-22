//go:build integration

package db

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Statements below take @device and @decision: an existing device and a new
// decision id for each case.
const (
	insertReversible = `INSERT INTO decisions (id, device_id, revision, task, options, action_class, action, decided_at)
		VALUES (@decision, @device, 1, 'sort_item', '{basket,discard}', 'reversible', 'basket', now())`
	insertWaiting = `INSERT INTO decisions (id, device_id, revision, task, options, action_class, prompt, decided_at)
		VALUES (@decision, @device, 1, 'sort_item', '{basket,discard}', 'irreversible', 'Is it a fruit?', now())`
	insertUncertain = `INSERT INTO decisions (id, device_id, revision, task, options, action_class, action, prompt, needs_review, local_confidence, decided_at)
		VALUES (@decision, @device, 1, 'sort_item', '{basket,discard}', 'reversible', 'basket', 'Is it a fruit?', true, 0.6, now())`
	insertDecideCall = `INSERT INTO model_calls (id, decision_id, purpose, model, action, latency_ms)
		VALUES (@call, @decision, 'decide', 'nemotron', 'discard', 1800)`
	insertReviewCall = `INSERT INTO model_calls (id, decision_id, purpose, model, action, latency_ms)
		VALUES (@call, @decision, 'review', 'nemotron', 'discard', 1800)`
	decideOnServer = `UPDATE decisions SET action = 'discard', model_call_id = @call WHERE id = @decision`
	correct        = `UPDATE decisions SET reviewed_at = now(), review_call_id = @call, corrected_action = 'discard' WHERE id = @decision`
)

const (
	checkViolation      = "23514"
	foreignKeyViolation = "23503"
	generatedColumn     = "428C9"
)

// caseArgs gives a case the device and ids of its own.
func caseArgs(device uuid.UUID) pgx.NamedArgs {
	return pgx.NamedArgs{
		"device":   device,
		"decision": uuid.Must(uuid.NewV7()),
		"call":     uuid.Must(uuid.NewV7()),
	}
}

// execAll runs the statements in order and stops at the first error.
func execAll(t *testing.T, pool *pgxpool.Pool, args pgx.NamedArgs, stmts []string) error {
	t.Helper()
	for _, stmt := range stmts {
		if _, err := pool.Exec(t.Context(), stmt, args); err != nil {
			return err
		}
	}
	return nil
}

func TestDecisionConstraints(t *testing.T) {
	t.Parallel()
	pool := migratedPool(t)
	device, _ := insertDevice(t, pool)

	tests := []struct {
		name     string
		stmts    []string
		wantCode string // empty when the statements must succeed
	}{
		{"reversible decided on the device", []string{insertReversible}, ""},
		{"irreversible waiting for the server", []string{insertWaiting}, ""},
		{"irreversible decided by the server", []string{insertWaiting, insertDecideCall, decideOnServer}, ""},
		{"reflex reported done", []string{`INSERT INTO decisions (id, device_id, revision, task, options, action_class, action, executed_at, decided_at)
			VALUES (@decision, @device, 1, 'safety', '{stop}', 'reflex', 'stop', now(), now())`}, ""},
		{"id is not a UUIDv7", []string{`INSERT INTO decisions (id, device_id, revision, task, options, action_class, action, decided_at)
			VALUES (gen_random_uuid(), @device, 1, 'sort_item', '{basket}', 'reversible', 'basket', now())`}, checkViolation},
		{"action outside the options", []string{insertReversible, `UPDATE decisions SET action = 'eat' WHERE id = @decision`}, checkViolation},
		{"options with a NULL", []string{`INSERT INTO decisions (id, device_id, revision, task, options, action_class, action, decided_at)
			VALUES (@decision, @device, 1, 'sort_item', ARRAY['basket', NULL], 'reversible', 'basket', now())`}, checkViolation},
		{"reversible without an action", []string{`INSERT INTO decisions (id, device_id, revision, task, options, action_class, decided_at)
			VALUES (@decision, @device, 1, 'sort_item', '{basket}', 'reversible', now())`}, checkViolation},
		{"reflex not executed", []string{`INSERT INTO decisions (id, device_id, revision, task, options, action_class, action, decided_at)
			VALUES (@decision, @device, 1, 'safety', '{stop}', 'reflex', 'stop', now())`}, checkViolation},
		{"irreversible without a prompt", []string{`INSERT INTO decisions (id, device_id, revision, task, options, action_class, decided_at)
			VALUES (@decision, @device, 1, 'sort_item', '{basket}', 'irreversible', now())`}, checkViolation},
		{"server decision without a model call", []string{insertWaiting, `UPDATE decisions SET action = 'discard' WHERE id = @decision`}, checkViolation},
		{"reversible cancelled", []string{insertReversible, `UPDATE decisions SET cancelled_at = now(), cancel_reason = 'gone' WHERE id = @decision`}, checkViolation},
		{"review without needs_review", []string{insertReversible, `UPDATE decisions SET reviewed_at = now() WHERE id = @decision`}, checkViolation},
		{"model call of another decision", []string{insertWaiting, `UPDATE decisions SET action = 'discard', model_call_id = uuidv7() WHERE id = @decision`}, foreignKeyViolation},
		{"reviewed and corrected", []string{insertUncertain, insertReviewCall, correct}, ""},
		{"review gave up", []string{insertUncertain, `UPDATE decisions SET reviewed_at = now() WHERE id = @decision`}, ""},
		{"correction without a review call", []string{
			insertUncertain, `UPDATE decisions SET reviewed_at = now(), corrected_action = 'discard' WHERE id = @decision`,
		}, checkViolation},
		{"correction that agrees with the device", []string{
			insertUncertain, insertReviewCall,
			`UPDATE decisions SET reviewed_at = now(), review_call_id = @call, corrected_action = 'basket' WHERE id = @decision`,
		}, checkViolation},
		{"correction outside the options", []string{
			insertUncertain, insertReviewCall,
			`UPDATE decisions SET reviewed_at = now(), review_call_id = @call, corrected_action = 'eat' WHERE id = @decision`,
		}, checkViolation},
		{"correction applied without a correction", []string{
			insertUncertain, `UPDATE decisions SET correction_applied_at = now() WHERE id = @decision`,
		}, checkViolation},
		{"review call of another decision", []string{
			insertUncertain, `UPDATE decisions SET reviewed_at = now(), review_call_id = uuidv7() WHERE id = @decision`,
		}, foreignKeyViolation},
		{"status written", []string{insertReversible, `UPDATE decisions SET status = 'executed' WHERE id = @decision`}, generatedColumn},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := execAll(t, pool, caseArgs(device), tt.stmts)
			if tt.wantCode == "" {
				if err != nil {
					t.Fatalf("got %v, want success", err)
				}
				return
			}
			pgErr, ok := errors.AsType[*pgconn.PgError](err)
			if !ok || pgErr.Code != tt.wantCode {
				t.Fatalf("got %v, want SQLSTATE %s", err, tt.wantCode)
			}
		})
	}
}

func TestDecisionStatus(t *testing.T) {
	t.Parallel()
	pool := migratedPool(t)
	device, _ := insertDevice(t, pool)

	tests := []struct {
		name       string
		stmts      []string
		wantStatus string
		wantSource string // empty for NULL
	}{
		{"decided on the device", []string{insertReversible}, "decided_on_device", "device"},
		{"executed", []string{insertReversible, `UPDATE decisions SET executed_at = now() WHERE id = @decision`}, "executed", "device"},
		{"waiting for the server", []string{insertWaiting}, "waiting_for_server", ""},
		{"decided on the server", []string{insertWaiting, insertDecideCall, decideOnServer}, "decided_on_server", "server"},
		{"cancelled", []string{insertWaiting, `UPDATE decisions SET cancelled_at = now(), cancel_reason = 'gone' WHERE id = @decision`}, "cancelled", ""},
		{"answered after cancel", []string{
			insertWaiting, `UPDATE decisions SET cancelled_at = now(), cancel_reason = 'gone' WHERE id = @decision`,
			insertDecideCall, decideOnServer,
		}, "cancelled", "server"},
		{"corrected", []string{insertUncertain, insertReviewCall, correct}, "corrected", "device"},
		{"correction applied", []string{
			insertUncertain, insertReviewCall, correct,
			`UPDATE decisions SET executed_at = now(), correction_applied_at = now() WHERE id = @decision`,
		}, "corrected", "device"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			args := caseArgs(device)
			if err := execAll(t, pool, args, tt.stmts); err != nil {
				t.Fatal(err)
			}
			var status string
			var source *string
			if err := pool.QueryRow(t.Context(), `SELECT status, source FROM decisions WHERE id = @decision`, args).
				Scan(&status, &source); err != nil {
				t.Fatal(err)
			}
			gotSource := ""
			if source != nil {
				gotSource = *source
			}
			if status != tt.wantStatus || gotSource != tt.wantSource {
				t.Errorf("got %s from %q, want %s from %q", status, gotSource, tt.wantStatus, tt.wantSource)
			}
		})
	}
}
