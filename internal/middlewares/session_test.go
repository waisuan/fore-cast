package middlewares_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/waisuan/alfred/internal/booker"
	"github.com/waisuan/alfred/internal/context"
	"github.com/waisuan/alfred/internal/credentials"
	"github.com/waisuan/alfred/internal/crypto"
	"github.com/waisuan/alfred/internal/middlewares"
	"github.com/waisuan/alfred/internal/session"
)

func newStoreWithSession(t *testing.T) (*session.Store, string) {
	t.Helper()
	store := session.NewStore(1 * time.Hour)
	sid, err := store.Create("alice")
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
	_, _ = w.Write([]byte(u.UserName + ":" + u.APIToken))
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
	assert.Equal(t, "alice:", rec.Body.String())
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
	sid, err := store.Create("bob")
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

func TestTokenRefresh_ObtainsTokenFromCredentials(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mockBooker := booker.NewMockClientInterface(ctrl)
	mockBooker.EXPECT().
		Login("alice", "pw123").
		Return("fresh-token", nil)

	encKey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	enc, err := crypto.Encrypt("pw123", encKey)
	require.NoError(t, err)

	mockCreds := credentials.NewMockService(ctrl)
	mockCreds.EXPECT().
		Get("alice").
		Return(&credentials.Credential{UserName: "alice", PasswordEnc: enc}, nil)

	store := session.NewStore(1 * time.Hour)
	sid, err := store.Create("alice")
	require.NoError(t, err)

	chain := middlewares.SessionAuth(store)(
		middlewares.TokenRefresh(mockBooker, mockCreds, encKey)(http.HandlerFunc(echoUser)),
	)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: middlewares.SessionCookieName(), Value: sid})
	rec := httptest.NewRecorder()
	chain.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "alice:fresh-token", rec.Body.String())
}
