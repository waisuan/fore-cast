package saujana

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLogin_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Status":true,"Token":"test-token-123"}`))
	}))
	defer srv.Close()

	client := NewClientWithBaseURL(srv.URL)
	token, err := client.Login("user", "pass")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if token != "test-token-123" {
		t.Errorf("token = %q, want test-token-123", token)
	}
}

func TestLogin_Rejected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Status":false,"Token":""}`))
	}))
	defer srv.Close()

	client := NewClientWithBaseURL(srv.URL)
	_, err := client.Login("user", "wrong")
	if err == nil {
		t.Fatal("Login expected error")
	}
}

func TestLogin_EmptyToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Status":true,"Token":""}`))
	}))
	defer srv.Close()

	client := NewClientWithBaseURL(srv.URL)
	_, err := client.Login("user", "pass")
	if err == nil {
		t.Fatal("Login expected error when Token empty")
	}
}

func TestLogin_HTMLResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<!DOCTYPE html><html></html>`))
	}))
	defer srv.Close()

	client := NewClientWithBaseURL(srv.URL)
	_, err := client.Login("user", "pass")
	if err == nil {
		t.Fatal("Login expected error for HTML response")
	}
}

func TestGetTeeTimeSlots_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Status":true,"Result":[{"CourseID":"BRC","TeeTime":"1899-12-30T07:37:00","Session":"Morning","TeeBox":"1"}]}`))
	}))
	defer srv.Close()

	client := NewClientWithBaseURL(srv.URL)
	slots, err := client.GetTeeTimeSlots("token", "BRC", "2026/02/25")
	if err != nil {
		t.Fatalf("GetTeeTimeSlots: %v", err)
	}
	if len(slots) != 1 {
		t.Fatalf("len(slots) = %d, want 1", len(slots))
	}
	if slots[0].TeeTime != "1899-12-30T07:37:00" || slots[0].CourseID != "BRC" || slots[0].TeeBox.String() != "1" {
		t.Errorf("slot = %+v", slots[0])
	}
}

func TestGetTeeTimeSlots_StatusFalse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Status":false,"Result":[]}`))
	}))
	defer srv.Close()

	client := NewClientWithBaseURL(srv.URL)
	_, err := client.GetTeeTimeSlots("token", "BRC", "2026/02/25")
	if err == nil {
		t.Fatal("GetTeeTimeSlots expected error when Status=false")
	}
}

func TestBookTeeTime_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Status":true,"Reason":"","Result":[{"Status":true,"BookingID":"BK001"}]}`))
	}))
	defer srv.Close()

	client := NewClientWithBaseURL(srv.URL)
	input := GolfNewBooking2Input{
		CourseID: "BRC", TxnDate: "2026/02/25", Session: "Morning", TeeBox: "1",
		TeeTime: "1899-12-30T07:37:00", AccountID: "user", TotalGuest: 4, IPaddress: "user", Holes: 18,
	}
	resp, err := client.BookTeeTime("token", input, false)
	if err != nil {
		t.Fatalf("BookTeeTime: %v", err)
	}
	if !resp.Status || len(resp.Result) == 0 || !resp.Result[0].Status || resp.Result[0].BookingID != "BK001" {
		t.Errorf("resp = %+v", resp)
	}
}

func TestCheckTeeTimeStatus_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Status":true,"Reason":"Available"}`))
	}))
	defer srv.Close()

	client := NewClientWithBaseURL(srv.URL)
	input := GolfCheckTeeTimeStatusInput{
		CourseID: "BRC", TxnDate: "2026/02/25", Session: "Morning", TeeBox: "1",
		TeeTime: "1899-12-30T07:37:00", UserName: "user", IPAddress: "user", Action: 0,
	}
	resp, err := client.CheckTeeTimeStatus("token", input)
	if err != nil {
		t.Fatalf("CheckTeeTimeStatus: %v", err)
	}
	if !resp.Status || resp.Reason != "Available" {
		t.Errorf("resp = %+v", resp)
	}
}

func TestGetBooking_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Status":true,"Result":[{"BookingID":"BK001","TxnDate":"2026/02/25","CourseID":"BRC","TeeTime":"07:37","Session":"Morning","TeeBox":"1","Pax":4,"Hole":18,"Name":"Member"}]}`))
	}))
	defer srv.Close()

	client := NewClientWithBaseURL(srv.URL)
	resp, err := client.GetBooking("token", "user", "", "")
	if err != nil {
		t.Fatalf("GetBooking: %v", err)
	}
	if !resp.Status || len(resp.Result) != 1 || resp.Result[0].BookingID != "BK001" {
		t.Errorf("resp = %+v", resp)
	}
}

func TestGetTeeTimeSlots_TeeBoxAsNumber(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Status":true,"Result":[{"CourseID":"BRC","TeeTime":"1899-12-30T07:37:00","Session":"Morning","TeeBox":10}]}`))
	}))
	defer srv.Close()

	client := NewClientWithBaseURL(srv.URL)
	slots, err := client.GetTeeTimeSlots("token", "BRC", "2026/02/25")
	if err != nil {
		t.Fatalf("GetTeeTimeSlots: %v", err)
	}
	if len(slots) != 1 {
		t.Fatalf("len(slots) = %d, want 1", len(slots))
	}
	if slots[0].TeeBox.String() != "10" {
		t.Errorf("TeeBox (number) = %q, want \"10\"", slots[0].TeeBox.String())
	}
}
