package app

import (
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httprate"
	"go.uber.org/zap"

	mw "github.com/twitocode/sift/internal/middleware"
)

func NewServer(cfg *Config, s *Services, log *zap.Logger) *chi.Mux {
	r := chi.NewRouter()

	r.Use(mw.AttachLogger(log))

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.AllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Use(middleware.Recoverer)
	r.Use(mw.LoggingMiddleware(log, cfg.AppEnv != "production"))
	r.Use(middleware.CleanPath)
	r.Use(middleware.RedirectSlashes)
	r.Use(httprate.LimitByIP(100, 1*time.Minute))

	addRoutes(r, cfg, s)

	return r
}