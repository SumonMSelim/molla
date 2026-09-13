package memory

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/SumonMSelim/molla/internal/platform"
)

func TestStatsStoreGetMissing(t *testing.T) {
	store := NewStatsStore()
	_, err := store.Get(context.Background(), "missing")
	if !errors.Is(err, platform.ErrNotFound) {
		t.Fatalf("Get() error = %v", err)
	}
}

func TestStatsStoreIncrementThenGet(t *testing.T) {
	store := NewStatsStore()
	ctx := context.Background()
	first := time.Unix(1_700_000_000, 0).UTC()
	later := first.Add(time.Minute)
	if err := store.Increment(ctx, "abc1234", 3, first); err != nil {
		t.Fatal(err)
	}
	if err := store.Increment(ctx, "abc1234", 7, later); err != nil {
		t.Fatal(err)
	}
	if err := store.Increment(ctx, "abc1234", 1, first); err != nil {
		t.Fatal(err)
	}
	got, err := store.Get(ctx, "abc1234")
	if err != nil {
		t.Fatal(err)
	}
	if got.ShortCode != "abc1234" || got.Clicks != 11 || !got.LastClickAt.Equal(later) {
		t.Fatalf("Get() = %+v", got)
	}
}

func TestStatsStoreIncrementSkipsEmptyOrZero(t *testing.T) {
	store := NewStatsStore()
	ctx := context.Background()
	if err := store.Increment(ctx, "", 5, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := store.Increment(ctx, "code", 0, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(ctx, "code"); !errors.Is(err, platform.ErrNotFound) {
		t.Fatalf("Get() error = %v", err)
	}
}

func TestStatsStoreConcurrentIncrement(t *testing.T) {
	store := NewStatsStore()
	ctx := context.Background()
	now := time.Unix(1_700_000_000, 0).UTC()
	var wg sync.WaitGroup
	for range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := store.Increment(ctx, "hot", 1, now); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	got, err := store.Get(ctx, "hot")
	if err != nil {
		t.Fatal(err)
	}
	if got.Clicks != 100 {
		t.Fatalf("clicks = %d", got.Clicks)
	}
}
