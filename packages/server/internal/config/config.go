// Package config reads the settings shared by the server and the Job from
// the environment.
package config

import "errors"

const defaultPort = "8080"

// Config holds the settings every entry point needs.
type Config struct {
	// Port the HTTP server listens on.
	Port string
	// DatabaseURL is a PostgreSQL connection string; required.
	DatabaseURL string
}

// Load reads the settings through getenv, which is os.Getenv outside tests.
func Load(getenv func(string) string) (Config, error) {
	cfg := Config{
		Port:        getenv("PORT"),
		DatabaseURL: getenv("DATABASE_URL"),
	}
	if cfg.Port == "" {
		cfg.Port = defaultPort
	}
	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is not set")
	}
	return cfg, nil
}
