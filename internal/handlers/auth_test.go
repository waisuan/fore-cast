package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/suite"
	"github.com/waisuan/alfred/internal/context"
	"github.com/waisuan/alfred/internal/credentials"
	"github.com/waisuan/alfred/internal/crypto"
	"github.com/waisuan/alfred/internal/middlewares"
	"github.com/waisuan/alfred/internal/session"
)

const testEncryptionKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

type AuthHandlerSuite struct {
	suite.Suite
	ctrl      *gomock.Controller
	mockCreds *credentials.MockService
	store     *session.Store
	handler   *AuthHandler
}

func (s *AuthHandlerSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.mockCreds = credentials.NewMockService(s.ctrl)
	s.store = session.NewStore(24 * time.Hour)
	s.handler = &AuthHandler{Credentials: s.mockCreds, Store: s.store, EncryptionKey: testEncryptionKey}
}

func (s *AuthHandlerSuite) TearDownTest() {
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
	s.handler = &AuthHandler{Credentials: s.mockCreds, Store: s.store, EncryptionKey: testEncryptionKey}

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
	req = req.WithContext(context.WithUser(req.Context(), &context.User{UserName: "bob", APIToken: "t"}))
	rec := httptest.NewRecorder()
	s.handler.Me(rec, req)
	s.Require().Equal(http.StatusOK, rec.Code)
	var resp LoginResponse
	s.Require().NoError(json.NewDecoder(rec.Body).Decode(&resp))
	s.Assert().Equal("bob", resp.User.Username)
}

func TestAuthHandlerSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(AuthHandlerSuite))
}
