package middlewares

import (
	"bytes"
	"log"
	"net/http"
	"strings"
	"time"
)

type statusRecorder struct {
	http.ResponseWriter
	status  int
	errBody bytes.Buffer
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.status >= 500 {
		r.errBody.Write(b)
	}
	return r.ResponseWriter.Write(b)
}

// Logging logs each HTTP request with method, path, status code, and latency.
// For 5xx responses the error body is included in the log line.
func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		elapsed := time.Since(start).Round(time.Microsecond)
		if rec.status >= 500 {
			log.Printf("%s %s %d %s err=%q", r.Method, r.URL.Path, rec.status, elapsed, strings.TrimSpace(rec.errBody.String()))
		} else {
			log.Printf("%s %s %d %s", r.Method, r.URL.Path, rec.status, elapsed)
		}
	})
}
