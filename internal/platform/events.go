package platform

import (
	"context"
	"time"
)

// ClickEvent is the provider-neutral redirect analytics event.
type ClickEvent struct {
	EventID      string
	ShortCode    string
	OccurredAt   time.Time
	SourceIPHash string
}

type EventPublisher interface {
	Publish(context.Context, ClickEvent) error
}
