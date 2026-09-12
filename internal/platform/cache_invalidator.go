package platform

import "context"

// CacheInvalidator synchronously establishes a deletion tombstone.
type CacheInvalidator interface {
	Invalidate(context.Context, Deletion) error
}
