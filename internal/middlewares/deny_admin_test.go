package middlewares_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	appctx "github.com/waisuan/alfred/internal/context"
	"github.com/waisuan/alfred/internal/middlewares"
)

func TestDenyAdmin_BlocksAdmin(t *testing.T) {
	t.Parallel()
	called := false
	h := middlewares.DenyAdmin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(appctx.WithUser(req.Context(), &appctx.User{UserName: "a", Role: appctx.RoleAdmin}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.False(t, called)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestDenyAdmin_AllowsMember(t *testing.T) {
	t.Parallel()
	called := false
	h := middlewares.DenyAdmin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(appctx.WithUser(req.Context(), &appctx.User{UserName: "m", Role: appctx.RoleNonAdmin}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.True(t, called)
	assert.Equal(t, http.StatusOK, rec.Code)
}
