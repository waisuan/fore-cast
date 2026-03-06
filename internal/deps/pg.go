package deps

import (
	"database/sql"
	"fmt"
	"io/fs"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/lib/pq"
)

// NewPostgresClient opens a Postgres connection, pings it, and runs
// pending migrations from the given filesystem. Both DATABASE_URL and
// migrationsFS are required. Returns the raw *sql.DB.
func NewPostgresClient(cfg *Config, migrationsFS fs.FS) (*sql.DB, error) {
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	conn, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("pg: open: %w", err)
	}
	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("pg: ping: %w", err)
	}

	if err := runMigrations(conn, migrationsFS); err != nil {
		return nil, fmt.Errorf("pg: migrate: %w", err)
	}

	return conn, nil
}

func runMigrations(conn *sql.DB, fsys fs.FS) error {
	src, err := iofs.New(fsys, ".")
	if err != nil {
		return fmt.Errorf("migration source: %w", err)
	}
	driver, err := postgres.WithInstance(conn, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("migration driver: %w", err)
	}
	m, err := migrate.NewWithInstance("iofs", src, "postgres", driver)
	if err != nil {
		return fmt.Errorf("new migrate: %w", err)
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}
	return nil
}
