package handler

import (
	"io"
	"net/http"
)

var storage = map[string]string{}

func SetShortUrl(w http.ResponseWriter, r *http.Request) {
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
	shortURL := "http://" + r.Host + "/" + id
	storage[id] = originURL

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(shortURL))
}

func GetUrlById(w http.ResponseWriter, r *http.Request) {
	if storage[r.PathValue("id")] == "" {
		http.Error(w, "URL not found", http.StatusBadRequest)
		return
	}

	w.Header().Set("Location", storage[r.PathValue("id")])
	w.WriteHeader(http.StatusTemporaryRedirect)
}
