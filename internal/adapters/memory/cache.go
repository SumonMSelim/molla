package memory

import (
	"context"
	"sync"
	"time"

	"github.com/SumonMSelim/molla/internal/platform"
)

type Cache struct {
	mu      sync.RWMutex
	records map[string]platform.CacheRecord
}

func NewCache() *Cache {
	return &Cache{records: make(map[string]platform.CacheRecord)}
}

func (c *Cache) Get(_ context.Context, code string, now time.Time) (platform.CacheRecord, platform.CacheOutcome, error) {
	c.mu.RLock()
	record, ok := c.records[code]
	c.mu.RUnlock()
	if !ok {
		return platform.CacheRecord{}, platform.CacheMiss, nil
	}
	record = cloneCacheRecord(record)
	if record.State == platform.CacheDeleted {
		if !record.StaleUntil.IsZero() && !now.Before(record.StaleUntil) {
			return platform.CacheRecord{}, platform.CacheMiss, nil
		}
		record.LongURL = ""
		return record, platform.CacheHitDeleted, nil
	}
	if !now.Before(record.ExpiresAt) || !now.Before(record.StaleUntil) {
		return platform.CacheRecord{}, platform.CacheExpired, nil
	}
	if now.Before(record.FreshUntil) {
		return record, platform.CacheFresh, nil
	}
	return record, platform.CacheStale, nil
}

func (c *Cache) Put(_ context.Context, record platform.CacheRecord) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if existing, ok := c.records[record.ShortCode]; ok && record.Version < existing.Version {
		return nil
	}
	if record.State == platform.CacheDeleted {
		record.LongURL = ""
	}
	c.records[record.ShortCode] = cloneCacheRecord(record)
	return nil
}

func cloneCacheRecord(record platform.CacheRecord) platform.CacheRecord {
	if record.DeletedAt != nil {
		deletedAt := *record.DeletedAt
		record.DeletedAt = &deletedAt
	}
	return record
}

var _ platform.Cache = (*Cache)(nil)
