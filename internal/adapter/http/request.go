package httpadapter

import (
	"encoding/json"
	"log/slog"
	"mesa-ads/internal/core/domain"
	"net/http"
)

// handleAdRequest processes an ad request and returns a creative. The
// request body is decoded into a model.UserContext. On success it
// returns a JSON representation of the selected creative. If no creative
// is available it returns HTTP 204 No Content. Any internal error
// results in HTTP 500. Parsing errors produce HTTP 400.
func (h *Handler) handleAdRequest(w http.ResponseWriter, r *http.Request) {
	var userCtx domain.UserContext
	if err := json.NewDecoder(r.Body).Decode(&userCtx); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	resp, err := h.svc.RequestAd(r.Context(), userCtx)
	if err != nil {
		h.logger.Error("request ad error", slog.Any("error", err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if resp == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err = json.NewEncoder(w).Encode(resp); err != nil {
		// encoding should rarely fail; log and send generic error
		h.logger.Error("encode response error", slog.Any("error", err))
	}
}
