package httpadapter

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"mesa-ads/internal/core/port"
)

// handleStatsOverview returns aggregated statistics for campaigns over a
// specified period. It accepts optional `from`, `to` (RFC3339 timestamps) and
// `campaign_id` query parameters. If no period is provided, it defaults to
// the last 24 hours. Invalid parameters result in HTTP 400. Internal errors
// produce HTTP 500. On success it writes a JSON representation of the stats.
func (h *Handler) handleStatsOverview(w http.ResponseWriter, r *http.Request) {
	var (
		q       = r.URL.Query()
		fromStr = q.Get("from")
		toStr   = q.Get("to")
		req     port.StatsReq
		err     error
	)

	if fromStr != "" {
		req.From, err = time.Parse(time.RFC3339, fromStr)
		if err != nil {
			http.Error(w, "invalid 'from' timestamp", http.StatusBadRequest)
			return
		}
	} else {
		req.From = time.Now().Add(-24 * time.Hour)
	}

	if toStr != "" {
		req.To, err = time.Parse(time.RFC3339, toStr)
		if err != nil {
			http.Error(w, "invalid 'to' timestamp", http.StatusBadRequest)
			return
		}
	} else {
		req.To = time.Now()
	}

	if cid := q.Get("campaign_id"); cid != "" {
		id, err := strconv.ParseInt(cid, 10, 64)
		if err != nil {
			http.Error(w, "invalid campaign_id", http.StatusBadRequest)
			return
		}
		req.CampaignID = &id
	}

	stats, err := h.svc.GetStats(r.Context(), req)
	if err != nil {
		h.logger.Error("stats error", slog.Any("error", err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err = json.NewEncoder(w).Encode(stats); err != nil {
		h.logger.Error("encode response error", slog.Any("error", err))
	}
}
