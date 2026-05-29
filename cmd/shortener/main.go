package main

import (
	"context"
	"io"
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
	store := storage.NewStorage()
	var db shortenService.Database

	postgres, err := shortenService.NewPostgres(context.Background(), cfg.DSN)
	if err != nil {
		log.Printf(" Postgres unavailable: %v", err)
	} else {
		db = postgres
		defer postgres.Close(context.Background())
	}

	consumer, err := shortenService.NewConsumer(cfg.StorageFile)
	if err != nil {
		log.Fatal(err)
	}
	defer consumer.Close()

	for {
		event, err := consumer.ReadEvent()

		if err == io.EOF {
			break
		}

		if err != nil {
			log.Fatal(err)
		}

		err = store.Set(event.ShortURL, event.OriginalURL)

		if err != nil {
			log.Fatal(err)
		}
	}

	producer, err := shortenService.NewProducer(cfg.StorageFile)
	if err != nil {
		log.Fatal(err)
	}
	defer producer.Close()

	service := shortenService.NewShortenService(store, producer, cfg.BaseURL)
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
