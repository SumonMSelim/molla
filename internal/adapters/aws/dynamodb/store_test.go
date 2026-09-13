package dynamodb

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/SumonMSelim/molla/internal/platform"
)

func TestLinkStoreCreateGetAndCollision(t *testing.T) {
	client, fake := testClient(t)
	store := NewLinkStore(client)
	ctx := context.Background()
	now := time.Unix(1_700_000_000, 0).UTC()
	link := platform.Link{
		ShortCode: "abc1234", LongURL: "https://example.com", OwnerID: "owner",
		CreatedAt: now, ExpiresAt: now.Add(time.Hour),
	}
	got, err := store.Create(ctx, link, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != 1 || !got.IsActive {
		t.Fatalf("created = %+v", got)
	}
	_, err = store.Create(ctx, link, &platform.Idempotency{Key: "k", RequestHash: "h", ExpiresAt: now.Add(time.Hour)})
	if !errors.Is(err, platform.ErrCollision) {
		t.Fatalf("collision error = %v", err)
	}
	if _, ok := fake.Item("Idempotency", "owner#k"); ok {
		t.Fatal("idempotency persisted after collision")
	}
	loaded, err := store.Get(ctx, "abc1234")
	if err != nil || loaded.LongURL != link.LongURL {
		t.Fatalf("Get = %+v err=%v", loaded, err)
	}
	if _, err := store.Get(ctx, "missing"); !errors.Is(err, platform.ErrNotFound) {
		t.Fatalf("missing Get error = %v", err)
	}
	if !strings.Contains(fake.LastRequest(), `"ConsistentRead":true`) {
		t.Fatalf("GetItem missing ConsistentRead: %s", fake.LastRequest())
	}
}

func TestLinkStoreIdempotentReplayAndConflict(t *testing.T) {
	client, _ := testClient(t)
	store := NewLinkStore(client)
	ctx := context.Background()
	now := time.Unix(1_700_000_000, 0).UTC()
	link := platform.Link{
		ShortCode: "idem001", LongURL: "https://example.com", OwnerID: "owner",
		CreatedAt: now, ExpiresAt: now.Add(time.Hour),
	}
	idem := &platform.Idempotency{Key: "retry", RequestHash: "hash-a", ExpiresAt: now.Add(24 * time.Hour)}
	first, err := store.Create(ctx, link, idem)
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.Create(ctx, link, idem)
	if err != nil {
		t.Fatal(err)
	}
	if first.ShortCode != second.ShortCode {
		t.Fatalf("replay codes %q vs %q", first.ShortCode, second.ShortCode)
	}
	conflict := *idem
	conflict.RequestHash = "hash-b"
	if _, err := store.Create(ctx, link, &conflict); !errors.Is(err, platform.ErrIdempotencyConflict) {
		t.Fatalf("conflict error = %v", err)
	}
}

func TestLinkStoreGeneratedCodeCollisionLeavesAllocatorFree(t *testing.T) {
	client, fake := testClient(t)
	store := NewLinkStore(client)
	alloc := NewIDAllocator(client, "us-east-1", 2)
	ctx := context.Background()
	now := time.Unix(1_700_000_000, 0).UTC()
	if _, err := store.Create(ctx, platform.Link{ShortCode: "taken01", OwnerID: "a", CreatedAt: now, ExpiresAt: now.Add(time.Hour)}, nil); err != nil {
		t.Fatal(err)
	}
	id1, err := alloc.Lease(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.Create(ctx, platform.Link{ShortCode: "taken01", OwnerID: "b", CreatedAt: now, ExpiresAt: now.Add(time.Hour)}, nil)
	if !errors.Is(err, platform.ErrCollision) {
		t.Fatalf("error = %v", err)
	}
	before := fake.RequestCount()
	id2, err := alloc.Lease(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if id1 == id2 {
		t.Fatal("allocator reused discarded id")
	}
	if fake.RequestCount() != before {
		t.Fatalf("second lease should use in-memory block, requests %d -> %d", before, fake.RequestCount())
	}
}

func TestLinkStoreSoftDelete(t *testing.T) {
	client, _ := testClient(t)
	store := NewLinkStore(client)
	ctx := context.Background()
	now := time.Unix(1_700_000_000, 0).UTC()
	_, err := store.Create(ctx, platform.Link{ShortCode: "delcode", OwnerID: "owner", CreatedAt: now, ExpiresAt: now.Add(time.Hour)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	other := platform.Principal{ActorID: "x", Role: platform.RoleDeveloper, OwnerID: "other"}
	if _, err := store.SoftDelete(ctx, other, "delcode", now, ""); !errors.Is(err, platform.ErrForbidden) {
		t.Fatalf("forbidden = %v", err)
	}
	owner := platform.Principal{ActorID: "dev", Role: platform.RoleDeveloper, OwnerID: "owner"}
	del, err := store.SoftDelete(ctx, owner, "delcode", now, "")
	if err != nil {
		t.Fatal(err)
	}
	if del.Version != 2 {
		t.Fatalf("version = %d", del.Version)
	}
	retry, err := store.SoftDelete(ctx, owner, "delcode", now.Add(time.Minute), "")
	if err != nil || retry.Version != 2 {
		t.Fatalf("retry = %+v err=%v", retry, err)
	}
}
