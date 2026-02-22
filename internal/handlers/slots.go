package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/waisuan/alfred/internal/context"
	"github.com/waisuan/alfred/internal/saujana"
	"github.com/waisuan/alfred/internal/slotutil"
)

// SlotsHandler handles GET /api/v1/slots.
type SlotsHandler struct {
	Saujana *saujana.Client
}

// SlotsResponse is the response for GET slots.
type SlotsResponse struct {
	Course string                `json:"course"`
	Slots  []saujana.TeeTimeSlot `json:"slots"`
}

func (h *SlotsHandler) Slots(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	u := context.UserFrom(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	date := r.URL.Query().Get("date")
	if date == "" {
		http.Error(w, "date query required (YYYY/MM/DD)", http.StatusBadRequest)
		return
	}
	if err := slotutil.ValidateDate(date); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	courseID := slotutil.CourseForDate(date)
	slots, err := h.Saujana.GetTeeTimeSlots(u.SaujanaToken, courseID, date)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	cutoff := r.URL.Query().Get("cutoff")
	if cutoff != "" {
		cutoffTeeTime, err := slotutil.ParseCutoff(cutoff)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		slots = slotutil.SlotsBeforeCutoff(slots, cutoffTeeTime)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(SlotsResponse{Course: courseID, Slots: slots})
}
