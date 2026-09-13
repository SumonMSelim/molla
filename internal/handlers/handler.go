// Package handlers implements the API contract as plain net/http handlers
// with the internal/platform ports injected. It never imports a cloud SDK or
// a runtime type; entrypoints under cmd adapt their runtime to it.
package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/SumonMSelim/molla/internal/core"
	"github.com/SumonMSelim/molla/internal/platform"
)

const (
	defaultPublicBase = "https://mol.la"
	maxCreateBody     = 16 * 1024
	idempotencyTTL    = 24 * time.Hour
	maxCodeAttempts   = 32
)

// Deps are the ports the API mux needs. Callers construct adapters.
//
// The create and stats routes are unauthenticated: anyone may shorten a URL
// or read a code's click count. There is no per-caller ownership in this
// release; takedown is operator-only via cmd/admin, which uses Deleter
// directly rather than through this HTTP API. A later release adds a
// UI-driven token system and reintroduces per-owner delete.
type Deps struct {
	Store      platform.LinkStore
	Allocator  platform.IDAllocator
	Clock      platform.Clock
	Permuter   *core.Permuter
	PublicBase string
	Stats      platform.StatsStore
}

type API struct {
	store      platform.LinkStore
	allocator  platform.IDAllocator
	clock      platform.Clock
	permuter   *core.Permuter
	publicBase string
	statsStore platform.StatsStore
}

// New returns the public, unauthenticated API mux.
func New(deps Deps) http.Handler {
	base := strings.TrimRight(deps.PublicBase, "/")
	if base == "" {
		base = defaultPublicBase
	}
	api := &API{
		store:      deps.Store,
		allocator:  deps.Allocator,
		clock:      deps.Clock,
		permuter:   deps.Permuter,
		publicBase: base,
		statsStore: deps.Stats,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/links", api.create)
	mux.HandleFunc("GET /api/v1/links/{short_code}/stats", api.stats)
	return mux
}

type errorResponse struct {
	Error string `json:"error"`
}

func writeError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, errorResponse{Error: code})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	payload, err := json.Marshal(body)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":"TEMPORARILY_UNAVAILABLE"}`))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(payload)
}
