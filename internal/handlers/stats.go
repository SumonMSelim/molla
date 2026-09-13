package handlers

import (
	"errors"
	"net/http"
	"time"

	"github.com/SumonMSelim/molla/internal/adapters/logging"
	"github.com/SumonMSelim/molla/internal/core"
	"github.com/SumonMSelim/molla/internal/platform"
)

type statsResponse struct {
	ShortCode   string `json:"short_code"`
	Clicks      int64  `json:"clicks"`
	CreatedAt   string `json:"created_at"`
	LastClickAt string `json:"last_click_at,omitempty"`
}

func (a *API) stats(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("short_code")
	if core.ValidateAlias(code) != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND")
		return
	}

	link, err := a.store.Get(r.Context(), code)
	if err != nil {
		if errors.Is(err, platform.ErrNotFound) {
			writeError(w, http.StatusNotFound, "NOT_FOUND")
			return
		}
		logging.FromContext(r.Context()).Error("link lookup failed", "short_code", code, "error", err)
		writeError(w, http.StatusServiceUnavailable, "TEMPORARILY_UNAVAILABLE")
		return
	}
	// Stats are public, but a taken-down or deleted link should not confirm
	// its own existence or leak its click history.
	if !link.IsActive {
		writeError(w, http.StatusNotFound, "NOT_FOUND")
		return
	}

	record, err := a.statsStore.Get(r.Context(), code)
	if err != nil {
		if !errors.Is(err, platform.ErrNotFound) {
			logging.FromContext(r.Context()).Error("stats lookup failed", "short_code", code, "error", err)
			writeError(w, http.StatusServiceUnavailable, "TEMPORARILY_UNAVAILABLE")
			return
		}
		record = platform.Stats{ShortCode: code}
	}

	resp := statsResponse{
		ShortCode: link.ShortCode,
		Clicks:    record.Clicks,
		CreatedAt: link.CreatedAt.UTC().Format(time.RFC3339),
	}
	if !record.LastClickAt.IsZero() {
		resp.LastClickAt = record.LastClickAt.UTC().Format(time.RFC3339)
	}
	writeJSON(w, http.StatusOK, resp)
}
