//go:build integration

package agentapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cerebellum-labs/cerebellum/packages/server/internal/db"
	"github.com/cerebellum-labs/cerebellum/packages/server/internal/db/dbtest"
	"github.com/cerebellum-labs/cerebellum/packages/server/internal/model"
	"github.com/cerebellum-labs/cerebellum/packages/server/internal/server"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	m.Run()
}

// fakeAsker answers with the replies in turn, the last one from then on.
// Each reply is an action, or "" for a call that fails.
type fakeAsker struct {
	mu      sync.Mutex
	replies []string
	delay   time.Duration
	asked   []model.Question
}

func (f *fakeAsker) Ask(ctx context.Context, q model.Question) (model.Call, error) {
	f.mu.Lock()
	f.asked = append(f.asked, q)
	reply := f.replies[min(len(f.asked), len(f.replies))-1]
	f.mu.Unlock()

	select {
	case <-time.After(f.delay):
	case <-ctx.Done():
		return model.Call{Model: "fake"}, ctx.Err()
	}
	in, out := int32(1000), int32(40)
	call := model.Call{Model: "fake", Latency: 1500 * time.Millisecond, InputTokens: &in, OutputTokens: &out, CostUSD: 0.0003}
	if reply == "" {
		call.Output = "I am not sure."
		return call, fmt.Errorf("%w: no JSON", model.ErrInvalidAnswer)
	}
	call.Action, call.Reason, call.Output = reply, "Because.", `{"action":"`+reply+`"}`
	return call, nil
}

func (f *fakeAsker) calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.asked)
}

type fakeEvents struct{ decisions, devices atomic.Int32 }

func (f *fakeEvents) PublishDecision(context.Context, uuid.UUID) { f.decisions.Add(1) }
func (f *fakeEvents) PublishDevice(context.Context, uuid.UUID)   { f.devices.Add(1) }

type fakeReviewer struct{ wakes atomic.Int32 }

func (f *fakeReviewer) Wake() { f.wakes.Add(1) }

type testAPI struct {
	t        *testing.T
	pool     *pgxpool.Pool
	srv      *httptest.Server
	asker    *fakeAsker
	events   *fakeEvents
	reviewer *fakeReviewer
	device   uuid.UUID
	token    string
}

func newTestAPI(t *testing.T, replies ...string) *testAPI {
	t.Helper()
	logger := slog.New(slog.DiscardHandler)
	pool := dbtest.NewPool(t, dbtest.NewDatabase(t))
	if err := db.Migrate(t.Context(), pool, logger); err != nil {
		t.Fatal(err)
	}
	a := &testAPI{t: t, pool: pool, asker: &fakeAsker{replies: replies}, events: &fakeEvents{}, reviewer: &fakeReviewer{}}
	a.device, a.token = a.addDevice("fruit-sorter-1")

	h := New(pool, a.asker, a.events, a.reviewer, logger)
	router, err := server.NewRouter(logger, server.WithRoutes(h.Register))
	if err != nil {
		t.Fatal(err)
	}
	a.srv = httptest.NewServer(router)
	t.Cleanup(a.srv.Close)
	return a
}

func (a *testAPI) addDevice(key string) (uuid.UUID, string) {
	a.t.Helper()
	token := "token-" + key
	hash := sha256.Sum256([]byte(token))
	var id uuid.UUID
	if err := a.pool.QueryRow(a.t.Context(), `INSERT INTO devices (key, name, token_hash) VALUES ($1, $1, $2) RETURNING id`,
		key, hash[:]).Scan(&id); err != nil {
		a.t.Fatal(err)
	}
	return id, token
}

// post sends body as JSON with the token and decodes the answer into out.
func (a *testAPI) post(path, token string, body, out any) int {
	a.t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		a.t.Fatal(err)
	}
	req, err := http.NewRequestWithContext(a.t.Context(), http.MethodPost, a.srv.URL+path, bytes.NewReader(data))
	if err != nil {
		a.t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := a.srv.Client().Do(req)
	if err != nil {
		a.t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		a.t.Fatal(err)
	}
	if out != nil && resp.StatusCode == http.StatusOK {
		if err := json.Unmarshal(raw, out); err != nil {
			a.t.Fatalf("%s: %v in %s", path, err, raw)
		}
	}
	return resp.StatusCode
}

var frame = base64.StdEncoding.EncodeToString([]byte{0xff, 0xd8, 0xff, 0xd9})

// waiting is an irreversible decision for /decide.
func waiting() map[string]any {
	return map[string]any{
		"id": uuid.Must(uuid.NewV7()), "revision": 1, "task": "sort_item", "options": []string{"basket", "discard"},
		"action_class": "irreversible", "proposed_action": "discard", "prompt": "Is it a fruit?",
		"metadata": map[string]any{"detector": map[string]any{"label": "unknown"}}, "local_model": "yolox-nano",
		"decided_at": time.Now().UTC(),
	}
}

// onDevice is a reversible decision the device made.
func onDevice() map[string]any {
	return map[string]any{
		"id": uuid.Must(uuid.NewV7()), "revision": 1, "task": "sort_item", "options": []string{"basket", "discard"},
		"action_class": "reversible", "proposed_action": "basket", "action": "basket",
		"local_model": "yolox-nano", "local_confidence": 0.94, "local_latency_ms": 36, "decided_at": time.Now().UTC(),
	}
}

type decisionOut struct {
	ID         uuid.UUID `json:"id"`
	Status     string    `json:"status"`
	Source     *string   `json:"source"`
	Action     *string   `json:"action"`
	Model      *string   `json:"model"`
	LatencyMs  *int32    `json:"latency_ms"`
	CostUsd    *float64  `json:"cost_usd"`
	HasFrame   bool      `json:"has_frame"`
	Revision   int32     `json:"revision"`
	ExecutedAt *string   `json:"executed_at"`
}

type itemErrorOut struct {
	DecisionID uuid.UUID `json:"decision_id"`
	Error      string    `json:"error"`
	Retry      bool      `json:"retry"`
}

type decideOut struct {
	Decisions []decisionOut  `json:"decisions"`
	Errors    []itemErrorOut `json:"errors"`
}

type syncOut struct {
	Decisions   []decisionOut `json:"decisions"`
	Corrections []struct {
		ID     uuid.UUID `json:"id"`
		Action string    `json:"action"`
	} `json:"corrections"`
	Errors     []itemErrorOut `json:"errors"`
	ServerTime string         `json:"server_time"`
}

func TestAuthenticate(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t, "discard")
	body := map[string]any{"items": []any{}}

	if code := api.post("/api/v1/sync", "", body, nil); code != http.StatusUnauthorized {
		t.Errorf("no token: status %d, want 401", code)
	}
	if code := api.post("/api/v1/sync", "wrong", body, nil); code != http.StatusUnauthorized {
		t.Errorf("wrong token: status %d, want 401", code)
	}
	if code := api.post("/api/v1/sync", api.token, body, nil); code != http.StatusOK {
		t.Errorf("right token: status %d, want 200", code)
	}
	if _, err := api.pool.Exec(t.Context(), `UPDATE devices SET deleted_at = now() WHERE id = $1`, api.device); err != nil {
		t.Fatal(err)
	}
	if code := api.post("/api/v1/sync", api.token, body, nil); code != http.StatusUnauthorized {
		t.Errorf("deleted device: status %d, want 401", code)
	}
}

func TestDecide(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t, "discard")
	d := waiting()

	var out decideOut
	if code := api.post("/api/v1/decide", api.token, map[string]any{"items": []any{map[string]any{"decision": d, "frame": frame}}}, &out); code != http.StatusOK {
		t.Fatalf("status %d", code)
	}
	if len(out.Errors) != 0 || len(out.Decisions) != 1 {
		t.Fatalf("response = %+v", out)
	}
	got := out.Decisions[0]
	if got.Status != "decided_on_server" || *got.Action != "discard" || *got.Source != "server" ||
		*got.Model != "fake" || *got.CostUsd != 0.0003 || !got.HasFrame {
		t.Errorf("decision = %+v", got)
	}
	q := api.asker.asked[0]
	if q.Prompt != "Is it a fruit?" || len(q.Options) != 2 || q.Frame == nil || !bytes.Contains(q.Metadata, []byte("unknown")) {
		t.Errorf("model asked %+v", q)
	}

	// The agent timed out and asks again: the answer is there, no new call.
	api.post("/api/v1/decide", api.token, map[string]any{"items": []any{map[string]any{"decision": d}}}, &out)
	if api.asker.calls() != 1 || *out.Decisions[0].Action != "discard" {
		t.Errorf("after a retry: %d calls, decision %+v", api.asker.calls(), out.Decisions)
	}
	if api.events.decisions.Load() == 0 || api.events.devices.Load() == 0 {
		t.Error("no dashboard events")
	}
}

func TestDecideRetriesAtOnce(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t, "basket")
	api.asker.delay = 300 * time.Millisecond
	body := map[string]any{"items": []any{map[string]any{"decision": waiting()}}}

	var wg sync.WaitGroup
	for range 3 {
		wg.Go(func() {
			var out decideOut
			api.post("/api/v1/decide", api.token, body, &out)
			if len(out.Decisions) != 1 || *out.Decisions[0].Action != "basket" {
				t.Errorf("response = %+v", out)
			}
		})
	}
	wg.Wait()
	if n := api.asker.calls(); n != 1 {
		t.Errorf("model called %d times for one decision, want 1", n)
	}
}

func TestDecideAnswers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		replies   []string
		wantCalls int
		wantError bool
	}{
		{"usable at once", []string{"discard"}, 1, false},
		{"usable on the second try", []string{"", "discard"}, 2, false},
		{"never usable", []string{""}, 2, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			api := newTestAPI(t, tt.replies...)
			d := waiting()
			var out decideOut
			api.post("/api/v1/decide", api.token, map[string]any{"items": []any{map[string]any{"decision": d}}}, &out)
			if (len(out.Errors) == 1) != tt.wantError {
				t.Errorf("errors = %+v, want error %v", out.Errors, tt.wantError)
			}
			if tt.wantError && !out.Errors[0].Retry {
				t.Error("a model failure is not marked retry")
			}
			var calls, failed int
			if err := api.pool.QueryRow(t.Context(),
				`SELECT count(*), count(*) FILTER (WHERE error IS NOT NULL) FROM model_calls WHERE decision_id = $1`, d["id"]).
				Scan(&calls, &failed); err != nil {
				t.Fatal(err)
			}
			if calls != tt.wantCalls || calls-failed != map[bool]int{true: 0, false: 1}[tt.wantError] {
				t.Errorf("recorded %d calls, %d failed", calls, failed)
			}
		})
	}
}

func TestDecideRejects(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t, "discard")

	noPrompt := waiting()
	delete(noPrompt, "prompt")
	badFrame := waiting()
	items := []any{
		map[string]any{"decision": onDevice()},
		map[string]any{"decision": noPrompt},
		map[string]any{"decision": badFrame, "frame": base64.StdEncoding.EncodeToString([]byte("not a jpeg"))},
	}
	var out decideOut
	api.post("/api/v1/decide", api.token, map[string]any{"items": items}, &out)
	if len(out.Decisions) != 0 || len(out.Errors) != 3 {
		t.Fatalf("response = %+v, want three errors", out)
	}
	for _, e := range out.Errors {
		if e.Retry {
			t.Errorf("rejected item %+v is marked retry", e)
		}
	}
	if api.asker.calls() != 0 {
		t.Error("the model was asked about rejected decisions")
	}
	if code := api.post("/api/v1/decide", api.token, map[string]any{"items": []any{}}, nil); code != http.StatusBadRequest {
		t.Errorf("no items: status %d, want 400", code)
	}
}

func TestSync(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t, "discard")
	d := onDevice()
	body := func(items ...any) map[string]any {
		return map[string]any{"agent_version": "0.1.0", "device_metadata": map[string]any{"camera": "usb"}, "items": items}
	}

	var out syncOut
	if code := api.post("/api/v1/sync", api.token, body(map[string]any{"decision": d, "frame": frame}), &out); code != http.StatusOK {
		t.Fatalf("status %d", code)
	}
	if len(out.Decisions) != 1 || out.Decisions[0].Status != "decided_on_device" || !out.Decisions[0].HasFrame || out.ServerTime == "" {
		t.Fatalf("first sync = %+v", out)
	}
	if got := out.Decisions[0].LatencyMs; got == nil || *got != 36 {
		t.Errorf("latency_ms = %v, want the local model's 36", got)
	}

	// Executed: a higher revision applies.
	d["revision"], d["executed_at"] = 2, time.Now().UTC()
	api.post("/api/v1/sync", api.token, body(map[string]any{"decision": d}), &out)
	if got := out.Decisions[0]; got.Status != "executed" || got.Revision != 2 {
		t.Errorf("after executing = %+v", got)
	}
	// A late retry of revision 1 doesn't roll it back.
	d["revision"], d["executed_at"] = 1, nil
	api.post("/api/v1/sync", api.token, body(map[string]any{"decision": d}), &out)
	if got := out.Decisions[0]; got.Status != "executed" || got.Revision != 2 {
		t.Errorf("after a stale retry = %+v", got)
	}

	var version, metadata string
	if err := api.pool.QueryRow(t.Context(), `SELECT agent_version, metadata::text FROM devices WHERE id = $1`, api.device).
		Scan(&version, &metadata); err != nil {
		t.Fatal(err)
	}
	if version != "0.1.0" || metadata != `{"camera": "usb"}` {
		t.Errorf("device = %q %q", version, metadata)
	}
}

func TestSyncRejects(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t, "discard")
	_, otherToken := api.addDevice("other-1")

	mine := onDevice()
	api.post("/api/v1/sync", api.token, map[string]any{"items": []any{map[string]any{"decision": mine}}}, nil)

	// Another device sends the same id with a higher revision.
	stolen := onDevice()
	stolen["id"], stolen["revision"], stolen["executed_at"] = mine["id"], 5, time.Now().UTC()
	noAction := onDevice()
	delete(noAction, "action")
	v4 := onDevice()
	v4["id"] = uuid.New()
	negative := onDevice()
	negative["local_latency_ms"] = -1

	var out syncOut
	api.post("/api/v1/sync", otherToken, map[string]any{"items": []any{
		map[string]any{"decision": stolen},
		map[string]any{"decision": noAction},
		map[string]any{"decision": v4},
		map[string]any{"decision": negative},
	}}, &out)
	if len(out.Errors) != 4 || len(out.Decisions) != 0 {
		t.Fatalf("response = %+v, want four errors", out)
	}
	var executed bool
	if err := api.pool.QueryRow(t.Context(), `SELECT executed_at IS NOT NULL FROM decisions WHERE id = $1`, mine["id"]).Scan(&executed); err != nil {
		t.Fatal(err)
	}
	if executed {
		t.Error("another device changed the decision")
	}

	if code := api.post("/api/v1/sync", api.token, map[string]any{"device_metadata": []int{1}}, nil); code != http.StatusBadRequest {
		t.Errorf("metadata not an object: status %d, want 400", code)
	}
}

func TestSyncCorrections(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t, "discard")
	d := onDevice()
	d["needs_review"], d["prompt"], d["local_confidence"] = true, "Is it a fruit?", 0.6

	var out syncOut
	api.post("/api/v1/sync", api.token, map[string]any{"items": []any{map[string]any{"decision": d}}}, &out)
	if api.reviewer.wakes.Load() != 1 {
		t.Errorf("reviewer woken %d times, want 1", api.reviewer.wakes.Load())
	}

	// The review disagrees.
	call := uuid.Must(uuid.NewV7())
	for _, stmt := range []string{
		`INSERT INTO model_calls (id, decision_id, purpose, model, action, reason, latency_ms) VALUES ($2, $1, 'review', 'fake', 'discard', 'A ball.', 1000)`,
		`UPDATE decisions SET reviewed_at = now(), review_call_id = $2, corrected_action = 'discard' WHERE id = $1`,
	} {
		if _, err := api.pool.Exec(t.Context(), stmt, d["id"], call); err != nil {
			t.Fatal(err)
		}
	}

	api.post("/api/v1/sync", api.token, map[string]any{"items": []any{}}, &out)
	if len(out.Corrections) != 1 || out.Corrections[0].ID != d["id"] || out.Corrections[0].Action != "discard" {
		t.Fatalf("corrections = %+v", out.Corrections)
	}

	// The robot carried it out.
	d["revision"], d["correction_applied_at"], d["executed_at"] = 2, time.Now().UTC(), time.Now().UTC()
	api.post("/api/v1/sync", api.token, map[string]any{"items": []any{map[string]any{"decision": d}}}, &out)
	if len(out.Corrections) != 0 || out.Decisions[0].Status != "corrected" {
		t.Errorf("after applying: corrections %+v, decision %+v, errors %+v", out.Corrections, out.Decisions, out.Errors)
	}
}
