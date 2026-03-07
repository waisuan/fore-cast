package credentials

import (
	"database/sql"
	"errors"
)

// Service defines operations on user credentials.
//
//go:generate mockgen -destination=./mock_service.go -package=credentials -source=credentials.go
type Service interface {
	Get(userName string) (*Credential, error)
	Upsert(userName, passwordEnc string) error
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
}

func (s *service) Get(userName string) (*Credential, error) {
	var c Credential
	err := s.conn.QueryRow(`
		SELECT user_name, password_enc FROM user_credentials WHERE user_name = $1`,
		userName).Scan(&c.UserName, &c.PasswordEnc)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *service) Upsert(userName, passwordEnc string) error {
	_, err := s.conn.Exec(`
		INSERT INTO user_credentials (user_name, password_enc) VALUES ($1, $2)
		ON CONFLICT (user_name) DO UPDATE SET password_enc = EXCLUDED.password_enc`,
		userName, passwordEnc)
	return err
}
