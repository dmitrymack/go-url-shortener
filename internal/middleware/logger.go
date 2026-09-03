package middleware

import (
	"net/http"
	"time"

	"go.uber.org/zap"
)

type (
	// responseData accumulates the status code and response body size for logging.
	responseData struct {
		status int
		size   int
	}

	// loggingResponseWriter wraps http.ResponseWriter, capturing the status
	// and size of the written response into responseData.
	loggingResponseWriter struct {
		http.ResponseWriter
		responseData *responseData
	}
)

// Write writes b to the underlying ResponseWriter and accumulates the
// response size.
func (r *loggingResponseWriter) Write(b []byte) (int, error) {
	size, err := r.ResponseWriter.Write(b)
	r.responseData.size += size
	return size, err
}

// WriteHeader writes statusCode to the underlying ResponseWriter and
// remembers it.
func (r *loggingResponseWriter) WriteHeader(statusCode int) {
	r.ResponseWriter.WriteHeader(statusCode)
	r.responseData.status = statusCode
}

// LoggingHandler returns middleware that logs the method, duration, status
// code, and body size of every HTTP request via logger.
func LoggingHandler(logger *zap.Logger) func(http.Handler) http.Handler {
	sugar := logger.Sugar()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			method := r.Method

			responseData := &responseData{
				status: http.StatusOK,
				size:   0,
			}

			loggingW := loggingResponseWriter{
				ResponseWriter: w,
				responseData:   responseData,
			}

			next.ServeHTTP(&loggingW, r)

			duration := time.Since(start)

			sugar.Infoln(
				"method", method,
				"duration", duration,
				"status", responseData.status,
				"size", responseData.size,
			)
		})
	}
}
