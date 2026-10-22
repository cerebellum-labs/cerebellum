// Package config reads the server's settings from the environment.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
)

const (
	defaultPort = "8080"
	// The cloud model by default: Nemotron 3.5 Super VL on Nebius Token
	// Factory, at its list price in USD per million tokens.
	defaultModelBaseURL    = "https://api.tokenfactory.nebius.com/v1/"
	defaultModelID         = "nvidia/Nemotron-3.5-Super-VL"
	defaultPriceInputPerM  = 0.30
	defaultPriceOutputPerM = 0.90
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

	// ModelBaseURL is the OpenAI-compatible API of the cloud model.
	ModelBaseURL string
	// ModelAPIKey is its key. Without it the server runs, but decisions
	// that need the cloud model fail and nothing is reviewed.
	ModelAPIKey string
	// ModelID names the model in that API.
	ModelID string
	// The model's price in USD per million tokens, for the cost of a call.
	ModelPriceInputPerM  float64
	ModelPriceOutputPerM float64
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

	errs := cfg.loadModel(getenv)
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

// loadModel reads the cloud model's settings and returns what is wrong
// with them.
func (cfg *Config) loadModel(getenv func(string) string) []error {
	cfg.ModelBaseURL = or(getenv("NEBIUS_BASE_URL"), defaultModelBaseURL)
	cfg.ModelAPIKey = getenv("NEBIUS_API_KEY")
	cfg.ModelID = or(getenv("MODEL_ID"), defaultModelID)

	var errs []error
	var err error
	if cfg.ModelPriceInputPerM, err = price(getenv, "MODEL_PRICE_INPUT_PER_M", defaultPriceInputPerM); err != nil {
		errs = append(errs, err)
	}
	if cfg.ModelPriceOutputPerM, err = price(getenv, "MODEL_PRICE_OUTPUT_PER_M", defaultPriceOutputPerM); err != nil {
		errs = append(errs, err)
	}
	if u, err := url.Parse(cfg.ModelBaseURL); err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		errs = append(errs, errors.New("NEBIUS_BASE_URL must be an http(s) URL"))
	}
	return errs
}

func or(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

// price reads a price per million tokens: a number, 0 or more.
func price(getenv func(string) string, key string, fallback float64) (float64, error) {
	s := getenv(key)
	if s == "" {
		return fallback, nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v < 0 {
		return 0, fmt.Errorf("%s must be a number, 0 or more", key)
	}
	return v, nil
}
