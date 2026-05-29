package handler

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/dmitrymack/go-url-shortener.git/internal/service"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	service  *service.ShortenService
	database service.Database
}

type RequestObject struct {
	URL string `json:"url"`
}

type ResponseObject struct {
	Result string `json:"result"`
}

func NewHandler(s *service.ShortenService, db service.Database) *Handler {
	return &Handler{
		service:  s,
		database: db,
	}
}

func (h *Handler) SetShortUrl(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	originURL := string(body)

	if originURL == "" {
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	shortURL, err := h.service.CreateShortURL(originURL)
	if err != nil {
		log.Printf("CreateShortURL error: %v", err)

		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(shortURL))
}

func (h *Handler) GetUrlById(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	value, ok := h.service.GetOriginalURL(id)
	if !ok {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusBadRequest)
		return
	}

	w.Header().Set("Location", value)
	w.WriteHeader(http.StatusTemporaryRedirect)
}

func (h *Handler) SetShortUrlByJSON(w http.ResponseWriter, r *http.Request) {
	var reqObj RequestObject

	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&reqObj); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	if reqObj.URL == "" {
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	shortURL, err := h.service.CreateShortURL(reqObj.URL)

	if err != nil {
		log.Printf("CreateShortURL error: %v", err)

		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	respObj := ResponseObject{
		Result: shortURL,
	}
	resp, err := json.Marshal(respObj)

	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	w.Write(resp)
}

func (h *Handler) PingDatabase(w http.ResponseWriter, r *http.Request) {
	err := h.database.Ping(r.Context())
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
