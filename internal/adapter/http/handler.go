package httpadapter

import (
	"mesa-ads/internal/core/port"
	"net/http"

	"github.com/go-chi/chi/v5"
	"log/slog"
)

// Handler contains dependencies and routes. It is an inbound adapter for HTTP.
// It holds a Service to execute business logic and a logger for structured
// logging. Routes are registered on a chi.Router for convenient method
// handling.
type Handler struct {
	svc    port.AdUseCase
	logger *slog.Logger
	router chi.Router
}

// NewHandler creates a handler with all routes configured. It accepts a
// Service implementation and a logger. The returned Handler registers
// handlers for each endpoint on a new chi.Router.
func NewHandler(svc port.AdUseCase, logger *slog.Logger) *Handler {
	h := &Handler{svc: svc, logger: logger}
	r := chi.NewRouter()

	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/ad/request", h.handleAdRequest)
		r.Get("/ad/click/{token}", h.handleAdClick)
		r.Get("/stats/overview", h.handleStatsOverview)
	})
	h.router = r
	return h
}

// Router returns the underlying http.Handler.
func (h *Handler) Router() http.Handler {
	return h.router
}
