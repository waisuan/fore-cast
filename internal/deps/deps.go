package deps

import (
	"database/sql"
	"io/fs"

	"github.com/waisuan/alfred/internal/booker"
	"github.com/waisuan/alfred/internal/history"
	"github.com/waisuan/alfred/internal/logger"
	"github.com/waisuan/alfred/internal/notify"
	"github.com/waisuan/alfred/internal/preset"
	"github.com/waisuan/alfred/internal/session"
)

// Dependencies is the top-level container for shared application resources.
type Dependencies struct {
	Config  *Config
	PG      *sql.DB
	Preset  preset.Service
	History history.Service
	Booker  booker.ClientInterface
	Notify  notify.Service
	Store   *session.Store
}

// Initialise loads configuration, opens a Postgres connection (with
// migrations), and creates the domain service layers. Both DATABASE_URL and
// a valid migrationsFS are required. The global logger is initialised here
// and can be used via logger.Info, logger.Warn, etc. throughout the application.
func Initialise(migrationsFS fs.FS) (*Dependencies, error) {
	logger.Init()

	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}

	pg, err := NewPostgresClient(cfg, migrationsFS)
	if err != nil {
		return nil, err
	}

	var bookerClient booker.ClientInterface
	if cfg.BookerDryRun {
		bookerClient = booker.NewDryRunClient(cfg.BookerDryRunScenario)
	} else {
		bookerClient = booker.NewClientWithOptions(booker.BaseURL, cfg.BookerHTTPTimeout)
	}

	return &Dependencies{
		Config:  cfg,
		PG:      pg,
		Preset:  preset.NewService(pg),
		History: history.NewService(pg),
		Booker:  bookerClient,
		Notify:  notify.NewService(cfg.NtfyBaseURL, cfg.NotifyHTTPTimeout),
		Store:   session.NewStore(cfg.SessionTTL),
	}, nil
}

// Shutdown releases resources held by Dependencies.
func (d *Dependencies) Shutdown() {
	if d.Store != nil {
		d.Store.Close()
	}
	if d.PG != nil {
		_ = d.PG.Close()
	}
	logger.Sync()
}
