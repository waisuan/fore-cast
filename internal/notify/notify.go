package notify

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Service sends push notifications via ntfy.sh.
//
//go:generate mockgen -destination=./mock_service.go -package=notify -source=notify.go
type Service interface {
	Send(topic, message string) error
}

type service struct {
	client  *http.Client
	baseURL string
}

// NewService returns a notification service with the given base URL and HTTP timeout.
func NewService(baseURL string, timeout time.Duration) Service {
	return &service{client: &http.Client{Timeout: timeout}, baseURL: baseURL}
}

// Send publishes a message to the given ntfy.sh topic.
// If topic is empty, it's a no-op and returns nil.
func (s *service) Send(topic, message string) error {
	if topic == "" {
		return nil
	}
	resp, err := s.client.Post(s.baseURL+"/"+topic, "text/plain", strings.NewReader(message))
	if err != nil {
		return fmt.Errorf("ntfy: %w", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("ntfy: HTTP %d", resp.StatusCode)
	}
	return nil
}

// GenerateTopic creates a unique ntfy topic for the given user.
func GenerateTopic(userName string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("fore-cast-%s-%s", userName, hex.EncodeToString(b))
}
