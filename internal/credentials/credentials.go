package credentials

import (
	"database/sql"
	"errors"

	"github.com/waisuan/alfred/internal/context"
)

// Service defines operations on user credentials.
//
//go:generate mockgen -destination=./mock_service.go -package=credentials -source=credentials.go
type Service interface {
	Get(userName string) (*Credential, error)
	Upsert(userName, passwordEnc string, role string) error
}

type service struct {
	conn *sql.DB
}

// NewService wraps an existing *sql.DB with credentials operations.
func NewService(conn *sql.DB) Service {
	return &service{conn: conn}
}

// Credential holds encrypted credentials for a user.
type Credential struct {
	UserName    string
	PasswordEnc string
	Role        string
}

func (s *service) Get(userName string) (*Credential, error) {
	var c Credential
	err := s.conn.QueryRow(`
		SELECT user_name, password_enc, role FROM user_credentials WHERE user_name = $1`,
		userName).Scan(&c.UserName, &c.PasswordEnc, &c.Role)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *service) Upsert(userName, passwordEnc, role string) error {
	if role == "" {
		role = context.RoleNonAdmin
	}
	_, err := s.conn.Exec(`
		INSERT INTO user_credentials (user_name, password_enc, role) VALUES ($1, $2, $3)
		ON CONFLICT (user_name) DO UPDATE SET password_enc = EXCLUDED.password_enc`,
		userName, passwordEnc, role)
	return err
}
