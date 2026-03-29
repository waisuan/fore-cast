// Package user contains user-centric persistence: listing accounts, setting roles, and full account deletion.
package user

import (
	"database/sql"
	"errors"
	"time"

	appctx "github.com/waisuan/alfred/internal/context"
)

// ErrNotFound is returned when no credentials row exists for the username.
var ErrNotFound = errors.New("user not found")

// ErrInvalidRole is returned when SetRole receives a role other than ADMIN or NON_ADMIN.
var ErrInvalidRole = errors.New("invalid role")

// Summary is a non-sensitive view of a row in user_credentials for admin listing.
type Summary struct {
	UserName  string    `json:"user_name"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

// List returns all users ordered by user_name (passwords are never exposed).
func List(db *sql.DB) ([]Summary, error) {
	rows, err := db.Query(`
		SELECT user_name, role, created_at FROM user_credentials ORDER BY user_name`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []Summary
	for rows.Next() {
		var s Summary
		if err := rows.Scan(&s.UserName, &s.Role, &s.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// SetRole updates the role for an existing user.
func SetRole(db *sql.DB, userName, role string) error {
	if role != appctx.RoleAdmin && role != appctx.RoleNonAdmin {
		return ErrInvalidRole
	}
	res, err := db.Exec(`UPDATE user_credentials SET role = $1 WHERE user_name = $2`, role, userName)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteUser removes all data for the user: preset, sessions, booking history, and credentials.
// Order respects FK booking_presets -> user_credentials.
func DeleteUser(db *sql.DB, userName string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`DELETE FROM booking_presets WHERE user_name = $1`, userName); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM user_sessions WHERE user_name = $1`, userName); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM booking_attempts WHERE user_name = $1`, userName); err != nil {
		return err
	}
	res, err := tx.Exec(`DELETE FROM user_credentials WHERE user_name = $1`, userName)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return tx.Commit()
}
