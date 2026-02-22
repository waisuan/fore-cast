package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds server and auth configuration.
type Config struct {
	Port          string
	SessionSecret string
	SessionTTL    time.Duration
	ReadTimeout   time.Duration
	WriteTimeout  time.Duration
	IdleTimeout   time.Duration
}

// Load reads configuration from environment variables.
func Load() *Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	secret := os.Getenv("SESSION_SECRET")
	if secret == "" {
		secret = "change-me-in-production"
	}
	ttl := 24 * time.Hour
	if s := os.Getenv("SESSION_TTL_HOURS"); s != "" {
		if h, err := strconv.Atoi(s); err == nil && h > 0 {
			ttl = time.Duration(h) * time.Hour
		}
	}
	return &Config{
		Port:          port,
		SessionSecret: secret,
		SessionTTL:    ttl,
		ReadTimeout:   15 * time.Second,
		WriteTimeout:  15 * time.Second,
		IdleTimeout:   60 * time.Second,
	}
}
