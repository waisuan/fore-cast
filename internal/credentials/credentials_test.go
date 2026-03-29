package credentials_test

import (
	"context"
	"database/sql"
	"log"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	appctx "github.com/waisuan/alfred/internal/context"
	"github.com/waisuan/alfred/internal/credentials"
	"github.com/waisuan/alfred/internal/deps"
	migrations "github.com/waisuan/alfred/migrations"
)

type ServiceSuite struct {
	suite.Suite
	container *postgres.PostgresContainer
	conn      *sql.DB
	svc       credentials.Service
	ctx       context.Context
}

func (s *ServiceSuite) SetupSuite() {
	s.ctx = context.Background()

	ctr, err := postgres.Run(s.ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("credstest"),
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
	s.svc = credentials.NewService(conn)
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
	_, _ = s.conn.Exec("DELETE FROM user_credentials")
}

func (s *ServiceSuite) TestUpsert_Insert_And_Get() {
	err := s.svc.Upsert("alice", "encrypted-pw", appctx.RoleNonAdmin)
	s.Require().NoError(err)

	c, err := s.svc.Get("alice")
	s.Require().NoError(err)
	s.Require().NotNil(c)
	s.Assert().Equal("alice", c.UserName)
	s.Assert().Equal("encrypted-pw", c.PasswordEnc)
	s.Assert().Equal(appctx.RoleNonAdmin, c.Role)
}

func (s *ServiceSuite) TestUpsert_Update() {
	err := s.svc.Upsert("bob", "v1", appctx.RoleAdmin)
	s.Require().NoError(err)

	err = s.svc.Upsert("bob", "v2", appctx.RoleNonAdmin)
	s.Require().NoError(err)

	c, err := s.svc.Get("bob")
	s.Require().NoError(err)
	s.Require().NotNil(c)
	s.Assert().Equal("v2", c.PasswordEnc)
	s.Assert().Equal(appctx.RoleAdmin, c.Role)
}

func (s *ServiceSuite) TestGet_NotFound() {
	c, err := s.svc.Get("nobody")
	s.Require().NoError(err)
	s.Assert().Nil(c)
}

func (s *ServiceSuite) TestGet_MultipleUsers() {
	err := s.svc.Upsert("user1", "pw1", appctx.RoleNonAdmin)
	s.Require().NoError(err)
	err = s.svc.Upsert("user2", "pw2", appctx.RoleNonAdmin)
	s.Require().NoError(err)

	c1, err := s.svc.Get("user1")
	s.Require().NoError(err)
	s.Require().NotNil(c1)
	s.Assert().Equal("pw1", c1.PasswordEnc)

	c2, err := s.svc.Get("user2")
	s.Require().NoError(err)
	s.Require().NotNil(c2)
	s.Assert().Equal("pw2", c2.PasswordEnc)
}

func TestServiceSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(ServiceSuite))
}
