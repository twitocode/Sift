package handlers

import (
	"net/http"

	"github.com/twitocode/sift/internal/common"
)

func HandleHealth() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		common.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}
