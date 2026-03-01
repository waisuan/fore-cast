package notify

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
)

// Send publishes a message to the given ntfy.sh topic.
// If topic is empty, it's a no-op and returns nil.
func Send(topic, message string) error {
	if topic == "" {
		return nil
	}
	return sendToURL("https://ntfy.sh/"+topic, message)
}

// GenerateTopic creates a unique ntfy topic for the given user.
func GenerateTopic(userName string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("fore-cast-%s-%s", userName, hex.EncodeToString(b))
}

func sendToURL(url, message string) error {
	resp, err := http.Post(url, "text/plain", strings.NewReader(message))
	if err != nil {
		return fmt.Errorf("ntfy: %w", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("ntfy: HTTP %d", resp.StatusCode)
	}
	return nil
}
