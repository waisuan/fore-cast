package notify

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSend_EmptyTopic(t *testing.T) {
	t.Parallel()
	err := Send("", "hello")
	assert.NoError(t, err)
}

func TestSend_Success(t *testing.T) {
	t.Parallel()
	var receivedBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 1024)
		n, _ := r.Body.Read(buf)
		receivedBody = string(buf[:n])
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	err := sendToURL(srv.URL, "test message")
	require.NoError(t, err)
	assert.Equal(t, "test message", receivedBody)
}

func TestGenerateTopic(t *testing.T) {
	t.Parallel()
	topic := GenerateTopic("M8110")
	assert.Contains(t, topic, "fore-cast-M8110-")
	assert.Len(t, topic, len("fore-cast-M8110-")+16) // 8 bytes = 16 hex chars

	topic2 := GenerateTopic("M8110")
	assert.NotEqual(t, topic, topic2, "topics should be unique")
}

func TestSend_ServerError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	err := sendToURL(srv.URL, "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}
