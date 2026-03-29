package middlewares

import (
	"net/http"

	"github.com/waisuan/alfred/internal/context"
)

// RequireAdmin returns 403 unless the authenticated user has the ADMIN role.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := context.UserFrom(r.Context())
		if u == nil || !u.IsAdmin() {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
