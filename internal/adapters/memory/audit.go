package memory

import (
	"context"
	"sync"

	"github.com/SumonMSelim/molla/internal/platform"
)

type AuditSink struct {
	mu     sync.RWMutex
	events []platform.AuditEvent
}

func (s *AuditSink) Record(_ context.Context, event platform.AuditEvent) error {
	s.mu.Lock()
	s.events = append(s.events, event)
	s.mu.Unlock()
	return nil
}

func (s *AuditSink) Events() []platform.AuditEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]platform.AuditEvent(nil), s.events...)
}

var _ platform.AuditSink = (*AuditSink)(nil)
