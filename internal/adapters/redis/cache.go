package redis

import (
	"context"
	"encoding/json"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/SumonMSelim/molla/internal/platform"
)

const keyPrefix = "url:"

type cachePayload struct {
	State      string `json:"state"`
	LongURL    string `json:"long_url,omitempty"`
	ExpiresAt  int64  `json:"expires_at,omitempty"`
	FreshUntil int64  `json:"fresh_until,omitempty"`
	StaleUntil int64  `json:"stale_until"`
	Version    int64  `json:"version"`
	DeletedAt  int64  `json:"deleted_at,omitempty"`
}

// Cache is a Redis implementation of platform.Cache.
type Cache struct {
	client *goredis.Client
}

func NewCache(client *goredis.Client) *Cache {
	return &Cache{client: client}
}

func (c *Cache) Get(ctx context.Context, code string, now time.Time) (platform.CacheRecord, platform.CacheOutcome, error) {
	raw, err := c.client.Get(ctx, keyPrefix+code).Bytes()
	if err == goredis.Nil {
		return platform.CacheRecord{}, platform.CacheMiss, nil
	}
	if err != nil {
		return platform.CacheRecord{}, platform.CacheMiss, err
	}
	var payload cachePayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return platform.CacheRecord{}, platform.CacheMiss, err
	}
	record := recordFromPayload(code, payload)
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

func (c *Cache) Put(ctx context.Context, record platform.CacheRecord) error {
	raw, err := c.client.Get(ctx, keyPrefix+record.ShortCode).Bytes()
	if err != nil && err != goredis.Nil {
		return err
	}
	if err == nil {
		var existing cachePayload
		if json.Unmarshal(raw, &existing) == nil && record.Version < existing.Version {
			return nil
		}
	}
	if record.State == platform.CacheDeleted {
		record.LongURL = ""
	}
	body, err := json.Marshal(payloadFromRecord(record))
	if err != nil {
		return err
	}
	ttl := record.StaleUntil.Sub(time.Now().UTC())
	if record.State == platform.CacheActive {
		ttl = record.ExpiresAt.Sub(time.Now().UTC())
	}
	if ttl < time.Second {
		ttl = time.Second
	}
	return c.client.Set(ctx, keyPrefix+record.ShortCode, body, ttl).Err()
}

func payloadFromRecord(record platform.CacheRecord) cachePayload {
	p := cachePayload{
		State:      string(record.State),
		LongURL:    record.LongURL,
		ExpiresAt:  unixOrZero(record.ExpiresAt),
		FreshUntil: unixOrZero(record.FreshUntil),
		StaleUntil: unixOrZero(record.StaleUntil),
		Version:    record.Version,
	}
	if record.DeletedAt != nil {
		p.DeletedAt = record.DeletedAt.UTC().Unix()
	}
	return p
}

func recordFromPayload(code string, p cachePayload) platform.CacheRecord {
	record := platform.CacheRecord{
		ShortCode:  code,
		State:      platform.CacheState(p.State),
		LongURL:    p.LongURL,
		ExpiresAt:  time.Unix(p.ExpiresAt, 0).UTC(),
		FreshUntil: time.Unix(p.FreshUntil, 0).UTC(),
		StaleUntil: time.Unix(p.StaleUntil, 0).UTC(),
		Version:    p.Version,
	}
	if p.DeletedAt != 0 {
		t := time.Unix(p.DeletedAt, 0).UTC()
		record.DeletedAt = &t
	}
	return record
}

func unixOrZero(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UTC().Unix()
}

var _ platform.Cache = (*Cache)(nil)
