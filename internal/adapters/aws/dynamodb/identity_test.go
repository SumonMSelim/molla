package dynamodb

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/SumonMSelim/molla/internal/platform"
)

func TestIdentityStoreHashesBeforeLookup(t *testing.T) {
	client, fake := testClient(t)
	store := NewIdentityStore(client)
	ctx := context.Background()
	issued := time.Unix(1_700_000_000, 0).UTC()
	token := "raw-secret-token"
	err := store.Store(ctx, token, platform.Credential{
		ActorID: "actor", OwnerID: "owner", Status: platform.CredentialActive,
		IssuedAt: issued, ExpiresAt: issued.Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	hash := platform.HashToken(token)
	if strings.Contains(fake.LastRequest(), token) {
		t.Fatal("raw token present in PutItem")
	}
	if !strings.Contains(fake.LastRequest(), hash) {
		t.Fatalf("hash missing from PutItem: %s", fake.LastRequest())
	}
	principal, err := store.Resolve(ctx, token, issued.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if principal.ActorID != "actor" || principal.OwnerID != "owner" || principal.Role != platform.RoleDeveloper {
		t.Fatalf("principal = %+v", principal)
	}
	if strings.Contains(fake.LastRequest(), token) {
		t.Fatal("raw token present in GetItem")
	}
	if !strings.Contains(fake.LastRequest(), `"ConsistentRead":true`) {
		t.Fatalf("Resolve GetItem not consistent: %s", fake.LastRequest())
	}
}

func TestIdentityStoreRejectsInvalidExpiry(t *testing.T) {
	client, _ := testClient(t)
	store := NewIdentityStore(client)
	ctx := context.Background()
	issued := time.Unix(1_700_000_000, 0).UTC()
	tooLong := platform.Credential{ActorID: "a", OwnerID: "o", IssuedAt: issued, ExpiresAt: issued.Add(platform.MaximumCredentialLifetime + time.Second)}
	if err := store.Store(ctx, "t", tooLong); !errors.Is(err, platform.ErrInvalidCredential) {
		t.Fatalf("too long = %v", err)
	}
	missing := platform.Credential{ActorID: "a", OwnerID: "o", IssuedAt: issued}
	if err := store.Store(ctx, "t", missing); !errors.Is(err, platform.ErrInvalidCredential) {
		t.Fatalf("missing expiry = %v", err)
	}
	ok := platform.Credential{ActorID: "a", OwnerID: "o", IssuedAt: issued, ExpiresAt: issued.Add(platform.MaximumCredentialLifetime)}
	if err := store.Store(ctx, "t", ok); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Resolve(ctx, "t", issued.Add(platform.MaximumCredentialLifetime)); !errors.Is(err, platform.ErrUnauthorized) {
		t.Fatalf("elapsed = %v", err)
	}
	if _, err := store.Resolve(ctx, "unknown", issued); !errors.Is(err, platform.ErrUnauthorized) {
		t.Fatalf("unknown = %v", err)
	}
	revoked := platform.Credential{ActorID: "a", OwnerID: "o", Status: platform.CredentialRevoked, IssuedAt: issued, ExpiresAt: issued.Add(time.Hour)}
	if err := store.Store(ctx, "revoked", revoked); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Resolve(ctx, "revoked", issued); !errors.Is(err, platform.ErrUnauthorized) {
		t.Fatalf("revoked = %v", err)
	}
}
