// Command shortener starts the URL shortener HTTP server: applies
// migrations and sets up the storage backend (PostgreSQL, file-based, or
// in-memory — in that priority order, depending on availability), audit
// sinks, and handler routes.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"

	"github.com/dmitrymack/go-url-shortener.git/internal/audit"
	"github.com/dmitrymack/go-url-shortener.git/internal/config"
	"github.com/dmitrymack/go-url-shortener.git/internal/handler"
	"github.com/dmitrymack/go-url-shortener.git/internal/middleware"
	shortenService "github.com/dmitrymack/go-url-shortener.git/internal/service"
	"github.com/dmitrymack/go-url-shortener.git/internal/storage"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
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

	auditLog := audit.NewLog()

	if cfg.AuditFile != "" {
		fileObserver, err := audit.NewFileObserver(cfg.AuditFile)
		if err != nil {
			log.Printf("audit file unavailable: %v", err)
		} else {
			auditLog.Register(fileObserver)
			defer fileObserver.Close()
		}
	}

	if cfg.AuditURL != "" {
		auditLog.Register(audit.NewRemoteObserver(cfg.AuditURL))
	}

	service := shortenService.NewShortenService(store, cfg.BaseURL)
	h := handler.NewHandler(service, db, auditLog)
	service.StartDeleteWorker()

	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatal(err)
	}
	defer logger.Sync()

	r := chi.NewRouter()
	r.Use(middleware.LoggingHandler(logger), middleware.GzipHandler, middleware.AuthorizerHandler)

	r.Mount("/debug", chimiddleware.Profiler())

	r.Get("/{id}", h.GetUrlById)
	r.Get("/ping", h.PingDatabase)
	r.Get("/api/user/urls", h.GetUserURLS)

	r.Post("/", h.SetShortUrl)
	r.Post("/api/shorten", h.SetShortUrlByJSON)
	r.Post("/api/shorten/batch", h.SetBatchURL)

	r.Delete("/api/user/urls", h.DeleteUserUrls)

	err = http.ListenAndServe(cfg.ServerAddress, r)

	if err != nil {
		logger.Fatal("failed to start server", zap.Error(err))
	}
}

// runMigrations applies migrations from the migrations directory to the
// database identified by the connection string dsn. No pending migrations
// is not treated as an error.
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
