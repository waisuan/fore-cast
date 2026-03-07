package preset

import (
	"database/sql"
	"errors"
	"time"
)

const (
	DefaultCutoff        = "8:15"
	DefaultRetryInterval = "1s"
	DefaultTimeout       = "10m"
	MinRetryInterval     = "100ms"
)

// MinRetryIntervalDuration is the minimum allowed retry interval (100ms).
const MinRetryIntervalDuration time.Duration = 100 * time.Millisecond

// Service defines operations on booking presets.
//
//go:generate mockgen -destination=./mock_service.go -package=preset -source=preset.go
type Service interface {
	UpsertPreset(p Preset) error
	GetPreset(userName string) (*Preset, error)
	GetEnabledPresets() ([]Preset, error)
	UpdatePresetRunStatus(userName string, status RunStatus, message string) error
}

type service struct {
	conn *sql.DB
}

// NewService wraps an existing *sql.DB with preset operations.
func NewService(conn *sql.DB) Service {
	return &service{conn: conn}
}

// Preset represents a user's auto-booker configuration.
type Preset struct {
	ID             int
	UserName       string
	PasswordEnc    string
	UpdatedAt      time.Time
	Course         sql.NullString
	Cutoff         string
	RetryInterval  string
	Timeout        string
	NtfyTopic      sql.NullString
	Enabled        bool
	LastRunStatus  string
	LastRunMessage string
	LastRunAt      sql.NullTime
}

func (s *service) UpsertPreset(p Preset) error {
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

func (s *service) GetPreset(userName string) (*Preset, error) {
	var p Preset
	err := s.conn.QueryRow(`
		SELECT id, user_name, password_enc, updated_at, course, cutoff, retry_interval, timeout, ntfy_topic, enabled,
		       last_run_status, last_run_message, last_run_at
		FROM booking_presets
		WHERE user_name = $1`, userName).
		Scan(&p.ID, &p.UserName, &p.PasswordEnc, &p.UpdatedAt, &p.Course, &p.Cutoff, &p.RetryInterval, &p.Timeout, &p.NtfyTopic, &p.Enabled,
			&p.LastRunStatus, &p.LastRunMessage, &p.LastRunAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *service) GetEnabledPresets() ([]Preset, error) {
	rows, err := s.conn.Query(`
		SELECT id, user_name, password_enc, updated_at, course, cutoff, retry_interval, timeout, ntfy_topic, enabled,
		       last_run_status, last_run_message, last_run_at
		FROM booking_presets
		WHERE enabled = true
		ORDER BY user_name`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var presets []Preset
	for rows.Next() {
		var p Preset
		if err := rows.Scan(&p.ID, &p.UserName, &p.PasswordEnc, &p.UpdatedAt, &p.Course, &p.Cutoff,
			&p.RetryInterval, &p.Timeout, &p.NtfyTopic, &p.Enabled,
			&p.LastRunStatus, &p.LastRunMessage, &p.LastRunAt); err != nil {
			return nil, err
		}
		presets = append(presets, p)
	}
	return presets, rows.Err()
}

func (s *service) UpdatePresetRunStatus(userName string, status RunStatus, message string) error {
	_, err := s.conn.Exec(`
		UPDATE booking_presets
		SET last_run_status = $2, last_run_message = $3, last_run_at = NOW()
		WHERE user_name = $1`,
		userName, status, message)
	return err
}
