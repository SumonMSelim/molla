package lambda

import (
	"context"
	"testing"
	"time"

	"github.com/SumonMSelim/molla/internal/adapters/memory"
	"github.com/SumonMSelim/molla/internal/platform"
)

func TestApplyRejectsMalformedAndHonorsVersion(t *testing.T) {
	cache := memory.NewCache()
	ctx := context.Background()
	if err := Apply(ctx, cache, InvalidateRequest{}); err != ErrMalformed {
		t.Fatalf("empty = %v", err)
	}
	now := time.Unix(1_700_000_000, 0).UTC()
	if err := Apply(ctx, cache, InvalidateRequest{ShortCode: "abc1234", Version: 3, DeletedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := Apply(ctx, cache, InvalidateRequest{ShortCode: "abc1234", Version: 2, DeletedAt: now.Add(-time.Hour)}); err != nil {
		t.Fatal(err)
	}
	got, outcome, err := cache.Get(ctx, "abc1234", now)
	if err != nil || outcome != platform.CacheHitDeleted || got.Version != 3 {
		t.Fatalf("got %+v %q %v", got, outcome, err)
	}
	if err := Apply(ctx, cache, InvalidateRequest{ShortCode: "abc1234", Version: 4, DeletedAt: now}); err != nil {
		t.Fatal(err)
	}
	got, _, err = cache.Get(ctx, "abc1234", now)
	if err != nil || got.Version != 4 {
		t.Fatalf("newer version = %+v", got)
	}
}
