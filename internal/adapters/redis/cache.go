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

// putScript is the version-aware compare-and-set from DESIGN.md, run as a
// single atomic Lua call: it reads the stored version and writes only when the
// incoming version is greater than or equal to it. A separate GET then SET
// would let a concurrent writer (e.g. the invalidate Lambda tombstoning a
// deleted link) land between the two calls and be overwritten with stale data.
var putScript = goredis.NewScript(`
local existing = redis.call('GET', KEYS[1])
if existing then
  local ok, decoded = pcall(cjson.decode, existing)
  if ok and type(decoded) == 'table' and decoded['version'] then
    if tonumber(decoded['version']) > tonumber(ARGV[2]) then
      return 0
    end
  end
end
redis.call('SET', KEYS[1], ARGV[1], 'PX', tonumber(ARGV[3]))
return 1
`)

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
	if record.State == platform.CacheDeleted {
		record.LongURL = ""
	}
	body, err := json.Marshal(payloadFromRecord(record))
	if err != nil {
		return err
	}
	ttl := record.StaleUntil.Sub(time.Now().UTC())
	if ttl < time.Second {
		ttl = time.Second
	}
	return putScript.Run(ctx, c.client,
		[]string{keyPrefix + record.ShortCode},
		body, record.Version, ttl.Milliseconds(),
	).Err()
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
