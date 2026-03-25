package deps

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/caarlos0/env/v9"
	"github.com/joho/godotenv"
	"github.com/waisuan/alfred/internal/logger"
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
	SessionTTL time.Duration `env:"SESSION_TTL" envDefault:"24h"`

	// Postgres
	DatabaseURL string `env:"DATABASE_URL"`

	// Encryption (for stored credentials)
	EncryptionKey string `env:"ENCRYPTION_KEY"`

	// Admin (for admin-only registration)
	AdminUser     string `env:"ADMIN_USER"`
	AdminPassword string `env:"ADMIN_PASSWORD"`

	// HTTP client timeouts (for outbound calls)
	BookerHTTPTimeout time.Duration `env:"BOOKER_HTTP_TIMEOUT" envDefault:"30s"`
	NotifyHTTPTimeout time.Duration `env:"NOTIFY_HTTP_TIMEOUT" envDefault:"10s"`
	NtfyBaseURL       string        `env:"NTFY_BASE_URL" envDefault:"https://ntfy.sh"`

	// Booker HTTP transport connection pool (Go's default MaxIdleConnsPerHost is 2; higher values reuse more)
	BookerMaxIdleConns        int           `env:"BOOKER_MAX_IDLE_CONNS" envDefault:"100"`
	BookerMaxIdleConnsPerHost int           `env:"BOOKER_MAX_IDLE_CONNS_PER_HOST" envDefault:"30"`
	BookerIdleConnTimeout     time.Duration `env:"BOOKER_IDLE_CONN_TIMEOUT" envDefault:"90s"`

	// Scheduler
	MaxConcurrentPresets int    `env:"MAX_CONCURRENT_PRESETS" envDefault:"5"`
	SchedulerTxnDate     string `env:"SCHEDULER_TXN_DATE"` // override target date (YYYY/MM/DD); empty = 1 week ahead

	// Pre-booking idle (scheduler only, not dry-run): in SchedulerTimezone, if local hour >= SchedulerBookingWaitMinHourMy
	// and time is before SchedulerBookingWaitHourMy:Minute, sleep until that instant before any Booker calls.
	SchedulerTimezone             string `env:"SCHEDULER_TIMEZONE" envDefault:"Asia/Kuala_Lumpur"`
	SchedulerBookingWaitHourMy    int    `env:"SCHEDULER_BOOKING_WAIT_HOUR_MY" envDefault:"21"`
	SchedulerBookingWaitMinuteMy  int    `env:"SCHEDULER_BOOKING_WAIT_MINUTE_MY" envDefault:"59"`
	SchedulerBookingWaitMinHourMy int    `env:"SCHEDULER_BOOKING_WAIT_MIN_HOUR_MY" envDefault:"21"` // only wait when local hour is >= this (avoids idling until evening on morning runs)

	// Dry-run (scheduler only): mock Booker API, no real HTTP calls.
	// BOOKER_DRY_RUN_SCENARIO: success | timeout | empty (default: timeout)
	// BOOKER_DRY_RUN_TIMEOUT: cap preset timeout for testing (e.g. 30s)
	BookerDryRun         bool          `env:"BOOKER_DRY_RUN" envDefault:"false"`
	BookerDryRunScenario string        `env:"BOOKER_DRY_RUN_SCENARIO" envDefault:"timeout"`
	BookerDryRunTimeout  time.Duration `env:"BOOKER_DRY_RUN_TIMEOUT" envDefault:"0"`
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
		logger.Warn("failed to get cwd", logger.Err(err))
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
