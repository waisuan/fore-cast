package middlewares_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"github.com/waisuan/alfred/internal/middlewares"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type LoggingSuite struct {
	suite.Suite
	buf *bytes.Buffer
}

func (s *LoggingSuite) SetupTest() {
	s.buf = &bytes.Buffer{}
	enc := zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig())
	core := zapcore.NewCore(enc, zapcore.AddSync(s.buf), zapcore.DebugLevel)
	zap.ReplaceGlobals(zap.New(core))
}

func (s *LoggingSuite) TearDownTest() {
	zap.ReplaceGlobals(zap.NewNop())
}

func (s *LoggingSuite) TestLogsRequestDetails() {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	handler := middlewares.Logging()(inner)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/preset", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(s.T(), http.StatusCreated, rr.Code)
	logLine := s.buf.String()
	assert.Contains(s.T(), logLine, "POST")
	assert.Contains(s.T(), logLine, "/api/v1/preset")
	assert.Contains(s.T(), logLine, "201")
}

func (s *LoggingSuite) TestDefaultsTo200() {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	handler := middlewares.Logging()(inner)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(s.T(), http.StatusOK, rr.Code)
	assert.Contains(s.T(), s.buf.String(), "200")
}

func (s *LoggingSuite) Test5xx_LogsStatusOnly() {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "connection refused", http.StatusInternalServerError)
	})
	handler := middlewares.Logging()(inner)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/history", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(s.T(), http.StatusInternalServerError, rr.Code)
	logLine := s.buf.String()
	assert.Contains(s.T(), logLine, "500")
	assert.NotContains(s.T(), logLine, "err=", "error detail is handled by internalError, not the logging middleware")
}

func TestLoggingSuite(t *testing.T) {
	suite.Run(t, new(LoggingSuite))
}
