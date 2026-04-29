package preset

import (
	"database/sql"
	"errors"
	"time"
)

// ErrCancelNotRunning is returned when cancel is requested but the preset is not in a running state.
var ErrCancelNotRunning = errors.New("scheduler is not running for this account")

// ErrPresetNotFound is returned when deleting a preset that does not exist.
var ErrPresetNotFound = errors.New("preset not found")

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
	ClearCourseOverride(userName string) error
	DeleteByUserName(userName string) error
}

type service struct {
	conn *sql.DB
}

// NewService wraps an existing *sql.DB with preset operations.
func NewService(conn *sql.DB) Service {
	return &service{conn: conn}
}

// Preset represents a user's auto-booker configuration. Credentials are stored
// in user_credentials; preset references user_name. See ResolveOverride for the
// OverrideCourse / OverrideUntil state machine.
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
	OverrideCourse  sql.NullString
	OverrideUntil   sql.NullTime
}

func (s *service) UpsertPreset(p Preset) error {
	_, err := s.conn.Exec(`
		INSERT INTO booking_presets (
			user_name, course, cutoff, retry_interval, timeout, ntfy_topic, enabled,
			override_course, override_until, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
		ON CONFLICT (user_name) DO UPDATE SET
			course = EXCLUDED.course,
			cutoff = EXCLUDED.cutoff,
			retry_interval = EXCLUDED.retry_interval,
			timeout = EXCLUDED.timeout,
			ntfy_topic = EXCLUDED.ntfy_topic,
			enabled = EXCLUDED.enabled,
			override_course = EXCLUDED.override_course,
			override_until = EXCLUDED.override_until,
			updated_at = NOW()`,
		p.UserName, p.Course, p.Cutoff, p.RetryInterval, p.Timeout, p.NtfyTopic, p.Enabled,
		p.OverrideCourse, p.OverrideUntil)
	return err
}

func (s *service) GetPreset(userName string) (*Preset, error) {
	var p Preset
	err := s.conn.QueryRow(`
		SELECT id, user_name, updated_at, course, cutoff, retry_interval, timeout, ntfy_topic, enabled,
		       last_run_status, last_run_message, last_run_at, cancel_requested,
		       override_course, override_until
		FROM booking_presets
		WHERE user_name = $1`, userName).
		Scan(&p.ID, &p.UserName, &p.UpdatedAt, &p.Course, &p.Cutoff, &p.RetryInterval, &p.Timeout, &p.NtfyTopic, &p.Enabled,
			&p.LastRunStatus, &p.LastRunMessage, &p.LastRunAt, &p.CancelRequested,
			&p.OverrideCourse, &p.OverrideUntil)
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
		       last_run_status, last_run_message, last_run_at, cancel_requested,
		       override_course, override_until
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
			&p.LastRunStatus, &p.LastRunMessage, &p.LastRunAt, &p.CancelRequested,
			&p.OverrideCourse, &p.OverrideUntil); err != nil {
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

// ClearCourseOverride wipes the temporary course override for a user. Called by
// the scheduler after a "next run only" run completes, or when an override has
// expired by the time the run starts.
func (s *service) ClearCourseOverride(userName string) error {
	_, err := s.conn.Exec(`
		UPDATE booking_presets
		SET override_course = NULL, override_until = NULL
		WHERE user_name = $1`,
		userName)
	return err
}

// OverrideState describes how a temporary override should be applied for a run:
//   - None: no override stored, use the default course.
//   - Active: override stored with a future expiry, use it.
//   - Once: override stored with no expiry — use it then clear it.
//   - Expired: override stored but past its expiry — clear it and use the default.
type OverrideState int

const (
	OverrideNone OverrideState = iota
	OverrideActive
	OverrideOnce
	OverrideExpired
)

// ResolveOverride evaluates the override fields against `now` (typically the
// scheduler's wall clock) and returns whether an override applies and the
// resulting course string (empty if no override / use default).
func ResolveOverride(p Preset, now time.Time) (state OverrideState, course string) {
	if !p.OverrideCourse.Valid || p.OverrideCourse.String == "" {
		return OverrideNone, ""
	}
	if !p.OverrideUntil.Valid {
		return OverrideOnce, p.OverrideCourse.String
	}
	if now.After(p.OverrideUntil.Time) {
		return OverrideExpired, ""
	}
	return OverrideActive, p.OverrideCourse.String
}

func (s *service) DeleteByUserName(userName string) error {
	res, err := s.conn.Exec(`DELETE FROM booking_presets WHERE user_name = $1`, userName)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrPresetNotFound
	}
	return nil
}
