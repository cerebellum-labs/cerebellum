package config

import (
	"maps"
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
			want: Config{Port: "8080", DatabaseURL: "postgres://db", AdminPassword: "pw", SessionSecret: []byte(secret)},
		},
		{
			name: "everything set",
			env:  with(map[string]string{"PORT": "9000", "DASHBOARD_ORIGIN": "https://demo.cerebellumlabs.dev"}),
			want: Config{
				Port: "9000", DatabaseURL: "postgres://db", AdminPassword: "pw", SessionSecret: []byte(secret),
				DashboardOrigin: "https://demo.cerebellumlabs.dev",
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
			if got.Port != tt.want.Port || got.DatabaseURL != tt.want.DatabaseURL || got.AdminPassword != tt.want.AdminPassword ||
				string(got.SessionSecret) != string(tt.want.SessionSecret) || got.DashboardOrigin != tt.want.DashboardOrigin {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}
