package db_test

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

	"github.com/waisuan/alfred/internal/db"
	"github.com/waisuan/alfred/internal/deps"
	migrations "github.com/waisuan/alfred/migrations"
)

type ServiceSuite struct {
	suite.Suite
	container *postgres.PostgresContainer
	conn      *sql.DB
	svc       *db.Service
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
	s.svc = db.NewService(conn)
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
	_, _ = s.conn.Exec("DELETE FROM booking_attempts")
	_, _ = s.conn.Exec("DELETE FROM booking_presets")
}

// --- booking_attempts tests ---

func (s *ServiceSuite) TestLogAttempt_And_GetAttempts() {
	err := s.svc.LogAttempt(db.Attempt{
		UserName:  "alice",
		CourseID:  "PLC",
		TxnDate:   "2026/03/04",
		TeeTime:   sql.NullString{String: "07:00", Valid: true},
		TeeBox:    sql.NullString{String: "1", Valid: true},
		BookingID: sql.NullString{String: "B1", Valid: true},
		Status:    "success",
		Message:   "booked slot",
	})
	s.Require().NoError(err)

	err = s.svc.LogAttempt(db.Attempt{
		UserName: "alice",
		CourseID: "BRC",
		TxnDate:  "2026/03/05",
		Status:   "failed",
		Message:  "no slots",
	})
	s.Require().NoError(err)

	// Different user -- should not appear in alice's results
	err = s.svc.LogAttempt(db.Attempt{
		UserName: "bob",
		CourseID: "PLC",
		TxnDate:  "2026/03/04",
		Status:   "success",
		Message:  "booked",
	})
	s.Require().NoError(err)

	attempts, err := s.svc.GetAttempts("alice", 10)
	s.Require().NoError(err)
	s.Assert().Len(attempts, 2)

	// Most recent first
	s.Assert().Equal("BRC", attempts[0].CourseID)
	s.Assert().Equal("failed", attempts[0].Status)
	s.Assert().False(attempts[0].TeeTime.Valid)

	s.Assert().Equal("PLC", attempts[1].CourseID)
	s.Assert().Equal("success", attempts[1].Status)
	s.Assert().Equal("B1", attempts[1].BookingID.String)
}

func (s *ServiceSuite) TestGetAttempts_Limit() {
	for i := 0; i < 5; i++ {
		err := s.svc.LogAttempt(db.Attempt{
			UserName: "alice",
			CourseID: "PLC",
			TxnDate:  "2026/03/04",
			Status:   "success",
			Message:  "booked",
		})
		s.Require().NoError(err)
	}

	attempts, err := s.svc.GetAttempts("alice", 3)
	s.Require().NoError(err)
	s.Assert().Len(attempts, 3)
}

func (s *ServiceSuite) TestGetAttempts_Empty() {
	attempts, err := s.svc.GetAttempts("nobody", 10)
	s.Require().NoError(err)
	s.Assert().Empty(attempts)
}

func (s *ServiceSuite) TestPruneAttempts() {
	err := s.svc.LogAttempt(db.Attempt{
		UserName: "alice",
		CourseID: "PLC",
		TxnDate:  "2026/03/04",
		Status:   "success",
		Message:  "booked",
	})
	s.Require().NoError(err)

	// Backdate the row so it's "old"
	_, err = s.conn.Exec("UPDATE booking_attempts SET created_at = NOW() - INTERVAL '60 days'")
	s.Require().NoError(err)

	// Add a recent row
	err = s.svc.LogAttempt(db.Attempt{
		UserName: "alice",
		CourseID: "PLC",
		TxnDate:  "2026/03/04",
		Status:   "success",
		Message:  "booked again",
	})
	s.Require().NoError(err)

	deleted, err := s.svc.PruneAttempts(30 * 24 * time.Hour)
	s.Require().NoError(err)
	s.Assert().Equal(int64(1), deleted)

	remaining, err := s.svc.GetAttempts("alice", 10)
	s.Require().NoError(err)
	s.Assert().Len(remaining, 1)
	s.Assert().Equal("booked again", remaining[0].Message)
}

// --- booking_presets tests ---

func (s *ServiceSuite) TestUpsertPreset_Insert_And_GetPreset() {
	err := s.svc.UpsertPreset(db.Preset{
		UserName:      "alice",
		PasswordEnc:   "encrypted-pw",
		Course:        sql.NullString{String: "PLC", Valid: true},
		Cutoff:        "8:15",
		RetryInterval: 5,
		Timeout:       "10m",
		NtfyTopic:     sql.NullString{String: "my-topic", Valid: true},
		Enabled:       true,
	})
	s.Require().NoError(err)

	preset, err := s.svc.GetPreset("alice")
	s.Require().NoError(err)
	s.Require().NotNil(preset)
	s.Assert().Equal("alice", preset.UserName)
	s.Assert().Equal("encrypted-pw", preset.PasswordEnc)
	s.Assert().Equal("PLC", preset.Course.String)
	s.Assert().True(preset.Course.Valid)
	s.Assert().Equal("8:15", preset.Cutoff)
	s.Assert().Equal(5, preset.RetryInterval)
	s.Assert().Equal("10m", preset.Timeout)
	s.Assert().Equal("my-topic", preset.NtfyTopic.String)
	s.Assert().True(preset.Enabled)
	s.Assert().False(preset.UpdatedAt.IsZero())
}

func (s *ServiceSuite) TestUpsertPreset_Update() {
	err := s.svc.UpsertPreset(db.Preset{
		UserName:      "alice",
		PasswordEnc:   "v1",
		Cutoff:        "8:15",
		RetryInterval: 5,
		Timeout:       "10m",
		Enabled:       false,
	})
	s.Require().NoError(err)

	// Update the same user
	err = s.svc.UpsertPreset(db.Preset{
		UserName:      "alice",
		PasswordEnc:   "v2",
		Course:        sql.NullString{String: "BRC", Valid: true},
		Cutoff:        "7:30",
		RetryInterval: 3,
		Timeout:       "5m",
		Enabled:       true,
	})
	s.Require().NoError(err)

	preset, err := s.svc.GetPreset("alice")
	s.Require().NoError(err)
	s.Require().NotNil(preset)
	s.Assert().Equal("v2", preset.PasswordEnc)
	s.Assert().Equal("BRC", preset.Course.String)
	s.Assert().Equal("7:30", preset.Cutoff)
	s.Assert().Equal(3, preset.RetryInterval)
	s.Assert().Equal("5m", preset.Timeout)
	s.Assert().True(preset.Enabled)
}

func (s *ServiceSuite) TestGetPreset_NotFound() {
	preset, err := s.svc.GetPreset("nobody")
	s.Require().NoError(err)
	s.Assert().Nil(preset)
}

func (s *ServiceSuite) TestGetEnabledPresets() {
	err := s.svc.UpsertPreset(db.Preset{
		UserName: "alice", PasswordEnc: "enc1", Cutoff: "8:15",
		RetryInterval: 5, Timeout: "10m", Enabled: true,
	})
	s.Require().NoError(err)

	err = s.svc.UpsertPreset(db.Preset{
		UserName: "bob", PasswordEnc: "enc2", Cutoff: "7:30",
		RetryInterval: 3, Timeout: "5m", Enabled: false,
	})
	s.Require().NoError(err)

	err = s.svc.UpsertPreset(db.Preset{
		UserName: "carol", PasswordEnc: "enc3", Cutoff: "8:00",
		RetryInterval: 5, Timeout: "10m", Enabled: true,
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

// --- NewPostgresClient + migrations test ---

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

	// Verify tables exist by running a simple query on each
	var count int
	err = conn.QueryRow("SELECT COUNT(*) FROM booking_attempts").Scan(&count)
	assert.NoError(t, err)

	err = conn.QueryRow("SELECT COUNT(*) FROM booking_presets").Scan(&count)
	assert.NoError(t, err)
}

func TestServiceSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(ServiceSuite))
}
