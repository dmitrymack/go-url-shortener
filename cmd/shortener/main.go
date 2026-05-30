package main

import (
	"context"
	"log"
	"net/http"

	"github.com/dmitrymack/go-url-shortener.git/internal/config"
	"github.com/dmitrymack/go-url-shortener.git/internal/handler"
	"github.com/dmitrymack/go-url-shortener.git/internal/middleware"
	shortenService "github.com/dmitrymack/go-url-shortener.git/internal/service"
	"github.com/dmitrymack/go-url-shortener.git/internal/storage"
	"github.com/go-chi/chi/v5"
	_ "github.com/jackc/pgx/v5/stdlib"
	"go.uber.org/zap"
)

func main() {
	cfg := config.NewConfig()
	var store shortenService.URLStorage
	var fileStorage *storage.FileStorage
	var db storage.Database

	postgres, err := storage.NewPostgres(context.Background(), cfg.DSN)
	if err != nil {
		log.Printf("Postgres unavailable: %v", err)

		fileStorage, err = storage.NewFileStorage(cfg.StorageFile)
		if err != nil {
			log.Printf("File unavailable: %v", err)
			store = storage.NewStorage()
		} else {
			store = fileStorage
			defer fileStorage.Close()
		}
	} else {
		store = postgres
		db = postgres
		defer postgres.Close(context.Background())
	}

	service := shortenService.NewShortenService(store, cfg.BaseURL)
	h := handler.NewHandler(service, db)

	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatal(err)
	}
	defer logger.Sync()

	r := chi.NewRouter()
	r.Use(middleware.LoggingHandler(logger), middleware.GzipHandler)
	r.Post("/", h.SetShortUrl)
	r.Get("/{id}", h.GetUrlById)
	r.Post("/api/shorten", h.SetShortUrlByJSON)
	r.Get("/ping", h.PingDatabase)

	err = http.ListenAndServe(cfg.ServerAddress, r)

	if err != nil {
		logger.Fatal("failed to start server", zap.Error(err))
	}
}
