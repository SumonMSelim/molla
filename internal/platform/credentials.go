package platform

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"
)

type CredentialStatus string

const (
	CredentialActive  CredentialStatus = "active"
	CredentialRevoked CredentialStatus = "revoked"

	MaximumCredentialLifetime = 90 * 24 * time.Hour
)

// Credential is non-secret developer-token metadata. Adapters must never persist
// the raw token; store HashToken(token) instead.
type Credential struct {
	ActorID   string
	OwnerID   string
	Status    CredentialStatus
	IssuedAt  time.Time
	ExpiresAt time.Time
}

// CredentialStore provisions, resolves, and revokes developer tokens.
type CredentialStore interface {
	Store(context.Context, string, Credential) error
	Resolve(context.Context, string, time.Time) (Principal, error)
	// Revoke marks the credential with the given token hash revoked. It returns
	// ErrNotFound when no credential holds that hash.
	Revoke(context.Context, string) error
}

// HashToken returns the SHA-256 hex digest of a raw developer token.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
