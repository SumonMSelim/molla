package dynamodb

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

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

func TestIdentityStoreRevoke(t *testing.T) {
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
	if err := store.Revoke(ctx, hash); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(fake.LastRequest(), token) {
		t.Fatal("raw token present in UpdateItem")
	}
	if !strings.Contains(fake.LastRequest(), "attribute_exists") {
		t.Fatalf("UpdateItem not conditioned on existence: %s", fake.LastRequest())
	}
	item, ok := fake.Item(tableCredentials, hash)
	if !ok {
		t.Fatal("credential missing")
	}
	if got, _ := item[attrStatus]["S"].(string); got != string(platform.CredentialRevoked) {
		t.Fatalf("status = %+v", item)
	}
	if _, err := store.Resolve(ctx, token, issued.Add(time.Minute)); !errors.Is(err, platform.ErrUnauthorized) {
		t.Fatalf("revoked Resolve() = %v", err)
	}
	if err := store.Revoke(ctx, ""); !errors.Is(err, platform.ErrNotFound) {
		t.Fatalf("empty hash = %v", err)
	}
}

// stubClient returns a canned error from UpdateItem. ddbfake cannot evaluate
// attribute_exists conditions, so the not-found mapping is exercised here.
type stubClient struct {
	clientAPI
	updateErr error
}

func (c stubClient) UpdateItem(context.Context, *dynamodb.UpdateItemInput, ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
	return nil, c.updateErr
}

func TestIdentityStoreRevokeMapsErrors(t *testing.T) {
	ctx := context.Background()
	missing := NewIdentityStore(stubClient{updateErr: &types.ConditionalCheckFailedException{}})
	if err := missing.Revoke(ctx, "deadbeef"); !errors.Is(err, platform.ErrNotFound) {
		t.Fatalf("conditional failure = %v, want ErrNotFound", err)
	}
	down := NewIdentityStore(stubClient{updateErr: errors.New("throttled")})
	if err := down.Revoke(ctx, "deadbeef"); !errors.Is(err, platform.ErrDependency) {
		t.Fatalf("transport failure = %v, want ErrDependency", err)
	}
}
