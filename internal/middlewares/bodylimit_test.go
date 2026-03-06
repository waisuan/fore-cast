package middlewares_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/waisuan/alfred/internal/middlewares"
)

func TestBodyLimit_SmallBodyPasses(t *testing.T) {
	t.Parallel()

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	})
	handler := middlewares.BodyLimit(inner)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"ok": true}`))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestBodyLimit_OversizedBodyRejected(t *testing.T) {
	t.Parallel()

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "body too large", http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	handler := middlewares.BodyLimit(inner)

	oversized := strings.Repeat("x", 2<<20) // 2 MB
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(oversized))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, rr.Code)
}
