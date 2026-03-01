package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/waisuan/alfred/internal/booker"
	"github.com/waisuan/alfred/internal/context"
)

// BookingHandler handles booking endpoints.
type BookingHandler struct {
	Booker booker.ClientInterface
}

// GetBooking handles GET /api/v1/booking.
func (h *BookingHandler) GetBooking(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	u := context.UserFrom(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	resp, err := h.Booker.GetBooking(u.APIToken, u.UserName, "", "")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// CheckStatusRequest is the body for POST /api/v1/booking/check-status.
type CheckStatusRequest struct {
	CourseID string `json:"courseID"`
	TxnDate  string `json:"txnDate"`
	Session  string `json:"session"`
	TeeBox   string `json:"teeBox"`
	TeeTime  string `json:"teeTime"`
}

// CheckStatus handles POST /api/v1/booking/check-status.
func (h *BookingHandler) CheckStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	u := context.UserFrom(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req CheckStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	input := booker.GolfCheckTeeTimeStatusInput{
		CourseID:  req.CourseID,
		TxnDate:   req.TxnDate,
		Session:   req.Session,
		TeeBox:    req.TeeBox,
		TeeTime:   req.TeeTime,
		UserName:  u.UserName,
		IPAddress: u.UserName,
		Action:    0,
	}
	resp, err := h.Booker.CheckTeeTimeStatus(u.APIToken, input)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// BookRequest is the body for POST /api/v1/booking/book.
type BookRequest struct {
	CourseID string `json:"courseID"`
	TxnDate  string `json:"txnDate"`
	Session  string `json:"session"`
	TeeBox   string `json:"teeBox"`
	TeeTime  string `json:"teeTime"`
}

// Book handles POST /api/v1/booking/book (check status then book).
func (h *BookingHandler) Book(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	u := context.UserFrom(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req BookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	checkInput := booker.GolfCheckTeeTimeStatusInput{
		CourseID:  req.CourseID,
		TxnDate:   req.TxnDate,
		Session:   req.Session,
		TeeBox:    req.TeeBox,
		TeeTime:   req.TeeTime,
		UserName:  u.UserName,
		IPAddress: u.UserName,
		Action:    0,
	}
	statusResp, err := h.Booker.CheckTeeTimeStatus(u.APIToken, checkInput)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !statusResp.Status {
		http.Error(w, "slot no longer available", http.StatusConflict)
		return
	}
	input := booker.GolfNewBooking2Input{
		CourseID:   req.CourseID,
		TxnDate:    req.TxnDate,
		Session:    req.Session,
		TeeBox:     req.TeeBox,
		TeeTime:    req.TeeTime,
		AccountID:  u.UserName,
		TotalGuest: 4,
		IPaddress:  u.UserName,
		Holes:      18,
	}
	bookResp, err := h.Booker.BookTeeTime(u.APIToken, input, false)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !bookResp.Status || len(bookResp.Result) == 0 || !bookResp.Result[0].Status {
		reason := bookResp.Reason
		if reason == "" {
			reason = "booking failed"
		}
		http.Error(w, reason, http.StatusConflict)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"bookingID": bookResp.Result[0].BookingID})
}
