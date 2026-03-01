package handlers

import (
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
	"github.com/waisuan/alfred/internal/history"
)

type HistoryHandlerSuite struct {
	suite.Suite
	ctrl    *gomock.Controller
	mockSvc *history.MockService
	handler *HistoryHandler
	user    *context.User
}

func (s *HistoryHandlerSuite) SetupTest() {
	s.ctrl = gomock.NewController(s.T())
	s.mockSvc = history.NewMockService(s.ctrl)
	s.handler = &HistoryHandler{Service: s.mockSvc}
	s.user = &context.User{UserName: "u", APIToken: "token"}
}

func (s *HistoryHandlerSuite) TearDownTest() {
	s.ctrl.Finish()
}

func (s *HistoryHandlerSuite) TestGetHistory_Unauthorized() {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/history", nil)
	rec := httptest.NewRecorder()
	s.handler.GetHistory(rec, req)
	s.Assert().Equal(http.StatusUnauthorized, rec.Code)
}

func (s *HistoryHandlerSuite) TestGetHistory_MethodNotAllowed() {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/history", nil)
	req = req.WithContext(context.WithUser(req.Context(), s.user))
	rec := httptest.NewRecorder()
	s.handler.GetHistory(rec, req)
	s.Assert().Equal(http.StatusMethodNotAllowed, rec.Code)
}

func (s *HistoryHandlerSuite) TestGetHistory_Success() {
	attempts := []history.Attempt{
		{
			ID:        1,
			CreatedAt: time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC),
			UserName:  "u",
			CourseID:  "PLC",
			TxnDate:   "2026/03/04",
			TeeTime:   sql.NullString{String: "07:00", Valid: true},
			TeeBox:    sql.NullString{String: "1", Valid: true},
			BookingID: sql.NullString{String: "B1", Valid: true},
			Status:    "success",
			Message:   "booked",
		},
	}
	s.mockSvc.EXPECT().
		GetAttempts("u", 50).
		Return(attempts, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/history", nil)
	req = req.WithContext(context.WithUser(req.Context(), s.user))
	rec := httptest.NewRecorder()
	s.handler.GetHistory(rec, req)

	s.Require().Equal(http.StatusOK, rec.Code)
	var resp HistoryResponse
	s.Require().NoError(json.NewDecoder(rec.Body).Decode(&resp))
	s.Assert().Len(resp.Attempts, 1)
	s.Assert().Equal("PLC", resp.Attempts[0].CourseID)
	s.Assert().Equal("B1", resp.Attempts[0].BookingID)
	s.Assert().Equal("success", resp.Attempts[0].Status)
}

func (s *HistoryHandlerSuite) TestGetHistory_Empty() {
	s.mockSvc.EXPECT().
		GetAttempts("u", 50).
		Return(nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/history", nil)
	req = req.WithContext(context.WithUser(req.Context(), s.user))
	rec := httptest.NewRecorder()
	s.handler.GetHistory(rec, req)

	s.Require().Equal(http.StatusOK, rec.Code)
	var resp HistoryResponse
	s.Require().NoError(json.NewDecoder(rec.Body).Decode(&resp))
	s.Assert().Empty(resp.Attempts)
}

func (s *HistoryHandlerSuite) TestGetHistory_DBError() {
	s.mockSvc.EXPECT().
		GetAttempts("u", 50).
		Return(nil, errors.New("connection refused"))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/history", nil)
	req = req.WithContext(context.WithUser(req.Context(), s.user))
	rec := httptest.NewRecorder()
	s.handler.GetHistory(rec, req)

	s.Assert().Equal(http.StatusInternalServerError, rec.Code)
}

func TestHistoryHandlerSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(HistoryHandlerSuite))
}

func TestHistoryResponse_JSONFormat(t *testing.T) {
	t.Parallel()
	resp := HistoryResponse{
		Attempts: []HistoryItem{
			{ID: 1, Status: "success", Message: "booked"},
		},
	}
	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	attempts, ok := out["attempts"].([]interface{})
	if !ok || len(attempts) != 1 {
		t.Fatalf("expected 1 attempt, got %v", out)
	}
}
