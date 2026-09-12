// Package platform declares the ports every handler depends on (LinkStore,
// StatsStore, Cache, EventPublisher, IDAllocator, Clock) and the shared link
// record. Nothing here talks to a cloud.
package platform

import "time"

// Role identifies the authorization rules applicable to a principal.
type Role string

const (
	RoleDeveloper Role = "developer"
	RoleOperator  Role = "operator"
)

// Principal is an authenticated actor. Developers must have an owner ID;
// operators do not.
type Principal struct {
	ActorID string
	Role    Role
	OwnerID string
}

// Link is the durable, provider-neutral link record.
type Link struct {
	ShortCode    string
	LongURL      string
	OwnerID      string
	IsCustom     bool
	IsActive     bool
	Version      int64
	CreatedAt    time.Time
	ExpiresAt    time.Time
	DeletedAt    *time.Time
	PurgeAt      time.Time
	DeletedBy    string
	DeleteRole   Role
	DeleteReason string
}

// Deletion is the durable result of a soft deletion.
type Deletion struct {
	ShortCode string
	OwnerID   string
	Version   int64
	DeletedAt time.Time
	PurgeAt   time.Time
}
