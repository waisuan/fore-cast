package middlewares

import (
	"bytes"
	"log"
	"net/http"
	"strings"
)

type errorMasker struct {
	http.ResponseWriter
	status   int
	buf      bytes.Buffer
	hijacked bool
}

func (m *errorMasker) WriteHeader(code int) {
	m.status = code
	if code >= 500 {
		m.hijacked = true
		return
	}
	m.ResponseWriter.WriteHeader(code)
}

func (m *errorMasker) Write(b []byte) (int, error) {
	if m.hijacked {
		return m.buf.Write(b)
	}
	return m.ResponseWriter.Write(b)
}

// ErrorMask intercepts 5xx responses: the real error body is logged
// server-side and replaced with a generic message for the client.
// Handlers can freely use http.Error(w, err.Error(), 500) without
// leaking internal details.
func ErrorMask(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m := &errorMasker{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(m, r)
		if m.hijacked {
			log.Printf("%s %s: %s", r.Method, r.URL.Path, strings.TrimSpace(m.buf.String()))
			http.Error(m.ResponseWriter, "internal server error", m.status)
		}
	})
}
