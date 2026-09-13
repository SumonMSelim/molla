package memory

import (
	"context"
	"sync"
	"time"

	"github.com/SumonMSelim/molla/internal/platform"
)

type statsRecord struct {
	clicks      int64
	lastClickAt time.Time
}

// StatsStore is an in-memory implementation of platform.StatsStore.
type StatsStore struct {
	mu    sync.RWMutex
	items map[string]statsRecord
}

func NewStatsStore() *StatsStore {
	return &StatsStore{items: make(map[string]statsRecord)}
}

func (s *StatsStore) Get(_ context.Context, code string) (platform.Stats, error) {
	s.mu.RLock()
	record, ok := s.items[code]
	s.mu.RUnlock()
	if !ok {
		return platform.Stats{}, platform.ErrNotFound
	}
	return platform.Stats{ShortCode: code, Clicks: record.clicks, LastClickAt: record.lastClickAt}, nil
}

func (s *StatsStore) Increment(_ context.Context, code string, delta int64, lastClickAt time.Time) error {
	if code == "" || delta == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	record := s.items[code]
	record.clicks += delta
	if lastClickAt.After(record.lastClickAt) {
		record.lastClickAt = lastClickAt
	}
	s.items[code] = record
	return nil
}

var _ platform.StatsStore = (*StatsStore)(nil)
