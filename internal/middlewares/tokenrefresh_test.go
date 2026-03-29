package middlewares_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/waisuan/alfred/internal/booker"
	appctx "github.com/waisuan/alfred/internal/context"
	"github.com/waisuan/alfred/internal/credentials"
	"github.com/waisuan/alfred/internal/crypto"
	"github.com/waisuan/alfred/internal/middlewares"
)

func tokenRefreshEcho(w http.ResponseWriter, r *http.Request) {
	u := appctx.UserFrom(r.Context())
	if u == nil {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("nil"))
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(u.UserName + ":" + u.Role + ":" + u.APIToken))
}

func TestTokenRefresh_NilUserPassesThrough(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mockBooker := booker.NewMockClientInterface(ctrl)
	mockCreds := credentials.NewMockService(ctrl)

	h := middlewares.TokenRefresh(mockBooker, mockCreds, tokenRefreshEncKey)(http.HandlerFunc(tokenRefreshEcho))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "nil", rec.Body.String())
}

func TestTokenRefresh_SkipsWhenTokenAlreadySet(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mockBooker := booker.NewMockClientInterface(ctrl)
	mockCreds := credentials.NewMockService(ctrl)

	h := middlewares.TokenRefresh(mockBooker, mockCreds, tokenRefreshEncKey)(http.HandlerFunc(tokenRefreshEcho))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(appctx.WithUser(req.Context(), &appctx.User{
		UserName: "alice",
		Role:     appctx.RoleNonAdmin,
		APIToken: "existing",
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "alice:NON_ADMIN:existing", rec.Body.String())
}

func TestTokenRefresh_GetError(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mockBooker := booker.NewMockClientInterface(ctrl)
	mockCreds := credentials.NewMockService(ctrl)
	mockCreds.EXPECT().Get("alice").Return(nil, errors.New("db error"))

	h := middlewares.TokenRefresh(mockBooker, mockCreds, tokenRefreshEncKey)(http.HandlerFunc(tokenRefreshEcho))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(appctx.WithUser(req.Context(), &appctx.User{UserName: "alice", Role: appctx.RoleNonAdmin}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "session expired")
}

func TestTokenRefresh_GetNilCredential(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mockBooker := booker.NewMockClientInterface(ctrl)
	mockCreds := credentials.NewMockService(ctrl)
	mockCreds.EXPECT().Get("alice").Return(nil, nil)

	h := middlewares.TokenRefresh(mockBooker, mockCreds, tokenRefreshEncKey)(http.HandlerFunc(tokenRefreshEcho))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(appctx.WithUser(req.Context(), &appctx.User{UserName: "alice", Role: appctx.RoleAdmin}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestTokenRefresh_DecryptFails(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mockBooker := booker.NewMockClientInterface(ctrl)
	mockCreds := credentials.NewMockService(ctrl)
	mockCreds.EXPECT().Get("alice").Return(&credentials.Credential{
		UserName:    "alice",
		PasswordEnc: "not-valid-ciphertext",
		Role:        appctx.RoleNonAdmin,
	}, nil)

	wrongKey := "0000000000000000000000000000000000000000000000000000000000000000"
	h := middlewares.TokenRefresh(mockBooker, mockCreds, wrongKey)(http.HandlerFunc(tokenRefreshEcho))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(appctx.WithUser(req.Context(), &appctx.User{UserName: "alice", Role: appctx.RoleNonAdmin}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestTokenRefresh_LoginFails(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mockBooker := booker.NewMockClientInterface(ctrl)
	mockBooker.EXPECT().Login("alice", "pw123").Return("", errors.New("club down"))
	mockCreds := credentials.NewMockService(ctrl)
	enc, err := crypto.Encrypt("pw123", tokenRefreshEncKey)
	assert.NoError(t, err)
	mockCreds.EXPECT().Get("alice").Return(&credentials.Credential{
		UserName:    "alice",
		PasswordEnc: enc,
		Role:        appctx.RoleNonAdmin,
	}, nil)

	h := middlewares.TokenRefresh(mockBooker, mockCreds, tokenRefreshEncKey)(http.HandlerFunc(tokenRefreshEcho))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(appctx.WithUser(req.Context(), &appctx.User{UserName: "alice", Role: appctx.RoleNonAdmin}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestTokenRefresh_SuccessPreservesRoleAndSetsToken(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mockBooker := booker.NewMockClientInterface(ctrl)
	mockBooker.EXPECT().Login("alice", "pw123").Return("fresh-token", nil)
	mockCreds := credentials.NewMockService(ctrl)
	enc, err := crypto.Encrypt("pw123", tokenRefreshEncKey)
	assert.NoError(t, err)
	mockCreds.EXPECT().Get("alice").Return(&credentials.Credential{
		UserName:    "alice",
		PasswordEnc: enc,
		Role:        appctx.RoleAdmin,
	}, nil)

	h := middlewares.TokenRefresh(mockBooker, mockCreds, tokenRefreshEncKey)(http.HandlerFunc(tokenRefreshEcho))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(appctx.WithUser(req.Context(), &appctx.User{UserName: "alice", Role: appctx.RoleAdmin}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "alice:ADMIN:fresh-token", rec.Body.String())
}
