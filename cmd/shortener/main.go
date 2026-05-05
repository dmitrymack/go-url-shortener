package main

import (
	"net/http"

	"github.com/dmitrymack/go-url-shortener.git/internal/config"
	"github.com/dmitrymack/go-url-shortener.git/internal/handler"
	"github.com/go-chi/chi/v5"
)

func main() {
	cfg := config.NewConfig()
	h := &handler.Handler{
		BaseURL: cfg.BaseURL,
	}

	r := chi.NewRouter()
	r.Post("/", h.SetShortUrl)
	r.Get("/{id}", h.GetUrlById)

	err := http.ListenAndServe(cfg.ServerAddress, r)

	if err != nil {
		panic(err)
	}
}
