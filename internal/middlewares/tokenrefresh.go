package middlewares

import (
	"net/http"

	"github.com/waisuan/alfred/internal/booker"
	"github.com/waisuan/alfred/internal/context"
	"github.com/waisuan/alfred/internal/credentials"
	"github.com/waisuan/alfred/internal/crypto"
)

// TokenRefresh obtains a 3rd party API token on-demand when handlers need it.
// Credentials are read from the credentials table; no token is stored in session.
func TokenRefresh(booker booker.ClientInterface, creds credentials.Service, encKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u := context.UserFrom(r.Context())
			if u == nil {
				next.ServeHTTP(w, r)
				return
			}
			if u.APIToken != "" {
				next.ServeHTTP(w, r)
				return
			}
			c, err := creds.Get(u.UserName)
			if err != nil || c == nil {
				http.Error(w, "session expired — please log in again", http.StatusUnauthorized)
				return
			}
			password, err := crypto.Decrypt(c.PasswordEnc, encKey)
			if err != nil {
				http.Error(w, "session expired — please log in again", http.StatusUnauthorized)
				return
			}
			token, err := booker.Login(u.UserName, password)
			if err != nil {
				http.Error(w, "session expired — please log in again", http.StatusUnauthorized)
				return
			}
			ctx := context.WithUser(r.Context(), &context.User{
				UserName: u.UserName,
				Role:     u.Role,
				APIToken: token,
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
