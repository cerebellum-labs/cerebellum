package dashboard

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/cerebellum-labs/cerebellum/packages/server/internal/events"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	m.Run()
}

var testSecret = []byte(strings.Repeat("k", 32))

// newTestHandler returns a handler with no database: enough for the routes
// that never reach it.
func newTestHandler(origin string) *Handler {
	return New(nil, &events.Hub{}, Config{
		AdminPassword: "secret",
		SessionSecret: testSecret,
		AllowedOrigin: origin,
	}, slog.New(slog.DiscardHandler))
}

func TestSessions(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	s := sessions{secret: testSecret}
	cookie := s.issue(now)
	_, signature, _ := strings.Cut(cookie, ".")

	tests := []struct {
		name  string
		value string
		at    time.Time
		want  bool
	}{
		{"fresh", cookie, now, true},
		{"a day before expiry", cookie, now.Add(sessionTTL - 24*time.Hour), true},
		{"expired", cookie, now.Add(sessionTTL), false},
		{"expiry changed", "9999999999." + signature, now, false},
		{"signature changed", cookie[:len(cookie)-2] + "AA", now, false},
		{"signed with another secret", sessions{secret: []byte(strings.Repeat("x", 32))}.issue(now), now, false},
		{"no signature", "1893456000", now, false},
		{"empty", "", now, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := s.valid(tt.value, tt.at); got != tt.want {
				t.Errorf("valid = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPasswordMatches(t *testing.T) {
	t.Parallel()
	tests := []struct {
		given string
		want  bool
	}{
		{"secret", true},
		{"Secret", false},
		{"secret ", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.given, func(t *testing.T) {
			t.Parallel()
			if got := passwordMatches(tt.given, "secret"); got != tt.want {
				t.Errorf("passwordMatches(%q) = %v, want %v", tt.given, got, tt.want)
			}
		})
	}
}

// serve sends one request through the dashboard routes.
func serve(h *Handler, req *http.Request) *httptest.ResponseRecorder {
	r := gin.New()
	h.Register(r)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestLogin(t *testing.T) {
	t.Parallel()
	h := newTestHandler("")

	w := serve(h, httptest.NewRequest(http.MethodPost, "/api/dashboard/login", strings.NewReader(`{"password":"wrong"}`)))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password: status %d, want 401", w.Code)
	}

	w = serve(h, httptest.NewRequest(http.MethodPost, "/api/dashboard/login", strings.NewReader(`{"password":"secret"}`)))
	if w.Code != http.StatusOK {
		t.Fatalf("right password: status %d, want 200: %s", w.Code, w.Body)
	}
	cookie := w.Result().Cookies()[0]
	if cookie.Name != sessionCookie || !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode || cookie.Path != "/" {
		t.Errorf("cookie = %+v, want %s, HttpOnly, Secure, SameSite=Lax, Path=/", cookie, sessionCookie)
	}
	if !h.sessions.valid(cookie.Value, time.Now()) {
		t.Error("the issued cookie does not validate")
	}
}

func TestLoginRateLimit(t *testing.T) {
	t.Parallel()
	h := newTestHandler("")
	var last int
	for range 6 {
		w := serve(h, httptest.NewRequest(http.MethodPost, "/api/dashboard/login", strings.NewReader(`{"password":"wrong"}`)))
		last = w.Code
	}
	if last != http.StatusTooManyRequests {
		t.Errorf("sixth attempt in a row: status %d, want 429", last)
	}
}

func TestRequireAdmin(t *testing.T) {
	t.Parallel()
	h := newTestHandler("")

	// Rejected before the handler, which would need the database.
	w := serve(h, httptest.NewRequest(http.MethodPost, "/api/dashboard/devices", strings.NewReader(`{"key":"x"}`)))
	if w.Code != http.StatusUnauthorized {
		t.Errorf("without a session: status %d, want 401", w.Code)
	}
	req := httptest.NewRequest(http.MethodDelete, "/api/dashboard/devices/0199f000-0000-7000-8000-000000000001", nil)
	req.Header.Set("Cookie", sessionCookie+"=1893456000.forged")
	if w := serve(h, req); w.Code != http.StatusUnauthorized {
		t.Errorf("with a forged session: status %d, want 401", w.Code)
	}
}

func TestCheckOrigin(t *testing.T) {
	t.Parallel()
	h := newTestHandler("https://demo.cerebellumlabs.dev")
	const (
		devices = "/api/dashboard/devices"
		frame   = "/api/dashboard/frames/not-an-id"
	)

	// A request that passes the check goes on to the session check (401)
	// or, for reading, to the handler (404 for the malformed id). One that
	// fails it gets 403.
	tests := []struct {
		name     string
		method   string
		path     string
		origin   string
		wantCode int
	}{
		{"the dashboard", http.MethodPost, devices, "https://demo.cerebellumlabs.dev", http.StatusUnauthorized},
		{"the API's own origin", http.MethodPost, devices, "https://api.cerebellumlabs.dev", http.StatusUnauthorized},
		{"no origin, not a browser", http.MethodPost, devices, "", http.StatusUnauthorized},
		{"a sibling subdomain", http.MethodPost, devices, "https://evil.cerebellumlabs.dev", http.StatusForbidden},
		{"another site", http.MethodPost, devices, "https://example.com", http.StatusForbidden},
		{"another site, reading", http.MethodGet, frame, "https://example.com", http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(`{}`))
			req.Host = "api.cerebellumlabs.dev"
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			if w := serve(h, req); w.Code != tt.wantCode {
				t.Errorf("status %d, want %d: %s", w.Code, tt.wantCode, w.Body)
			}
		})
	}
}
