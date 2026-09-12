package platform

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound            = errors.New("not found")
	ErrForbidden           = errors.New("forbidden")
	ErrCollision           = errors.New("link collision")
	ErrIdempotencyConflict = errors.New("idempotency conflict")
	ErrDependency          = errors.New("dependency unavailable")
	ErrUnauthorized        = errors.New("unauthorized")
	ErrInvalidCredential   = errors.New("invalid credential")
)

const DeleteRetention = 2_592_000 * time.Second

// Idempotency describes the optional owner-scoped create replay record.
type Idempotency struct {
	Key         string
	RequestHash string
	ExpiresAt   time.Time
}

// LinkStore is the authoritative link persistence port.
type LinkStore interface {
	Create(context.Context, Link, *Idempotency) (Link, error)
	Get(context.Context, string) (Link, error)
	SoftDelete(context.Context, Principal, string, time.Time, string) (Deletion, error)
}
