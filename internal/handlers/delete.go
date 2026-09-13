package handlers

import (
	"context"
	"errors"
	"net/http"

	"github.com/SumonMSelim/molla/internal/adapters/logging"
	"github.com/SumonMSelim/molla/internal/core"
	"github.com/SumonMSelim/molla/internal/platform"
)

const (
	auditDeleted          = "deleted"
	auditInvalidateFailed = "invalidate_failed"
)

// Deleter is the shared owner/operator deletion service.
type Deleter struct {
	Store       platform.LinkStore
	Invalidator platform.CacheInvalidator
	Audit       platform.AuditSink
	Clock       platform.Clock
}

func (d Deleter) Delete(ctx context.Context, principal platform.Principal, code, reason string) error {
	now := d.Clock.Now().UTC()
	deletion, err := d.Store.SoftDelete(ctx, principal, code, now, reason)
	if err != nil {
		if errors.Is(err, platform.ErrDependency) {
			logging.FromContext(ctx).Error("soft delete failed", "short_code", code, "actor_id", principal.ActorID, "error", err)
		}
		return err
	}
	log := logging.FromContext(ctx).With("short_code", code, "actor_id", principal.ActorID)
	if err := d.Invalidator.Invalidate(ctx, deletion); err != nil {
		log.Error("cache invalidation failed after soft delete", "error", err)
		if auditErr := d.Audit.Record(ctx, platform.AuditEvent{
			ActorID:   principal.ActorID,
			Role:      principal.Role,
			OwnerID:   deletion.OwnerID,
			ShortCode: deletion.ShortCode,
			Reason:    reason,
			Outcome:   auditInvalidateFailed,
			Timestamp: now,
		}); auditErr != nil {
			log.Error("audit write failed", "outcome", auditInvalidateFailed, "error", auditErr)
		}
		return platform.ErrDependency
	}
	if auditErr := d.Audit.Record(ctx, platform.AuditEvent{
		ActorID:   principal.ActorID,
		Role:      principal.Role,
		OwnerID:   deletion.OwnerID,
		ShortCode: deletion.ShortCode,
		Reason:    reason,
		Outcome:   auditDeleted,
		Timestamp: now,
	}); auditErr != nil {
		log.Error("audit write failed", "outcome", auditDeleted, "error", auditErr)
	}
	return nil
}

func (a *API) delete(w http.ResponseWriter, r *http.Request) {
	principal, ok := PrincipalFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED")
		return
	}
	code := r.PathValue("short_code")
	if core.ValidateAlias(code) != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND")
		return
	}
	err := a.deleter.Delete(r.Context(), principal, code, "")
	if err == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeDeleteError(w, err)
}

func writeDeleteError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, platform.ErrNotFound):
		writeError(w, http.StatusNotFound, "NOT_FOUND")
	case errors.Is(err, platform.ErrForbidden):
		writeError(w, http.StatusForbidden, "FORBIDDEN")
	default:
		writeError(w, http.StatusServiceUnavailable, "TEMPORARILY_UNAVAILABLE")
	}
}
