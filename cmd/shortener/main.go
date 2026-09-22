// Command shortener starts the URL shortener's HTTP and gRPC servers side
// by side, over the same storage, service, and audit sinks.
package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	shortenerpb "github.com/dmitrymack/go-url-shortener.git/api/proto"
	"github.com/dmitrymack/go-url-shortener.git/internal/audit"
	"github.com/dmitrymack/go-url-shortener.git/internal/config"
	"github.com/dmitrymack/go-url-shortener.git/internal/grpcserver"
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
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// Build metadata, set via -ldflags -X main.buildVersion=... etc. Defaults
// to "N/A" so it's meaningful even for a plain go build.
var (
	buildVersion = "N/A"
	buildDate    = "N/A"
	buildCommit  = "N/A"
)

// shutdownTimeout bounds how long graceful shutdown waits for in-flight
// requests before srv.Close forcibly drops any that are still open.
const shutdownTimeout = 10 * time.Second

func main() {
	printBuildInfo()

	logger, err := zap.NewDevelopment()
	if err != nil {
		// No logger exists yet, so this one failure has nowhere else to go.
		log.Fatal(err)
	}

	// run's deferred cleanup (closing the DB pool, storage files, the
	// logger) must all fire before the process exits, which os.Exit skips
	// — so main itself stays defer-free and only picks the exit code.
	os.Exit(run(logger))
}

func run(logger *zap.Logger) int {
	defer logger.Sync()

	cfg := config.NewConfig(logger)
	var store shortenService.URLStorage
	var fileStorage *storage.FileStorage
	var db storage.Database
	var postgres *storage.Postgres

	err := runMigrations(cfg.DSN)
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
	r.Use(middleware.LoggingHandler(logger), middleware.GzipHandler)

	r.Group(func(r chi.Router) {
		r.Use(middleware.AuthorizerHandler)

		r.Get("/{id}", h.GetURLByID)
		r.Get("/ping", h.PingDatabase)
		r.Get("/api/user/urls", h.GetUserURLS)

		r.Post("/", h.SetShortURL)
		r.Post("/api/shorten", h.SetShortURLByJSON)
		r.Post("/api/shorten/batch", h.SetBatchURL)

		r.Delete("/api/user/urls", h.DeleteUserUrls)
	})

	// Internal endpoint: called by other services, not users, so it skips
	// the cookie-based authorizer and is guarded by the trusted subnet.
	r.With(middleware.TrustedSubnetHandler(cfg.TrustedNet)).Get("/api/internal/stats", h.GetStats)

	srv := &http.Server{
		Addr:    cfg.ServerAddress,
		Handler: r,
	}

	// Generated once and shared by the HTTP and gRPC servers, so enabling
	// HTTPS applies uniformly to both, per the same cfg.EnableHTTPS switch.
	var tlsCert *tls.Certificate
	if cfg.EnableHTTPS {
		cert, err := selfSignedCert()
		if err != nil {
			logger.Fatal("failed to generate TLS certificate", zap.Error(err))
		}
		tlsCert = &cert
	}

	serverErr := make(chan error, 1)
	go func() {
		if tlsCert != nil {
			srv.TLSConfig = &tls.Config{Certificates: []tls.Certificate{*tlsCert}}
			serverErr <- srv.ListenAndServeTLS("", "")
			return
		}
		serverErr <- srv.ListenAndServe()
	}()

	grpcOpts := []grpc.ServerOption{grpc.UnaryInterceptor(middleware.GRPCAuthInterceptor(logger))}
	if tlsCert != nil {
		grpcOpts = append(grpcOpts, grpc.Creds(credentials.NewTLS(&tls.Config{Certificates: []tls.Certificate{*tlsCert}})))
	}

	grpcServer := grpc.NewServer(grpcOpts...)
	shortenerpb.RegisterShortenerServiceServer(grpcServer, grpcserver.NewServer(service, auditLog, logger))

	grpcListener, err := net.Listen("tcp", cfg.GRPCAddress)
	if err != nil {
		logger.Fatal("failed to listen for gRPC", zap.Error(err))
	}

	grpcErr := make(chan error, 1)
	go func() {
		grpcErr <- grpcServer.Serve(grpcListener)
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer stop()

	// runCtx ends the wait below on a signal (via ctx) or on either server
	// failing after it started serving, so a runtime crash drains the
	// delete/audit queues and stops the other server exactly like a
	// signal would, instead of exiting mid-flight.
	runCtx, cancelRun := context.WithCancel(ctx)
	defer cancelRun()

	var (
		runErr     error
		runErrOnce sync.Once
	)
	go func() {
		if err := <-serverErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
			runErrOnce.Do(func() { runErr = fmt.Errorf("HTTP server: %w", err) })
			cancelRun()
		}
	}()
	go func() {
		if err := <-grpcErr; err != nil {
			runErrOnce.Do(func() { runErr = fmt.Errorf("gRPC server: %w", err) })
			cancelRun()
		}
	}()

	<-runCtx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("HTTP server shutdown timed out, closing forcibly", zap.Error(err))
		if err := srv.Close(); err != nil {
			logger.Error("HTTP server close failed", zap.Error(err))
		}
	}

	grpcStopped := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(grpcStopped)
	}()

	select {
	case <-grpcStopped:
	case <-time.After(shutdownTimeout):
		logger.Error("gRPC server shutdown timed out, closing forcibly")
		grpcServer.Stop()
	}

	// Both servers are down, so no more requests can enqueue a deletion or
	// an audit event — safe to drain and stop both.
	service.Stop()
	auditLog.Stop()

	if runErr != nil {
		logger.Error("server exiting after a runtime failure", zap.Error(runErr))
		return 1
	}

	logger.Info("server shut down gracefully")
	return 0
}

// printBuildInfo prints the buildVersion/buildDate/buildCommit values (set
// at compile time via -ldflags -X) to stdout.
func printBuildInfo() {
	fmt.Printf("Build version: %s\n", buildVersion)
	fmt.Printf("Build date: %s\n", buildDate)
	fmt.Printf("Build commit: %s\n", buildCommit)
}

// startProfilerServer starts the pprof debug server on addr, which should
// be unreachable from outside the host — /debug/pprof leaks internals.
func startProfilerServer(addr string, logger *zap.Logger) {
	profilerRouter := chi.NewRouter()
	profilerRouter.Mount("/debug", chimiddleware.Profiler())

	go func() {
		if err := http.ListenAndServe(addr, profilerRouter); err != nil {
			logger.Error("profiler server failed", zap.Error(err))
		}
	}()
}

// runMigrations applies migrations from the migrations directory to dsn.
// No pending migrations is not treated as an error.
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
