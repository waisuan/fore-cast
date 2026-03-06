package middlewares_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"github.com/waisuan/alfred/internal/middlewares"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type ErrorMaskSuite struct {
	suite.Suite
	buf *bytes.Buffer
}

func (s *ErrorMaskSuite) SetupTest() {
	s.buf = &bytes.Buffer{}
	enc := zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig())
	core := zapcore.NewCore(enc, zapcore.AddSync(s.buf), zapcore.DebugLevel)
	zap.ReplaceGlobals(zap.New(core))
}

func (s *ErrorMaskSuite) TearDownTest() {
	zap.ReplaceGlobals(zap.NewNop())
}

func (s *ErrorMaskSuite) Test5xx_MasksBody() {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "get presets: connection refused", http.StatusInternalServerError)
	})
	handler := middlewares.ErrorMask()(inner)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/preset", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(s.T(), http.StatusInternalServerError, rr.Code)
	assert.Equal(s.T(), "internal server error\n", rr.Body.String())
	assert.Contains(s.T(), s.buf.String(), "get presets: connection refused")
	assert.Contains(s.T(), s.buf.String(), "GET")
	assert.Contains(s.T(), s.buf.String(), "/api/v1/preset")
}

func (s *ErrorMaskSuite) Test4xx_PassesThrough() {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "invalid body", http.StatusBadRequest)
	})
	handler := middlewares.ErrorMask()(inner)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/preset", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(s.T(), http.StatusBadRequest, rr.Code)
	assert.True(s.T(), strings.Contains(rr.Body.String(), "invalid body"))
}

func (s *ErrorMaskSuite) Test2xx_PassesThrough() {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	handler := middlewares.ErrorMask()(inner)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(s.T(), http.StatusOK, rr.Code)
	assert.Equal(s.T(), `{"ok":true}`, rr.Body.String())
}

func (s *ErrorMaskSuite) Test503_AlsoMasked() {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
	})
	handler := middlewares.ErrorMask()(inner)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(s.T(), http.StatusServiceUnavailable, rr.Code)
	assert.Equal(s.T(), "internal server error\n", rr.Body.String())
	assert.Contains(s.T(), s.buf.String(), "database unavailable")
}

func TestErrorMaskSuite(t *testing.T) {
	suite.Run(t, new(ErrorMaskSuite))
}
