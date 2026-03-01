package booker

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/suite"
)

type ClientSuite struct {
	suite.Suite
}

func (s *ClientSuite) TestLogin_Success() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Status":true,"Token":"test-token-123"}`))
	}))
	defer srv.Close()

	client := NewClientWithOptions(srv.URL, defaultHTTPTimeout)
	token, err := client.Login("user", "pass")
	s.Require().NoError(err)
	s.Assert().Equal("test-token-123", token)
}

func (s *ClientSuite) TestLogin_Rejected() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Status":false,"Token":""}`))
	}))
	defer srv.Close()

	client := NewClientWithOptions(srv.URL, defaultHTTPTimeout)
	_, err := client.Login("user", "wrong")
	s.Require().Error(err)
}

func (s *ClientSuite) TestLogin_EmptyToken() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Status":true,"Token":""}`))
	}))
	defer srv.Close()

	client := NewClientWithOptions(srv.URL, defaultHTTPTimeout)
	_, err := client.Login("user", "pass")
	s.Require().Error(err)
}

func (s *ClientSuite) TestLogin_HTMLResponse() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<!DOCTYPE html><html></html>`))
	}))
	defer srv.Close()

	client := NewClientWithOptions(srv.URL, defaultHTTPTimeout)
	_, err := client.Login("user", "pass")
	s.Require().Error(err)
}

func (s *ClientSuite) TestGetTeeTimeSlots_Success() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Status":true,"Result":[{"CourseID":"BRC","TeeTime":"1899-12-30T07:37:00","Session":"Morning","TeeBox":"1"}]}`))
	}))
	defer srv.Close()

	client := NewClientWithOptions(srv.URL, defaultHTTPTimeout)
	slots, err := client.GetTeeTimeSlots("token", "BRC", "2026/02/25")
	s.Require().NoError(err)
	s.Require().Len(slots, 1)
	s.Assert().Equal("1899-12-30T07:37:00", slots[0].TeeTime)
	s.Assert().Equal("BRC", slots[0].CourseID)
	s.Assert().Equal("1", slots[0].TeeBox.String())
}

func (s *ClientSuite) TestGetTeeTimeSlots_StatusFalse() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Status":false,"Result":[]}`))
	}))
	defer srv.Close()

	client := NewClientWithOptions(srv.URL, defaultHTTPTimeout)
	_, err := client.GetTeeTimeSlots("token", "BRC", "2026/02/25")
	s.Require().Error(err)
}

func (s *ClientSuite) TestBookTeeTime_Success() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Status":true,"Reason":"","Result":[{"Status":true,"BookingID":"BK001"}]}`))
	}))
	defer srv.Close()

	client := NewClientWithOptions(srv.URL, defaultHTTPTimeout)
	input := GolfNewBooking2Input{
		CourseID: "BRC", TxnDate: "2026/02/25", Session: "Morning", TeeBox: "1",
		TeeTime: "1899-12-30T07:37:00", AccountID: "user", TotalGuest: 4, IPaddress: "user", Holes: 18,
	}
	resp, err := client.BookTeeTime("token", input, false)
	s.Require().NoError(err)
	s.Require().True(resp.Status)
	s.Require().NotEmpty(resp.Result)
	s.Assert().True(resp.Result[0].Status)
	s.Assert().Equal("BK001", resp.Result[0].BookingID)
}

func (s *ClientSuite) TestCheckTeeTimeStatus_Success() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Status":true,"Reason":"Available"}`))
	}))
	defer srv.Close()

	client := NewClientWithOptions(srv.URL, defaultHTTPTimeout)
	input := GolfCheckTeeTimeStatusInput{
		CourseID: "BRC", TxnDate: "2026/02/25", Session: "Morning", TeeBox: "1",
		TeeTime: "1899-12-30T07:37:00", UserName: "user", IPAddress: "user", Action: 0,
	}
	resp, err := client.CheckTeeTimeStatus("token", input)
	s.Require().NoError(err)
	s.Assert().True(resp.Status)
	s.Assert().Equal("Available", resp.Reason)
}

func (s *ClientSuite) TestGetBooking_Success() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Status":true,"Result":[{"BookingID":"BK001","TxnDate":"2026/02/25","CourseID":"BRC","TeeTime":"07:37","Session":"Morning","TeeBox":"1","Pax":4,"Hole":18,"Name":"Member"}]}`))
	}))
	defer srv.Close()

	client := NewClientWithOptions(srv.URL, defaultHTTPTimeout)
	resp, err := client.GetBooking("token", "user", "", "")
	s.Require().NoError(err)
	s.Require().True(resp.Status)
	s.Require().Len(resp.Result, 1)
	s.Assert().Equal("BK001", resp.Result[0].BookingID)
}

func (s *ClientSuite) TestGetTeeTimeSlots_TeeBoxAsNumber() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Status":true,"Result":[{"CourseID":"BRC","TeeTime":"1899-12-30T07:37:00","Session":"Morning","TeeBox":10}]}`))
	}))
	defer srv.Close()

	client := NewClientWithOptions(srv.URL, defaultHTTPTimeout)
	slots, err := client.GetTeeTimeSlots("token", "BRC", "2026/02/25")
	s.Require().NoError(err)
	s.Require().Len(slots, 1)
	s.Assert().Equal("10", slots[0].TeeBox.String())
}

func TestClientSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(ClientSuite))
}
