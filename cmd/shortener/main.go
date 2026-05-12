package main

import (
	"log"
	"net/http"

	"github.com/dmitrymack/go-url-shortener.git/internal/config"
	"github.com/dmitrymack/go-url-shortener.git/internal/handler"
	"github.com/dmitrymack/go-url-shortener.git/internal/middleware"
	shortenService "github.com/dmitrymack/go-url-shortener.git/internal/service"
	"github.com/dmitrymack/go-url-shortener.git/internal/storage"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

func main() {
	cfg := config.NewConfig()
	store := storage.NewStorage()
	service := shortenService.NewShortenService(store, cfg.BaseURL)
	h := handler.NewHandler(service)

	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatal(err)
	}
	defer logger.Sync()

	r := chi.NewRouter()
	r.Use(middleware.LoggingHandler(logger))
	r.Post("/", h.SetShortUrl)
	r.Get("/{id}", h.GetUrlById)
	r.Post("/api/shorten", h.SetShortUrlByJSON)

	err = http.ListenAndServe(cfg.ServerAddress, r)

	if err != nil {
		logger.Fatal("failed to start server", zap.Error(err))
	}
}
