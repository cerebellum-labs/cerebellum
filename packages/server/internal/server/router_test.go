package server

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

func TestRouterErrors(t *testing.T) {
	t.Parallel()
	r, err := NewRouter(discardLogger)
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
	}

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
		wantBody   string
	}{
		{
			"unknown route", http.MethodGet, "/nope", http.StatusNotFound,
			`{"error":{"code":"not_found","message":"route not found"}}`,
		},
		{
			"wrong method", http.MethodPost, "/health", http.StatusMethodNotAllowed,
			`{"error":{"code":"method_not_allowed","message":"method not allowed"}}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(tt.method, tt.path, nil))

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", w.Code, tt.wantStatus)
			}
			if got := w.Body.String(); got != tt.wantBody {
				t.Errorf("body = %q, want %q", got, tt.wantBody)
			}
		})
	}
}
