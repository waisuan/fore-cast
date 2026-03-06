package middlewares

import (
	"net/http"
	"time"

	"github.com/waisuan/alfred/internal/logger"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// Logging returns a middleware that logs each HTTP request with method, path, status code, and latency.
// Error detail for 5xx responses is handled by handlers.internalError.
func Logging() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)
			logger.Info("http_request",
				logger.String("method", r.Method),
				logger.String("path", r.URL.Path),
				logger.Int("status", rec.status),
				logger.Duration("latency", time.Since(start).Round(time.Microsecond)))
		})
	}
}
