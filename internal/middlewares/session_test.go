package middlewares_test

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/waisuan/alfred/internal/booker"
	appctx "github.com/waisuan/alfred/internal/context"
	"github.com/waisuan/alfred/internal/credentials"
	"github.com/waisuan/alfred/internal/crypto"
	"github.com/waisuan/alfred/internal/deps"
	"github.com/waisuan/alfred/internal/middlewares"
	"github.com/waisuan/alfred/internal/session"
	migrations "github.com/waisuan/alfred/migrations"
)

const tokenRefreshEncKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

type sessionAuthSuite struct {
	suite.Suite
	container *postgres.PostgresContainer
	conn      *sql.DB
	store     *session.Store
	ctx       context.Context
}

func (s *sessionAuthSuite) SetupSuite() {
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

	s.store = session.NewStore(s.conn, time.Hour)
}

func (s *sessionAuthSuite) TearDownSuite() {
	if s.store != nil {
		s.store.Close()
	}
	if s.conn != nil {
		_ = s.conn.Close()
	}
	if s.container != nil {
		if err := testcontainers.TerminateContainer(s.container); err != nil {
			log.Printf("failed to terminate container: %v", err)
		}
	}
}

func (s *sessionAuthSuite) TearDownTest() {
	_, _ = s.conn.Exec("DELETE FROM user_sessions")
}

func echoUser(w http.ResponseWriter, r *http.Request) {
	u := appctx.UserFrom(r.Context())
	if u == nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(u.UserName + ":" + u.APIToken))
}

func (s *sessionAuthSuite) TestSessionAuth_ValidSession() {
	sid, err := s.store.Create("alice")
	s.Require().NoError(err)
	handler := middlewares.SessionAuth(s.store)(http.HandlerFunc(echoUser))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: middlewares.SessionCookieName(), Value: sid})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	s.Assert().Equal(http.StatusOK, rec.Code)
	s.Assert().Equal("alice:", rec.Body.String())
}

func (s *sessionAuthSuite) TestSessionAuth_NoCookie() {
	handler := middlewares.SessionAuth(s.store)(http.HandlerFunc(echoUser))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	s.Assert().Equal(http.StatusUnauthorized, rec.Code)
}

func (s *sessionAuthSuite) TestSessionAuth_EmptyCookieValue() {
	handler := middlewares.SessionAuth(s.store)(http.HandlerFunc(echoUser))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: middlewares.SessionCookieName(), Value: ""})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	s.Assert().Equal(http.StatusUnauthorized, rec.Code)
}

func (s *sessionAuthSuite) TestSessionAuth_UnknownSessionID() {
	handler := middlewares.SessionAuth(s.store)(http.HandlerFunc(echoUser))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: middlewares.SessionCookieName(), Value: "nonexistent"})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	s.Assert().Equal(http.StatusUnauthorized, rec.Code)
}

func (s *sessionAuthSuite) TestSessionAuth_ExpiredSession() {
	shortStore := session.NewStore(s.conn, time.Millisecond)
	defer shortStore.Close()

	sid, err := shortStore.Create("bob")
	s.Require().NoError(err)

	time.Sleep(5 * time.Millisecond)

	handler := middlewares.SessionAuth(shortStore)(http.HandlerFunc(echoUser))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: middlewares.SessionCookieName(), Value: sid})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	s.Assert().Equal(http.StatusUnauthorized, rec.Code)
}

func (s *sessionAuthSuite) TestTokenRefresh_ObtainsTokenFromCredentials() {
	ctrl := gomock.NewController(s.T())
	mockBooker := booker.NewMockClientInterface(ctrl)
	mockBooker.EXPECT().
		Login("alice", "pw123").
		Return("fresh-token", nil)

	enc, err := crypto.Encrypt("pw123", tokenRefreshEncKey)
	s.Require().NoError(err)

	mockCreds := credentials.NewMockService(ctrl)
	mockCreds.EXPECT().
		Get("alice").
		Return(&credentials.Credential{UserName: "alice", PasswordEnc: enc}, nil)

	sid, err := s.store.Create("alice")
	s.Require().NoError(err)

	chain := middlewares.SessionAuth(s.store)(
		middlewares.TokenRefresh(mockBooker, mockCreds, tokenRefreshEncKey)(http.HandlerFunc(echoUser)),
	)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: middlewares.SessionCookieName(), Value: sid})
	rec := httptest.NewRecorder()
	chain.ServeHTTP(rec, req)

	s.Assert().Equal(http.StatusOK, rec.Code)
	s.Assert().Equal("alice:fresh-token", rec.Body.String())
}

func TestSessionAuthSuite(t *testing.T) {
	suite.Run(t, new(sessionAuthSuite))
}

func TestSessionCookieName(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "session", middlewares.SessionCookieName())
}
