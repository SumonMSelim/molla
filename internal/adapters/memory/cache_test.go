package memory

import (
	"context"
	"testing"
	"time"

	"github.com/SumonMSelim/molla/internal/platform"
)

func TestCacheOutcomesAndDefensiveCopies(t *testing.T) {
	cache := NewCache()
	ctx := context.Background()
	base := time.Unix(1_700_000_000, 0).UTC()
	record := platform.CacheRecord{
		ShortCode: "code", State: platform.CacheActive, LongURL: "https://example.com",
		ExpiresAt: base.Add(3 * time.Hour), FreshUntil: base.Add(time.Hour),
		StaleUntil: base.Add(2 * time.Hour), Version: 1,
	}
	if err := cache.Put(ctx, record); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		now  time.Time
		want platform.CacheOutcome
	}{
		{"fresh", base, platform.CacheFresh},
		{"stale at fresh boundary", base.Add(time.Hour), platform.CacheStale},
		{"expired at stale boundary", base.Add(2 * time.Hour), platform.CacheExpired},
		{"expired at link expiry", base.Add(3 * time.Hour), platform.CacheExpired},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, outcome, err := cache.Get(ctx, "code", test.now)
			if err != nil || outcome != test.want {
				t.Fatalf("Get() outcome/error = %q/%v, want %q/nil", outcome, err, test.want)
			}
			if outcome == platform.CacheExpired && got.LongURL != "" {
				t.Fatalf("expired Get() returned destination %q", got.LongURL)
			}
		})
	}
	if _, outcome, _ := cache.Get(ctx, "missing", base); outcome != platform.CacheMiss {
		t.Fatalf("missing outcome = %q", outcome)
	}
}

func TestCacheVersionCompareAndSetProtectsTombstone(t *testing.T) {
	cache := NewCache()
	ctx := context.Background()
	now := time.Unix(1_700_000_000, 0).UTC()
	deletedAt := now
	tombstone := platform.CacheRecord{
		ShortCode: "code", State: platform.CacheDeleted, LongURL: "must disappear",
		Version: 2, DeletedAt: &deletedAt,
	}
	if err := cache.Put(ctx, tombstone); err != nil {
		t.Fatal(err)
	}
	older := platform.CacheRecord{
		ShortCode: "code", State: platform.CacheActive, LongURL: "https://old.example",
		Version: 1, ExpiresAt: now.Add(time.Hour), FreshUntil: now.Add(time.Hour), StaleUntil: now.Add(time.Hour),
	}
	if err := cache.Put(ctx, older); err != nil {
		t.Fatal(err)
	}
	got, outcome, err := cache.Get(ctx, "code", now)
	if err != nil || outcome != platform.CacheHitDeleted || got.LongURL != "" || got.Version != 2 {
		t.Fatalf("Get() = %+v, %q, %v", got, outcome, err)
	}
	*got.DeletedAt = time.Time{}
	again, _, _ := cache.Get(ctx, "code", now)
	if !again.DeletedAt.Equal(now) {
		t.Fatal("caller mutated cached tombstone")
	}
}

func TestInvalidatorEventsAndAuditAreInspectable(t *testing.T) {
	ctx := context.Background()
	now := time.Unix(1_700_000_000, 0).UTC()
	cache := NewCache()
	invalidator := NewCacheInvalidator(cache)
	deletion := platform.Deletion{ShortCode: "code", OwnerID: "owner", Version: 2, DeletedAt: now}
	if err := invalidator.Invalidate(ctx, deletion); err != nil {
		t.Fatal(err)
	}
	if got := invalidator.Tombstones(); len(got) != 1 || got[0] != deletion {
		t.Fatalf("Tombstones() = %+v", got)
	}
	if _, outcome, err := cache.Get(ctx, deletion.ShortCode, now.Add(platform.DeleteTombstoneTTL)); err != nil || outcome != platform.CacheMiss {
		t.Fatalf("expired tombstone outcome/error = %q/%v, want %q/nil", outcome, err, platform.CacheMiss)
	}

	publisher := &EventPublisher{}
	click := platform.ClickEvent{EventID: "event", ShortCode: "code", OccurredAt: now, SourceIPHash: "hash"}
	if err := publisher.Publish(ctx, click); err != nil {
		t.Fatal(err)
	}
	events := publisher.Events()
	if len(events) != 1 || events[0] != click {
		t.Fatalf("Events() = %+v", events)
	}
	events[0].EventID = "changed"
	if publisher.Events()[0].EventID != click.EventID {
		t.Fatal("caller mutated published events")
	}

	audits := &AuditSink{}
	audit := platform.AuditEvent{
		ActorID: "actor", Role: platform.RoleDeveloper, OwnerID: "owner",
		ShortCode: "code", Outcome: "deleted", Timestamp: now,
	}
	if err := audits.Record(ctx, audit); err != nil {
		t.Fatal(err)
	}
	gotAudits := audits.Events()
	if len(gotAudits) != 1 || gotAudits[0] != audit {
		t.Fatalf("audit Events() = %+v", gotAudits)
	}
	gotAudits[0].Outcome = "changed"
	if audits.Events()[0].Outcome != audit.Outcome {
		t.Fatal("caller mutated audit events")
	}
}
