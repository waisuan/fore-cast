package middlewares

import "net/http"

const maxRequestBodyBytes = 1 << 20 // 1 MB

// BodyLimit caps the size of incoming request bodies to prevent abuse.
func BodyLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
		next.ServeHTTP(w, r)
	})
}
