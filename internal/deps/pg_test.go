package deps_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/waisuan/alfred/internal/deps"
	migrations "github.com/waisuan/alfred/migrations"
)

func TestNewPostgresClient_RunsMigrations(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	ctr, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("migratetest"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		postgres.BasicWaitStrategies(),
	)
	require.NoError(t, err)
	defer func() {
		_ = testcontainers.TerminateContainer(ctr)
	}()

	connStr, err := ctr.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	cfg := &deps.Config{DatabaseURL: connStr}
	conn, err := deps.NewPostgresClient(cfg, migrations.FS)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	var count int
	err = conn.QueryRow("SELECT COUNT(*) FROM booking_attempts").Scan(&count)
	assert.NoError(t, err)

	err = conn.QueryRow("SELECT COUNT(*) FROM booking_presets").Scan(&count)
	assert.NoError(t, err)
}
