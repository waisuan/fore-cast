package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/suite"
	"github.com/waisuan/alfred/internal/context"
	"github.com/waisuan/alfred/internal/preset"
)

type PresetHandlerSuite struct {
	suite.Suite
	ctrl    *gomock.Controller
	mockSvc *preset.MockService
	handler *PresetHandler
	user    *context.User
}

func (s *PresetHandlerSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.mockSvc = preset.NewMockService(s.ctrl)
	s.handler = &PresetHandler{Service: s.mockSvc}
	s.user = &context.User{UserName: "u", APIToken: "token"}
}

func (s *PresetHandlerSuite) TearDownTest() {
	s.ctrl.Finish()
}

// --- GetPreset ---

func (s *PresetHandlerSuite) TestGetPreset_Unauthorized() {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/preset", nil)
	rec := httptest.NewRecorder()
	s.handler.GetPreset(rec, req)
	s.Assert().Equal(http.StatusUnauthorized, rec.Code)
}

func (s *PresetHandlerSuite) TestGetPreset_MethodNotAllowed() {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/preset", nil)
	req = req.WithContext(context.WithUser(req.Context(), s.user))
	rec := httptest.NewRecorder()
	s.handler.GetPreset(rec, req)
	s.Assert().Equal(http.StatusMethodNotAllowed, rec.Code)
}

func (s *PresetHandlerSuite) TestGetPreset_NotFound_ReturnsDefaults() {
	s.mockSvc.EXPECT().
		GetPreset("u").
		Return(nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/preset", nil)
	req = req.WithContext(context.WithUser(req.Context(), s.user))
	rec := httptest.NewRecorder()
	s.handler.GetPreset(rec, req)

	s.Require().Equal(http.StatusOK, rec.Code)
	var resp PresetResponse
	s.Require().NoError(json.NewDecoder(rec.Body).Decode(&resp))
	s.Assert().Equal("u", resp.UserName)
	s.Assert().Equal(preset.DefaultCutoff, resp.Cutoff)
	s.Assert().Equal(preset.DefaultRetryInterval, resp.RetryInterval)
	s.Assert().Equal(preset.DefaultTimeout, resp.Timeout)
	s.Assert().Equal(preset.DefaultCutoff, resp.Defaults.Cutoff)
	s.Assert().Equal(preset.DefaultRetryInterval, resp.Defaults.RetryInterval)
	s.Assert().Equal(preset.DefaultTimeout, resp.Defaults.Timeout)
}

func (s *PresetHandlerSuite) TestGetPreset_Found() {
	s.mockSvc.EXPECT().
		GetPreset("u").
		Return(&preset.Preset{
			UserName:      "u",
			Course:        sql.NullString{String: "PLC", Valid: true},
			Cutoff:        "7:30",
			RetryInterval: "2s",
			Timeout:       "5m",
			NtfyTopic:     sql.NullString{String: "my-topic", Valid: true},
			Enabled:       true,
			UpdatedAt:     time.Now(),
		}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/preset", nil)
	req = req.WithContext(context.WithUser(req.Context(), s.user))
	rec := httptest.NewRecorder()
	s.handler.GetPreset(rec, req)

	s.Require().Equal(http.StatusOK, rec.Code)
	var resp PresetResponse
	s.Require().NoError(json.NewDecoder(rec.Body).Decode(&resp))
	s.Assert().Equal("u", resp.UserName)
	s.Assert().Equal("PLC", resp.Course)
	s.Assert().Equal("7:30", resp.Cutoff)
	s.Assert().Equal("2s", resp.RetryInterval)
	s.Assert().Equal("5m", resp.Timeout)
	s.Assert().Equal("my-topic", resp.NtfyTopic)
	s.Assert().True(resp.EnableNotifications)
	s.Assert().True(resp.Enabled)

	s.Assert().Equal(preset.DefaultCutoff, resp.Defaults.Cutoff)
	s.Assert().Equal(preset.DefaultRetryInterval, resp.Defaults.RetryInterval)
	s.Assert().Equal(preset.DefaultTimeout, resp.Defaults.Timeout)
}

func (s *PresetHandlerSuite) TestGetPreset_Found_NoTopic() {
	s.mockSvc.EXPECT().
		GetPreset("u").
		Return(&preset.Preset{
			UserName:      "u",
			Cutoff:        "8:15",
			RetryInterval: "1s",
			Timeout:       "10m",
		}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/preset", nil)
	req = req.WithContext(context.WithUser(req.Context(), s.user))
	rec := httptest.NewRecorder()
	s.handler.GetPreset(rec, req)

	s.Require().Equal(http.StatusOK, rec.Code)
	var resp PresetResponse
	s.Require().NoError(json.NewDecoder(rec.Body).Decode(&resp))
	s.Assert().Empty(resp.NtfyTopic)
	s.Assert().False(resp.EnableNotifications)
}

func (s *PresetHandlerSuite) TestGetPreset_DBError() {
	s.mockSvc.EXPECT().
		GetPreset("u").
		Return(nil, errors.New("db down"))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/preset", nil)
	req = req.WithContext(context.WithUser(req.Context(), s.user))
	rec := httptest.NewRecorder()
	s.handler.GetPreset(rec, req)

	s.Assert().Equal(http.StatusInternalServerError, rec.Code)
}

// --- SavePreset ---

func (s *PresetHandlerSuite) TestSavePreset_Unauthorized() {
	req := httptest.NewRequest(http.MethodPut, "/api/v1/preset", nil)
	rec := httptest.NewRecorder()
	s.handler.SavePreset(rec, req)
	s.Assert().Equal(http.StatusUnauthorized, rec.Code)
}

func (s *PresetHandlerSuite) TestSavePreset_MethodNotAllowed() {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/preset", nil)
	req = req.WithContext(context.WithUser(req.Context(), s.user))
	rec := httptest.NewRecorder()
	s.handler.SavePreset(rec, req)
	s.Assert().Equal(http.StatusMethodNotAllowed, rec.Code)
}

func (s *PresetHandlerSuite) TestSavePreset_InvalidBody() {
	req := httptest.NewRequest(http.MethodPut, "/api/v1/preset", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithUser(req.Context(), s.user))
	rec := httptest.NewRecorder()
	s.handler.SavePreset(rec, req)
	s.Assert().Equal(http.StatusBadRequest, rec.Code)
}

func (s *PresetHandlerSuite) TestSavePreset_Success() {
	s.mockSvc.EXPECT().GetPreset("u").Return(nil, nil)
	s.mockSvc.EXPECT().
		UpsertPreset(gomock.Any()).
		DoAndReturn(func(p preset.Preset) error {
			s.Assert().Equal("u", p.UserName)
			s.Assert().Equal("PLC", p.Course.String)
			s.Assert().Equal("7:30", p.Cutoff)
			s.Assert().Equal("2s", p.RetryInterval)
			s.Assert().Equal("5m", p.Timeout)
			s.Assert().True(p.Enabled)
			return nil
		})

	body, _ := json.Marshal(PresetRequest{
		Course:        "PLC",
		Cutoff:        "7:30",
		RetryInterval: "2s",
		Timeout:       "5m",
		Enabled:       true,
	})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/preset", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithUser(req.Context(), s.user))
	rec := httptest.NewRecorder()
	s.handler.SavePreset(rec, req)

	s.Require().Equal(http.StatusOK, rec.Code)
	var resp map[string]string
	s.Require().NoError(json.NewDecoder(rec.Body).Decode(&resp))
	s.Assert().Equal("saved", resp["status"])
}

func (s *PresetHandlerSuite) TestSavePreset_DBError() {
	s.mockSvc.EXPECT().GetPreset("u").Return(nil, nil)
	s.mockSvc.EXPECT().
		UpsertPreset(gomock.Any()).
		Return(errors.New("db down"))

	body, _ := json.Marshal(PresetRequest{
		Cutoff:  "8:15",
		Timeout: "10m",
	})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/preset", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithUser(req.Context(), s.user))
	rec := httptest.NewRecorder()
	s.handler.SavePreset(rec, req)

	s.Assert().Equal(http.StatusInternalServerError, rec.Code)
}

func (s *PresetHandlerSuite) TestSavePreset_DefaultsApplied() {
	s.mockSvc.EXPECT().GetPreset("u").Return(nil, nil)
	s.mockSvc.EXPECT().
		UpsertPreset(gomock.Any()).
		DoAndReturn(func(p preset.Preset) error {
			s.Assert().Equal("8:15", p.Cutoff)
			s.Assert().Equal("1s", p.RetryInterval)
			s.Assert().Equal("10m", p.Timeout)
			return nil
		})

	body, _ := json.Marshal(PresetRequest{})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/preset", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithUser(req.Context(), s.user))
	rec := httptest.NewRecorder()
	s.handler.SavePreset(rec, req)

	s.Assert().Equal(http.StatusOK, rec.Code)
}

func (s *PresetHandlerSuite) TestSavePreset_InvalidRetryInterval_FallsBackToDefault() {
	s.mockSvc.EXPECT().GetPreset("u").Return(nil, nil)
	s.mockSvc.EXPECT().
		UpsertPreset(gomock.Any()).
		DoAndReturn(func(p preset.Preset) error {
			s.Assert().Equal("1s", p.RetryInterval, "invalid duration should fall back to default")
			return nil
		})

	body, _ := json.Marshal(PresetRequest{
		RetryInterval: "not-a-duration",
		Cutoff:        "8:15",
		Timeout:       "10m",
	})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/preset", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithUser(req.Context(), s.user))
	rec := httptest.NewRecorder()
	s.handler.SavePreset(rec, req)

	s.Assert().Equal(http.StatusOK, rec.Code)
}

func (s *PresetHandlerSuite) TestSavePreset_RetryIntervalZeroOrLow_Accepted() {
	s.mockSvc.EXPECT().GetPreset("u").Return(nil, nil)
	s.mockSvc.EXPECT().
		UpsertPreset(gomock.Any()).
		DoAndReturn(func(p preset.Preset) error {
			s.Assert().Equal("50ms", p.RetryInterval, "50ms should be accepted as-is (min is 0s)")
			return nil
		})

	body, _ := json.Marshal(PresetRequest{
		RetryInterval: "50ms",
		Cutoff:        "8:15",
		Timeout:       "10m",
	})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/preset", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithUser(req.Context(), s.user))
	rec := httptest.NewRecorder()
	s.handler.SavePreset(rec, req)

	s.Assert().Equal(http.StatusOK, rec.Code)
}

func (s *PresetHandlerSuite) TestSavePreset_EnableNotifications_GeneratesTopic() {
	s.mockSvc.EXPECT().GetPreset("u").Return(nil, nil)
	s.mockSvc.EXPECT().
		UpsertPreset(gomock.Any()).
		DoAndReturn(func(p preset.Preset) error {
			s.Assert().True(p.NtfyTopic.Valid)
			s.Assert().Contains(p.NtfyTopic.String, "fore-cast-u-")
			return nil
		})

	enable := true
	body, _ := json.Marshal(PresetRequest{
		EnableNotifications: &enable,
	})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/preset", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithUser(req.Context(), s.user))
	rec := httptest.NewRecorder()
	s.handler.SavePreset(rec, req)

	s.Assert().Equal(http.StatusOK, rec.Code)
}

func (s *PresetHandlerSuite) TestSavePreset_EnableNotifications_PreservesExistingTopic() {
	s.mockSvc.EXPECT().GetPreset("u").Return(&preset.Preset{
		UserName:  "u",
		NtfyTopic: sql.NullString{String: "fore-cast-u-existing", Valid: true},
	}, nil)
	s.mockSvc.EXPECT().
		UpsertPreset(gomock.Any()).
		DoAndReturn(func(p preset.Preset) error {
			s.Assert().Equal("fore-cast-u-existing", p.NtfyTopic.String)
			return nil
		})

	enable := true
	body, _ := json.Marshal(PresetRequest{
		EnableNotifications: &enable,
	})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/preset", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithUser(req.Context(), s.user))
	rec := httptest.NewRecorder()
	s.handler.SavePreset(rec, req)

	s.Assert().Equal(http.StatusOK, rec.Code)
}

func (s *PresetHandlerSuite) TestSavePreset_DisableNotifications_ClearsTopic() {
	s.mockSvc.EXPECT().GetPreset("u").Return(&preset.Preset{
		UserName:  "u",
		NtfyTopic: sql.NullString{String: "fore-cast-u-existing", Valid: true},
	}, nil)
	s.mockSvc.EXPECT().
		UpsertPreset(gomock.Any()).
		DoAndReturn(func(p preset.Preset) error {
			s.Assert().False(p.NtfyTopic.Valid)
			s.Assert().Empty(p.NtfyTopic.String)
			return nil
		})

	disable := false
	body, _ := json.Marshal(PresetRequest{
		EnableNotifications: &disable,
	})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/preset", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithUser(req.Context(), s.user))
	rec := httptest.NewRecorder()
	s.handler.SavePreset(rec, req)

	s.Assert().Equal(http.StatusOK, rec.Code)
}

func TestPresetHandlerSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(PresetHandlerSuite))
}
