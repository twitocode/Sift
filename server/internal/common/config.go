package common

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

type Config struct {
	Host                 string   `env:"HOST"`
	Port                 string   `env:"PORT" envDefault:"8000"`
	AppEnv               string   `env:"APP_ENV"`
	DatabaseURL          string   `env:"DATABASE_URL"`
	FrontendAppURL       string   `env:"FRONTEND_APP_URL"`
	SecretKey            string   `env:"SECRET_KEY"`
	InternalServiceToken string   `env:"INTERNAL_SERVICE_TOKEN"`
	AllowedOrigins       []string `env:"ALLOWED_ORIGINS" envSeparator:","`
	OpenRouterAPIKey     string   `env:"OPENROUTER_API_KEY"`
	SpiderCount          int      `env:"SPIDER_COUNT"`
	CrawlCount           int      `env:"CRAWL_COUNT"`
	MaxURLsPerHost       int      `env:"MAX_URLS_PER_HOST"`
	MaxHostQueues        int      `env:"MAX_HOST_QUEUES"`
	MaxPendingURLs       int      `env:"MAX_PENDING_URLS"`
	DispatchDelay        int      `env:"JOB_DISPATCH_DELAY"`
	ShowCrawlStats       bool     `env:"SHOW_CRAWL_STATS"`
	ShowIndexingStats       bool     `env:"SHOW_INDEXING_STATS"`
}

func loadDotEnv() {
	for _, path := range []string{
		".env",
		"server/.env",
		filepath.Join("..", ".env"),
		filepath.Join("..", "..", ".env"),
	} {
		if err := godotenv.Load(path); err == nil {
			return
		}
	}
}

func SQLitePath() string {
	for _, path := range []string{
		"db/sqlite/sift.db",
		filepath.Join("..", "..", "db", "sqlite", "sift.db"),
	} {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return "db/sqlite/sift.db"
}

func NewConfig(_ func(string) string) *Config {
	loadDotEnv()

	cfg, err := env.ParseAs[Config]()

	if err != nil {
		panic(fmt.Errorf("parse environment config: %w", err))
	}

	if len(cfg.AllowedOrigins) == 0 {
		if cfg.FrontendAppURL != "" {
			cfg.AllowedOrigins = []string{cfg.FrontendAppURL}
		} else {
			cfg.AllowedOrigins = []string{"http://localhost:3000"}
		}
	}

	return &cfg
}
