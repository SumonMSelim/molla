package platform

import (
	"context"
	"time"
)

type CacheState string

const (
	CacheActive        CacheState = "active"
	CacheDeleted       CacheState = "deleted"
	DeleteTombstoneTTL            = 24 * time.Hour
)

type CacheOutcome string

const (
	CacheMiss       CacheOutcome = "miss"
	CacheFresh      CacheOutcome = "fresh"
	CacheStale      CacheOutcome = "stale"
	CacheExpired    CacheOutcome = "expired"
	CacheHitDeleted CacheOutcome = "deleted"
)

// CacheRecord is an active destination or deletion tombstone.
type CacheRecord struct {
	ShortCode  string
	State      CacheState
	LongURL    string
	ExpiresAt  time.Time
	FreshUntil time.Time
	StaleUntil time.Time
	Version    int64
	DeletedAt  *time.Time
}

// Cache stores records with version-aware compare-and-set semantics.
type Cache interface {
	Get(context.Context, string, time.Time) (CacheRecord, CacheOutcome, error)
	Put(context.Context, CacheRecord) error
}
