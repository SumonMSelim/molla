package platform

import "time"

type Clock interface {
	Now() time.Time
}
