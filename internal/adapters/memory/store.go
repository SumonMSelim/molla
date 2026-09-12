// Package memory implements every internal/platform port in memory. It is
// the default in every test and the reference behaviour real adapters match.
package memory

import (
	"context"
	"sync"
	"time"

	"github.com/SumonMSelim/molla/internal/platform"
)

type idempotencyRecord struct {
	requestHash string
	shortCode   string
	expiresAt   time.Time
}

// LinkStore is a transactional in-memory implementation of platform.LinkStore.
type LinkStore struct {
	mu          sync.RWMutex
	links       map[string]platform.Link
	idempotency map[string]idempotencyRecord
}

func NewLinkStore() *LinkStore {
	return &LinkStore{
		links:       make(map[string]platform.Link),
		idempotency: make(map[string]idempotencyRecord),
	}
}

func (s *LinkStore) Create(_ context.Context, link platform.Link, idem *platform.Idempotency) (platform.Link, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if idem != nil {
		key := link.OwnerID + "\x00" + idem.Key
		if existing, ok := s.idempotency[key]; ok {
			if existing.requestHash != idem.RequestHash {
				return platform.Link{}, platform.ErrIdempotencyConflict
			}
			return cloneLink(s.links[existing.shortCode]), nil
		}
	}
	if _, exists := s.links[link.ShortCode]; exists {
		return platform.Link{}, platform.ErrCollision
	}

	link.Version = 1
	link.IsActive = true
	link.DeletedAt = nil
	link.DeletedBy = ""
	link.DeleteRole = ""
	link.DeleteReason = ""
	link.PurgeAt = link.ExpiresAt
	s.links[link.ShortCode] = cloneLink(link)
	if idem != nil {
		s.idempotency[link.OwnerID+"\x00"+idem.Key] = idempotencyRecord{
			requestHash: idem.RequestHash,
			shortCode:   link.ShortCode,
			expiresAt:   idem.ExpiresAt,
		}
	}
	return cloneLink(link), nil
}

func (s *LinkStore) Get(_ context.Context, code string) (platform.Link, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	link, ok := s.links[code]
	if !ok {
		return platform.Link{}, platform.ErrNotFound
	}
	return cloneLink(link), nil
}

func (s *LinkStore) SoftDelete(_ context.Context, principal platform.Principal, code string, now time.Time, reason string) (platform.Deletion, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	link, ok := s.links[code]
	if !ok {
		return platform.Deletion{}, platform.ErrNotFound
	}
	if !authorized(principal, link, reason) {
		return platform.Deletion{}, platform.ErrForbidden
	}
	if !link.IsActive {
		if link.DeletedBy != principal.ActorID || link.DeleteRole != principal.Role {
			return platform.Deletion{}, platform.ErrForbidden
		}
		return deletionFrom(link), nil
	}

	deletedAt := now
	link.IsActive = false
	link.Version++
	link.DeletedAt = &deletedAt
	link.PurgeAt = deletedAt.Add(platform.DeleteRetention)
	link.DeletedBy = principal.ActorID
	link.DeleteRole = principal.Role
	link.DeleteReason = reason
	s.links[code] = cloneLink(link)
	return deletionFrom(link), nil
}

func authorized(principal platform.Principal, link platform.Link, reason string) bool {
	if principal.ActorID == "" {
		return false
	}
	switch principal.Role {
	case platform.RoleDeveloper:
		return principal.OwnerID != "" && principal.OwnerID == link.OwnerID
	case platform.RoleOperator:
		return reason != ""
	default:
		return false
	}
}

func deletionFrom(link platform.Link) platform.Deletion {
	return platform.Deletion{
		ShortCode: link.ShortCode,
		OwnerID:   link.OwnerID,
		Version:   link.Version,
		DeletedAt: *link.DeletedAt,
		PurgeAt:   link.PurgeAt,
	}
}

func cloneLink(link platform.Link) platform.Link {
	if link.DeletedAt != nil {
		deletedAt := *link.DeletedAt
		link.DeletedAt = &deletedAt
	}
	return link
}

var _ platform.LinkStore = (*LinkStore)(nil)
