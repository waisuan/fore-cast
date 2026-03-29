package preset

import (
	"database/sql"
	"errors"
	"time"
)

// ErrCancelNotRunning is returned when cancel is requested but the preset is not in a running state.
var ErrCancelNotRunning = errors.New("scheduler is not running for this account")

const (
	DefaultCutoff        = "8:15"
	DefaultRetryInterval = "1s"
	DefaultTimeout       = "10m"
	MinRetryInterval     = "0s"
)

// MinRetryIntervalDuration is the minimum allowed retry interval (0s).
const MinRetryIntervalDuration time.Duration = 0

// Service defines operations on booking presets.
//
//go:generate mockgen -destination=./mock_service.go -package=preset -source=preset.go
type Service interface {
	UpsertPreset(p Preset) error
	GetPreset(userName string) (*Preset, error)
	GetEnabledPresets() ([]Preset, error)
	UpdatePresetRunStatus(userName string, status RunStatus, message string) error
	RequestCancelRun(userName string) error
	ClearCancelRequested(userName string) error
}

type service struct {
	conn *sql.DB
}

// NewService wraps an existing *sql.DB with preset operations.
func NewService(conn *sql.DB) Service {
	return &service{conn: conn}
}

// Preset represents a user's auto-booker configuration. Credentials are stored
// in user_credentials; preset references user_name.
type Preset struct {
	ID              int
	UserName        string
	UpdatedAt       time.Time
	Course          sql.NullString
	Cutoff          string
	RetryInterval   string
	Timeout         string
	NtfyTopic       sql.NullString
	Enabled         bool
	LastRunStatus   string
	LastRunMessage  string
	LastRunAt       sql.NullTime
	CancelRequested bool
}

func (s *service) UpsertPreset(p Preset) error {
	_, err := s.conn.Exec(`
		INSERT INTO booking_presets (user_name, course, cutoff, retry_interval, timeout, ntfy_topic, enabled, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
		ON CONFLICT (user_name) DO UPDATE SET
			course = EXCLUDED.course,
			cutoff = EXCLUDED.cutoff,
			retry_interval = EXCLUDED.retry_interval,
			timeout = EXCLUDED.timeout,
			ntfy_topic = EXCLUDED.ntfy_topic,
			enabled = EXCLUDED.enabled,
			updated_at = NOW()`,
		p.UserName, p.Course, p.Cutoff, p.RetryInterval, p.Timeout, p.NtfyTopic, p.Enabled)
	return err
}

func (s *service) GetPreset(userName string) (*Preset, error) {
	var p Preset
	err := s.conn.QueryRow(`
		SELECT id, user_name, updated_at, course, cutoff, retry_interval, timeout, ntfy_topic, enabled,
		       last_run_status, last_run_message, last_run_at, cancel_requested
		FROM booking_presets
		WHERE user_name = $1`, userName).
		Scan(&p.ID, &p.UserName, &p.UpdatedAt, &p.Course, &p.Cutoff, &p.RetryInterval, &p.Timeout, &p.NtfyTopic, &p.Enabled,
			&p.LastRunStatus, &p.LastRunMessage, &p.LastRunAt, &p.CancelRequested)
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
		SELECT id, user_name, updated_at, course, cutoff, retry_interval, timeout, ntfy_topic, enabled,
		       last_run_status, last_run_message, last_run_at, cancel_requested
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
		if err := rows.Scan(&p.ID, &p.UserName, &p.UpdatedAt, &p.Course, &p.Cutoff,
			&p.RetryInterval, &p.Timeout, &p.NtfyTopic, &p.Enabled,
			&p.LastRunStatus, &p.LastRunMessage, &p.LastRunAt, &p.CancelRequested); err != nil {
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

func (s *service) RequestCancelRun(userName string) error {
	res, err := s.conn.Exec(`
		UPDATE booking_presets
		SET cancel_requested = true
		WHERE user_name = $1 AND last_run_status = $2`,
		userName, RunStatusRunning)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrCancelNotRunning
	}
	return nil
}

func (s *service) ClearCancelRequested(userName string) error {
	_, err := s.conn.Exec(`
		UPDATE booking_presets
		SET cancel_requested = false
		WHERE user_name = $1`,
		userName)
	return err
}
