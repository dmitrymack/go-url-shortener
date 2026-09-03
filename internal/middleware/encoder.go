package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"sync"
)

// gzipWriterPool reuses *gzip.Writer values across requests so the
// compressor's internal buffers aren't reallocated on every response.
var gzipWriterPool = sync.Pool{
	New: func() any {
		return gzip.NewWriter(io.Discard)
	},
}

// gzipWriter wraps http.ResponseWriter, transparently gzip-compressing the
// response body for JSON and HTML responses.
type gzipWriter struct {
	http.ResponseWriter
	Writer  *gzip.Writer
	useGzip bool
}

// WriteHeader enables gzip compression for JSON and HTML responses (sets
// the Content-Encoding header and takes a *gzip.Writer from the pool), then
// forwards the call to the underlying ResponseWriter.
func (w *gzipWriter) WriteHeader(statusCode int) {
	contentType := w.Header().Get("Content-Type")

	if strings.Contains(contentType, "application/json") ||
		strings.Contains(contentType, "text/html") {

		w.Header().Set("Content-Encoding", "gzip")
		w.useGzip = true
		w.Writer = gzipWriterPool.Get().(*gzip.Writer)
		w.Writer.Reset(w.ResponseWriter)
	}

	w.ResponseWriter.WriteHeader(statusCode)
}

// Write writes b through the gzip compressor if it's enabled, otherwise
// directly to the underlying ResponseWriter.
func (w *gzipWriter) Write(b []byte) (int, error) {
	if !w.useGzip {
		return w.ResponseWriter.Write(b)
	}

	return w.Writer.Write(b)
}

// GzipHandler returns middleware that transparently decompresses a
// gzip-encoded request body (if Content-Encoding: gzip is set) and
// compresses a JSON/HTML response body when the client supports gzip
// (Accept-Encoding: gzip).
func GzipHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.Header.Get("Content-Encoding"), "gzip") {
			gzr, err := gzip.NewReader(r.Body)
			if err != nil {
				http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
				return
			}
			defer gzr.Close()

			r.Body = io.NopCloser(gzr)
		}

		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		cw := &gzipWriter{
			ResponseWriter: w,
		}

		defer func() {
			if cw.Writer != nil {
				_ = cw.Writer.Close()
				gzipWriterPool.Put(cw.Writer)
			}
		}()

		next.ServeHTTP(cw, r)
	})
}
