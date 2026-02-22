package middlewares

import (
	"net/http"

	"github.com/waisuan/alfred/internal/context"
	"github.com/waisuan/alfred/internal/session"
)

const sessionCookieName = "session"

// SessionAuth validates the session cookie and sets user in request context.
func SessionAuth(store *session.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(sessionCookieName)
			if err != nil || cookie == nil || cookie.Value == "" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			data := store.Get(cookie.Value)
			if data == nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			ctx := context.WithUser(r.Context(), &context.User{
				UserName:     data.UserName,
				SaujanaToken: data.SaujanaToken,
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// SessionCookieName returns the cookie name used for sessions (for handlers to set/clear).
func SessionCookieName() string { return sessionCookieName }
