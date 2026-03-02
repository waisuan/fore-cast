package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/suite"
	"github.com/waisuan/alfred/internal/booker"
	"github.com/waisuan/alfred/internal/context"
)

type BookingHandlerSuite struct {
	suite.Suite
	ctrl       *gomock.Controller
	mockBooker *booker.MockClientInterface
	handler    *BookingHandler
	user       *context.User
}

func (s *BookingHandlerSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.mockBooker = booker.NewMockClientInterface(s.ctrl)
	s.handler = &BookingHandler{Booker: s.mockBooker}
	s.user = &context.User{UserName: "u", APIToken: "token"}
}

func (s *BookingHandlerSuite) TearDownTest() {
	s.ctrl.Finish()
}

func (s *BookingHandlerSuite) TestGetBooking_Success() {
	resp := &booker.GetBookingResponse{Status: true, Result: []booker.GetBookingResultItem{{BookingID: "B1"}}}
	s.mockBooker.EXPECT().
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
	s.mockBooker.EXPECT().
		GetBooking("token", "u", "", "").
		Return(nil, http.ErrHandlerTimeout)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/booking", nil)
	req = req.WithContext(context.WithUser(req.Context(), s.user))
	rec := httptest.NewRecorder()
	s.handler.GetBooking(rec, req)
	s.Assert().Equal(http.StatusInternalServerError, rec.Code)
}

func (s *BookingHandlerSuite) TestGetBooking_InvalidToken() {
	resp := &booker.GetBookingResponse{Status: false, Reason: "CODE103 - Invalid Token", Result: nil}
	s.mockBooker.EXPECT().
		GetBooking("token", "u", "", "").
		Return(resp, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/booking", nil)
	req = req.WithContext(context.WithUser(req.Context(), s.user))
	rec := httptest.NewRecorder()
	s.handler.GetBooking(rec, req)

	s.Assert().Equal(http.StatusUnauthorized, rec.Code)
	s.Assert().Contains(rec.Body.String(), "session expired")
}

func (s *BookingHandlerSuite) TestCheckStatus_Success() {
	resp := &booker.CheckTeeTimeStatusResponse{Status: true}
	s.mockBooker.EXPECT().
		CheckTeeTimeStatus("token", gomock.Any()).
		DoAndReturn(func(_ string, in booker.GolfCheckTeeTimeStatusInput) (*booker.CheckTeeTimeStatusResponse, error) {
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

func (s *BookingHandlerSuite) TestCheckStatus_InvalidToken() {
	s.mockBooker.EXPECT().
		CheckTeeTimeStatus("token", gomock.Any()).
		Return(&booker.CheckTeeTimeStatusResponse{Status: false, Reason: "CODE103 - Invalid Token"}, nil)

	body, _ := json.Marshal(CheckStatusRequest{
		CourseID: "PLC", TxnDate: "2026/02/25", Session: "1", TeeBox: "1", TeeTime: "07:00",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/booking/check-status", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithUser(req.Context(), s.user))
	rec := httptest.NewRecorder()
	s.handler.CheckStatus(rec, req)

	s.Assert().Equal(http.StatusUnauthorized, rec.Code)
	s.Assert().Contains(rec.Body.String(), "session expired")
}

func (s *BookingHandlerSuite) TestBook_Success() {
	s.mockBooker.EXPECT().
		CheckTeeTimeStatus("token", gomock.Any()).
		Return(&booker.CheckTeeTimeStatusResponse{Status: true}, nil)
	s.mockBooker.EXPECT().
		BookTeeTime("token", gomock.Any(), false).
		Return(&booker.BookingResponse{Status: true, Result: []booker.BookingResultItem{{Status: true, BookingID: "B1"}}}, nil)

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
	s.mockBooker.EXPECT().
		CheckTeeTimeStatus("token", gomock.Any()).
		Return(&booker.CheckTeeTimeStatusResponse{Status: false}, nil)

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
	s.mockBooker.EXPECT().
		CheckTeeTimeStatus("token", gomock.Any()).
		Return(&booker.CheckTeeTimeStatusResponse{Status: true}, nil)
	s.mockBooker.EXPECT().
		BookTeeTime("token", gomock.Any(), false).
		Return(&booker.BookingResponse{Status: false, Reason: "slot taken"}, nil)

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

func (s *BookingHandlerSuite) TestBook_InvalidTokenFromCheckStatus() {
	s.mockBooker.EXPECT().
		CheckTeeTimeStatus("token", gomock.Any()).
		Return(&booker.CheckTeeTimeStatusResponse{Status: false, Reason: "CODE103 - Invalid Token"}, nil)

	body, _ := json.Marshal(BookRequest{
		CourseID: "PLC", TxnDate: "2026/02/25", Session: "1", TeeBox: "1", TeeTime: "07:00",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/booking/book", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithUser(req.Context(), s.user))
	rec := httptest.NewRecorder()
	s.handler.Book(rec, req)

	s.Assert().Equal(http.StatusUnauthorized, rec.Code)
	s.Assert().Contains(rec.Body.String(), "session expired")
}

func (s *BookingHandlerSuite) TestBook_InvalidTokenFromBook() {
	s.mockBooker.EXPECT().
		CheckTeeTimeStatus("token", gomock.Any()).
		Return(&booker.CheckTeeTimeStatusResponse{Status: true}, nil)
	s.mockBooker.EXPECT().
		BookTeeTime("token", gomock.Any(), false).
		Return(&booker.BookingResponse{Status: false, Reason: "CODE103 - Invalid Token"}, nil)

	body, _ := json.Marshal(BookRequest{
		CourseID: "PLC", TxnDate: "2026/02/25", Session: "1", TeeBox: "1", TeeTime: "07:00",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/booking/book", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithUser(req.Context(), s.user))
	rec := httptest.NewRecorder()
	s.handler.Book(rec, req)

	s.Assert().Equal(http.StatusUnauthorized, rec.Code)
	s.Assert().Contains(rec.Body.String(), "session expired")
}

func TestBookingHandlerSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(BookingHandlerSuite))
}
