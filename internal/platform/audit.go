package platform

import (
	"context"
	"time"
)

// AuditEvent records a structured deletion or takedown outcome.
type AuditEvent struct {
	ActorID   string
	Role      Role
	OwnerID   string
	ShortCode string
	Reason    string
	Outcome   string
	Timestamp time.Time
}

type AuditSink interface {
	Record(context.Context, AuditEvent) error
}
