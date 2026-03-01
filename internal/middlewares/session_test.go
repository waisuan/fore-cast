package middlewares_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/waisuan/alfred/internal/context"
	"github.com/waisuan/alfred/internal/middlewares"
	"github.com/waisuan/alfred/internal/session"
)

func newStoreWithSession(t *testing.T) (*session.Store, string) {
	t.Helper()
	store := session.NewStore(1 * time.Hour)
	sid, err := store.Create("token-abc", "alice", "pw123")
	require.NoError(t, err)
	return store, sid
}

func echoUser(w http.ResponseWriter, r *http.Request) {
	u := context.UserFrom(r.Context())
	if u == nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(u.UserName + ":" + u.APIToken + ":" + u.Password))
}

func TestSessionAuth_ValidSession(t *testing.T) {
	t.Parallel()
	store, sid := newStoreWithSession(t)
	handler := middlewares.SessionAuth(store)(http.HandlerFunc(echoUser))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: middlewares.SessionCookieName(), Value: sid})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "alice:token-abc:pw123", rec.Body.String())
}

func TestSessionAuth_NoCookie(t *testing.T) {
	t.Parallel()
	store := session.NewStore(1 * time.Hour)
	handler := middlewares.SessionAuth(store)(http.HandlerFunc(echoUser))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestSessionAuth_EmptyCookieValue(t *testing.T) {
	t.Parallel()
	store := session.NewStore(1 * time.Hour)
	handler := middlewares.SessionAuth(store)(http.HandlerFunc(echoUser))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: middlewares.SessionCookieName(), Value: ""})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestSessionAuth_UnknownSessionID(t *testing.T) {
	t.Parallel()
	store := session.NewStore(1 * time.Hour)
	handler := middlewares.SessionAuth(store)(http.HandlerFunc(echoUser))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: middlewares.SessionCookieName(), Value: "nonexistent"})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestSessionAuth_ExpiredSession(t *testing.T) {
	t.Parallel()
	store := session.NewStore(1 * time.Millisecond)
	sid, err := store.Create("token", "bob", "pw")
	require.NoError(t, err)

	time.Sleep(5 * time.Millisecond)

	handler := middlewares.SessionAuth(store)(http.HandlerFunc(echoUser))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: middlewares.SessionCookieName(), Value: sid})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestSessionCookieName(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "session", middlewares.SessionCookieName())
}
