package middlewares_test

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/waisuan/alfred/internal/middlewares"
)

func TestLogging_LogsRequestDetails(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(nil)

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	handler := middlewares.Logging(inner)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/preset", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	logLine := buf.String()
	assert.Contains(t, logLine, "POST")
	assert.Contains(t, logLine, "/api/v1/preset")
	assert.Contains(t, logLine, "201")
}

func TestLogging_DefaultsTo200(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(nil)

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	handler := middlewares.Logging(inner)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, buf.String(), "200")
}

func TestLogging_5xx_LogsStatusOnly(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(nil)

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "connection refused", http.StatusInternalServerError)
	})
	handler := middlewares.Logging(inner)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/history", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	logLine := buf.String()
	assert.Contains(t, logLine, "500")
	assert.NotContains(t, logLine, "err=", "error detail is handled by internalError, not the logging middleware")
}
