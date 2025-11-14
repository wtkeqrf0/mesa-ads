package httpadapter

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// handleAdClick handles click redirects and records click events. It expects
// a {token} path parameter bound by the router. On success it redirects
// the user to the landing URL. Missing or invalid tokens result in
// HTTP 400, while unknown tokens result in HTTP 404. Internal errors are
// logged and treated as 404 to avoid leaking information.
func (h *Handler) handleAdClick(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if token == "" {
		http.Error(w, "missing token", http.StatusBadRequest)
		return
	}
	landingURL, err := h.svc.RegisterClick(r.Context(), token)
	if err != nil {
		h.logger.Error("click error", slog.Any("error", err))
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, landingURL, http.StatusFound)
}
