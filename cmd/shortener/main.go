package main

import (
	"log"
	"net/http"

	"github.com/dmitrymack/go-url-shortener.git/internal/config"
	"github.com/dmitrymack/go-url-shortener.git/internal/handler"
	shortenService "github.com/dmitrymack/go-url-shortener.git/internal/service"
	"github.com/dmitrymack/go-url-shortener.git/internal/storage"
	"github.com/go-chi/chi/v5"
)

func main() {
	cfg := config.NewConfig()
	store := storage.NewStorage()
	service := shortenService.NewShortenService(store, cfg.BaseURL)
	h := handler.NewHandler(service)

	r := chi.NewRouter()
	r.Post("/", h.SetShortUrl)
	r.Get("/{id}", h.GetUrlById)

	err := http.ListenAndServe(cfg.ServerAddress, r)

	if err != nil {
		log.Fatal(err)
	}
}
