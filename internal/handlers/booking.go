package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/waisuan/alfred/internal/booker"
	"github.com/waisuan/alfred/internal/context"
	"github.com/waisuan/alfred/internal/slotutil"
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

// AutoRequest is the body for POST /api/v1/booking/auto.
type AutoRequest struct {
	Date             string `json:"date"`
	Cutoff           string `json:"cutoff"`
	Retries          int    `json:"retries"`
	RetryIntervalSec int    `json:"retry_interval_sec"`
}

// Auto handles POST /api/v1/booking/auto.
func (h *BookingHandler) Auto(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	u := context.UserFrom(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req AutoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if req.Date == "" {
		http.Error(w, "date required", http.StatusBadRequest)
		return
	}
	if err := slotutil.ValidateDate(req.Date); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	cutoffTeeTime, err := slotutil.ParseCutoff(req.Cutoff)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Retries < 1 {
		req.Retries = 1
	}
	if req.RetryIntervalSec < 1 {
		req.RetryIntervalSec = 5
	}
	courseID := slotutil.CourseForDate(req.Date)
	for round := 0; round < req.Retries; round++ {
		slots, err := h.Booker.GetTeeTimeSlots(u.APIToken, courseID, req.Date)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		beforeCutoff := slotutil.SlotsBeforeCutoff(slots, cutoffTeeTime)
		if len(beforeCutoff) == 0 {
			if round == req.Retries-1 {
				http.Error(w, "no slots available before cutoff", http.StatusNotFound)
				return
			}
			continue
		}
		for i := range beforeCutoff {
			slot := &beforeCutoff[i]
			checkInput := booker.GolfCheckTeeTimeStatusInput{
				CourseID:  slot.CourseID,
				TxnDate:   req.Date,
				Session:   slot.Session,
				TeeBox:    slot.TeeBox.String(),
				TeeTime:   slot.TeeTime,
				UserName:  u.UserName,
				IPAddress: u.UserName,
				Action:    0,
			}
			statusResp, err := h.Booker.CheckTeeTimeStatus(u.APIToken, checkInput)
			if err != nil || !statusResp.Status {
				continue
			}
			input := booker.GolfNewBooking2Input{
				CourseID:   slot.CourseID,
				TxnDate:    req.Date,
				Session:    slot.Session,
				TeeBox:     slot.TeeBox.String(),
				TeeTime:    slot.TeeTime,
				AccountID:  u.UserName,
				TotalGuest: 4,
				IPaddress:  u.UserName,
				Holes:      18,
			}
			bookResp, err := h.Booker.BookTeeTime(u.APIToken, input, false)
			if err != nil || !bookResp.Status || len(bookResp.Result) == 0 || !bookResp.Result[0].Status {
				continue
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"bookingID": bookResp.Result[0].BookingID})
			return
		}
		if round < req.Retries-1 {
			time.Sleep(time.Duration(req.RetryIntervalSec) * time.Second)
		}
	}
	http.Error(w, "no slot booked this round", http.StatusNotFound)
}
