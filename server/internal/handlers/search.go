package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/twitocode/sift/internal/common"
	"github.com/twitocode/sift/internal/services"
)

func HandleSearch(ss *services.SearchService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := chi.URLParam(r, "query")

		results := ss.Search(r.Context(), q)
		common.WriteJSON(w, http.StatusOK, results)
	}
}
