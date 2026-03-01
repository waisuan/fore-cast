package db

import (
	"database/sql"
	"time"
)

// ServiceInterface defines the operations available on the booking database.
// Callers should depend on this interface rather than the concrete *Service,
// allowing tests to inject a mock (e.g. via mockgen) instead of a real DB.
//
//go:generate mockgen -destination=./mock_service.go -package=db -source=db.go
type ServiceInterface interface {
	LogAttempt(a Attempt) error
	PruneAttempts(retention time.Duration) (int64, error)
	GetAttempts(userName string, limit int) ([]Attempt, error)
	UpsertPreset(p Preset) error
	GetPreset(userName string) (*Preset, error)
	GetEnabledPresets() ([]Preset, error)
}

// Service provides application-level database operations.
// It does not own the connection — callers are responsible for opening and
// closing the underlying *sql.DB (typically via deps.NewPostgresClient).
type Service struct {
	conn *sql.DB
}

// NewService wraps an existing *sql.DB with application-specific methods.
func NewService(conn *sql.DB) *Service {
	return &Service{conn: conn}
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
}

// LogAttempt inserts a booking attempt record.
func (s *Service) LogAttempt(a Attempt) error {
	_, err := s.conn.Exec(`
		INSERT INTO booking_attempts (user_name, course_id, txn_date, tee_time, tee_box, booking_id, status, message)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		a.UserName, a.CourseID, a.TxnDate, a.TeeTime, a.TeeBox, a.BookingID, a.Status, a.Message)
	return err
}

// PruneAttempts deletes booking attempts older than the retention period.
func (s *Service) PruneAttempts(retention time.Duration) (int64, error) {
	res, err := s.conn.Exec(`DELETE FROM booking_attempts WHERE created_at < NOW() - $1::interval`, retention.String())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// GetAttempts returns the most recent booking attempts for a user.
func (s *Service) GetAttempts(userName string, limit int) ([]Attempt, error) {
	rows, err := s.conn.Query(`
		SELECT id, created_at, user_name, course_id, txn_date, tee_time, tee_box, booking_id, status, message
		FROM booking_attempts
		WHERE user_name = $1
		ORDER BY created_at DESC
		LIMIT $2`, userName, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var attempts []Attempt
	for rows.Next() {
		var a Attempt
		if err := rows.Scan(&a.ID, &a.CreatedAt, &a.UserName, &a.CourseID, &a.TxnDate,
			&a.TeeTime, &a.TeeBox, &a.BookingID, &a.Status, &a.Message); err != nil {
			return nil, err
		}
		attempts = append(attempts, a)
	}
	return attempts, rows.Err()
}

// Preset represents a user's auto-booker configuration.
type Preset struct {
	ID            int
	UserName      string
	PasswordEnc   string
	UpdatedAt     time.Time
	Course        sql.NullString
	Cutoff        string
	RetryInterval int
	Timeout       string
	NtfyTopic     sql.NullString
	Enabled       bool
}

// UpsertPreset inserts or updates a booking preset for the given user.
func (s *Service) UpsertPreset(p Preset) error {
	_, err := s.conn.Exec(`
		INSERT INTO booking_presets (user_name, password_enc, course, cutoff, retry_interval, timeout, ntfy_topic, enabled, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
		ON CONFLICT (user_name) DO UPDATE SET
			password_enc = EXCLUDED.password_enc,
			course = EXCLUDED.course,
			cutoff = EXCLUDED.cutoff,
			retry_interval = EXCLUDED.retry_interval,
			timeout = EXCLUDED.timeout,
			ntfy_topic = EXCLUDED.ntfy_topic,
			enabled = EXCLUDED.enabled,
			updated_at = NOW()`,
		p.UserName, p.PasswordEnc, p.Course, p.Cutoff, p.RetryInterval, p.Timeout, p.NtfyTopic, p.Enabled)
	return err
}

// GetPreset returns the preset for a given user, or nil if not found.
func (s *Service) GetPreset(userName string) (*Preset, error) {
	var p Preset
	err := s.conn.QueryRow(`
		SELECT id, user_name, password_enc, updated_at, course, cutoff, retry_interval, timeout, ntfy_topic, enabled
		FROM booking_presets
		WHERE user_name = $1`, userName).
		Scan(&p.ID, &p.UserName, &p.PasswordEnc, &p.UpdatedAt, &p.Course, &p.Cutoff, &p.RetryInterval, &p.Timeout, &p.NtfyTopic, &p.Enabled)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// GetEnabledPresets returns all presets that are enabled.
func (s *Service) GetEnabledPresets() ([]Preset, error) {
	rows, err := s.conn.Query(`
		SELECT id, user_name, password_enc, updated_at, course, cutoff, retry_interval, timeout, ntfy_topic, enabled
		FROM booking_presets
		WHERE enabled = true
		ORDER BY user_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var presets []Preset
	for rows.Next() {
		var p Preset
		if err := rows.Scan(&p.ID, &p.UserName, &p.PasswordEnc, &p.UpdatedAt, &p.Course, &p.Cutoff,
			&p.RetryInterval, &p.Timeout, &p.NtfyTopic, &p.Enabled); err != nil {
			return nil, err
		}
		presets = append(presets, p)
	}
	return presets, rows.Err()
}
