package main

import (
	"fmt"
	"log"
	"time"

	"github.com/waisuan/alfred/internal/deps"
	"github.com/waisuan/alfred/migrations"
)

const retentionDays = 30

func main() {
	if err := run(); err != nil {
		log.Fatalf("cleanup: %v", err)
	}
}

func run() error {
	d, err := deps.Initialise(migrations.FS)
	if err != nil {
		return fmt.Errorf("init deps: %w", err)
	}
	defer d.Shutdown()

	retention := time.Duration(retentionDays) * 24 * time.Hour
	deleted, err := d.History.PruneAttempts(retention)
	if err != nil {
		return fmt.Errorf("prune: %w", err)
	}

	log.Printf("pruned %d booking attempt(s) older than %d days", deleted, retentionDays)
	return nil
}
