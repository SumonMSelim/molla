package main

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"

	awslambda "github.com/SumonMSelim/molla/internal/adapters/aws/lambda"
	redisadapter "github.com/SumonMSelim/molla/internal/adapters/redis"
	"github.com/SumonMSelim/molla/internal/platform"
	goredis "github.com/redis/go-redis/v9"
)

func TestHandleWithTombstoneVersions(t *testing.T) {
	mini := miniredis.RunT(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	if err := handleWith(context.Background(), awslambda.InvalidateRequest{ShortCode: "abc1234", Version: 2, DeletedAt: now}, mini.Addr()); err != nil {
		t.Fatal(err)
	}
	if err := handleWith(context.Background(), awslambda.InvalidateRequest{ShortCode: "abc1234", Version: 1, DeletedAt: now}, mini.Addr()); err != nil {
		t.Fatal(err)
	}
	client := goredis.NewClient(&goredis.Options{Addr: mini.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	cache := redisadapter.NewCache(client)
	got, outcome, err := cache.Get(context.Background(), "abc1234", now)
	if err != nil || outcome != platform.CacheHitDeleted || got.Version != 2 {
		t.Fatalf("got %+v %q %v", got, outcome, err)
	}
	if err := handleWith(context.Background(), awslambda.InvalidateRequest{}, mini.Addr()); err == nil {
		t.Fatal("expected malformed")
	}
	if err := handleWith(context.Background(), awslambda.InvalidateRequest{ShortCode: "x", Version: 1, DeletedAt: now}, ""); err == nil {
		t.Fatal("expected missing redis")
	}
}
