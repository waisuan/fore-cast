package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	appctx "github.com/waisuan/alfred/internal/context"
	"github.com/waisuan/alfred/internal/credentials"
	"github.com/waisuan/alfred/internal/crypto"
	"github.com/waisuan/alfred/internal/deps"
	"github.com/waisuan/alfred/internal/middlewares"
	"github.com/waisuan/alfred/internal/session"
	migrations "github.com/waisuan/alfred/migrations"
)

const testEncryptionKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

type AuthHandlerSuite struct {
	suite.Suite
	ctrl      *gomock.Controller
	mockCreds *credentials.MockService
	store     *session.Store
	handler   *AuthHandler

	container *postgres.PostgresContainer
	conn      *sql.DB
	ctx       context.Context
}

func (s *AuthHandlerSuite) SetupSuite() {
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

	s.store = session.NewStore(s.conn, 24*time.Hour)
}

func (s *AuthHandlerSuite) TearDownSuite() {
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

func (s *AuthHandlerSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.mockCreds = credentials.NewMockService(s.ctrl)
	s.handler = &AuthHandler{Credentials: s.mockCreds, Store: s.store, EncryptionKey: testEncryptionKey}
}

func (s *AuthHandlerSuite) TearDownTest() {
	_, _ = s.conn.Exec("DELETE FROM user_sessions")
	s.ctrl.Finish()
}

func (s *AuthHandlerSuite) TestLogin_Success() {
	passwordEnc, err := crypto.Encrypt("secret", testEncryptionKey)
	s.Require().NoError(err)
	s.mockCreds.EXPECT().
		Get("alice").
		Return(&credentials.Credential{UserName: "alice", PasswordEnc: passwordEnc}, nil)

	body, _ := json.Marshal(LoginRequest{Username: "alice", Password: "secret"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	s.handler.Login(rec, req)

	s.Require().Equal(http.StatusOK, rec.Code)
	s.Assert().Equal("application/json", rec.Header().Get("Content-Type"))
	cookies := rec.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == middlewares.SessionCookieName() {
			sessionCookie = c
			break
		}
	}
	s.Require().NotNil(sessionCookie)
	s.Assert().NotEmpty(sessionCookie.Value)
	var resp LoginResponse
	s.Require().NoError(json.NewDecoder(rec.Body).Decode(&resp))
	s.Assert().Equal("alice", resp.User.Username)
}

func (s *AuthHandlerSuite) TestLogin_MethodNotAllowed() {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/login", nil)
	rec := httptest.NewRecorder()
	s.handler.Login(rec, req)
	s.Assert().Equal(http.StatusMethodNotAllowed, rec.Code)
}

func (s *AuthHandlerSuite) TestLogin_InvalidBody() {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handler.Login(rec, req)
	s.Assert().Equal(http.StatusBadRequest, rec.Code)
}

func (s *AuthHandlerSuite) TestLogin_UsernameAndPasswordRequired() {
	body, _ := json.Marshal(LoginRequest{Username: "", Password: "x"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handler.Login(rec, req)
	s.Assert().Equal(http.StatusBadRequest, rec.Code)
}

func (s *AuthHandlerSuite) TestLogin_UserNotRegistered() {
	s.mockCreds.EXPECT().
		Get("unknown").
		Return(nil, nil)

	body, _ := json.Marshal(LoginRequest{Username: "unknown", Password: "any"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handler.Login(rec, req)
	s.Assert().Equal(http.StatusUnauthorized, rec.Code)
}

func (s *AuthHandlerSuite) TestLogin_InvalidCredentials() {
	passwordEnc, err := crypto.Encrypt("correct", testEncryptionKey)
	s.Require().NoError(err)
	s.mockCreds.EXPECT().
		Get("alice").
		Return(&credentials.Credential{UserName: "alice", PasswordEnc: passwordEnc}, nil)

	body, _ := json.Marshal(LoginRequest{Username: "alice", Password: "wrong"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handler.Login(rec, req)
	s.Assert().Equal(http.StatusUnauthorized, rec.Code)
}

func (s *AuthHandlerSuite) TestLogout() {
	sid, err := s.store.Create("user")
	s.Require().NoError(err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: middlewares.SessionCookieName(), Value: sid})
	rec := httptest.NewRecorder()
	s.handler.Logout(rec, req)

	s.Assert().Equal(http.StatusOK, rec.Code)
	cookies := rec.Result().Cookies()
	var cleared bool
	for _, c := range cookies {
		if c.Name == middlewares.SessionCookieName() && c.Value == "" {
			cleared = true
			break
		}
	}
	s.Assert().True(cleared, "expected session cookie to be cleared")
}

func (s *AuthHandlerSuite) TestMe_Unauthorized() {
	s.handler = &AuthHandler{}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	rec := httptest.NewRecorder()
	s.handler.Me(rec, req)
	s.Assert().Equal(http.StatusUnauthorized, rec.Code)
}

func (s *AuthHandlerSuite) TestMe_Success() {
	s.handler = &AuthHandler{}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req = req.WithContext(appctx.WithUser(req.Context(), &appctx.User{UserName: "bob", APIToken: "t"}))
	rec := httptest.NewRecorder()
	s.handler.Me(rec, req)
	s.Require().Equal(http.StatusOK, rec.Code)
	var resp LoginResponse
	s.Require().NoError(json.NewDecoder(rec.Body).Decode(&resp))
	s.Assert().Equal("bob", resp.User.Username)
}

func TestAuthHandlerSuite(t *testing.T) {
	suite.Run(t, new(AuthHandlerSuite))
}
