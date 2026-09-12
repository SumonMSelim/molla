package memory

import (
	"context"
	"sync"

	"github.com/SumonMSelim/molla/internal/platform"
)

type IDAllocator struct {
	mu   sync.Mutex
	next int64
}

func NewIDAllocator(first int64) *IDAllocator {
	return &IDAllocator{next: first}
}

func (a *IDAllocator) Lease(_ context.Context) (int64, error) {
	a.mu.Lock()
	id := a.next
	a.next++
	a.mu.Unlock()
	return id, nil
}

var _ platform.IDAllocator = (*IDAllocator)(nil)
