package platform

import "context"

type IDAllocator interface {
	Lease(context.Context) (int64, error)
}
