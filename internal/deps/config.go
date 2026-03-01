package deps

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/caarlos0/env/v9"
	"github.com/joho/godotenv"
)

// Config holds all configuration settings for the application.
type Config struct {
	Env string `env:"APP_ENV" envDefault:"development"`

	// Server
	Port         string        `env:"PORT" envDefault:"8080"`
	ReadTimeout  time.Duration `env:"HTTP_READ_TIMEOUT" envDefault:"15s"`
	WriteTimeout time.Duration `env:"HTTP_WRITE_TIMEOUT" envDefault:"15s"`
	IdleTimeout  time.Duration `env:"HTTP_IDLE_TIMEOUT" envDefault:"60s"`

	// Session
	SessionSecret string        `env:"SESSION_SECRET" envDefault:"change-me-in-production"`
	SessionTTL    time.Duration `env:"SESSION_TTL" envDefault:"24h"`

	// Postgres
	DatabaseURL string `env:"DATABASE_URL"`

	// Encryption (for stored credentials)
	EncryptionKey string `env:"ENCRYPTION_KEY"`
}

// LoadConfig loads configuration from environment variables and optional .env files.
func LoadConfig() (*Config, error) {
	appEnv := os.Getenv("APP_ENV")

	if appEnv != "" {
		file := dir(".env." + appEnv)
		if err := godotenv.Load(file); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("error loading %s: %w", file, err)
		}
	}

	cfg := Config{}
	if err := env.Parse(&cfg); err != nil {
		return nil, fmt.Errorf("error parsing config: %w", err)
	}

	return &cfg, nil
}

// dir walks up from cwd to find go.mod, then returns the absolute path
// of the given file relative to that root.
func dir(envFile string) string {
	currentDir, err := os.Getwd()
	if err != nil {
		log.Printf("deps: could not get cwd: %v", err)
		return envFile
	}
	for {
		if _, err := os.Stat(filepath.Join(currentDir, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(currentDir)
		if parent == currentDir {
			return envFile
		}
		currentDir = parent
	}
	return filepath.Join(currentDir, envFile)
}
