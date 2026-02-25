package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/suite"
	"github.com/waisuan/alfred/internal/context"
	"github.com/waisuan/alfred/internal/saujana"
)

type BookingHandlerSuite struct {
	suite.Suite
	ctrl        *gomock.Controller
	mockSaujana *saujana.MockClientInterface
	handler     *BookingHandler
	user        *context.User
}

func (s *BookingHandlerSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.mockSaujana = saujana.NewMockClientInterface(s.ctrl)
	s.handler = &BookingHandler{Saujana: s.mockSaujana}
	s.user = &context.User{UserName: "u", SaujanaToken: "token"}
}

func (s *BookingHandlerSuite) TearDownTest() {
	s.ctrl.Finish()
}

func (s *BookingHandlerSuite) TestGetBooking_Success() {
	resp := &saujana.GetBookingResponse{Status: true, Result: []saujana.GetBookingResultItem{{BookingID: "B1"}}}
	s.mockSaujana.EXPECT().
		GetBooking("token", "u", "", "").
		Return(resp, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/booking", nil)
	req = req.WithContext(context.WithUser(req.Context(), s.user))
	rec := httptest.NewRecorder()
	s.handler.GetBooking(rec, req)

	s.Assert().Equal(http.StatusOK, rec.Code)
}

func (s *BookingHandlerSuite) TestGetBooking_MethodNotAllowed() {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/booking", nil)
	req = req.WithContext(context.WithUser(req.Context(), s.user))
	rec := httptest.NewRecorder()
	s.handler.GetBooking(rec, req)
	s.Assert().Equal(http.StatusMethodNotAllowed, rec.Code)
}

func (s *BookingHandlerSuite) TestGetBooking_Unauthorized() {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/booking", nil)
	rec := httptest.NewRecorder()
	s.handler.GetBooking(rec, req)
	s.Assert().Equal(http.StatusUnauthorized, rec.Code)
}

func (s *BookingHandlerSuite) TestGetBooking_GetBookingError() {
	s.mockSaujana.EXPECT().
		GetBooking("token", "u", "", "").
		Return(nil, http.ErrHandlerTimeout)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/booking", nil)
	req = req.WithContext(context.WithUser(req.Context(), s.user))
	rec := httptest.NewRecorder()
	s.handler.GetBooking(rec, req)
	s.Assert().Equal(http.StatusInternalServerError, rec.Code)
}

func (s *BookingHandlerSuite) TestCheckStatus_Success() {
	resp := &saujana.CheckTeeTimeStatusResponse{Status: true}
	s.mockSaujana.EXPECT().
		CheckTeeTimeStatus("token", gomock.Any()).
		DoAndReturn(func(_ string, in saujana.GolfCheckTeeTimeStatusInput) (*saujana.CheckTeeTimeStatusResponse, error) {
			s.Assert().Equal("PLC", in.CourseID)
			s.Assert().Equal("2026/02/25", in.TxnDate)
			s.Assert().Equal("07:00", in.TeeTime)
			return resp, nil
		})

	body, _ := json.Marshal(CheckStatusRequest{
		CourseID: "PLC", TxnDate: "2026/02/25", Session: "1", TeeBox: "1", TeeTime: "07:00",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/booking/check-status", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithUser(req.Context(), s.user))
	rec := httptest.NewRecorder()
	s.handler.CheckStatus(rec, req)

	s.Assert().Equal(http.StatusOK, rec.Code)
}

func (s *BookingHandlerSuite) TestCheckStatus_InvalidBody() {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/booking/check-status", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithUser(req.Context(), s.user))
	rec := httptest.NewRecorder()
	s.handler.CheckStatus(rec, req)
	s.Assert().Equal(http.StatusBadRequest, rec.Code)
}

func (s *BookingHandlerSuite) TestBook_Success() {
	s.mockSaujana.EXPECT().
		CheckTeeTimeStatus("token", gomock.Any()).
		Return(&saujana.CheckTeeTimeStatusResponse{Status: true}, nil)
	s.mockSaujana.EXPECT().
		BookTeeTime("token", gomock.Any(), false).
		Return(&saujana.BookingResponse{Status: true, Result: []saujana.BookingResultItem{{Status: true, BookingID: "B1"}}}, nil)

	body, _ := json.Marshal(BookRequest{
		CourseID: "PLC", TxnDate: "2026/02/25", Session: "1", TeeBox: "1", TeeTime: "07:00",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/booking/book", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithUser(req.Context(), s.user))
	rec := httptest.NewRecorder()
	s.handler.Book(rec, req)

	s.Require().Equal(http.StatusOK, rec.Code)
	var out map[string]string
	s.Require().NoError(json.NewDecoder(rec.Body).Decode(&out))
	s.Assert().Equal("B1", out["bookingID"])
}

func (s *BookingHandlerSuite) TestBook_SlotNoLongerAvailable() {
	s.mockSaujana.EXPECT().
		CheckTeeTimeStatus("token", gomock.Any()).
		Return(&saujana.CheckTeeTimeStatusResponse{Status: false}, nil)

	body, _ := json.Marshal(BookRequest{
		CourseID: "PLC", TxnDate: "2026/02/25", Session: "1", TeeBox: "1", TeeTime: "07:00",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/booking/book", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithUser(req.Context(), s.user))
	rec := httptest.NewRecorder()
	s.handler.Book(rec, req)

	s.Assert().Equal(http.StatusConflict, rec.Code)
}

func (s *BookingHandlerSuite) TestBook_BookReturnsFailure() {
	s.mockSaujana.EXPECT().
		CheckTeeTimeStatus("token", gomock.Any()).
		Return(&saujana.CheckTeeTimeStatusResponse{Status: true}, nil)
	s.mockSaujana.EXPECT().
		BookTeeTime("token", gomock.Any(), false).
		Return(&saujana.BookingResponse{Status: false, Reason: "slot taken"}, nil)

	body, _ := json.Marshal(BookRequest{
		CourseID: "PLC", TxnDate: "2026/02/25", Session: "1", TeeBox: "1", TeeTime: "07:00",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/booking/book", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithUser(req.Context(), s.user))
	rec := httptest.NewRecorder()
	s.handler.Book(rec, req)

	s.Assert().Equal(http.StatusConflict, rec.Code)
}

func (s *BookingHandlerSuite) TestAuto_DateRequired() {
	body, _ := json.Marshal(AutoRequest{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/booking/auto", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithUser(req.Context(), s.user))
	rec := httptest.NewRecorder()
	s.handler.Auto(rec, req)
	s.Assert().Equal(http.StatusBadRequest, rec.Code)
}

func (s *BookingHandlerSuite) TestAuto_SuccessOnFirstSlot() {
	slots := []saujana.TeeTimeSlot{
		{CourseID: "PLC", TeeTime: "07:00", Session: "1", TeeBox: saujana.StringOrNumber("1"), TxnDate: "2026/02/25"},
	}
	s.mockSaujana.EXPECT().
		GetTeeTimeSlots("token", "PLC", "2026/02/25").
		Return(slots, nil)
	s.mockSaujana.EXPECT().
		CheckTeeTimeStatus("token", gomock.Any()).
		Return(&saujana.CheckTeeTimeStatusResponse{Status: true}, nil)
	s.mockSaujana.EXPECT().
		BookTeeTime("token", gomock.Any(), false).
		Return(&saujana.BookingResponse{Status: true, Result: []saujana.BookingResultItem{{Status: true, BookingID: "A1"}}}, nil)

	body, _ := json.Marshal(AutoRequest{Date: "2026/02/25", Cutoff: "8:15", Retries: 1})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/booking/auto", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithUser(req.Context(), s.user))
	rec := httptest.NewRecorder()
	s.handler.Auto(rec, req)

	s.Require().Equal(http.StatusOK, rec.Code)
	var out map[string]string
	s.Require().NoError(json.NewDecoder(rec.Body).Decode(&out))
	s.Assert().Equal("A1", out["bookingID"])
}

func TestBookingHandlerSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(BookingHandlerSuite))
}
