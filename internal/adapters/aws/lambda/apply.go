package lambda

import (
	"context"
	"errors"
	"time"

	"github.com/SumonMSelim/molla/internal/platform"
)

var ErrMalformed = errors.New("malformed invalidation payload")

// Apply writes a versioned deletion tombstone. Older versions do not overwrite.
func Apply(ctx context.Context, cache platform.Cache, req InvalidateRequest) error {
	if req.ShortCode == "" || req.Version < 1 || req.DeletedAt.IsZero() {
		return ErrMalformed
	}
	deletedAt := req.DeletedAt.UTC()
	return cache.Put(ctx, platform.CacheRecord{
		ShortCode:  req.ShortCode,
		State:      platform.CacheDeleted,
		Version:    req.Version,
		StaleUntil: deletedAt.Add(platform.DeleteTombstoneTTL),
		DeletedAt:  &deletedAt,
	})
}

func Tombstone(code string, version int64, deletedAt time.Time) platform.CacheRecord {
	t := deletedAt.UTC()
	return platform.CacheRecord{
		ShortCode:  code,
		State:      platform.CacheDeleted,
		Version:    version,
		StaleUntil: t.Add(platform.DeleteTombstoneTTL),
		DeletedAt:  &t,
	}
}
