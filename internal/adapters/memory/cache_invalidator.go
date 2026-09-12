package memory

import (
	"context"
	"sync"

	"github.com/SumonMSelim/molla/internal/platform"
)

type CacheInvalidator struct {
	cache      *Cache
	mu         sync.RWMutex
	tombstones []platform.Deletion
}

func NewCacheInvalidator(cache *Cache) *CacheInvalidator {
	return &CacheInvalidator{cache: cache}
}

func (i *CacheInvalidator) Invalidate(ctx context.Context, deletion platform.Deletion) error {
	deletedAt := deletion.DeletedAt
	record := platform.CacheRecord{
		ShortCode:  deletion.ShortCode,
		State:      platform.CacheDeleted,
		Version:    deletion.Version,
		StaleUntil: deletion.DeletedAt.Add(platform.DeleteTombstoneTTL),
		DeletedAt:  &deletedAt,
	}
	if err := i.cache.Put(ctx, record); err != nil {
		return err
	}
	i.mu.Lock()
	i.tombstones = append(i.tombstones, deletion)
	i.mu.Unlock()
	return nil
}

func (i *CacheInvalidator) Tombstones() []platform.Deletion {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return append([]platform.Deletion(nil), i.tombstones...)
}

var _ platform.CacheInvalidator = (*CacheInvalidator)(nil)
