// Package config assembles the configuration from flags, environment
// variables, and an optional JSON config file (in that priority order,
// highest first).
package config

import (
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"

	"go.uber.org/zap"
)

// Config holds the server startup parameters.
type Config struct {
	ServerAddress string     // -a, SERVER_ADDRESS, server_address
	BaseURL       string     // -b, BASE_URL, base_url
	StorageFile   string     // -f, FILE_STORAGE_PATH, file_storage_path
	DSN           string     // -d, DATABASE_DSN, database_dsn; empty disables the DB
	AuditFile     string     // --audit-file, AUDIT_FILE, audit_file; empty disables file auditing
	AuditURL      string     // --audit-url, AUDIT_URL, audit_url; empty disables remote auditing
	EnableHTTPS   bool       // -s, ENABLE_HTTPS, enable_https
	TrustedSubnet string     // -t, TRUSTED_SUBNET, trusted_subnet (CIDR); empty denies access to the stats endpoint
	TrustedNet    *net.IPNet // TrustedSubnet parsed; nil if TrustedSubnet is empty
	ConfigFile    string     // -c/-config, CONFIG
}

// fileConfig is the JSON config file's shape. Pointer fields distinguish a
// key left out of the file (nil) from one explicitly set to its zero value.
type fileConfig struct {
	ServerAddress *string `json:"server_address"`
	BaseURL       *string `json:"base_url"`
	StorageFile   *string `json:"file_storage_path"`
	DSN           *string `json:"database_dsn"`
	EnableHTTPS   *bool   `json:"enable_https"`
	TrustedSubnet *string `json:"trusted_subnet"`
	AuditFile     *string `json:"audit_file"`
	AuditURL      *string `json:"audit_url"`
}

// NewConfig assembles the Config from flags, environment variables, and an
// optional JSON config file (-c/-config, CONFIG). It exits via logger.Fatal
// if the config file can't be read/parsed, or BaseURL or TrustedSubnet
// aren't valid.
func NewConfig(logger *zap.Logger) *Config {
	cfg := &Config{}

	flag.StringVar(&cfg.ServerAddress, "a", "localhost:8080", "Input host and port of server")
	flag.StringVar(&cfg.BaseURL, "b", "http://localhost:8080", "Base URL")
	flag.StringVar(&cfg.StorageFile, "f", "storage.jsonl", "Storage file name")
	flag.StringVar(&cfg.DSN, "d", "", "Input Database DSN")
	flag.StringVar(&cfg.AuditFile, "audit-file", "", "Audit log file path")
	flag.StringVar(&cfg.AuditURL, "audit-url", "", "Audit log remote server URL")
	flag.BoolVar(&cfg.EnableHTTPS, "s", false, "Enable HTTPS")
	flag.StringVar(&cfg.TrustedSubnet, "t", "", "Trusted subnet (CIDR) allowed to use the internal stats endpoint")
	flag.StringVar(&cfg.ConfigFile, "c", "", "Config file path")
	flag.StringVar(&cfg.ConfigFile, "config", "", "Config file path")

	flag.Parse()

	// set holds the flag names explicitly provided via a flag or env var;
	// the config file must not override those.
	set := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) { set[f.Name] = true })

	if envServerAddr := os.Getenv("SERVER_ADDRESS"); envServerAddr != "" {
		cfg.ServerAddress = envServerAddr
		set["a"] = true
	}

	if envBaseURL := os.Getenv("BASE_URL"); envBaseURL != "" {
		cfg.BaseURL = envBaseURL
		set["b"] = true
	}

	if envStorageFile := os.Getenv("FILE_STORAGE_PATH"); envStorageFile != "" {
		cfg.StorageFile = envStorageFile
		set["f"] = true
	}

	if envDSN := os.Getenv("DATABASE_DSN"); envDSN != "" {
		cfg.DSN = envDSN
		set["d"] = true
	}

	if envAuditFile := os.Getenv("AUDIT_FILE"); envAuditFile != "" {
		cfg.AuditFile = envAuditFile
		set["audit-file"] = true
	}

	if envAuditURL := os.Getenv("AUDIT_URL"); envAuditURL != "" {
		cfg.AuditURL = envAuditURL
		set["audit-url"] = true
	}

	if envEnableHTTPS := os.Getenv("ENABLE_HTTPS"); envEnableHTTPS != "" {
		if v, err := strconv.ParseBool(envEnableHTTPS); err == nil {
			cfg.EnableHTTPS = v
			set["s"] = true
		}
	}

	if envTrustedSubnet := os.Getenv("TRUSTED_SUBNET"); envTrustedSubnet != "" {
		cfg.TrustedSubnet = envTrustedSubnet
		set["t"] = true
	}

	if envConfigFile := os.Getenv("CONFIG"); envConfigFile != "" {
		cfg.ConfigFile = envConfigFile
	}

	if cfg.ConfigFile != "" {
		fc, err := loadFileConfig(cfg.ConfigFile)
		if err != nil {
			logger.Fatal("failed to load config file", zap.Error(err))
		}
		applyFileConfig(cfg, fc, set)
	}

	_, err := url.ParseRequestURI(cfg.BaseURL)
	if err != nil {
		logger.Fatal("invalid base URL", zap.Error(err))
	}

	if cfg.TrustedSubnet != "" {
		_, cfg.TrustedNet, err = net.ParseCIDR(cfg.TrustedSubnet)
		if err != nil {
			logger.Fatal("invalid trusted subnet", zap.Error(err))
		}
	}

	return cfg
}

// loadFileConfig reads and parses the JSON config file at path.
func loadFileConfig(path string) (fileConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return fileConfig{}, fmt.Errorf("reading config file: %w", err)
	}

	var fc fileConfig
	if err := json.Unmarshal(data, &fc); err != nil {
		return fileConfig{}, fmt.Errorf("parsing config file: %w", err)
	}

	return fc, nil
}

// applyFileConfig fills cfg's fields from fc, skipping ones already in set
// or left unset in fc.
func applyFileConfig(cfg *Config, fc fileConfig, set map[string]bool) {
	if fc.ServerAddress != nil && !set["a"] {
		cfg.ServerAddress = *fc.ServerAddress
	}
	if fc.BaseURL != nil && !set["b"] {
		cfg.BaseURL = *fc.BaseURL
	}
	if fc.StorageFile != nil && !set["f"] {
		cfg.StorageFile = *fc.StorageFile
	}
	if fc.DSN != nil && !set["d"] {
		cfg.DSN = *fc.DSN
	}
	if fc.EnableHTTPS != nil && !set["s"] {
		cfg.EnableHTTPS = *fc.EnableHTTPS
	}
	if fc.TrustedSubnet != nil && !set["t"] {
		cfg.TrustedSubnet = *fc.TrustedSubnet
	}
	if fc.AuditFile != nil && !set["audit-file"] {
		cfg.AuditFile = *fc.AuditFile
	}
	if fc.AuditURL != nil && !set["audit-url"] {
		cfg.AuditURL = *fc.AuditURL
	}
}
