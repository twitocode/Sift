package app

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/twitocode/sift/internal/common"
	"github.com/twitocode/sift/internal/handlers"
)

func addRoutes(r *chi.Mux, cfg *Config, s *Services) {
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		common.WriteJSON(w, http.StatusOK, "Sift API")
	})

	r.Get("/", handlers.HandleHealth())
}
