package main

import "os"

const defaultPort = "8080"

// config is read from the environment.
type config struct {
	port string
}

func loadConfig() config {
	return config{
		port: envOr("PORT", defaultPort),
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
