package session

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"time"
)

// Data holds session data for an authenticated user. Session stores only
// identity; 3rd party tokens are obtained on-demand when needed.
type Data struct {
	UserName  string
	ExpiresAt time.Time
}

// Store persists sessions in Postgres with TTL; expired rows are removed by a
// background reaper and ignored on read.
type Store struct {
	db   *sql.DB
	ttl  time.Duration
	stop chan struct{}
}

// NewStore returns a new session store backed by Postgres with the given TTL.
// A background goroutine deletes expired rows every TTL/2 (min 1 minute).
func NewStore(db *sql.DB, ttl time.Duration) *Store {
	s := &Store{
		db:   db,
		ttl:  ttl,
		stop: make(chan struct{}),
	}
	go s.reapLoop()
	return s
}

// Close stops the background reaper goroutine.
func (s *Store) Close() {
	select {
	case <-s.stop:
	default:
		close(s.stop)
	}
}

func (s *Store) reapLoop() {
	interval := s.ttl / 2
	if interval < time.Minute {
		interval = time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			_, _ = s.db.Exec(`DELETE FROM user_sessions WHERE expires_at < NOW()`)
		case <-s.stop:
			return
		}
	}
}

// Create creates a new session and returns its ID.
func (s *Store) Create(userName string) (string, error) {
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		return "", err
	}
	sid := hex.EncodeToString(id)
	expiresAt := time.Now().Add(s.ttl)
	_, err := s.db.Exec(
		`INSERT INTO user_sessions (id, user_name, expires_at) VALUES ($1, $2, $3)`,
		sid, userName, expiresAt,
	)
	if err != nil {
		return "", err
	}
	return sid, nil
}

// Get returns session data if the session exists and has not expired.
func (s *Store) Get(sessionID string) *Data {
	var userName string
	var expiresAt time.Time
	err := s.db.QueryRow(
		`SELECT user_name, expires_at FROM user_sessions WHERE id = $1 AND expires_at > NOW()`,
		sessionID,
	).Scan(&userName, &expiresAt)
	if err != nil {
		return nil
	}
	return &Data{UserName: userName, ExpiresAt: expiresAt}
}

// Delete removes a session.
func (s *Store) Delete(sessionID string) {
	_, _ = s.db.Exec(`DELETE FROM user_sessions WHERE id = $1`, sessionID)
}

// TTL returns the session time-to-live.
func (s *Store) TTL() time.Duration {
	return s.ttl
}
