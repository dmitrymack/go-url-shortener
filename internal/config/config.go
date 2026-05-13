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
}

func NewConfig() *Config {
	cfg := &Config{}

	flag.StringVar(&cfg.ServerAddress, "a", "localhost:8080", "Input host and port of server")
	flag.StringVar(&cfg.BaseURL, "b", "http://localhost:8080", "Base URL")
	flag.StringVar(&cfg.StorageFile, "f", "storage.jsonl", "Storage file name")

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

	_, err := url.ParseRequestURI(cfg.BaseURL)
	if err != nil {
		log.Fatal("invalid base URL:", err)
	}

	return cfg
}
