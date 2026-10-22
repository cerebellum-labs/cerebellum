// Package config reads the server's settings from the environment.
package config

import (
	"errors"
	"fmt"
	"net/url"
)

const (
	defaultPort = "8080"
	// The session secret signs the admin cookie with HMAC-SHA256; shorter
	// than its 32-byte output it would be the weak link.
	minSessionSecretLen = 32
)

// Config holds the server's settings.
type Config struct {
	// Port the HTTP server listens on.
	Port string
	// DatabaseURL is a PostgreSQL connection string; required.
	DatabaseURL string
	// AdminPassword unlocks changes in the dashboard; required.
	AdminPassword string
	// SessionSecret signs the dashboard's admin cookie; required, at least
	// 32 bytes.
	SessionSecret []byte
	// DashboardOrigin is the dashboard's origin when it runs on a host of
	// its own, like https://demo.cerebellumlabs.dev: it may call the API
	// with the admin cookie and open the event stream. Empty when the
	// dashboard shares the API's origin.
	DashboardOrigin string
}

// Load reads the settings through getenv, which is os.Getenv outside tests.
func Load(getenv func(string) string) (Config, error) {
	cfg := Config{
		Port:            getenv("PORT"),
		DatabaseURL:     getenv("DATABASE_URL"),
		AdminPassword:   getenv("ADMIN_PASSWORD"),
		SessionSecret:   []byte(getenv("SESSION_SECRET")),
		DashboardOrigin: getenv("DASHBOARD_ORIGIN"),
	}
	if cfg.Port == "" {
		cfg.Port = defaultPort
	}

	var errs []error
	if cfg.DatabaseURL == "" {
		errs = append(errs, errors.New("DATABASE_URL is not set"))
	}
	if cfg.AdminPassword == "" {
		errs = append(errs, errors.New("ADMIN_PASSWORD is not set"))
	}
	if len(cfg.SessionSecret) < minSessionSecretLen {
		errs = append(errs, fmt.Errorf("SESSION_SECRET must be at least %d bytes", minSessionSecretLen))
	}
	if cfg.DashboardOrigin != "" && !isOrigin(cfg.DashboardOrigin) {
		errs = append(errs, errors.New("DASHBOARD_ORIGIN must be a scheme and host, like https://demo.cerebellumlabs.dev"))
	}
	if err := errors.Join(errs...); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// isOrigin reports whether s is an origin as browsers send it: scheme and
// host, with no path.
func isOrigin(s string) bool {
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" &&
		u.Path == "" && u.RawQuery == "" && u.Fragment == "" && u.User == nil
}
