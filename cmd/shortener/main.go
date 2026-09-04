// Command shortener starts the URL shortener HTTP server: applies
// migrations and sets up the storage backend (PostgreSQL, file-based, or
// in-memory — in that priority order, depending on availability), audit
// sinks, and handler routes.
package main

import (
	"context"
	"errors"
	"fmt"
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

// Build metadata, set at compile time via:
//
//	go build -ldflags "-X main.buildVersion=... -X main.buildDate=... -X main.buildCommit=..."
//
// Left empty (printed as "N/A") for a plain go build.
var (
	buildVersion string
	buildDate    string
	buildCommit  string
)

func main() {
	printBuildInfo()

	logger, err := zap.NewDevelopment()
	if err != nil {
		// No logger exists yet, so this one failure has nowhere else to go.
		log.Fatal(err)
	}
	defer logger.Sync()

	cfg := config.NewConfig(logger)
	var store shortenService.URLStorage
	var fileStorage *storage.FileStorage
	var db storage.Database
	var postgres *storage.Postgres

	err = runMigrations(cfg.DSN)
	if err != nil {
		logger.Error("migration failed", zap.Error(err))
	} else {
		postgres, err = storage.NewPostgres(context.Background(), cfg.DSN)
		if err != nil {
			logger.Error("postgres unavailable", zap.Error(err))
		}
	}

	if postgres != nil {
		store = postgres
		db = postgres
		defer postgres.Close(context.Background())
	} else {
		fileStorage, err = storage.NewFileStorage(cfg.StorageFile)
		if err != nil {
			logger.Error("file storage unavailable", zap.Error(err))
			store = storage.NewStorage()
		} else {
			store = fileStorage
			defer fileStorage.Close()
		}
	}

	auditLog := audit.NewLog(logger)

	if cfg.AuditFile != "" {
		fileObserver, err := audit.NewFileObserver(cfg.AuditFile, logger)
		if err != nil {
			logger.Error("audit file unavailable", zap.Error(err))
		} else {
			auditLog.Register(fileObserver)
			defer fileObserver.Close()
		}
	}

	if cfg.AuditURL != "" {
		auditLog.Register(audit.NewRemoteObserver(cfg.AuditURL, logger))
	}

	service := shortenService.NewShortenService(store, cfg.BaseURL, logger)
	h := handler.NewHandler(service, db, auditLog, logger)
	service.StartDeleteWorker()

	startProfilerServer("localhost:6060", logger)

	r := chi.NewRouter()
	r.Use(middleware.LoggingHandler(logger), middleware.GzipHandler, middleware.AuthorizerHandler)

	r.Get("/{id}", h.GetURLByID)
	r.Get("/ping", h.PingDatabase)
	r.Get("/api/user/urls", h.GetUserURLS)

	r.Post("/", h.SetShortURL)
	r.Post("/api/shorten", h.SetShortURLByJSON)
	r.Post("/api/shorten/batch", h.SetBatchURL)

	r.Delete("/api/user/urls", h.DeleteUserUrls)

	err = http.ListenAndServe(cfg.ServerAddress, r)

	if err != nil {
		logger.Fatal("failed to start server", zap.Error(err))
	}
}

// printBuildInfo prints the buildVersion/buildDate/buildCommit values (set
// at compile time via -ldflags -X) to stdout, substituting "N/A" for
// whichever were left unset.
func printBuildInfo() {
	fmt.Printf("Build version: %s\n", orNA(buildVersion))
	fmt.Printf("Build date: %s\n", orNA(buildDate))
	fmt.Printf("Build commit: %s\n", orNA(buildCommit))
}

// orNA returns s, or "N/A" if s is empty.
func orNA(s string) string {
	if s == "" {
		return "N/A"
	}
	return s
}

// startProfilerServer starts the pprof debug server on addr in a background
// goroutine, separate from the public router. addr should be bound to
// localhost or another interface unreachable from outside the host, since
// /debug/pprof exposes heap dumps, goroutine traces, and CPU profiles that
// could otherwise leak the service's internal structure to an attacker.
func startProfilerServer(addr string, logger *zap.Logger) {
	profilerRouter := chi.NewRouter()
	profilerRouter.Mount("/debug", chimiddleware.Profiler())

	go func() {
		if err := http.ListenAndServe(addr, profilerRouter); err != nil {
			logger.Error("profiler server failed", zap.Error(err))
		}
	}()
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
