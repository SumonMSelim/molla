package memory

import (
	"context"
	"sync"

	"github.com/SumonMSelim/molla/internal/platform"
)

type EventPublisher struct {
	mu     sync.RWMutex
	events []platform.ClickEvent
}

func (p *EventPublisher) Publish(_ context.Context, event platform.ClickEvent) error {
	p.mu.Lock()
	p.events = append(p.events, event)
	p.mu.Unlock()
	return nil
}

func (p *EventPublisher) Events() []platform.ClickEvent {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return append([]platform.ClickEvent(nil), p.events...)
}

var _ platform.EventPublisher = (*EventPublisher)(nil)
