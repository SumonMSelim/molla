package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"

	"github.com/SumonMSelim/molla/internal/platform"
)

func testCache(t *testing.T) (*Cache, *miniredis.Miniredis) {
	t.Helper()
	mini := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mini.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return NewCache(client), mini
}

func TestCacheMissIsNotError(t *testing.T) {
	cache, _ := testCache(t)
	_, outcome, err := cache.Get(context.Background(), "missing", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if outcome != platform.CacheMiss {
		t.Fatalf("outcome = %q", outcome)
	}
}

func TestCacheVersionCompareAndOutcomes(t *testing.T) {
	cache, _ := testCache(t)
	ctx := context.Background()
	base := time.Unix(1_700_000_000, 0).UTC()
	active := platform.CacheRecord{
		ShortCode: "code", State: platform.CacheActive, LongURL: "https://example.com",
		ExpiresAt: base.Add(3 * time.Hour), FreshUntil: base.Add(time.Hour),
		StaleUntil: base.Add(2 * time.Hour), Version: 2,
	}
	if err := cache.Put(ctx, active); err != nil {
		t.Fatal(err)
	}
	older := active
	older.Version = 1
	older.LongURL = "https://stale.example"
	if err := cache.Put(ctx, older); err != nil {
		t.Fatal(err)
	}
	got, outcome, err := cache.Get(ctx, "code", base)
	if err != nil || outcome != platform.CacheFresh || got.LongURL != "https://example.com" {
		t.Fatalf("fresh = %+v %q %v", got, outcome, err)
	}
	deletedAt := base
	tombstone := platform.CacheRecord{
		ShortCode: "code", State: platform.CacheDeleted, Version: 3,
		StaleUntil: base.Add(24 * time.Hour), DeletedAt: &deletedAt,
	}
	if err := cache.Put(ctx, tombstone); err != nil {
		t.Fatal(err)
	}
	got, outcome, err = cache.Get(ctx, "code", base)
	if err != nil || outcome != platform.CacheHitDeleted || got.LongURL != "" {
		t.Fatalf("tombstone = %+v %q %v", got, outcome, err)
	}
}

func TestPutDoesNotOverwriteNewerVersion(t *testing.T) {
	cache, _ := testCache(t)
	ctx := context.Background()
	base := time.Now().UTC()
	tombstoneAt := base
	tombstone := platform.CacheRecord{
		ShortCode: "code", State: platform.CacheDeleted, Version: 2,
		StaleUntil: base.Add(24 * time.Hour), DeletedAt: &tombstoneAt,
	}
	if err := cache.Put(ctx, tombstone); err != nil {
		t.Fatal(err)
	}
	// A delayed lower-version active populate must not resurrect the link.
	older := platform.CacheRecord{
		ShortCode: "code", State: platform.CacheActive, LongURL: "https://stale.example",
		ExpiresAt: base.Add(3 * time.Hour), FreshUntil: base.Add(time.Hour),
		StaleUntil: base.Add(2 * time.Hour), Version: 1,
	}
	if err := cache.Put(ctx, older); err != nil {
		t.Fatal(err)
	}
	got, outcome, err := cache.Get(ctx, "code", base)
	if err != nil {
		t.Fatal(err)
	}
	if outcome != platform.CacheHitDeleted || got.Version != 2 || got.LongURL != "" {
		t.Fatalf("lower version overwrote newer: %+v %q", got, outcome)
	}
}

func TestPutEqualVersionIsIdempotentWrite(t *testing.T) {
	cache, _ := testCache(t)
	ctx := context.Background()
	base := time.Now().UTC()
	record := platform.CacheRecord{
		ShortCode: "code", State: platform.CacheActive, LongURL: "https://example.com",
		ExpiresAt: base.Add(3 * time.Hour), FreshUntil: base.Add(time.Hour),
		StaleUntil: base.Add(2 * time.Hour), Version: 5,
	}
	if err := cache.Put(ctx, record); err != nil {
		t.Fatal(err)
	}
	retry := record
	retry.LongURL = "https://retry.example"
	if err := cache.Put(ctx, retry); err != nil {
		t.Fatal(err)
	}
	got, _, err := cache.Get(ctx, "code", base)
	if err != nil || got.LongURL != "https://retry.example" {
		t.Fatalf("equal version should write: %+v %v", got, err)
	}
}

func TestPutActiveTTLBoundedByStaleUntil(t *testing.T) {
	cache, mini := testCache(t)
	ctx := context.Background()
	base := time.Now().UTC()
	record := platform.CacheRecord{
		ShortCode: "code", State: platform.CacheActive, LongURL: "https://example.com",
		// ExpiresAt is years out; the key must not outlive StaleUntil.
		ExpiresAt: base.Add(5 * 365 * 24 * time.Hour), FreshUntil: base.Add(time.Hour),
		StaleUntil: base.Add(2 * time.Hour), Version: 1,
	}
	if err := cache.Put(ctx, record); err != nil {
		t.Fatal(err)
	}
	ttl := mini.TTL(keyPrefix + "code")
	if ttl <= 0 {
		t.Fatalf("expected a TTL, got %v", ttl)
	}
	if ttl > 2*time.Hour {
		t.Fatalf("ttl = %v, want <= 2h (StaleUntil), not ExpiresAt", ttl)
	}
	if ttl < 2*time.Hour-time.Minute {
		t.Fatalf("ttl = %v, want ~2h", ttl)
	}
	// Past StaleUntil the key is gone, so the record cannot be served stale.
	mini.FastForward(2 * time.Hour)
	if _, outcome, err := cache.Get(ctx, "code", base.Add(2*time.Hour)); err != nil || outcome != platform.CacheMiss {
		t.Fatalf("after StaleUntil: %q %v", outcome, err)
	}
}
