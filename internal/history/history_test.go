package history_test

import (
	"context"
	"database/sql"
	"log"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/waisuan/alfred/internal/deps"
	"github.com/waisuan/alfred/internal/history"
	migrations "github.com/waisuan/alfred/migrations"
)

type ServiceSuite struct {
	suite.Suite
	container *postgres.PostgresContainer
	conn      *sql.DB
	svc       history.Service
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
	s.svc = history.NewService(conn)
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
}

func (s *ServiceSuite) TestLogAttempt_And_GetAttempts() {
	err := s.svc.LogAttempt(history.Attempt{
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

	err = s.svc.LogAttempt(history.Attempt{
		UserName: "alice",
		CourseID: "BRC",
		TxnDate:  "2026/03/05",
		Status:   "failed",
		Message:  "no slots",
	})
	s.Require().NoError(err)

	// Different user — should not appear in alice's results
	err = s.svc.LogAttempt(history.Attempt{
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
		err := s.svc.LogAttempt(history.Attempt{
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
	err := s.svc.LogAttempt(history.Attempt{
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
	err = s.svc.LogAttempt(history.Attempt{
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

func (s *ServiceSuite) TestPruneAttempts_ShortRetention_UsesHours() {
	// Retention < 24h uses "1 hour" interval (0 days fallback)
	err := s.svc.LogAttempt(history.Attempt{
		UserName: "alice",
		CourseID: "PLC",
		TxnDate:  "2026/03/04",
		Status:   "success",
		Message:  "old",
	})
	s.Require().NoError(err)
	_, err = s.conn.Exec("UPDATE booking_attempts SET created_at = NOW() - INTERVAL '2 hours'")
	s.Require().NoError(err)

	err = s.svc.LogAttempt(history.Attempt{
		UserName: "alice",
		CourseID: "PLC",
		TxnDate:  "2026/03/04",
		Status:   "success",
		Message:  "recent",
	})
	s.Require().NoError(err)

	deleted, err := s.svc.PruneAttempts(1 * time.Hour)
	s.Require().NoError(err)
	s.Assert().Equal(int64(1), deleted)

	remaining, err := s.svc.GetAttempts("alice", 10)
	s.Require().NoError(err)
	s.Assert().Len(remaining, 1)
	s.Assert().Equal("recent", remaining[0].Message)
}

func TestServiceSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(ServiceSuite))
}
