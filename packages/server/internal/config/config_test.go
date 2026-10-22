package config

import (
	"maps"
	"reflect"
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	t.Parallel()

	secret := strings.Repeat("s", minSessionSecretLen)
	required := map[string]string{
		"DATABASE_URL":   "postgres://db",
		"ADMIN_PASSWORD": "pw",
		"SESSION_SECRET": secret,
	}
	with := func(extra map[string]string) map[string]string {
		env := maps.Clone(required)
		maps.Copy(env, extra)
		return env
	}

	tests := []struct {
		name    string
		env     map[string]string
		want    Config
		wantErr string // substring of the error; empty for success
	}{
		{
			name: "defaults",
			env:  required,
			want: Config{
				Port: "8080", DatabaseURL: "postgres://db", AdminPassword: "pw", SessionSecret: []byte(secret),
				ModelBaseURL: "https://api.tokenfactory.nebius.com/v1/", ModelID: "nvidia/Nemotron-3.5-Super-VL",
				ModelPriceInputPerM: 0.30, ModelPriceOutputPerM: 0.90,
			},
		},
		{
			name: "another model",
			env: with(map[string]string{
				"NEBIUS_API_KEY": "key", "MODEL_ID": "other", "NEBIUS_BASE_URL": "https://example.com/v1",
				"MODEL_PRICE_INPUT_PER_M": "0", "MODEL_PRICE_OUTPUT_PER_M": "1.5",
			}),
			want: Config{
				Port: "8080", DatabaseURL: "postgres://db", AdminPassword: "pw", SessionSecret: []byte(secret),
				ModelBaseURL: "https://example.com/v1", ModelAPIKey: "key", ModelID: "other",
				ModelPriceInputPerM: 0, ModelPriceOutputPerM: 1.5,
			},
		},
		{name: "negative price", env: with(map[string]string{"MODEL_PRICE_INPUT_PER_M": "-1"}), wantErr: "MODEL_PRICE_INPUT_PER_M"},
		{name: "price not a number", env: with(map[string]string{"MODEL_PRICE_OUTPUT_PER_M": "cheap"}), wantErr: "MODEL_PRICE_OUTPUT_PER_M"},
		{name: "model URL without a scheme", env: with(map[string]string{"NEBIUS_BASE_URL": "api.example.com"}), wantErr: "NEBIUS_BASE_URL"},
		{
			name: "everything set",
			env:  with(map[string]string{"PORT": "9000", "DASHBOARD_ORIGIN": "https://demo.cerebellumlabs.dev"}),
			want: Config{
				Port: "9000", DatabaseURL: "postgres://db", AdminPassword: "pw", SessionSecret: []byte(secret),
				DashboardOrigin: "https://demo.cerebellumlabs.dev",
				ModelBaseURL:    "https://api.tokenfactory.nebius.com/v1/", ModelID: "nvidia/Nemotron-3.5-Super-VL",
				ModelPriceInputPerM: 0.30, ModelPriceOutputPerM: 0.90,
			},
		},
		{name: "nothing set", env: map[string]string{}, wantErr: "DATABASE_URL is not set"},
		{name: "no admin password", env: with(map[string]string{"ADMIN_PASSWORD": ""}), wantErr: "ADMIN_PASSWORD"},
		{name: "short session secret", env: with(map[string]string{"SESSION_SECRET": "short"}), wantErr: "SESSION_SECRET"},
		{name: "origin with a path", env: with(map[string]string{"DASHBOARD_ORIGIN": "https://demo.cerebellumlabs.dev/"}), wantErr: "DASHBOARD_ORIGIN"},
		{name: "origin without a scheme", env: with(map[string]string{"DASHBOARD_ORIGIN": "demo.cerebellumlabs.dev"}), wantErr: "DASHBOARD_ORIGIN"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := Load(func(key string) string { return tt.env[key] })
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want one mentioning %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}
