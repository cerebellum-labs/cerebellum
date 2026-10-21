package health

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	m.Run()
}

func TestHealth(t *testing.T) {
	r := gin.New()
	Register(r)

	tests := []struct {
		method   string
		wantBody string
	}{
		{http.MethodGet, `{"status":"ok"}`},
		{http.MethodHead, ""},
	}
	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(tt.method, Path, nil))

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
			}
			if got := w.Header().Get("Cache-Control"); got != "no-store" {
				t.Errorf("Cache-Control = %q, want %q", got, "no-store")
			}
			if tt.method == http.MethodGet {
				if got := w.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
					t.Errorf("Content-Type = %q", got)
				}
				if got := w.Body.String(); got != tt.wantBody {
					t.Errorf("body = %q, want %q", got, tt.wantBody)
				}
			}
		})
	}
}
