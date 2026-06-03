package main

import (
	"context"
	"errors"
	"log"
	"net/http"

	"github.com/dmitrymack/go-url-shortener.git/internal/config"
	"github.com/dmitrymack/go-url-shortener.git/internal/handler"
	"github.com/dmitrymack/go-url-shortener.git/internal/middleware"
	shortenService "github.com/dmitrymack/go-url-shortener.git/internal/service"
	"github.com/dmitrymack/go-url-shortener.git/internal/storage"
	"github.com/go-chi/chi/v5"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib"
	"go.uber.org/zap"
)

func main() {
	cfg := config.NewConfig()
	var store shortenService.URLStorage
	var fileStorage *storage.FileStorage
	var db storage.Database
	var postgres *storage.Postgres

	err := runMigrations(cfg.DSN)
	if err != nil {
		log.Printf("Migration failed: %v", err)
	} else {
		postgres, err = storage.NewPostgres(context.Background(), cfg.DSN)
		if err != nil {
			log.Printf("Postgres unavailable: %v", err)
		}
	}

	if postgres != nil {
		store = postgres
		db = postgres
		defer postgres.Close(context.Background())
	} else {
		fileStorage, err = storage.NewFileStorage(cfg.StorageFile)
		if err != nil {
			log.Printf("File unavailable: %v", err)
			store = storage.NewStorage()
		} else {
			store = fileStorage
			defer fileStorage.Close()
		}
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

	r.Get("/{id}", h.GetUrlById)
	r.Get("/ping", h.PingDatabase)

	r.Post("/", h.SetShortUrl)
	r.Post("/api/shorten", h.SetShortUrlByJSON)
	r.Post("/api/shorten/batch", h.SetBatchURL)

	err = http.ListenAndServe(cfg.ServerAddress, r)

	if err != nil {
		logger.Fatal("failed to start server", zap.Error(err))
	}
}

func runMigrations(dsn string) error {
	m, err := migrate.New(
		"file://migrations",
		dsn,
	)
	if err != nil {
		return err
	}

	defer m.Close()

	err = m.Up()
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}

	return nil
}
