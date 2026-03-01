package deps

import (
	"database/sql"
	"io/fs"

	"github.com/waisuan/alfred/internal/db"
)

// Dependencies is the top-level container for shared application resources.
type Dependencies struct {
	Config  *Config
	PG      *sql.DB
	Service db.ServiceInterface
}

// Initialise loads configuration, opens a Postgres connection (with
// migrations), and creates the db.Service layer. Both DATABASE_URL and
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
		Service: db.NewService(pg),
	}, nil
}

// Shutdown releases resources held by Dependencies.
func (d *Dependencies) Shutdown() {
	if d.PG != nil {
		_ = d.PG.Close()
	}
}
