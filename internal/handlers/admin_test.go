package handlers

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/suite"
	"github.com/waisuan/alfred/internal/booker"
	"github.com/waisuan/alfred/internal/credentials"
	"github.com/waisuan/alfred/internal/deps"
)

type AdminHandlerSuite struct {
	suite.Suite
	ctrl       *gomock.Controller
	mockBooker *booker.MockClientInterface
	mockCreds  *credentials.MockService
	config     *deps.Config
	handler    *AdminHandler
}

func (s *AdminHandlerSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.mockBooker = booker.NewMockClientInterface(s.ctrl)
	s.mockCreds = credentials.NewMockService(s.ctrl)
	s.config = &deps.Config{
		AdminUser:     "admin",
		AdminPassword: "adminpass",
		EncryptionKey: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
	}
	s.handler = &AdminHandler{
		Config:      s.config,
		Booker:      s.mockBooker,
		Credentials: s.mockCreds,
	}
}

func (s *AdminHandlerSuite) TearDownTest() {
	s.ctrl.Finish()
}

func basicAuth(user, pass string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(user+":"+pass))
}

func (s *AdminHandlerSuite) TestRegister_Success() {
	s.mockBooker.EXPECT().
		Login("newuser", "newpass").
		Return("token", nil)
	s.mockCreds.EXPECT().
		Upsert("newuser", gomock.Any()).
		Return(nil)

	body, _ := json.Marshal(RegisterRequest{Username: "newuser", Password: "newpass"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", basicAuth("admin", "adminpass"))
	rec := httptest.NewRecorder()

	s.handler.Register(rec, req)

	s.Require().Equal(http.StatusOK, rec.Code)
	s.Assert().Equal("application/json", rec.Header().Get("Content-Type"))
	var resp map[string]string
	s.Require().NoError(json.NewDecoder(rec.Body).Decode(&resp))
	s.Assert().Equal("registered", resp["status"])
	s.Assert().Equal("newuser", resp["username"])
}

func (s *AdminHandlerSuite) TestRegister_MethodNotAllowed() {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/register", nil)
	rec := httptest.NewRecorder()
	s.handler.Register(rec, req)
	s.Assert().Equal(http.StatusMethodNotAllowed, rec.Code)
}

func (s *AdminHandlerSuite) TestRegister_NoBasicAuth() {
	body, _ := json.Marshal(RegisterRequest{Username: "u", Password: "p"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	s.handler.Register(rec, req)

	s.Assert().Equal(http.StatusUnauthorized, rec.Code)
	s.Assert().Equal(`Basic realm="Admin"`, rec.Header().Get("WWW-Authenticate"))
}

func (s *AdminHandlerSuite) TestRegister_InvalidBasicAuth() {
	body, _ := json.Marshal(RegisterRequest{Username: "u", Password: "p"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", basicAuth("wrong", "wrong"))
	rec := httptest.NewRecorder()

	s.handler.Register(rec, req)

	s.Assert().Equal(http.StatusUnauthorized, rec.Code)
}

func (s *AdminHandlerSuite) TestRegister_NoAdminConfig() {
	s.handler.Config.AdminUser = ""
	s.handler.Config.AdminPassword = ""

	body, _ := json.Marshal(RegisterRequest{Username: "u", Password: "p"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", basicAuth("admin", "adminpass"))
	rec := httptest.NewRecorder()

	s.handler.Register(rec, req)

	s.Assert().Equal(http.StatusUnauthorized, rec.Code)
}

func (s *AdminHandlerSuite) TestRegister_InvalidBody() {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/register", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", basicAuth("admin", "adminpass"))
	rec := httptest.NewRecorder()

	s.handler.Register(rec, req)

	s.Assert().Equal(http.StatusBadRequest, rec.Code)
}

func (s *AdminHandlerSuite) TestRegister_UsernamePasswordRequired() {
	body, _ := json.Marshal(RegisterRequest{Username: "", Password: "p"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", basicAuth("admin", "adminpass"))
	rec := httptest.NewRecorder()

	s.handler.Register(rec, req)

	s.Assert().Equal(http.StatusBadRequest, rec.Code)
}

func (s *AdminHandlerSuite) TestRegister_InvalidUserCredentials() {
	s.mockBooker.EXPECT().
		Login("baduser", "badpass").
		Return("", http.ErrHandlerTimeout)

	body, _ := json.Marshal(RegisterRequest{Username: "baduser", Password: "badpass"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", basicAuth("admin", "adminpass"))
	rec := httptest.NewRecorder()

	s.handler.Register(rec, req)

	s.Assert().Equal(http.StatusUnauthorized, rec.Code)
}

func (s *AdminHandlerSuite) TestRegister_NoEncryptionKey() {
	s.handler.Config.EncryptionKey = ""

	body, _ := json.Marshal(RegisterRequest{Username: "u", Password: "p"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", basicAuth("admin", "adminpass"))
	rec := httptest.NewRecorder()

	s.handler.Register(rec, req)

	s.Assert().Equal(http.StatusInternalServerError, rec.Code)
}

func TestAdminHandlerSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(AdminHandlerSuite))
}
