package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/suite"

	"github.com/waisuan/alfred/internal/booker"
	appctx "github.com/waisuan/alfred/internal/context"
	"github.com/waisuan/alfred/internal/credentials"
	"github.com/waisuan/alfred/internal/deps"
	"github.com/waisuan/alfred/internal/preset"
)

type AdminHandlerSuite struct {
	suite.Suite
	ctrl       *gomock.Controller
	mockBooker *booker.MockClientInterface
	mockCreds  *credentials.MockService
	mockPreset *preset.MockService
	config     *deps.Config
	handler    *AdminHandler
}

func (s *AdminHandlerSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.mockBooker = booker.NewMockClientInterface(s.ctrl)
	s.mockCreds = credentials.NewMockService(s.ctrl)
	s.mockPreset = preset.NewMockService(s.ctrl)
	s.config = &deps.Config{
		EncryptionKey: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
	}
	s.handler = &AdminHandler{
		Config:      s.config,
		Booker:      s.mockBooker,
		Credentials: s.mockCreds,
		Preset:      s.mockPreset,
	}
}

func (s *AdminHandlerSuite) TearDownTest() {
	s.ctrl.Finish()
}

func withAdmin(ctx context.Context) context.Context {
	return appctx.WithUser(ctx, &appctx.User{UserName: "admin", Role: appctx.RoleAdmin})
}

func (s *AdminHandlerSuite) TestRegister_Success() {
	s.mockBooker.EXPECT().
		Login("newuser", "newpass").
		Return("token", nil)
	s.mockCreds.EXPECT().
		Upsert("newuser", gomock.Any(), appctx.RoleNonAdmin).
		Return(nil)

	body, _ := json.Marshal(RegisterRequest{Username: "newuser", Password: "newpass"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/register", bytes.NewReader(body))
	req = req.WithContext(withAdmin(req.Context()))
	req.Header.Set("Content-Type", "application/json")
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
	req = req.WithContext(withAdmin(req.Context()))
	rec := httptest.NewRecorder()
	s.handler.Register(rec, req)
	s.Assert().Equal(http.StatusMethodNotAllowed, rec.Code)
}

func (s *AdminHandlerSuite) TestRegister_InvalidBody() {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/register", bytes.NewReader([]byte("not json")))
	req = req.WithContext(withAdmin(req.Context()))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	s.handler.Register(rec, req)

	s.Assert().Equal(http.StatusBadRequest, rec.Code)
}

func (s *AdminHandlerSuite) TestRegister_UsernamePasswordRequired() {
	body, _ := json.Marshal(RegisterRequest{Username: "", Password: "p"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/register", bytes.NewReader(body))
	req = req.WithContext(withAdmin(req.Context()))
	req.Header.Set("Content-Type", "application/json")
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
	req = req.WithContext(withAdmin(req.Context()))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	s.handler.Register(rec, req)

	s.Assert().Equal(http.StatusUnauthorized, rec.Code)
}

func (s *AdminHandlerSuite) TestRegister_NoEncryptionKey() {
	s.handler.Config.EncryptionKey = ""

	body, _ := json.Marshal(RegisterRequest{Username: "u", Password: "p"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/register", bytes.NewReader(body))
	req = req.WithContext(withAdmin(req.Context()))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	s.handler.Register(rec, req)

	s.Assert().Equal(http.StatusInternalServerError, rec.Code)
}

func deleteUserRequest(username string) *http.Request {
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/users/"+username, nil)
	return mux.SetURLVars(req, map[string]string{"username": username})
}

func deletePresetRequest(username string) *http.Request {
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/presets/"+username, nil)
	return mux.SetURLVars(req, map[string]string{"username": username})
}

func (s *AdminHandlerSuite) TestDeleteUser_NoDatabase() {
	req := deleteUserRequest("alice")
	req = req.WithContext(withAdmin(req.Context()))
	req.Header.Set("Authorization", "unused")
	rec := httptest.NewRecorder()

	s.handler.DeleteUser(rec, req)

	s.Assert().Equal(http.StatusInternalServerError, rec.Code)
}

func (s *AdminHandlerSuite) TestDeleteUser_MethodNotAllowed() {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users/x", nil)
	req = mux.SetURLVars(req, map[string]string{"username": "x"})
	req = req.WithContext(withAdmin(req.Context()))
	rec := httptest.NewRecorder()

	s.handler.DeleteUser(rec, req)

	s.Assert().Equal(http.StatusMethodNotAllowed, rec.Code)
}

func (s *AdminHandlerSuite) TestDeletePreset_Success() {
	s.mockPreset.EXPECT().DeleteByUserName("alice").Return(nil)

	req := deletePresetRequest("alice")
	req = req.WithContext(withAdmin(req.Context()))
	rec := httptest.NewRecorder()

	s.handler.DeletePreset(rec, req)

	s.Require().Equal(http.StatusOK, rec.Code)
	var resp map[string]string
	s.Require().NoError(json.NewDecoder(rec.Body).Decode(&resp))
	s.Assert().Equal("deleted", resp["status"])
	s.Assert().Equal("alice", resp["username"])
}

func (s *AdminHandlerSuite) TestDeletePreset_NotFound() {
	s.mockPreset.EXPECT().DeleteByUserName("nobody").Return(preset.ErrPresetNotFound)

	req := deletePresetRequest("nobody")
	req = req.WithContext(withAdmin(req.Context()))
	rec := httptest.NewRecorder()

	s.handler.DeletePreset(rec, req)

	s.Assert().Equal(http.StatusNotFound, rec.Code)
}

func (s *AdminHandlerSuite) TestDeletePreset_MethodNotAllowed() {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/presets/x", nil)
	req = mux.SetURLVars(req, map[string]string{"username": "x"})
	req = req.WithContext(withAdmin(req.Context()))
	rec := httptest.NewRecorder()

	s.handler.DeletePreset(rec, req)

	s.Assert().Equal(http.StatusMethodNotAllowed, rec.Code)
}

func TestAdminHandlerSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(AdminHandlerSuite))
}
