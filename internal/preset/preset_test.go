package preset_test

import (
	"context"
	"database/sql"
	"log"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/waisuan/alfred/internal/deps"
	"github.com/waisuan/alfred/internal/preset"
	migrations "github.com/waisuan/alfred/migrations"
)

type ServiceSuite struct {
	suite.Suite
	container *postgres.PostgresContainer
	conn      *sql.DB
	svc       preset.Service
	ctx       context.Context
}

func (s *ServiceSuite) SetupSuite() {
	s.ctx = context.Background()

	ctr, err := postgres.Run(s.ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("forecasttest"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		postgres.BasicWaitStrategies(),
	)
	s.Require().NoError(err)
	s.container = ctr

	connStr, err := ctr.ConnectionString(s.ctx, "sslmode=disable")
	s.Require().NoError(err)

	cfg := &deps.Config{DatabaseURL: connStr}
	conn, err := deps.NewPostgresClient(cfg, migrations.FS)
	s.Require().NoError(err)

	s.conn = conn
	s.svc = preset.NewService(conn)
}

func (s *ServiceSuite) TearDownSuite() {
	if s.conn != nil {
		_ = s.conn.Close()
	}
	if s.container != nil {
		if err := testcontainers.TerminateContainer(s.container); err != nil {
			log.Printf("failed to terminate container: %v", err)
		}
	}
}

func (s *ServiceSuite) TearDownTest() {
	_, _ = s.conn.Exec("DELETE FROM booking_presets")
}

func (s *ServiceSuite) TestUpsertPreset_Insert_And_GetPreset() {
	err := s.svc.UpsertPreset(preset.Preset{
		UserName:      "alice",
		PasswordEnc:   "encrypted-pw",
		Course:        sql.NullString{String: "PLC", Valid: true},
		Cutoff:        "8:15",
		RetryInterval: "5s",
		Timeout:       "10m",
		NtfyTopic:     sql.NullString{String: "my-topic", Valid: true},
		Enabled:       true,
	})
	s.Require().NoError(err)

	p, err := s.svc.GetPreset("alice")
	s.Require().NoError(err)
	s.Require().NotNil(p)
	s.Assert().Equal("alice", p.UserName)
	s.Assert().Equal("encrypted-pw", p.PasswordEnc)
	s.Assert().Equal("PLC", p.Course.String)
	s.Assert().True(p.Course.Valid)
	s.Assert().Equal("8:15", p.Cutoff)
	s.Assert().Equal("5s", p.RetryInterval)
	s.Assert().Equal("10m", p.Timeout)
	s.Assert().Equal("my-topic", p.NtfyTopic.String)
	s.Assert().True(p.Enabled)
	s.Assert().False(p.UpdatedAt.IsZero())
}

func (s *ServiceSuite) TestUpsertPreset_Update() {
	err := s.svc.UpsertPreset(preset.Preset{
		UserName:      "alice",
		PasswordEnc:   "v1",
		Cutoff:        "8:15",
		RetryInterval: "5s",
		Timeout:       "10m",
		Enabled:       false,
	})
	s.Require().NoError(err)

	err = s.svc.UpsertPreset(preset.Preset{
		UserName:      "alice",
		PasswordEnc:   "v2",
		Course:        sql.NullString{String: "BRC", Valid: true},
		Cutoff:        "7:30",
		RetryInterval: "3s",
		Timeout:       "5m",
		Enabled:       true,
	})
	s.Require().NoError(err)

	p, err := s.svc.GetPreset("alice")
	s.Require().NoError(err)
	s.Require().NotNil(p)
	s.Assert().Equal("v2", p.PasswordEnc)
	s.Assert().Equal("BRC", p.Course.String)
	s.Assert().Equal("7:30", p.Cutoff)
	s.Assert().Equal("3s", p.RetryInterval)
	s.Assert().Equal("5m", p.Timeout)
	s.Assert().True(p.Enabled)
}

func (s *ServiceSuite) TestGetPreset_NotFound() {
	p, err := s.svc.GetPreset("nobody")
	s.Require().NoError(err)
	s.Assert().Nil(p)
}

func (s *ServiceSuite) TestGetEnabledPresets() {
	err := s.svc.UpsertPreset(preset.Preset{
		UserName: "alice", PasswordEnc: "enc1", Cutoff: "8:15",
		RetryInterval: "5s", Timeout: "10m", Enabled: true,
	})
	s.Require().NoError(err)

	err = s.svc.UpsertPreset(preset.Preset{
		UserName: "bob", PasswordEnc: "enc2", Cutoff: "7:30",
		RetryInterval: "3s", Timeout: "5m", Enabled: false,
	})
	s.Require().NoError(err)

	err = s.svc.UpsertPreset(preset.Preset{
		UserName: "carol", PasswordEnc: "enc3", Cutoff: "8:00",
		RetryInterval: "5s", Timeout: "10m", Enabled: true,
	})
	s.Require().NoError(err)

	presets, err := s.svc.GetEnabledPresets()
	s.Require().NoError(err)
	s.Assert().Len(presets, 2)
	s.Assert().Equal("alice", presets[0].UserName)
	s.Assert().Equal("carol", presets[1].UserName)
}

func (s *ServiceSuite) TestGetEnabledPresets_Empty() {
	presets, err := s.svc.GetEnabledPresets()
	s.Require().NoError(err)
	s.Assert().Empty(presets)
}

// TestUpdatePresetRunStatus verifies the run status update method.
func TestUpdatePresetRunStatus(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	ctr, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("statustest"),
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

	svc := preset.NewService(conn)

	err = svc.UpsertPreset(preset.Preset{
		UserName: "alice", PasswordEnc: "enc", Cutoff: "8:15",
		RetryInterval: "1s", Timeout: "10m", Enabled: true,
	})
	require.NoError(t, err)

	err = svc.UpdatePresetRunStatus("alice", preset.RunStatusRunning, "starting")
	require.NoError(t, err)

	p, err := svc.GetPreset("alice")
	require.NoError(t, err)
	assert.Equal(t, string(preset.RunStatusRunning), p.LastRunStatus)
	assert.Equal(t, "starting", p.LastRunMessage)
	assert.True(t, p.LastRunAt.Valid)
	assert.WithinDuration(t, time.Now(), p.LastRunAt.Time, 5*time.Second)
}

func TestServiceSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(ServiceSuite))
}
