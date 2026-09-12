package memory

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/SumonMSelim/molla/internal/platform"
)

func TestLinkStoreConcurrentIdempotentCreate(t *testing.T) {
	store := NewLinkStore()
	now := time.Unix(1_700_000_000, 0).UTC()
	link := platform.Link{
		ShortCode: "abc1234", LongURL: "https://example.com", OwnerID: "owner",
		CreatedAt: now, ExpiresAt: now.Add(time.Hour),
	}
	idem := &platform.Idempotency{Key: "retry", RequestHash: "hash", ExpiresAt: now.Add(24 * time.Hour)}

	start := make(chan struct{})
	results := make(chan platform.Link, 2)
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			got, err := store.Create(context.Background(), link, idem)
			results <- got
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
	}
	for got := range results {
		if got.ShortCode != link.ShortCode || got.Version != 1 {
			t.Fatalf("Create() = %+v", got)
		}
	}
	if len(store.links) != 1 || len(store.idempotency) != 1 {
		t.Fatalf("stored links/idempotency = %d/%d, want 1/1", len(store.links), len(store.idempotency))
	}

	conflict := *idem
	conflict.RequestHash = "different"
	if _, err := store.Create(context.Background(), link, &conflict); !errors.Is(err, platform.ErrIdempotencyConflict) {
		t.Fatalf("conflicting Create() error = %v", err)
	}
}

func TestLinkStoreCollisionDoesNotPersistIdempotency(t *testing.T) {
	store := NewLinkStore()
	link := platform.Link{ShortCode: "taken", OwnerID: "first"}
	if _, err := store.Create(context.Background(), link, nil); err != nil {
		t.Fatal(err)
	}
	_, err := store.Create(context.Background(), platform.Link{ShortCode: "taken", OwnerID: "second"},
		&platform.Idempotency{Key: "key", RequestHash: "hash"})
	if !errors.Is(err, platform.ErrCollision) {
		t.Fatalf("Create() error = %v", err)
	}
	if len(store.idempotency) != 0 {
		t.Fatalf("idempotency records = %d, want 0", len(store.idempotency))
	}
}

func TestLinkStoreGetAndSoftDelete(t *testing.T) {
	store := NewLinkStore()
	ctx := context.Background()
	now := time.Unix(1_700_000_000, 123).UTC()
	link := platform.Link{ShortCode: "code", OwnerID: "owner", CreatedAt: now, ExpiresAt: now.Add(time.Hour)}
	if _, err := store.Create(ctx, link, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(ctx, "missing"); !errors.Is(err, platform.ErrNotFound) {
		t.Fatalf("Get() error = %v", err)
	}
	other := platform.Principal{ActorID: "other", Role: platform.RoleDeveloper, OwnerID: "other"}
	if _, err := store.SoftDelete(ctx, other, "code", now, ""); !errors.Is(err, platform.ErrForbidden) {
		t.Fatalf("other-owner SoftDelete() error = %v", err)
	}

	owner := platform.Principal{ActorID: "developer-key", Role: platform.RoleDeveloper, OwnerID: "owner"}
	deletion, err := store.SoftDelete(ctx, owner, "code", now, "")
	if err != nil {
		t.Fatal(err)
	}
	if deletion.Version != 2 || !deletion.PurgeAt.Equal(now.Add(2_592_000*time.Second)) {
		t.Fatalf("SoftDelete() = %+v", deletion)
	}
	retryAt := now.Add(time.Minute)
	retry, err := store.SoftDelete(ctx, owner, "code", retryAt, "")
	if err != nil {
		t.Fatal(err)
	}
	if retry != deletion {
		t.Fatalf("retry = %+v, want %+v", retry, deletion)
	}

	got, err := store.Get(ctx, "code")
	if err != nil {
		t.Fatal(err)
	}
	if got.IsActive || got.Version != 2 || got.DeletedAt == nil || !got.DeletedAt.Equal(now) {
		t.Fatalf("deleted link = %+v", got)
	}
	*got.DeletedAt = time.Time{}
	again, _ := store.Get(ctx, "code")
	if !again.DeletedAt.Equal(now) {
		t.Fatal("caller mutated stored link")
	}
}

func TestLinkStoreOperatorRules(t *testing.T) {
	store := NewLinkStore()
	ctx := context.Background()
	now := time.Unix(1_700_000_000, 0).UTC()
	_, _ = store.Create(ctx, platform.Link{ShortCode: "code", OwnerID: "owner"}, nil)
	operator := platform.Principal{ActorID: "arn:operator", Role: platform.RoleOperator}
	if _, err := store.SoftDelete(ctx, operator, "code", now, ""); !errors.Is(err, platform.ErrForbidden) {
		t.Fatalf("empty-reason SoftDelete() error = %v", err)
	}
	if _, err := store.SoftDelete(ctx, operator, "code", now, "abuse"); err != nil {
		t.Fatal(err)
	}
	otherOperator := platform.Principal{ActorID: "arn:other", Role: platform.RoleOperator}
	if _, err := store.SoftDelete(ctx, otherOperator, "code", now, "abuse"); !errors.Is(err, platform.ErrForbidden) {
		t.Fatalf("other-operator retry error = %v", err)
	}
}
