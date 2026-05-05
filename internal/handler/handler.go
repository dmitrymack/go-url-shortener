package handler

import (
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
)

var storage = map[string]string{}

type Handler struct {
	BaseURL string
}

func (h *Handler) SetShortUrl(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST requests are allowed", http.StatusBadRequest)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	originURL := string(body)

	if originURL == "" {
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	id := "abc123"
	shortURL := h.BaseURL + "/" + id
	storage[id] = originURL

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(shortURL))
}

func (h *Handler) GetUrlById(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if storage[id] == "" {
		http.Error(w, "URL not found", http.StatusBadRequest)
		return
	}

	w.Header().Set("Location", storage[id])
	w.WriteHeader(http.StatusTemporaryRedirect)
}
