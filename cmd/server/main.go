// Command server serves every route from internal/handlers over plain
// net/http in one process. It is the local development target (in-memory
// adapters) and the entrypoint for any container or VM runtime.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/SumonMSelim/molla/internal/adapters/memory"
	"github.com/SumonMSelim/molla/internal/core"
	"github.com/SumonMSelim/molla/internal/handlers"
	"github.com/SumonMSelim/molla/internal/platform"
)

type liveClock struct{}

func (liveClock) Now() time.Time { return time.Now().UTC() }

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func newHandler() (http.Handler, error) {
	permuter, err := core.NewPermuter([]byte(os.Getenv("MOLLA_PERMUTATION_KEY")))
	if err != nil {
		return nil, err
	}
	privacy := os.Getenv("MOLLA_PRIVACY_KEY")
	if privacy == "" {
		return nil, errors.New("MOLLA_PRIVACY_KEY required")
	}

	clock := liveClock{}
	now := clock.Now()
	store := memory.NewLinkStore()
	cache := memory.NewCache()
	creds := memory.NewCredentialStore()
	token := envOr("MOLLA_DEV_API_KEY", "dev-local-key")
	err = creds.Store(context.Background(), token, platform.Credential{
		ActorID:   "local-dev",
		OwnerID:   "local-owner",
		Status:    platform.CredentialActive,
		IssuedAt:  now,
		ExpiresAt: now.Add(platform.MaximumCredentialLifetime),
	})
	if err != nil {
		return nil, err
	}

	api := handlers.New(handlers.Deps{
		Store:       store,
		Allocator:   memory.NewIDAllocator(0),
		Clock:       clock,
		Credentials: creds,
		Permuter:    permuter,
		PublicBase:  envOr("MOLLA_PUBLIC_BASE", "http://127.0.0.1:8080"),
		Stats:       memory.NewStatsStore(),
		Invalidator: memory.NewCacheInvalidator(cache),
		Audit:       &memory.AuditSink{},
	})
	redirect := handlers.NewRedirect(handlers.RedirectDeps{
		Store:      store,
		Cache:      cache,
		Publisher:  &memory.EventPublisher{},
		Clock:      clock,
		PrivacyKey: []byte(privacy),
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			api.ServeHTTP(w, r)
			return
		}
		redirect.ServeHTTP(w, r)
	}), nil
}

func main() {
	handler, err := newHandler()
	if err != nil {
		log.Fatal(err)
	}
	addr := envOr("MOLLA_LISTEN", ":8080")
	log.Fatal(http.ListenAndServe(addr, handler))
}
