package core

import (
	"context"
	"time"

	"github.com/SumonMSelim/molla/internal/platform"
)

type clickBucket struct {
	n    int64
	last time.Time
}

// AggregateClicks folds a batch of click events into one Increment per short code.
func AggregateClicks(ctx context.Context, store platform.StatsStore, events []platform.ClickEvent) error {
	grouped := make(map[string]clickBucket)
	for _, event := range events {
		if event.ShortCode == "" {
			continue
		}
		bucket := grouped[event.ShortCode]
		bucket.n++
		if event.OccurredAt.After(bucket.last) {
			bucket.last = event.OccurredAt
		}
		grouped[event.ShortCode] = bucket
	}
	for code, bucket := range grouped {
		if err := store.Increment(ctx, code, bucket.n, bucket.last); err != nil {
			return err
		}
	}
	return nil
}
