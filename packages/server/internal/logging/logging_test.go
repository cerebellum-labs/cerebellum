package logging

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestNew(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		id   string
		want any // request_id in the record; nil when absent
	}{
		{"with a request id", "req-1", "req-1"},
		{"without a request id", "", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			// WithAttrs must keep the wrapper, or derived loggers lose the id.
			logger := New(&buf).With("service", "server")

			ctx := t.Context()
			if tt.id != "" {
				ctx = WithRequestID(ctx, tt.id)
			}
			logger.InfoContext(ctx, "hello")

			var rec map[string]any
			if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
				t.Fatalf("log line is not JSON: %q", buf.String())
			}
			if rec["request_id"] != tt.want {
				t.Errorf("request_id = %v, want %v", rec["request_id"], tt.want)
			}
			if rec["service"] != "server" {
				t.Errorf("service = %v, want server", rec["service"])
			}
		})
	}
}
