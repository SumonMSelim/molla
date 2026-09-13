package platform

import (
	"context"
	"time"
)

// Stats is the asynchronously aggregated click counter for one short code.
type Stats struct {
	ShortCode   string
	Clicks      int64
	LastClickAt time.Time
}

// StatsStore is the analytics counter port. Redirect never writes it.
type StatsStore interface {
	Get(context.Context, string) (Stats, error)
	Increment(ctx context.Context, shortCode string, delta int64, lastClickAt time.Time) error
}
