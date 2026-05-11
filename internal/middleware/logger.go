package middleware

import (
	"net/http"
	"time"

	"go.uber.org/zap"
)

type (
	responseData struct {
		status int
		size   int
	}

	loggingResponseWriter struct {
		http.ResponseWriter
		responseData *responseData
	}
)

func (r *loggingResponseWriter) Write(b []byte) (int, error) {
	size, err := r.ResponseWriter.Write(b)
	r.responseData.size += size
	return size, err
}

func (r *loggingResponseWriter) WriteHeader(statusCode int) {
	r.ResponseWriter.WriteHeader(statusCode)
	r.responseData.status = statusCode
}

func LoggingHandler(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sugar := logger.Sugar()
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
