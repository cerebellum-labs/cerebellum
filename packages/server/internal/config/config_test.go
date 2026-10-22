package config

import "testing"

func TestLoad(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		env     map[string]string
		want    Config
		wantErr bool
	}{
		{
			name: "defaults",
			env:  map[string]string{"DATABASE_URL": "postgres://db"},
			want: Config{Port: "8080", DatabaseURL: "postgres://db"},
		},
		{
			name: "port from the environment",
			env:  map[string]string{"DATABASE_URL": "postgres://db", "PORT": "9000"},
			want: Config{Port: "9000", DatabaseURL: "postgres://db"},
		},
		{
			name:    "no database url",
			env:     map[string]string{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := Load(func(key string) string { return tt.env[key] })
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, want error %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}
