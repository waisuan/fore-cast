package main

import (
	"fmt"
	"time"

	"github.com/waisuan/alfred/internal/deps"
	"github.com/waisuan/alfred/internal/logger"
	"github.com/waisuan/alfred/migrations"
)

const retentionDays = 30

func main() {
	d, err := deps.Initialise(migrations.FS)
	if err != nil {
		logger.Fatal("init deps", logger.Err(err))
	}
	defer d.Shutdown()

	if err := run(d); err != nil {
		logger.Fatal("cleanup", logger.Err(err))
	}
}

func run(d *deps.Dependencies) error {
	retention := time.Duration(retentionDays) * 24 * time.Hour
	deleted, err := d.History.PruneAttempts(retention)
	if err != nil {
		return fmt.Errorf("prune: %w", err)
	}

	logger.Info("pruned booking attempts", logger.Int64("deleted", deleted), logger.Int("retention_days", retentionDays))
	return nil
}
