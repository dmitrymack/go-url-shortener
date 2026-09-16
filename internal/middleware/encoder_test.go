package middleware

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGzipHandler_CompressesJSONWhenAccepted(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"hello":"world"}`))
	})

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()

	GzipHandler(next).ServeHTTP(w, r)

	res := w.Result()
	defer res.Body.Close()

	assert.Equal(t, "gzip", res.Header.Get("Content-Encoding"))

	gzr, err := gzip.NewReader(res.Body)
	require.NoError(t, err)
	defer gzr.Close()

	body, err := io.ReadAll(gzr)
	require.NoError(t, err)
	assert.Equal(t, `{"hello":"world"}`, string(body))
}

func TestGzipHandler_CompressesHTMLWhenAccepted(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<html></html>"))
	})

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()

	GzipHandler(next).ServeHTTP(w, r)

	res := w.Result()
	defer res.Body.Close()

	assert.Equal(t, "gzip", res.Header.Get("Content-Encoding"))
}

func TestGzipHandler_SkipsOtherContentTypesEvenWhenAccepted(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hello"))
	})

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()

	GzipHandler(next).ServeHTTP(w, r)

	res := w.Result()
	defer res.Body.Close()

	assert.Empty(t, res.Header.Get("Content-Encoding"))
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	assert.Equal(t, "hello", string(body))
}

func TestGzipHandler_NoCompressionWhenNotAccepted(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"hello":"world"}`))
	})

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	GzipHandler(next).ServeHTTP(w, r)

	res := w.Result()
	defer res.Body.Close()

	assert.Empty(t, res.Header.Get("Content-Encoding"))
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	assert.Equal(t, `{"hello":"world"}`, string(body))
}

func TestGzipHandler_DecompressesRequestBody(t *testing.T) {
	var gotBody string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.WriteHeader(http.StatusOK)
	})

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	gz.Write([]byte("original body"))
	require.NoError(t, gz.Close())

	r := httptest.NewRequest(http.MethodPost, "/", &buf)
	r.Header.Set("Content-Encoding", "gzip")
	w := httptest.NewRecorder()

	GzipHandler(next).ServeHTTP(w, r)

	assert.Equal(t, "original body", gotBody)
}

func TestGzipHandler_InvalidRequestGzip(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next should not be called for an invalid gzip body")
	})

	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("not gzip")))
	r.Header.Set("Content-Encoding", "gzip")
	w := httptest.NewRecorder()

	GzipHandler(next).ServeHTTP(w, r)

	res := w.Result()
	defer res.Body.Close()
	assert.Equal(t, http.StatusBadRequest, res.StatusCode)
}
