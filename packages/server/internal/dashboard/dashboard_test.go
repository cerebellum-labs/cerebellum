//go:build integration

package dashboard

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cerebellum-labs/cerebellum/packages/server/internal/db"
	"github.com/cerebellum-labs/cerebellum/packages/server/internal/db/dbtest"
	"github.com/cerebellum-labs/cerebellum/packages/server/internal/events"
	"github.com/cerebellum-labs/cerebellum/packages/server/internal/server"
)

// testAPI is the dashboard API on a database of its own, served over HTTP.
type testAPI struct {
	t      *testing.T
	pool   *pgxpool.Pool
	srv    *httptest.Server
	cookie *http.Cookie
}

func newTestAPI(t *testing.T) *testAPI {
	t.Helper()
	logger := slog.New(slog.DiscardHandler)
	pool := dbtest.NewPool(t, dbtest.NewDatabase(t))
	if err := db.Migrate(t.Context(), pool, logger); err != nil {
		t.Fatal(err)
	}
	h := New(db.New(pool), &events.Hub{}, Config{AdminPassword: "secret", SessionSecret: testSecret}, logger)
	router, err := server.NewRouter(logger, server.WithRoutes(h.Register))
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)
	return &testAPI{t: t, pool: pool, srv: srv}
}

// do sends a request with the admin cookie, if signed in, and decodes a JSON
// answer into out when out is not nil.
func (a *testAPI) do(method, path, body string, out any) int {
	a.t.Helper()
	req, err := http.NewRequestWithContext(a.t.Context(), method, a.srv.URL+path, strings.NewReader(body))
	if err != nil {
		a.t.Fatal(err)
	}
	if a.cookie != nil {
		req.AddCookie(a.cookie)
	}
	resp, err := a.srv.Client().Do(req)
	if err != nil {
		a.t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		a.t.Fatal(err)
	}
	for _, c := range resp.Cookies() {
		if c.Name == sessionCookie {
			a.cookie = c
		}
	}
	if out != nil && resp.StatusCode < 300 {
		if err := json.Unmarshal(data, out); err != nil {
			a.t.Fatalf("%s %s: %v in %s", method, path, err, data)
		}
	}
	return resp.StatusCode
}

func (a *testAPI) login() {
	a.t.Helper()
	if code := a.do(http.MethodPost, "/api/dashboard/login", `{"password":"secret"}`, nil); code != http.StatusOK {
		a.t.Fatalf("login: status %d", code)
	}
}

type deviceJSON struct {
	ID   uuid.UUID `json:"id"`
	Key  string    `json:"key"`
	Name string    `json:"name"`
}

func TestDevices(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	q := db.New(api.pool)

	if code := api.do(http.MethodPost, "/api/dashboard/devices", `{"key":"fruit-sorter-1"}`, nil); code != http.StatusUnauthorized {
		t.Fatalf("create without a session: status %d, want 401", code)
	}
	api.login()

	var created struct {
		Device deviceJSON `json:"device"`
		Token  string     `json:"token"`
	}
	if code := api.do(http.MethodPost, "/api/dashboard/devices", `{"key":"fruit-sorter-1","name":" Fruit sorter "}`, &created); code != http.StatusCreated {
		t.Fatalf("create: status %d", code)
	}
	if created.Device.Name != "Fruit sorter" || created.Token == "" {
		t.Errorf("created %+v with token %q", created.Device, created.Token)
	}
	if code := api.do(http.MethodPost, "/api/dashboard/devices", `{"key":"fruit-sorter-1"}`, nil); code != http.StatusConflict {
		t.Errorf("duplicate key: status %d, want 409", code)
	}
	if code := api.do(http.MethodPost, "/api/dashboard/devices", `{"key":"Fruit Sorter"}`, nil); code != http.StatusBadRequest {
		t.Errorf("bad key: status %d, want 400", code)
	}

	hash := func(token string) []byte { s := sha256.Sum256([]byte(token)); return s[:] }
	if _, err := q.GetDeviceByTokenHash(t.Context(), hash(created.Token)); err != nil {
		t.Fatalf("the new token does not authenticate: %v", err)
	}

	var rotated struct {
		Token string `json:"token"`
	}
	path := "/api/dashboard/devices/" + created.Device.ID.String()
	if code := api.do(http.MethodPost, path+"/rotate-token", "", &rotated); code != http.StatusOK {
		t.Fatalf("rotate: status %d", code)
	}
	if _, err := q.GetDeviceByTokenHash(t.Context(), hash(created.Token)); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("the old token still authenticates: %v", err)
	}
	if _, err := q.GetDeviceByTokenHash(t.Context(), hash(rotated.Token)); err != nil {
		t.Errorf("the rotated token does not authenticate: %v", err)
	}

	if code := api.do(http.MethodDelete, path, "", nil); code != http.StatusOK {
		t.Fatalf("delete: status %d", code)
	}
	if code := api.do(http.MethodDelete, path, "", nil); code != http.StatusNotFound {
		t.Errorf("delete again: status %d, want 404", code)
	}
	var list struct {
		Devices []deviceJSON `json:"devices"`
	}
	api.do(http.MethodGet, "/api/dashboard/devices", "", &list)
	if len(list.Devices) != 0 {
		t.Errorf("devices after delete: %+v, want none", list.Devices)
	}
}

// seedJournal adds a device with three decisions, oldest first: one decided
// on the device with a frame, one decided by the server, and an uncertain
// one the review corrected. It returns their ids in that order.
func seedJournal(t *testing.T, pool *pgxpool.Pool) (device uuid.UUID, ids [3]uuid.UUID) {
	t.Helper()
	ctx := t.Context()
	hash := sha256.Sum256([]byte("token"))
	if err := pool.QueryRow(ctx, `INSERT INTO devices (key, name, token_hash) VALUES ('fruit-sorter-1', 'Fruit sorter', $1) RETURNING id`,
		hash[:]).Scan(&device); err != nil {
		t.Fatal(err)
	}
	for i := range ids {
		ids[i] = uuid.Must(uuid.NewV7())
	}
	decideCall, reviewCall := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	args := pgx.NamedArgs{"device": device, "a": ids[0], "b": ids[1], "c": ids[2], "decide": decideCall, "review": reviewCall}
	stmts := []string{
		`INSERT INTO decisions (id, device_id, revision, task, options, action_class, action, proposed_action, local_model, local_confidence, decided_at)
		 VALUES (@a, @device, 1, 'sort_item', '{basket,discard}', 'reversible', 'basket', 'basket', 'yolox-nano', 0.95, now())`,
		`INSERT INTO frames (decision_id, jpeg) VALUES (@a, '\xffd8ffd9')`,
		`INSERT INTO decisions (id, device_id, revision, task, options, action_class, prompt, decided_at)
		 VALUES (@b, @device, 1, 'sort_item', '{basket,discard}', 'irreversible', 'Is it a fruit?', now())`,
		`INSERT INTO model_calls (id, decision_id, purpose, model, action, confidence, reason, latency_ms, input_tokens, output_tokens, cost_usd)
		 VALUES (@decide, @b, 'decide', 'nemotron', 'discard', 0.9, 'A paper cup.', 1800, 900, 40, 0.0002)`,
		`UPDATE decisions SET action = 'discard', model_call_id = @decide WHERE id = @b`,
		`INSERT INTO decisions (id, device_id, revision, task, options, action_class, action, prompt, needs_review, local_confidence, decided_at)
		 VALUES (@c, @device, 1, 'sort_item', '{basket,discard}', 'reversible', 'basket', 'Is it a fruit?', true, 0.6, now())`,
		`INSERT INTO model_calls (id, decision_id, purpose, model, action, reason, latency_ms, cost_usd)
		 VALUES (@review, @c, 'review', 'nemotron', 'discard', 'Not a fruit.', 2000, 0.0001)`,
		`UPDATE decisions SET reviewed_at = now(), review_call_id = @review, corrected_action = 'discard' WHERE id = @c`,
	}
	for _, stmt := range stmts {
		if _, err := pool.Exec(ctx, stmt, args); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	return device, ids
}

type decisionJSON struct {
	ID              uuid.UUID `json:"id"`
	Status          string    `json:"status"`
	Source          *string   `json:"source"`
	Model           *string   `json:"model"`
	Reason          *string   `json:"reason"`
	CostUsd         *float64  `json:"cost_usd"`
	CorrectedAction *string   `json:"corrected_action"`
	HasFrame        bool      `json:"has_frame"`
	DeviceKey       string    `json:"device_key"`
	DecidedAt       string    `json:"decided_at"`
}

func TestJournal(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	device, ids := seedJournal(t, api.pool)

	var page struct {
		Decisions  []decisionJSON `json:"decisions"`
		NextBefore *uuid.UUID     `json:"next_before"`
	}
	if code := api.do(http.MethodGet, "/api/dashboard/decisions?limit=2", "", &page); code != http.StatusOK {
		t.Fatalf("list: status %d", code)
	}
	if len(page.Decisions) != 2 || page.Decisions[0].ID != ids[2] || page.Decisions[1].ID != ids[1] {
		t.Fatalf("first page = %+v, want the two newest", page.Decisions)
	}
	if page.NextBefore == nil || *page.NextBefore != ids[1] {
		t.Fatalf("next_before = %v, want %s", page.NextBefore, ids[1])
	}
	server := page.Decisions[1]
	if server.Status != "decided_on_server" || server.Model == nil || *server.Model != "nemotron" ||
		server.CostUsd == nil || *server.CostUsd != 0.0002 || server.Reason == nil || *server.Reason != "A paper cup." {
		t.Errorf("server decision = %+v", server)
	}
	if !strings.HasSuffix(server.DecidedAt, "Z") || len(server.DecidedAt) != len("2026-10-21T18:09:00.123Z") {
		t.Errorf("decided_at = %q, want UTC with milliseconds", server.DecidedAt)
	}

	api.do(http.MethodGet, "/api/dashboard/decisions?before="+page.NextBefore.String(), "", &page)
	if len(page.Decisions) != 1 || page.Decisions[0].ID != ids[0] || page.NextBefore != nil {
		t.Fatalf("last page = %+v, next %v", page.Decisions, page.NextBefore)
	}
	if d := page.Decisions[0]; !d.HasFrame || d.Model == nil || *d.Model != "yolox-nano" || d.DeviceKey != "fruit-sorter-1" {
		t.Errorf("device decision = %+v", d)
	}

	api.do(http.MethodGet, "/api/dashboard/decisions?status=corrected&device_id="+device.String(), "", &page)
	if len(page.Decisions) != 1 || page.Decisions[0].ID != ids[2] || *page.Decisions[0].CorrectedAction != "discard" {
		t.Errorf("corrected = %+v", page.Decisions)
	}
	for _, query := range []string{"status=done", "limit=0", "limit=201", "device_id=x", "before=x"} {
		if code := api.do(http.MethodGet, "/api/dashboard/decisions?"+query, "", nil); code != http.StatusBadRequest {
			t.Errorf("?%s: status %d, want 400", query, code)
		}
	}

	var one struct {
		Decision    decisionJSON `json:"decision"`
		Corrections []struct {
			ID     uuid.UUID `json:"id"`
			Action string    `json:"action"`
			Reason string    `json:"reason"`
			Model  string    `json:"model"`
		} `json:"corrections"`
	}
	if code := api.do(http.MethodGet, "/api/dashboard/decisions/"+ids[2].String(), "", &one); code != http.StatusOK {
		t.Fatalf("get: status %d", code)
	}
	if len(one.Corrections) != 1 || one.Corrections[0].ID != ids[2] || one.Corrections[0].Reason != "Not a fruit." || one.Corrections[0].Model != "nemotron" {
		t.Errorf("corrections = %+v", one.Corrections)
	}
	if code := api.do(http.MethodGet, "/api/dashboard/decisions/"+uuid.Must(uuid.NewV7()).String(), "", nil); code != http.StatusNotFound {
		t.Errorf("missing decision: status %d, want 404", code)
	}
}

func TestStats(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	device, _ := seedJournal(t, api.pool)

	var stats struct {
		Total        int64               `json:"total"`
		BySource     map[string]int64    `json:"by_source"`
		ByStatus     map[string]int64    `json:"by_status"`
		ByClass      map[string]int64    `json:"by_class"`
		Corrections  int64               `json:"corrections"`
		CostUsd      float64             `json:"cost_usd"`
		AvgLatencyMs map[string]*float64 `json:"avg_latency_ms"`
	}
	if code := api.do(http.MethodGet, "/api/dashboard/stats?device_id="+device.String(), "", &stats); code != http.StatusOK {
		t.Fatalf("stats: status %d", code)
	}
	if stats.Total != 3 || stats.BySource["device"] != 2 || stats.BySource["server"] != 1 ||
		stats.ByStatus["corrected"] != 1 || stats.ByClass["reversible"] != 2 || stats.Corrections != 1 {
		t.Errorf("counts = %+v", stats)
	}
	// Both calls count: the decision and the review.
	if stats.CostUsd < 0.000299 || stats.CostUsd > 0.000301 {
		t.Errorf("cost = %v, want 0.0003", stats.CostUsd)
	}
	if avg := stats.AvgLatencyMs["server"]; avg == nil || *avg != 1800 || stats.AvgLatencyMs["device"] != nil {
		t.Errorf("avg latency = %v / %v, want 1800 / null", avg, stats.AvgLatencyMs["device"])
	}

	api.do(http.MethodGet, "/api/dashboard/stats?device_id="+uuid.Must(uuid.NewV7()).String(), "", &stats)
	if stats.Total != 0 || stats.AvgLatencyMs["server"] != nil {
		t.Errorf("another device's stats = %+v", stats)
	}
}

func TestFrame(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	_, ids := seedJournal(t, api.pool)

	resp, err := api.srv.Client().Get(api.srv.URL + "/api/dashboard/frames/" + ids[0].String())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != "image/jpeg" ||
		!strings.Contains(resp.Header.Get("Cache-Control"), "immutable") || string(body) != "\xff\xd8\xff\xd9" {
		t.Errorf("frame: status %d, headers %v, body %x", resp.StatusCode, resp.Header, body)
	}
	if code := api.do(http.MethodGet, "/api/dashboard/frames/"+ids[1].String(), "", nil); code != http.StatusNotFound {
		t.Errorf("decision without a frame: status %d, want 404", code)
	}
}

func TestStream(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	conn, resp, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(api.srv.URL, "http")+"/api/dashboard/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.CloseNow() }()
	if resp.Body != nil {
		_ = resp.Body.Close()
	}

	api.login()
	if code := api.do(http.MethodPost, "/api/dashboard/devices", `{"key":"fruit-sorter-1"}`, nil); code != http.StatusCreated {
		t.Fatalf("create: status %d", code)
	}

	_, msg, err := conn.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var event struct {
		Type   string     `json:"type"`
		Device deviceJSON `json:"device"`
	}
	if err := json.Unmarshal(msg, &event); err != nil {
		t.Fatalf("event %s: %v", msg, err)
	}
	if event.Type != "device.updated" || event.Device.Key != "fruit-sorter-1" {
		t.Errorf("event = %s", msg)
	}

	// Another site's page may not open the stream.
	_, resp, err = websocket.Dial(ctx, "ws"+strings.TrimPrefix(api.srv.URL, "http")+"/api/dashboard/ws",
		&websocket.DialOptions{HTTPHeader: http.Header{"Origin": {"https://example.com"}}})
	if err == nil || resp == nil || resp.StatusCode != http.StatusForbidden {
		t.Errorf("foreign origin: err %v, response %v, want 403", err, resp)
	}
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
}
