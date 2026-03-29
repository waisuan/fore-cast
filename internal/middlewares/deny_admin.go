package middlewares

import (
	"net/http"

	"github.com/waisuan/alfred/internal/context"
)

// DenyAdmin returns 403 for users with the ADMIN role (booking and member features are disabled).
func DenyAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := context.UserFrom(r.Context())
		if u != nil && u.IsAdmin() {
			http.Error(w, "admin accounts cannot use booking features", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
