package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/suite"
	"github.com/waisuan/alfred/internal/booker"
	"github.com/waisuan/alfred/internal/context"
)

type SlotsHandlerSuite struct {
	suite.Suite
	ctrl       *gomock.Controller
	mockBooker *booker.MockClientInterface
	handler    *SlotsHandler
	user       *context.User
}

func (s *SlotsHandlerSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.mockBooker = booker.NewMockClientInterface(s.ctrl)
	s.handler = &SlotsHandler{Booker: s.mockBooker}
	s.user = &context.User{UserName: "u", APIToken: "token"}
}

func (s *SlotsHandlerSuite) TearDownTest() {
	s.ctrl.Finish()
}

func (s *SlotsHandlerSuite) TestSlots_Success() {
	slots := []booker.TeeTimeSlot{
		{CourseID: "PLC", TeeTime: "07:00", Session: "1", TeeBox: booker.StringOrNumber("1")},
	}
	s.mockBooker.EXPECT().
		GetTeeTimeSlots("token", "PLC", "2026/02/25").
		Return(slots, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/slots?date=2026/02/25", nil)
	req = req.WithContext(context.WithUser(req.Context(), s.user))
	rec := httptest.NewRecorder()
	s.handler.Slots(rec, req)

	s.Assert().Equal(http.StatusOK, rec.Code)
}

func (s *SlotsHandlerSuite) TestSlots_MethodNotAllowed() {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/slots?date=2026/02/25", nil)
	req = req.WithContext(context.WithUser(req.Context(), s.user))
	rec := httptest.NewRecorder()
	s.handler.Slots(rec, req)
	s.Assert().Equal(http.StatusMethodNotAllowed, rec.Code)
}

func (s *SlotsHandlerSuite) TestSlots_Unauthorized() {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/slots?date=2026/02/25", nil)
	rec := httptest.NewRecorder()
	s.handler.Slots(rec, req)
	s.Assert().Equal(http.StatusUnauthorized, rec.Code)
}

func (s *SlotsHandlerSuite) TestSlots_DateQueryRequired() {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/slots", nil)
	req = req.WithContext(context.WithUser(req.Context(), s.user))
	rec := httptest.NewRecorder()
	s.handler.Slots(rec, req)
	s.Assert().Equal(http.StatusBadRequest, rec.Code)
}

func (s *SlotsHandlerSuite) TestSlots_InvalidDate() {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/slots?date=invalid", nil)
	req = req.WithContext(context.WithUser(req.Context(), s.user))
	rec := httptest.NewRecorder()
	s.handler.Slots(rec, req)
	s.Assert().Equal(http.StatusBadRequest, rec.Code)
}

func (s *SlotsHandlerSuite) TestSlots_GetTeeTimeSlotsError() {
	s.mockBooker.EXPECT().
		GetTeeTimeSlots("token", "PLC", "2026/02/25").
		Return(nil, http.ErrHandlerTimeout)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/slots?date=2026/02/25", nil)
	req = req.WithContext(context.WithUser(req.Context(), s.user))
	rec := httptest.NewRecorder()
	s.handler.Slots(rec, req)
	s.Assert().Equal(http.StatusInternalServerError, rec.Code)
}

func (s *SlotsHandlerSuite) TestSlots_InvalidToken() {
	s.mockBooker.EXPECT().
		GetTeeTimeSlots("token", "PLC", "2026/02/25").
		Return(nil, fmt.Errorf("get tee time: %w", booker.ErrInvalidToken))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/slots?date=2026/02/25", nil)
	req = req.WithContext(context.WithUser(req.Context(), s.user))
	rec := httptest.NewRecorder()
	s.handler.Slots(rec, req)

	s.Assert().Equal(http.StatusUnauthorized, rec.Code)
	s.Assert().Contains(rec.Body.String(), "session expired")
}

func (s *SlotsHandlerSuite) TestSlots_WithCourseParam() {
	slots := []booker.TeeTimeSlot{
		{CourseID: "BRC", TeeTime: "07:00", Session: "1", TeeBox: booker.StringOrNumber("1")},
	}
	// 2026/02/25 is Wednesday -> default would be PLC; course=BRC overrides
	s.mockBooker.EXPECT().
		GetTeeTimeSlots("token", "BRC", "2026/02/25").
		Return(slots, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/slots?date=2026/02/25&course=BRC", nil)
	req = req.WithContext(context.WithUser(req.Context(), s.user))
	rec := httptest.NewRecorder()
	s.handler.Slots(rec, req)

	s.Assert().Equal(http.StatusOK, rec.Code)
	var resp SlotsResponse
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &resp))
	s.Assert().Equal("BRC", resp.Course)
}

func (s *SlotsHandlerSuite) TestSlots_InvalidCourseParam() {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/slots?date=2026/02/25&course=INVALID", nil)
	req = req.WithContext(context.WithUser(req.Context(), s.user))
	rec := httptest.NewRecorder()
	s.handler.Slots(rec, req)
	s.Assert().Equal(http.StatusBadRequest, rec.Code)
}

func (s *SlotsHandlerSuite) TestSlots_WithCutoffFilter() {
	slots := []booker.TeeTimeSlot{
		{CourseID: "PLC", TeeTime: "1899-12-30T07:00:00", Session: "1", TeeBox: booker.StringOrNumber("1")},
		{CourseID: "PLC", TeeTime: "1899-12-30T09:00:00", Session: "1", TeeBox: booker.StringOrNumber("1")},
	}
	s.mockBooker.EXPECT().
		GetTeeTimeSlots("token", "PLC", "2026/02/25").
		Return(slots, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/slots?date=2026/02/25&cutoff=8:15", nil)
	req = req.WithContext(context.WithUser(req.Context(), s.user))
	rec := httptest.NewRecorder()
	s.handler.Slots(rec, req)

	s.Require().Equal(http.StatusOK, rec.Code)
	var resp SlotsResponse
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &resp))
	s.Assert().Len(resp.Slots, 1)
	s.Assert().Equal("1899-12-30T07:00:00", resp.Slots[0].TeeTime)
}

func TestSlotsHandlerSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(SlotsHandlerSuite))
}
