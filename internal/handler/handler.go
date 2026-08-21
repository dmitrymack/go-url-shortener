// Package handler contains the HTTP handlers of the URL shortener service:
// creating short links, following them, batch shortening, and operations on
// the current user's links.
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"

	"github.com/dmitrymack/go-url-shortener.git/internal/audit"
	"github.com/dmitrymack/go-url-shortener.git/internal/contextKeys"
	"github.com/dmitrymack/go-url-shortener.git/internal/service"
	"github.com/dmitrymack/go-url-shortener.git/internal/storage"
	"github.com/go-chi/chi/v5"
)

// Handler implements the URL shortener HTTP handlers on top of a
// ShortenService, a database connection (used for the health check), and an
// audit event publisher.
type Handler struct {
	service  *service.ShortenService
	database storage.Database
	auditor  audit.Publisher
}

// RequestObject is the POST /api/shorten request body: the URL to shorten.
type RequestObject struct {
	URL string `json:"url"`
}

// ResponseObject is the POST /api/shorten response body: the resulting short URL.
type ResponseObject struct {
	Result string `json:"result"`
}

// BatchRequest is a single item of the POST /api/shorten/batch request: an
// original URL with a correlation ID the client uses to match it against
// the response.
type BatchRequest struct {
	CorrelationID string `json:"correlation_id"`
	OriginalURL   string `json:"original_url"`
}

// BatchResponse is a single item of the batch shortening response.
type BatchResponse struct {
	CorrelationID string `json:"correlation_id"`
	ShortURL      string `json:"short_url"`
}

// NewHandler creates a Handler backed by the link shortening service s, a
// database connection db (may be nil if the DB handler isn't used), and an
// audit event publisher auditor (may be nil if auditing is disabled).
func NewHandler(s *service.ShortenService, db storage.Database, auditor audit.Publisher) *Handler {
	return &Handler{
		service:  s,
		database: db,
		auditor:  auditor,
	}
}

// auditEvent sends an audit event to all registered observers, if auditing
// is enabled (auditor != nil).
func (h *Handler) auditEvent(ctx context.Context, action, url string) {
	if h.auditor == nil {
		return
	}

	userID, _ := ctx.Value(contextKeys.UserIDContextKey).(string)
	h.auditor.Notify(audit.NewEvent(action, userID, url))
}

// SetShortUrl handles POST /. It accepts the original URL as plain text in
// the request body and returns the short URL as plain text with status 201.
// If the URL was already shortened before, it returns the existing short
// link with status 409.
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

	shortURL, err := h.service.CreateShortURL(r.Context(), originURL)
	if errors.Is(err, storage.ErrDuplicateOriginalURL) {
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(shortURL))
		h.auditEvent(r.Context(), audit.ActionShorten, originURL)
		return
	}

	if err != nil {
		log.Printf("CreateShortURL error: %v", err)

		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(shortURL))
	h.auditEvent(r.Context(), audit.ActionShorten, originURL)
}

// GetUrlById handles GET /{id}. It looks up the original URL by the short
// identifier id and returns a temporary redirect (307) to it. If the link
// was deleted, it returns status 410; if not found, status 400.
func (h *Handler) GetUrlById(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	value, err := h.service.GetOriginalURL(id)
	if errors.Is(err, storage.ErrDeleted) {
		w.WriteHeader(http.StatusGone)
		return
	}
	if err != nil {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusBadRequest)
		return
	}

	w.Header().Set("Location", value)
	w.WriteHeader(http.StatusTemporaryRedirect)
	h.auditEvent(r.Context(), audit.ActionFollow, value)
}

// SetShortUrlByJSON handles POST /api/shorten. It accepts the original URL
// as JSON (RequestObject) and returns the short URL as JSON too
// (ResponseObject) with status 201. If the URL was already shortened before,
// it returns the existing short link with status 409.
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

	shortURL, err := h.service.CreateShortURL(r.Context(), reqObj.URL)
	statusCode := http.StatusCreated

	if errors.Is(err, storage.ErrDuplicateOriginalURL) {
		statusCode = http.StatusConflict
	} else if err != nil {
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
	w.WriteHeader(statusCode)
	w.Write(resp)
	h.auditEvent(r.Context(), audit.ActionShorten, reqObj.URL)
}

// PingDatabase handles GET /ping. It checks database availability and
// returns 200 if the connection is alive, otherwise 500.
func (h *Handler) PingDatabase(w http.ResponseWriter, r *http.Request) {
	if h.database == nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if err := h.database.Ping(r.Context()); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// SetBatchURL handles POST /api/shorten/batch. It accepts a list of
// original URLs with correlation IDs ([]BatchRequest) and returns a list of
// shortened links ([]BatchResponse) with status 201.
func (h *Handler) SetBatchURL(w http.ResponseWriter, r *http.Request) {
	var req []BatchRequest
	var resp []BatchResponse

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	originURLs := make([]string, 0, len(req))
	for _, item := range req {
		originURLs = append(originURLs, item.OriginalURL)
	}

	shortURLs, err := h.service.CreateBatchShortURL(r.Context(), originURLs)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	for i, shortURL := range shortURLs {
		resp = append(resp, BatchResponse{
			CorrelationID: req[i].CorrelationID,
			ShortURL:      shortURL,
		})
	}

	respJson, err := json.Marshal(resp)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	w.Write(respJson)
}

// GetUserURLS handles GET /api/user/urls. It returns all short links created
// by the current user (identified via the authorization cookie). If there
// are no links, it returns status 204.
func (h *Handler) GetUserURLS(w http.ResponseWriter, r *http.Request) {
	userId := r.Context().Value(contextKeys.UserIDContextKey).(string)
	userUrls, err := h.service.GetUrlsByUser(userId)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if len(userUrls) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	respJson, err := json.Marshal(userUrls)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(respJson)
}

// DeleteUserUrls handles DELETE /api/user/urls. It accepts a list of short
// identifiers and asynchronously (via the service's deletion queue) marks
// the corresponding links of the current user as deleted. It returns status
// 202 without waiting for the deletion to actually happen.
func (h *Handler) DeleteUserUrls(w http.ResponseWriter, r *http.Request) {
	var ids []string
	userID := r.Context().Value(contextKeys.UserIDContextKey).(string)

	err := json.NewDecoder(r.Body).Decode(&ids)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	h.service.EnqueueDelete(service.DeleteTask{
		UserID: userID,
		IDs:    ids,
	})

	w.WriteHeader(http.StatusAccepted)
}
