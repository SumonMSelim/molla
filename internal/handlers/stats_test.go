package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/SumonMSelim/molla/internal/adapters/memory"
	"github.com/SumonMSelim/molla/internal/core"
	"github.com/SumonMSelim/molla/internal/platform"
)

func TestStatsClicksAndZeroDefault(t *testing.T) {
	h := newHarness(t, "")
	rec := postCreate(h.handler, "", `{"long_url":"https://example.com","alias":"statzzz"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create = %d %s", rec.Code, rec.Body.String())
	}
	zero := getStats(h.handler, "statzzz")
	if zero.Code != http.StatusOK {
		t.Fatalf("status = %d %s", zero.Code, zero.Body.String())
	}
	got := decodeStats(t, zero)
	if got.ShortCode != "statzzz" || got.Clicks != 0 || got.CreatedAt != "2026-09-12T00:00:00Z" || got.LastClickAt != "" {
		t.Fatalf("zero stats = %+v", got)
	}

	last := h.now.Add(2 * time.Hour)
	if err := h.stats.Increment(context.Background(), "statzzz", 10, last); err != nil {
		t.Fatal(err)
	}
	with := getStats(h.handler, "statzzz")
	if with.Code != http.StatusOK {
		t.Fatalf("status = %d %s", with.Code, with.Body.String())
	}
	got = decodeStats(t, with)
	if got.Clicks != 10 || got.LastClickAt != last.UTC().Format(time.RFC3339) {
		t.Fatalf("stats = %+v", got)
	}
}

func TestStatsInactiveAndExpiredAndUnknown(t *testing.T) {
	h := newHarness(t, "")
	if rec := postCreate(h.handler, "", `{"long_url":"https://example.com","alias":"deletdx"}`); rec.Code != http.StatusCreated {
		t.Fatalf("create = %d", rec.Code)
	}
	if rec := postCreate(h.handler, "", `{"long_url":"https://example.com","alias":"expcode","expires_in":60}`); rec.Code != http.StatusCreated {
		t.Fatalf("create expiry = %d", rec.Code)
	}
	deleter := Deleter{
		Store:       h.store,
		Invalidator: memory.NewCacheInvalidator(h.cache),
		Audit:       &memory.AuditSink{},
		Clock:       h.clock,
	}
	operator := platform.Principal{ActorID: "arn:aws:iam::1:user/ops", Role: platform.RoleOperator}
	if err := deleter.Delete(context.Background(), operator, "deletdx", "malware"); err != nil {
		t.Fatal(err)
	}
	deleted := getStats(h.handler, "deletdx")
	if deleted.Code != http.StatusNotFound || decodeError(t, deleted) != "NOT_FOUND" {
		t.Fatalf("deleted link stats = %d %s", deleted.Code, deleted.Body.String())
	}

	h.clock.Advance(61 * time.Second)
	expired := getStats(h.handler, "expcode")
	if expired.Code != http.StatusOK || decodeStats(t, expired).ShortCode != "expcode" {
		t.Fatalf("expired stats = %d %s", expired.Code, expired.Body.String())
	}
	h.clock.Set(h.now)

	missing := getStats(h.handler, "noexist")
	if missing.Code != http.StatusNotFound || decodeError(t, missing) != "NOT_FOUND" {
		t.Fatalf("unknown = %d %s", missing.Code, missing.Body.String())
	}
	invalid := getStats(h.handler, "ab")
	if invalid.Code != http.StatusNotFound {
		t.Fatalf("invalid code = %d", invalid.Code)
	}
}

func TestStatsStoreAndLinkErrors(t *testing.T) {
	now := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
	permuter, err := core.NewPermuter([]byte(testPermuteKey))
	if err != nil {
		t.Fatal(err)
	}
	t.Run("link dependency", func(t *testing.T) {
		h := New(Deps{
			Store:     failLinkStore{err: platform.ErrDependency},
			Allocator: &leaseAllocator{},
			Clock:     memory.NewClock(now),
			Permuter:  permuter,
			Stats:     memory.NewStatsStore(),
		})
		rec := getStats(h, "statzzz")
		if rec.Code != http.StatusServiceUnavailable || decodeError(t, rec) != "TEMPORARILY_UNAVAILABLE" {
			t.Fatalf("status/body = %d %s", rec.Code, rec.Body.String())
		}
	})
	t.Run("stats dependency", func(t *testing.T) {
		store := memory.NewLinkStore()
		link := platform.Link{
			ShortCode: "statzzz", LongURL: "https://example.com", IsActive: true,
			CreatedAt: now, ExpiresAt: now.Add(time.Hour),
		}
		if _, err := store.Create(context.Background(), link, nil); err != nil {
			t.Fatal(err)
		}
		h := New(Deps{
			Store:     store,
			Allocator: &leaseAllocator{},
			Clock:     memory.NewClock(now),
			Permuter:  permuter,
			Stats:     failStats{err: platform.ErrDependency},
		})
		rec := getStats(h, "statzzz")
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d %s", rec.Code, rec.Body.String())
		}
	})
	t.Run("invalid code", func(t *testing.T) {
		api := &API{}
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/links/ab/stats", nil)
		req.SetPathValue("short_code", "ab")
		api.stats(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d", rec.Code)
		}
	})
}

type failLinkStore struct{ err error }

func (s failLinkStore) Create(context.Context, platform.Link, *platform.Idempotency) (platform.Link, error) {
	return platform.Link{}, s.err
}
func (s failLinkStore) Get(context.Context, string) (platform.Link, error) {
	return platform.Link{}, s.err
}
func (s failLinkStore) SoftDelete(context.Context, platform.Principal, string, time.Time, string) (platform.Deletion, error) {
	return platform.Deletion{}, s.err
}

type failStats struct{ err error }

func (s failStats) Get(context.Context, string) (platform.Stats, error) {
	return platform.Stats{}, s.err
}
func (s failStats) Increment(context.Context, string, int64, time.Time) error {
	return s.err
}
