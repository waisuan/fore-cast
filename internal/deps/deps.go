package deps

import (
	"database/sql"
	"io/fs"

	"github.com/waisuan/alfred/internal/history"
	"github.com/waisuan/alfred/internal/preset"
)

// Dependencies is the top-level container for shared application resources.
type Dependencies struct {
	Config  *Config
	PG      *sql.DB
	Preset  preset.Service
	History history.Service
}

// Initialise loads configuration, opens a Postgres connection (with
// migrations), and creates the domain service layers. Both DATABASE_URL and
// a valid migrationsFS are required.
func Initialise(migrationsFS fs.FS) (*Dependencies, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}

	pg, err := NewPostgresClient(cfg, migrationsFS)
	if err != nil {
		return nil, err
	}

	return &Dependencies{
		Config:  cfg,
		PG:      pg,
		Preset:  preset.NewService(pg),
		History: history.NewService(pg),
	}, nil
}

// Shutdown releases resources held by Dependencies.
func (d *Dependencies) Shutdown() {
	if d.PG != nil {
		_ = d.PG.Close()
	}
}
