package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestLoggingHandler_LogsRequestDetails(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	logger := zap.New(core)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("hello"))
	})

	r := httptest.NewRequest(http.MethodPost, "/", nil)
	w := httptest.NewRecorder()

	LoggingHandler(logger)(next).ServeHTTP(w, r)

	res := w.Result()
	defer res.Body.Close()

	assert.Equal(t, http.StatusCreated, res.StatusCode)
	assert.Equal(t, "hello", w.Body.String())

	require.Equal(t, 1, logs.Len())
	msg := logs.All()[0].Message
	assert.Contains(t, msg, "POST")
	assert.Contains(t, msg, "201")
	assert.Contains(t, msg, "5") // response body size in bytes
}

func TestLoggingHandler_DefaultsToStatusOKWhenNotExplicitlySet(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	logger := zap.New(core)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handler writes the body directly without calling WriteHeader.
		w.Write([]byte("ok"))
	})

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	LoggingHandler(logger)(next).ServeHTTP(w, r)

	require.Equal(t, 1, logs.Len())
	assert.Contains(t, logs.All()[0].Message, "200")
}
