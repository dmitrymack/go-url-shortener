package config

import (
	"flag"
	"log"
	"net/url"
	"os"
)

type Config struct {
	ServerAddress string
	BaseURL       string
	StorageFile   string
	DSN           string
	AuditFile     string
	AuditURL      string
}

func NewConfig() *Config {
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
		log.Fatal("invalid base URL:", err)
	}

	return cfg
}
