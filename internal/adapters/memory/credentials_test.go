package memory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/SumonMSelim/molla/internal/platform"
)

func TestCredentialStoreHashesTokenAndResolvesPrincipal(t *testing.T) {
	store := NewCredentialStore()
	ctx := context.Background()
	issued := time.Unix(1_700_000_000, 0).UTC()
	expires := issued.Add(24 * time.Hour)
	token := "dev-test-token"
	cred := platform.Credential{
		ActorID: "actor", OwnerID: "owner", Status: platform.CredentialActive,
		IssuedAt: issued, ExpiresAt: expires,
	}
	if err := store.Store(ctx, token, cred); err != nil {
		t.Fatal(err)
	}

	sum := sha256.Sum256([]byte(token))
	wantHash := hex.EncodeToString(sum[:])
	if len(store.byHash) != 1 {
		t.Fatalf("stored records = %d, want 1", len(store.byHash))
	}
	if _, ok := store.byHash[token]; ok {
		t.Fatal("raw token retained")
	}
	if _, ok := store.byHash[wantHash]; !ok {
		t.Fatalf("missing hash %s", wantHash)
	}

	got, err := store.Resolve(ctx, token, issued)
	if err != nil {
		t.Fatal(err)
	}
	want := platform.Principal{ActorID: "actor", Role: platform.RoleDeveloper, OwnerID: "owner"}
	if got != want {
		t.Fatalf("Resolve() = %+v, want %+v", got, want)
	}
	if _, err := store.Resolve(ctx, token, expires); !errors.Is(err, platform.ErrUnauthorized) {
		t.Fatalf("expired Resolve() error = %v", err)
	}
}

func TestCredentialStoreRejectsInvalidAndRevoked(t *testing.T) {
	store := NewCredentialStore()
	ctx := context.Background()
	issued := time.Unix(1_700_000_000, 0).UTC()
	valid := platform.Credential{
		ActorID: "actor", OwnerID: "owner", IssuedAt: issued, ExpiresAt: issued.Add(time.Hour),
	}

	tooLong := valid
	tooLong.ExpiresAt = issued.Add(platform.MaximumCredentialLifetime + time.Second)
	if err := store.Store(ctx, "token", tooLong); !errors.Is(err, platform.ErrInvalidCredential) {
		t.Fatalf("overlong Store() error = %v", err)
	}
	boundary := valid
	boundary.ExpiresAt = issued.Add(platform.MaximumCredentialLifetime)
	if err := store.Store(ctx, "token", boundary); err != nil {
		t.Fatal(err)
	}

	missingExpiry := valid
	missingExpiry.ExpiresAt = time.Time{}
	if err := store.Store(ctx, "other", missingExpiry); !errors.Is(err, platform.ErrInvalidCredential) {
		t.Fatalf("missing expiry Store() error = %v", err)
	}
	if _, err := store.Resolve(ctx, "unknown", issued); !errors.Is(err, platform.ErrUnauthorized) {
		t.Fatalf("unknown Resolve() error = %v", err)
	}

	revoked := valid
	revoked.Status = platform.CredentialRevoked
	if err := store.Store(ctx, "revoked-token", revoked); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Resolve(ctx, "revoked-token", issued); !errors.Is(err, platform.ErrUnauthorized) {
		t.Fatalf("revoked Resolve() error = %v", err)
	}
}

func TestCredentialStoreRevoke(t *testing.T) {
	store := NewCredentialStore()
	ctx := context.Background()
	issued := time.Unix(1_700_000_000, 0).UTC()
	token := "dev-test-token"
	cred := platform.Credential{
		ActorID: "actor", OwnerID: "owner", Status: platform.CredentialActive,
		IssuedAt: issued, ExpiresAt: issued.Add(24 * time.Hour),
	}
	if err := store.Store(ctx, token, cred); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Resolve(ctx, token, issued); err != nil {
		t.Fatal(err)
	}
	if err := store.Revoke(ctx, platform.HashToken(token)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Resolve(ctx, token, issued); !errors.Is(err, platform.ErrUnauthorized) {
		t.Fatalf("revoked Resolve() error = %v", err)
	}
	if err := store.Revoke(ctx, platform.HashToken("never-issued")); !errors.Is(err, platform.ErrNotFound) {
		t.Fatalf("unknown Revoke() error = %v", err)
	}
}
