package session_test

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
	"github.com/waisuan/alfred/internal/session"
	migrations "github.com/waisuan/alfred/migrations"
)

type StoreSuite struct {
	suite.Suite
	container *postgres.PostgresContainer
	conn      *sql.DB
	ctx       context.Context
}

func (s *StoreSuite) SetupSuite() {
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
}

func (s *StoreSuite) TearDownSuite() {
	if s.conn != nil {
		_ = s.conn.Close()
	}
	if s.container != nil {
		if err := testcontainers.TerminateContainer(s.container); err != nil {
			log.Printf("failed to terminate container: %v", err)
		}
	}
}

func (s *StoreSuite) TearDownTest() {
	_, _ = s.conn.Exec("DELETE FROM user_sessions")
}

func (s *StoreSuite) TestCreateGetDelete() {
	store := session.NewStore(s.conn, 24*time.Hour)
	defer store.Close()

	sid, err := store.Create("user1")
	s.Require().NoError(err)
	s.Require().NotEmpty(sid)

	data := store.Get(sid)
	s.Require().NotNil(data)
	s.Assert().Equal("user1", data.UserName)

	store.Delete(sid)
	data = store.Get(sid)
	s.Assert().Nil(data)
}

func (s *StoreSuite) TestExpiry() {
	store := session.NewStore(s.conn, 10*time.Millisecond)
	defer store.Close()

	sid, err := store.Create("u")
	s.Require().NoError(err)

	time.Sleep(15 * time.Millisecond)
	data := store.Get(sid)
	s.Assert().Nil(data)
}

func TestStoreSuite(t *testing.T) {
	suite.Run(t, new(StoreSuite))
}
