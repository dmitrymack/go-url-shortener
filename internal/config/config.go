package config

import (
	"flag"
	"log"
	"net/url"
)

type Config struct {
	ServerAddress string
	BaseURL       string
}

func NewConfig() *Config {
	cfg := &Config{}

	flag.StringVar(&cfg.ServerAddress, "a", "localhost:8080", "Input host and port of server")
	flag.StringVar(&cfg.BaseURL, "b", "http://localhost:8080", "Base URL")

	flag.Parse()

	_, err := url.ParseRequestURI(cfg.BaseURL)
	if err != nil {
		log.Fatal("invalid base URL:", err)
	}

	return cfg
}
