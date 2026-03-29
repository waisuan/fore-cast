package user_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	appctx "github.com/waisuan/alfred/internal/context"
	"github.com/waisuan/alfred/internal/deps"
	"github.com/waisuan/alfred/internal/user"
	migrations "github.com/waisuan/alfred/migrations"
)

func TestDeleteUser_NotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	ctr, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("udeltest"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		postgres.BasicWaitStrategies(),
	)
	require.NoError(t, err)
	defer func() { _ = testcontainers.TerminateContainer(ctr) }()

	connStr, err := ctr.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	cfg := &deps.Config{DatabaseURL: connStr}
	conn, err := deps.NewPostgresClient(cfg, migrations.FS)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	err = user.DeleteUser(conn, "missing")
	assert.ErrorIs(t, err, user.ErrNotFound)
}

func TestDeleteUser_RemovesAll(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	ctr, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("udeltest2"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		postgres.BasicWaitStrategies(),
	)
	require.NoError(t, err)
	defer func() { _ = testcontainers.TerminateContainer(ctr) }()

	connStr, err := ctr.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	cfg := &deps.Config{DatabaseURL: connStr}
	conn, err := deps.NewPostgresClient(cfg, migrations.FS)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	_, err = conn.Exec(`INSERT INTO user_credentials (user_name, password_enc) VALUES ($1, $2)`, "bob", "enc")
	require.NoError(t, err)
	_, err = conn.Exec(`
		INSERT INTO booking_presets (user_name, cutoff, retry_interval, timeout, enabled)
		VALUES ($1, '8:15', '1s', '10m', false)`,
		"bob")
	require.NoError(t, err)
	_, err = conn.Exec(`INSERT INTO user_sessions (id, user_name, expires_at) VALUES ($1, $2, NOW() + interval '1 hour')`, "sess1", "bob")
	require.NoError(t, err)
	_, err = conn.Exec(`
		INSERT INTO booking_attempts (user_name, course_id, txn_date, status, message)
		VALUES ($1, 'c1', '2025-01-01', 'ok', 'msg')`,
		"bob")
	require.NoError(t, err)

	require.NoError(t, user.DeleteUser(conn, "bob"))

	var n int
	err = conn.QueryRow(`SELECT COUNT(*) FROM user_credentials WHERE user_name = $1`, "bob").Scan(&n)
	require.NoError(t, err)
	assert.Equal(t, 0, n)
	err = conn.QueryRow(`SELECT COUNT(*) FROM booking_presets WHERE user_name = $1`, "bob").Scan(&n)
	require.NoError(t, err)
	assert.Equal(t, 0, n)
	err = conn.QueryRow(`SELECT COUNT(*) FROM user_sessions WHERE user_name = $1`, "bob").Scan(&n)
	require.NoError(t, err)
	assert.Equal(t, 0, n)
	err = conn.QueryRow(`SELECT COUNT(*) FROM booking_attempts WHERE user_name = $1`, "bob").Scan(&n)
	require.NoError(t, err)
	assert.Equal(t, 0, n)
}

func TestDeleteUser_CredentialsWithoutPreset(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	ctr, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("udeltest3"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		postgres.BasicWaitStrategies(),
	)
	require.NoError(t, err)
	defer func() { _ = testcontainers.TerminateContainer(ctr) }()

	connStr, err := ctr.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	cfg := &deps.Config{DatabaseURL: connStr}
	conn, err := deps.NewPostgresClient(cfg, migrations.FS)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	_, err = conn.Exec(`INSERT INTO user_credentials (user_name, password_enc) VALUES ($1, $2)`, "onlycreds", "enc")
	require.NoError(t, err)

	require.NoError(t, user.DeleteUser(conn, "onlycreds"))

	var n int
	err = conn.QueryRow(`SELECT COUNT(*) FROM user_credentials WHERE user_name = $1`, "onlycreds").Scan(&n)
	require.NoError(t, err)
	assert.Equal(t, 0, n)
}

func TestList_OrderAndFields(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	ctr, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("ulist"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		postgres.BasicWaitStrategies(),
	)
	require.NoError(t, err)
	defer func() { _ = testcontainers.TerminateContainer(ctr) }()

	connStr, err := ctr.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	cfg := &deps.Config{DatabaseURL: connStr}
	conn, err := deps.NewPostgresClient(cfg, migrations.FS)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	_, err = conn.Exec(`INSERT INTO user_credentials (user_name, password_enc, role) VALUES ('zoe', 'e', 'NON_ADMIN'), ('amy', 'e', 'ADMIN')`)
	require.NoError(t, err)

	rows, err := user.List(conn)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, "amy", rows[0].UserName)
	assert.Equal(t, appctx.RoleAdmin, rows[0].Role)
	assert.Equal(t, "zoe", rows[1].UserName)
	assert.Equal(t, appctx.RoleNonAdmin, rows[1].Role)
	assert.False(t, rows[0].CreatedAt.IsZero())
}

func TestSetRole_And_Errors(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	ctr, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("urole"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		postgres.BasicWaitStrategies(),
	)
	require.NoError(t, err)
	defer func() { _ = testcontainers.TerminateContainer(ctr) }()

	connStr, err := ctr.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	cfg := &deps.Config{DatabaseURL: connStr}
	conn, err := deps.NewPostgresClient(cfg, migrations.FS)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	_, err = conn.Exec(`INSERT INTO user_credentials (user_name, password_enc, role) VALUES ('pat', 'e', 'NON_ADMIN')`)
	require.NoError(t, err)

	assert.ErrorIs(t, user.SetRole(conn, "pat", "SUPERUSER"), user.ErrInvalidRole)
	assert.ErrorIs(t, user.SetRole(conn, "nobody", appctx.RoleAdmin), user.ErrNotFound)

	require.NoError(t, user.SetRole(conn, "pat", appctx.RoleAdmin))
	var role string
	err = conn.QueryRow(`SELECT role FROM user_credentials WHERE user_name = 'pat'`).Scan(&role)
	require.NoError(t, err)
	assert.Equal(t, appctx.RoleAdmin, role)

	require.NoError(t, user.SetRole(conn, "pat", appctx.RoleNonAdmin))
	err = conn.QueryRow(`SELECT role FROM user_credentials WHERE user_name = 'pat'`).Scan(&role)
	require.NoError(t, err)
	assert.Equal(t, appctx.RoleNonAdmin, role)
}
