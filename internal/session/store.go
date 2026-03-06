package session

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// Data holds session data for an authenticated user.
type Data struct {
	APIToken  string
	UserName  string
	Password  string
	ExpiresAt time.Time
}

// Store is an in-memory session store with TTL.
type Store struct {
	mu       sync.RWMutex
	sessions map[string]*Data
	ttl      time.Duration
	stop     chan struct{}
}

// NewStore returns a new session store with the given TTL. A background
// goroutine evicts expired sessions every TTL/2 (min 1 minute).
func NewStore(ttl time.Duration) *Store {
	s := &Store{
		sessions: make(map[string]*Data),
		ttl:      ttl,
		stop:     make(chan struct{}),
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
			s.evictExpired()
		case <-s.stop:
			return
		}
	}
}

func (s *Store) evictExpired() {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, d := range s.sessions {
		if now.After(d.ExpiresAt) {
			delete(s.sessions, id)
		}
	}
}

// Create creates a new session and returns its ID.
func (s *Store) Create(apiToken, userName, password string) (string, error) {
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		return "", err
	}
	sid := hex.EncodeToString(id)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[sid] = &Data{
		APIToken:  apiToken,
		UserName:  userName,
		Password:  password,
		ExpiresAt: time.Now().Add(s.ttl),
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

// TTL returns the session time-to-live.
func (s *Store) TTL() time.Duration {
	return s.ttl
}
