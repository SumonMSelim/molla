package dynamodb

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/SumonMSelim/molla/internal/platform"
)

func TestStatsStoreIncrementAndGet(t *testing.T) {
	client, _ := testClient(t)
	store := NewStatsStore(client)
	ctx := context.Background()
	if _, err := store.Get(ctx, "missing"); !errors.Is(err, platform.ErrNotFound) {
		t.Fatalf("Get missing = %v", err)
	}
	at := time.Unix(1_700_000_010, 0).UTC()
	if err := store.Increment(ctx, "abc1234", 10, at); err != nil {
		t.Fatal(err)
	}
	got, err := store.Get(ctx, "abc1234")
	if err != nil {
		t.Fatal(err)
	}
	if got.Clicks != 10 || !got.LastClickAt.Equal(at) || got.ShortCode != "abc1234" {
		t.Fatalf("stats = %+v", got)
	}
}

func TestStatsStoreSkipsEmpty(t *testing.T) {
	client, fake := testClient(t)
	store := NewStatsStore(client)
	if err := store.Increment(context.Background(), "", 5, time.Now()); err != nil {
		t.Fatal(err)
	}
	if fake.RequestCount() != 0 {
		t.Fatalf("requests = %d", fake.RequestCount())
	}
}
