package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"sync"
)

var gzipWriterPool = sync.Pool{
	New: func() any {
		return gzip.NewWriter(io.Discard)
	},
}

type gzipWriter struct {
	http.ResponseWriter
	Writer  *gzip.Writer
	useGzip bool
}

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

func (w *gzipWriter) Write(b []byte) (int, error) {
	if !w.useGzip {
		return w.ResponseWriter.Write(b)
	}

	return w.Writer.Write(b)
}

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
