// Package config assembles the URL shortener service configuration from
// command-line flags and environment variables (environment variables take
// priority over flags).
package config

import (
	"flag"
	"net/url"
	"os"

	"go.uber.org/zap"
)

// Config holds the server startup parameters.
type Config struct {
	// ServerAddress is the address the HTTP server listens on (flag -a, env SERVER_ADDRESS).
	ServerAddress string
	// BaseURL is the base URL prepended to short identifiers (flag -b, env BASE_URL).
	BaseURL string
	// StorageFile is the path to the file store's file (flag -f, env FILE_STORAGE_PATH).
	StorageFile string
	// DSN is the PostgreSQL connection string (flag -d, env DATABASE_DSN). An empty string disables the DB.
	DSN string
	// AuditFile is the path to the audit file sink (flag --audit-file, env AUDIT_FILE). An empty string disables file auditing.
	AuditFile string
	// AuditURL is the URL of the remote audit sink (flag --audit-url, env AUDIT_URL). An empty string disables remote auditing.
	AuditURL string
}

// NewConfig parses command-line flags and environment variables and returns
// the resulting configuration. It terminates the process with an error,
// logged via logger, if BaseURL is not a valid URL.
func NewConfig(logger *zap.Logger) *Config {
	cfg := &Config{}

	flag.StringVar(&cfg.ServerAddress, "a", "localhost:8080", "Input host and port of server")
	flag.StringVar(&cfg.BaseURL, "b", "http://localhost:8080", "Base URL")
	flag.StringVar(&cfg.StorageFile, "f", "storage.jsonl", "Storage file name")
	flag.StringVar(&cfg.DSN, "d", "", "Input Database DSN")
	flag.StringVar(&cfg.AuditFile, "audit-file", "", "Audit log file path")
	flag.StringVar(&cfg.AuditURL, "audit-url", "", "Audit log remote server URL")

	flag.Parse()

	if envServerAddr := os.Getenv("SERVER_ADDRESS"); envServerAddr != "" {
		cfg.ServerAddress = envServerAddr
	}

	if envBaseURL := os.Getenv("BASE_URL"); envBaseURL != "" {
		cfg.BaseURL = envBaseURL
	}

	if envStorageFile := os.Getenv("FILE_STORAGE_PATH"); envStorageFile != "" {
		cfg.StorageFile = envStorageFile
	}

	if envDSN := os.Getenv("DATABASE_DSN"); envDSN != "" {
		cfg.DSN = envDSN
	}

	if envAuditFile := os.Getenv("AUDIT_FILE"); envAuditFile != "" {
		cfg.AuditFile = envAuditFile
	}

	if envAuditURL := os.Getenv("AUDIT_URL"); envAuditURL != "" {
		cfg.AuditURL = envAuditURL
	}

	_, err := url.ParseRequestURI(cfg.BaseURL)
	if err != nil {
		logger.Fatal("invalid base URL", zap.Error(err))
	}

	return cfg
}
