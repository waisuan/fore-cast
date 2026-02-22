package session

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// Data holds session data for an authenticated user.
type Data struct {
	SaujanaToken string
	UserName     string
	ExpiresAt    time.Time
}

// Store is an in-memory session store with TTL.
type Store struct {
	mu       sync.RWMutex
	sessions map[string]*Data
	ttl      time.Duration
}

// NewStore returns a new session store with the given TTL.
func NewStore(ttl time.Duration) *Store {
	return &Store{
		sessions: make(map[string]*Data),
		ttl:      ttl,
	}
}

// Create creates a new session and returns its ID.
func (s *Store) Create(saujanaToken, userName string) (string, error) {
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		return "", err
	}
	sid := hex.EncodeToString(id)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[sid] = &Data{
		SaujanaToken: saujanaToken,
		UserName:     userName,
		ExpiresAt:    time.Now().Add(s.ttl),
	}
	return sid, nil
}

// Get returns session data if the session exists and has not expired.
func (s *Store) Get(sessionID string) *Data {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d := s.sessions[sessionID]
	if d == nil || time.Now().After(d.ExpiresAt) {
		return nil
	}
	return d
}

// Delete removes a session.
func (s *Store) Delete(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, sessionID)
}
