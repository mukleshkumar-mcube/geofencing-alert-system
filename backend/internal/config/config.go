package config

import (
	"os"
	"path/filepath"
	"strconv"
)

// Config is the process configuration read from the environment.
type Config struct {
	Port         string
	DatabasePath string
	Seed         bool
	APIKey       string
	RatePerMin   int
}

// Load reads configuration. SEED_DATA defaults on so a fresh database
// includes the San Francisco sample. Set SEED_DATA=false to start empty.
// API_KEY empty disables authentication.
func Load() Config {
	return Config{
		Port:         env("PORT", "8080"),
		DatabasePath: env("DATABASE_PATH", filepath.Join("data", "geofence.db")),
		Seed:         env("SEED_DATA", "true") != "false",
		APIKey:       os.Getenv("API_KEY"),
		RatePerMin:   envInt("RATE_LIMIT_PER_MIN", 120),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
