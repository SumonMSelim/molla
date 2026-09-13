package handlers

import (
	"context"
	"errors"
	"net/http"

	"github.com/SumonMSelim/molla/internal/adapters/logging"
	"github.com/SumonMSelim/molla/internal/platform"
)

func (a *API) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("X-Api-Key")
		if token == "" {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED")
			return
		}
		principal, err := a.credentials.Resolve(r.Context(), token, a.clock.Now())
		if err != nil {
			if errors.Is(err, platform.ErrDependency) {
				logging.FromContext(r.Context()).Error("credential resolve failed", "error", err)
				writeError(w, http.StatusServiceUnavailable, "TEMPORARILY_UNAVAILABLE")
				return
			}
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED")
			return
		}
		ctx := context.WithValue(r.Context(), principalContextKey{}, principal)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
