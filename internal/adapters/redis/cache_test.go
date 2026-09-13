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
