package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORS(t *testing.T) {
	t.Parallel()
	const dashboard = "https://demo.cerebellumlabs.dev"
	r, err := NewRouter(discardLogger, WithAllowedOrigin(dashboard))
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name        string
		method      string
		origin      string
		preflight   bool
		wantStatus  int
		wantAllowed bool
	}{
		{"the dashboard", http.MethodGet, dashboard, false, http.StatusOK, true},
		{"the dashboard's preflight", http.MethodOptions, dashboard, true, http.StatusNoContent, true},
		{"another site", http.MethodGet, "https://example.com", false, http.StatusOK, false},
		{"another site's preflight", http.MethodOptions, "https://example.com", true, http.StatusMethodNotAllowed, false},
		{"no origin", http.MethodGet, "", false, http.StatusOK, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(tt.method, "/health", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			if tt.preflight {
				req.Header.Set("Access-Control-Request-Method", http.MethodPost)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status %d, want %d", w.Code, tt.wantStatus)
			}
			allowed := w.Header().Get("Access-Control-Allow-Origin") == tt.origin &&
				w.Header().Get("Access-Control-Allow-Credentials") == "true"
			if allowed != tt.wantAllowed {
				t.Errorf("allowed = %v, want %v (headers %v)", allowed, tt.wantAllowed, w.Header())
			}
			if got := w.Header().Get("X-Content-Type-Options"); got != "nosniff" {
				t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
			}
		})
	}
}
