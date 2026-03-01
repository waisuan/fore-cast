package middlewares_test

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/waisuan/alfred/internal/middlewares"
)

func TestErrorMask_5xx_MasksBody(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(nil)

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "get presets: connection refused", http.StatusInternalServerError)
	})
	handler := middlewares.ErrorMask(inner)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/preset", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Equal(t, "internal server error\n", rr.Body.String())
	assert.Contains(t, buf.String(), "get presets: connection refused")
	assert.Contains(t, buf.String(), "GET /api/v1/preset")
}

func TestErrorMask_4xx_PassesThrough(t *testing.T) {
	t.Parallel()

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "invalid body", http.StatusBadRequest)
	})
	handler := middlewares.ErrorMask(inner)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/preset", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.True(t, strings.Contains(rr.Body.String(), "invalid body"))
}

func TestErrorMask_2xx_PassesThrough(t *testing.T) {
	t.Parallel()

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	handler := middlewares.ErrorMask(inner)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, `{"ok":true}`, rr.Body.String())
}

func TestErrorMask_503_AlsoMasked(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(nil)

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
	})
	handler := middlewares.ErrorMask(inner)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
	assert.Equal(t, "internal server error\n", rr.Body.String())
	assert.Contains(t, buf.String(), "database unavailable")
}
