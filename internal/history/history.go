package history

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// Service defines operations on booking attempt history.
//
//go:generate mockgen -destination=./mock_service.go -package=history -source=history.go
type Service interface {
	LogAttempt(a Attempt) error
	PruneAttempts(retention time.Duration) (int64, error)
	GetAttempts(userName string, limit int) ([]Attempt, error)
}

type service struct {
	conn *sql.DB
}

// NewService wraps an existing *sql.DB with history operations.
func NewService(conn *sql.DB) Service {
	return &service{conn: conn}
}

// Attempt represents a single booking attempt log entry.
type Attempt struct {
	ID        int
	CreatedAt time.Time
	UserName  string
	CourseID  string
	TxnDate   string
	TeeTime   sql.NullString
	TeeBox    sql.NullString
	BookingID sql.NullString
	Status    string
	Message   string
	// Steps is the run log as a JSON array of {at, text}; nil stores "[]".
	Steps json.RawMessage
}

func (s *service) LogAttempt(a Attempt) error {
	steps := []byte(a.Steps)
	if len(steps) == 0 {
		steps = []byte("[]")
	}
	_, err := s.conn.Exec(`
		INSERT INTO booking_attempts (user_name, course_id, txn_date, tee_time, tee_box, booking_id, status, message, steps)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		a.UserName, a.CourseID, a.TxnDate, a.TeeTime, a.TeeBox, a.BookingID, a.Status, a.Message, steps)
	return err
}

func (s *service) PruneAttempts(retention time.Duration) (int64, error) {
	// Use explicit PostgreSQL interval format (e.g. "30 days") instead of Go duration string.
	interval := fmt.Sprintf("%d days", int(retention.Hours()/24))
	if interval == "0 days" {
		interval = "1 hour" // minimum to avoid empty/zero interval
	}
	res, err := s.conn.Exec(`DELETE FROM booking_attempts WHERE created_at < NOW() - $1::interval`, interval)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *service) GetAttempts(userName string, limit int) ([]Attempt, error) {
	rows, err := s.conn.Query(`
		SELECT id, created_at, user_name, course_id, txn_date, tee_time, tee_box, booking_id, status, message, steps
		FROM booking_attempts
		WHERE user_name = $1
		ORDER BY created_at DESC
		LIMIT $2`, userName, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var attempts []Attempt
	for rows.Next() {
		var a Attempt
		if err := rows.Scan(&a.ID, &a.CreatedAt, &a.UserName, &a.CourseID, &a.TxnDate,
			&a.TeeTime, &a.TeeBox, &a.BookingID, &a.Status, &a.Message, &a.Steps); err != nil {
			return nil, err
		}
		attempts = append(attempts, a)
	}
	return attempts, rows.Err()
}
