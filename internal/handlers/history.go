package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/waisuan/alfred/internal/context"
	"github.com/waisuan/alfred/internal/history"
)

// HistoryHandler handles GET /api/v1/history.
type HistoryHandler struct {
	Service history.Service
}

// HistoryResponse is the JSON response for the history endpoint.
type HistoryResponse struct {
	Attempts []HistoryItem `json:"attempts"`
}

// HistoryItem is a single booking attempt in the response.
type HistoryItem struct {
	ID        int    `json:"id"`
	CreatedAt string `json:"created_at"`
	CourseID  string `json:"course_id"`
	TxnDate   string `json:"txn_date"`
	TeeTime   string `json:"tee_time,omitempty"`
	TeeBox    string `json:"tee_box,omitempty"`
	BookingID string `json:"booking_id,omitempty"`
	Status    string `json:"status"`
	Message   string `json:"message"`
}

// GetHistory handles GET /api/v1/history.
func (h *HistoryHandler) GetHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	u := context.UserFrom(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	attempts, err := h.Service.GetAttempts(u.UserName, 50)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	items := make([]HistoryItem, 0, len(attempts))
	for _, a := range attempts {
		items = append(items, HistoryItem{
			ID:        a.ID,
			CreatedAt: a.CreatedAt.Format("2006-01-02T15:04:05Z"),
			CourseID:  a.CourseID,
			TxnDate:   a.TxnDate,
			TeeTime:   a.TeeTime.String,
			TeeBox:    a.TeeBox.String,
			BookingID: a.BookingID.String,
			Status:    a.Status,
			Message:   a.Message,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(HistoryResponse{Attempts: items})
}
