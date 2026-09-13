// Package handlers implements the API contract as plain net/http handlers
// with the internal/platform ports injected. It never imports a cloud SDK or
// a runtime type; entrypoints under cmd adapt their runtime to it.
package handlers

import (
	"context"
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

type principalContextKey struct{}

// Deps are the ports the API mux needs. Callers construct adapters.
type Deps struct {
	Store       platform.LinkStore
	Allocator   platform.IDAllocator
	Clock       platform.Clock
	Credentials platform.CredentialStore
	Permuter    *core.Permuter
	PublicBase  string
	Stats       platform.StatsStore
	Invalidator platform.CacheInvalidator
	Audit       platform.AuditSink
}

type API struct {
	store       platform.LinkStore
	allocator   platform.IDAllocator
	clock       platform.Clock
	credentials platform.CredentialStore
	permuter    *core.Permuter
	publicBase  string
	statsStore  platform.StatsStore
	deleter     Deleter
}

// New returns the authenticated API mux.
func New(deps Deps) http.Handler {
	base := strings.TrimRight(deps.PublicBase, "/")
	if base == "" {
		base = defaultPublicBase
	}
	api := &API{
		store:       deps.Store,
		allocator:   deps.Allocator,
		clock:       deps.Clock,
		credentials: deps.Credentials,
		permuter:    deps.Permuter,
		publicBase:  base,
		statsStore:  deps.Stats,
		deleter: Deleter{
			Store:       deps.Store,
			Invalidator: deps.Invalidator,
			Audit:       deps.Audit,
			Clock:       deps.Clock,
		},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/links", api.create)
	mux.HandleFunc("GET /api/v1/links/{short_code}/stats", api.stats)
	mux.HandleFunc("DELETE /api/v1/links/{short_code}", api.delete)
	return api.authenticate(mux)
}

// PrincipalFromContext returns the authenticated principal injected by middleware.
func PrincipalFromContext(ctx context.Context) (platform.Principal, bool) {
	principal, ok := ctx.Value(principalContextKey{}).(platform.Principal)
	return principal, ok
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
