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
	rec := postCreate(h.handler, testAPIToken, "", `{"long_url":"https://example.com","alias":"statzzz"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create = %d %s", rec.Code, rec.Body.String())
	}
	zero := getStats(h.handler, testAPIToken, "statzzz")
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
	with := getStats(h.handler, testAPIToken, "statzzz")
	if with.Code != http.StatusOK {
		t.Fatalf("status = %d %s", with.Code, with.Body.String())
	}
	got = decodeStats(t, with)
	if got.Clicks != 10 || got.LastClickAt != last.UTC().Format(time.RFC3339) {
		t.Fatalf("stats = %+v", got)
	}
}

func TestStatsInactiveExpiredUnknownAndForbidden(t *testing.T) {
	h := newHarness(t, "")
	if rec := postCreate(h.handler, testAPIToken, "", `{"long_url":"https://example.com","alias":"ownedxx"}`); rec.Code != http.StatusCreated {
		t.Fatalf("create = %d", rec.Code)
	}
	if rec := postCreate(h.handler, testAPIToken, "", `{"long_url":"https://example.com","alias":"expcode","expires_in":60}`); rec.Code != http.StatusCreated {
		t.Fatalf("create expiry = %d", rec.Code)
	}
	if rec := deleteLink(h.handler, testAPIToken, "ownedxx"); rec.Code != http.StatusNoContent {
		t.Fatalf("delete = %d %s", rec.Code, rec.Body.String())
	}
	inactive := getStats(h.handler, testAPIToken, "ownedxx")
	if inactive.Code != http.StatusOK || decodeStats(t, inactive).Clicks != 0 {
		t.Fatalf("inactive stats = %d %s", inactive.Code, inactive.Body.String())
	}

	h.clock.Advance(61 * time.Second)
	expired := getStats(h.handler, testAPIToken, "expcode")
	if expired.Code != http.StatusOK || decodeStats(t, expired).ShortCode != "expcode" {
		t.Fatalf("expired stats = %d %s", expired.Code, expired.Body.String())
	}
	h.clock.Set(h.now)

	missing := getStats(h.handler, testAPIToken, "noexist")
	if missing.Code != http.StatusNotFound || decodeError(t, missing) != "NOT_FOUND" {
		t.Fatalf("unknown = %d %s", missing.Code, missing.Body.String())
	}
	other := getStats(h.handler, testOtherToken, "expcode")
	if other.Code != http.StatusForbidden || decodeError(t, other) != "FORBIDDEN" {
		t.Fatalf("other owner = %d %s", other.Code, other.Body.String())
	}
	invalid := getStats(h.handler, testAPIToken, "ab")
	if invalid.Code != http.StatusNotFound {
		t.Fatalf("invalid code = %d", invalid.Code)
	}
	unauth := getStats(h.handler, "", "expcode")
	if unauth.Code != http.StatusUnauthorized {
		t.Fatalf("unauth = %d", unauth.Code)
	}
}

func TestStatsStoreAndLinkErrors(t *testing.T) {
	now := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
	permuter, err := core.NewPermuter([]byte(testPermuteKey))
	if err != nil {
		t.Fatal(err)
	}
	creds := newTestCredentials(t)
	t.Run("link dependency", func(t *testing.T) {
		h := New(Deps{
			Store:       failLinkStore{err: platform.ErrDependency},
			Allocator:   &leaseAllocator{},
			Clock:       memory.NewClock(now),
			Credentials: creds,
			Permuter:    permuter,
			Stats:       memory.NewStatsStore(),
			Invalidator: memory.NewCacheInvalidator(memory.NewCache()),
			Audit:       &memory.AuditSink{},
		})
		rec := getStats(h, testAPIToken, "statzzz")
		if rec.Code != http.StatusServiceUnavailable || decodeError(t, rec) != "TEMPORARILY_UNAVAILABLE" {
			t.Fatalf("status/body = %d %s", rec.Code, rec.Body.String())
		}
	})
	t.Run("stats dependency", func(t *testing.T) {
		store := memory.NewLinkStore()
		link := platform.Link{
			ShortCode: "statzzz", LongURL: "https://example.com", OwnerID: testOwner,
			CreatedAt: now, ExpiresAt: now.Add(time.Hour),
		}
		if _, err := store.Create(context.Background(), link, nil); err != nil {
			t.Fatal(err)
		}
		h := New(Deps{
			Store:       store,
			Allocator:   &leaseAllocator{},
			Clock:       memory.NewClock(now),
			Credentials: creds,
			Permuter:    permuter,
			Stats:       failStats{err: platform.ErrDependency},
			Invalidator: memory.NewCacheInvalidator(memory.NewCache()),
			Audit:       &memory.AuditSink{},
		})
		rec := getStats(h, testAPIToken, "statzzz")
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d %s", rec.Code, rec.Body.String())
		}
	})
	t.Run("missing principal", func(t *testing.T) {
		api := &API{}
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/links/statzzz/stats", nil)
		req.SetPathValue("short_code", "statzzz")
		api.stats(rec, req)
		if rec.Code != http.StatusUnauthorized {
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
