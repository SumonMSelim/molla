package memory

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestIDAllocatorConcurrentLeasesAreUnique(t *testing.T) {
	const total = 10_000
	allocator := NewIDAllocator(0)
	ids := make(chan int64, total)
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range total / 2 {
				id, err := allocator.Lease(context.Background())
				if err != nil {
					errs <- err
					return
				}
				ids <- id
			}
		}()
	}
	wg.Wait()
	close(ids)
	close(errs)
	for err := range errs {
		t.Fatalf("Lease() error = %v", err)
	}

	seen := make(map[int64]bool, total)
	for id := range ids {
		if seen[id] {
			t.Fatalf("duplicate ID %d", id)
		}
		seen[id] = true
	}
	if len(seen) != total {
		t.Fatalf("unique IDs = %d, want %d", len(seen), total)
	}
}

func TestClockIsInjectable(t *testing.T) {
	start := time.Unix(1_700_000_000, 0).UTC()
	clock := NewClock(start)
	if got := clock.Now(); !got.Equal(start) {
		t.Fatalf("Now() = %v", got)
	}
	clock.Advance(time.Minute)
	if got := clock.Now(); !got.Equal(start.Add(time.Minute)) {
		t.Fatalf("Now() after Advance = %v", got)
	}
	clock.Set(start)
	if got := clock.Now(); !got.Equal(start) {
		t.Fatalf("Now() after Set = %v", got)
	}
}
