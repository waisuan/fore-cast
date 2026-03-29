package middlewares_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	appctx "github.com/waisuan/alfred/internal/context"
	"github.com/waisuan/alfred/internal/middlewares"
)

func TestRequireAdmin_AllowsAdmin(t *testing.T) {
	t.Parallel()
	called := false
	h := middlewares.RequireAdmin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(appctx.WithUser(req.Context(), &appctx.User{UserName: "a", Role: appctx.RoleAdmin}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.True(t, called)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRequireAdmin_ForbidsNonAdmin(t *testing.T) {
	t.Parallel()
	h := middlewares.RequireAdmin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next should not run")
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(appctx.WithUser(req.Context(), &appctx.User{UserName: "u", Role: appctx.RoleNonAdmin}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestRequireAdmin_ForbidsNoUser(t *testing.T) {
	t.Parallel()
	h := middlewares.RequireAdmin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next should not run")
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}
