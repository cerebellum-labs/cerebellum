package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/cerebellum-labs/cerebellum/packages/server/internal/httperr"
	"github.com/cerebellum-labs/cerebellum/packages/server/internal/logging"
)

// newTestRouter returns the production router, logging to logger, with
// extra routes that panic and read the body, so the middleware can be
// exercised end to end.
func newTestRouter(t *testing.T, logger *slog.Logger) *gin.Engine {
	t.Helper()
	r, err := NewRouter(logger)
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
	}
	r.GET("/test/panic", func(*gin.Context) { panic("boom") })
	r.POST("/test/read", func(c *gin.Context) {
		if _, err := io.ReadAll(c.Request.Body); err != nil {
			if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
				httperr.PayloadTooLarge(c)
				return
			}
			httperr.InternalError(c)
			return
		}
		c.Status(http.StatusNoContent)
	})
	return r
}

// discardLogger is for tests that don't look at the logs.
var discardLogger = slog.New(slog.DiscardHandler)

// logRecords decodes every JSON line written to buf.
func logRecords(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var records []map[string]any
	for line := range strings.SplitSeq(strings.TrimSpace(buf.String()), "\n") {
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("log line is not JSON: %q", line)
		}
		records = append(records, rec)
	}
	return records
}

func TestRecoveryLogsPanicAsJSON(t *testing.T) {
	t.Parallel()
	var logs bytes.Buffer
	r := newTestRouter(t, logging.New(&logs))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test/panic", nil)
	req.Header.Set(HeaderRequestID, "req-1")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
	if want := `{"error":{"code":"internal_error","message":"internal server error"}}`; w.Body.String() != want {
		t.Errorf("body = %q, want %q", w.Body.String(), want)
	}

	records := logRecords(t, &logs)
	if len(records) != 2 {
		t.Fatalf("got %d log records, want 2 (panic, access): %s", len(records), logs.String())
	}
	panicRec, accessRec := records[0], records[1]
	if panicRec["msg"] != "panic recovered" || panicRec["panic"] != "boom" {
		t.Errorf("panic record = %v", panicRec)
	}
	if stack, _ := panicRec["stack"].(string); !strings.Contains(stack, "goroutine") {
		t.Errorf("stack = %q, want a goroutine trace", stack)
	}
	if panicRec["request_id"] != "req-1" || accessRec["request_id"] != "req-1" {
		t.Errorf("request_id = %v / %v, want req-1 on both", panicRec["request_id"], accessRec["request_id"])
	}
	if accessRec["status"] != float64(http.StatusInternalServerError) {
		t.Errorf("access status = %v, want 500", accessRec["status"])
	}
}

func TestRequestID(t *testing.T) {
	t.Parallel()
	r := newTestRouter(t, discardLogger)

	tests := []struct {
		name     string
		incoming string
		wantSame bool
	}{
		{"absent", "", false},
		{"valid is reused", "device-42.abc_1", true},
		{"newline is replaced", "x\ninjected", false},
		{"too long is replaced", strings.Repeat("a", maxRequestIDLen+1), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/nope", nil)
			if tt.incoming != "" {
				req.Header.Set(HeaderRequestID, tt.incoming)
			}
			r.ServeHTTP(w, req)

			got := w.Header().Get(HeaderRequestID)
			if tt.wantSame {
				if got != tt.incoming {
					t.Errorf("request id = %q, want %q", got, tt.incoming)
				}
				return
			}
			if got == "" || got == tt.incoming || !validRequestID(got) {
				t.Errorf("request id = %q, want a fresh valid id", got)
			}
		})
	}
}

func TestBodyLimit(t *testing.T) {
	t.Parallel()
	r := newTestRouter(t, discardLogger)
	tooLarge := `{"error":{"code":"payload_too_large","message":"request body too large"}}`

	tests := []struct {
		name       string
		body       io.Reader
		size       int64 // Content-Length; -1 sends the body chunked
		wantStatus int
		wantBody   string
	}{
		{"within limit", bytes.NewReader(make([]byte, 1024)), 1024, http.StatusNoContent, ""},
		{"declared over limit", bytes.NewReader(make([]byte, maxBodyBytes+1)), maxBodyBytes + 1, http.StatusRequestEntityTooLarge, tooLarge},
		{"chunked over limit", bytes.NewReader(make([]byte, maxBodyBytes+1)), -1, http.StatusRequestEntityTooLarge, tooLarge},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/test/read", tt.body)
			req.ContentLength = tt.size
			r.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", w.Code, tt.wantStatus)
			}
			if got := w.Body.String(); got != tt.wantBody {
				t.Errorf("body = %q, want %q", got, tt.wantBody)
			}
		})
	}
}
