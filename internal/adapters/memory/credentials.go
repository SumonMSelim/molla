package memory

import (
	"context"
	"sync"
	"time"

	"github.com/SumonMSelim/molla/internal/platform"
)

type storedCredential struct {
	actorID   string
	ownerID   string
	status    platform.CredentialStatus
	expiresAt time.Time
}

// CredentialStore is an in-memory platform.CredentialStore. It retains token
// hashes only.
type CredentialStore struct {
	mu     sync.RWMutex
	byHash map[string]storedCredential
}

func NewCredentialStore() *CredentialStore {
	return &CredentialStore{byHash: make(map[string]storedCredential)}
}

func (s *CredentialStore) Store(_ context.Context, token string, cred platform.Credential) error {
	if token == "" || cred.ActorID == "" || cred.OwnerID == "" {
		return platform.ErrInvalidCredential
	}
	if cred.IssuedAt.IsZero() || cred.ExpiresAt.IsZero() {
		return platform.ErrInvalidCredential
	}
	if !cred.ExpiresAt.After(cred.IssuedAt) {
		return platform.ErrInvalidCredential
	}
	if cred.ExpiresAt.After(cred.IssuedAt.Add(platform.MaximumCredentialLifetime)) {
		return platform.ErrInvalidCredential
	}
	status := cred.Status
	if status == "" {
		status = platform.CredentialActive
	}
	if status != platform.CredentialActive && status != platform.CredentialRevoked {
		return platform.ErrInvalidCredential
	}

	s.mu.Lock()
	s.byHash[platform.HashToken(token)] = storedCredential{
		actorID:   cred.ActorID,
		ownerID:   cred.OwnerID,
		status:    status,
		expiresAt: cred.ExpiresAt,
	}
	s.mu.Unlock()
	return nil
}

func (s *CredentialStore) Resolve(_ context.Context, token string, now time.Time) (platform.Principal, error) {
	if token == "" {
		return platform.Principal{}, platform.ErrUnauthorized
	}

	s.mu.RLock()
	cred, ok := s.byHash[platform.HashToken(token)]
	s.mu.RUnlock()
	if !ok || cred.status != platform.CredentialActive || cred.expiresAt.IsZero() || !now.Before(cred.expiresAt) {
		return platform.Principal{}, platform.ErrUnauthorized
	}
	return platform.Principal{
		ActorID: cred.actorID,
		Role:    platform.RoleDeveloper,
		OwnerID: cred.ownerID,
	}, nil
}

var _ platform.CredentialStore = (*CredentialStore)(nil)
