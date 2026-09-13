package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SumonMSelim/molla/internal/adapters/memory"
	"github.com/SumonMSelim/molla/internal/platform"
)

const (
	testPrivacyKey = "privacy-test-key"
	testRedirectIP = "203.0.113.10"
)

type redirectHarness struct {
	handler   http.Handler
	store     *memory.LinkStore
	gets      *countingGetStore
	cache     *memory.Cache
	publisher *memory.EventPublisher
	clock     *memory.Clock
	now       time.Time
}

func newRedirectHarness(t *testing.T) *redirectHarness {
	t.Helper()
	now := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
	store := memory.NewLinkStore()
	gets := &countingGetStore{LinkStore: store}
	cache := memory.NewCache()
	publisher := &memory.EventPublisher{}
	clock := memory.NewClock(now)
	return &redirectHarness{
		handler: NewRedirect(RedirectDeps{
			Store:      gets,
			Cache:      cache,
			Publisher:  publisher,
			Clock:      clock,
			PrivacyKey: []byte(testPrivacyKey),
		}),
		store:     store,
		gets:      gets,
		cache:     cache,
		publisher: publisher,
		clock:     clock,
		now:       now,
	}
}

func (h *redirectHarness) seed(t *testing.T, code, url string, expires time.Time) platform.Link {
	t.Helper()
	link, err := h.store.Create(context.Background(), platform.Link{
		ShortCode: code, LongURL: url, OwnerID: "owner",
		IsCustom: true, CreatedAt: h.now, ExpiresAt: expires,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return link
}

func getRedirect(h http.Handler, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.RemoteAddr = testRedirectIP + ":1234"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

type countingGetStore struct {
	platform.LinkStore
	gets atomic.Int32
}

func (s *countingGetStore) Get(ctx context.Context, code string) (platform.Link, error) {
	s.gets.Add(1)
	return s.LinkStore.Get(ctx, code)
}

type forbiddenGetStore struct{ t *testing.T }

func (s forbiddenGetStore) Create(context.Context, platform.Link, *platform.Idempotency) (platform.Link, error) {
	return platform.Link{}, platform.ErrForbidden
}
func (s forbiddenGetStore) Get(context.Context, string) (platform.Link, error) {
	s.t.Fatal("LinkStore.Get invoked")
	return platform.Link{}, platform.ErrNotFound
}
func (s forbiddenGetStore) SoftDelete(context.Context, platform.Principal, string, time.Time, string) (platform.Deletion, error) {
	return platform.Deletion{}, platform.ErrNotFound
}

type failGetStore struct{ err error }

func (s failGetStore) Create(context.Context, platform.Link, *platform.Idempotency) (platform.Link, error) {
	return platform.Link{}, s.err
}
func (s failGetStore) Get(context.Context, string) (platform.Link, error) {
	return platform.Link{}, s.err
}
func (s failGetStore) SoftDelete(context.Context, platform.Principal, string, time.Time, string) (platform.Deletion, error) {
	return platform.Deletion{}, s.err
}

type failPublisher struct {
	calls atomic.Int32
	err   error
}

func (p *failPublisher) Publish(context.Context, platform.ClickEvent) error {
	p.calls.Add(1)
	return p.err
}

type failCache struct {
	inner  *memory.Cache
	getErr error
	putErr error
}

func (c failCache) Get(ctx context.Context, code string, now time.Time) (platform.CacheRecord, platform.CacheOutcome, error) {
	if c.getErr != nil {
		return platform.CacheRecord{}, "", c.getErr
	}
	return c.inner.Get(ctx, code, now)
}
func (c failCache) Put(ctx context.Context, record platform.CacheRecord) error {
	if c.putErr != nil {
		return c.putErr
	}
	return c.inner.Put(ctx, record)
}

func TestRedirectCacheMissPopulatesAndRedirects(t *testing.T) {
	h := newRedirectHarness(t)
	h.seed(t, "my-link", "https://example.com/dest", h.now.Add(time.Hour))
	rec := getRedirect(h.handler, "/my-link")
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "https://example.com/dest" {
		t.Fatalf("status/location = %d %q", rec.Code, rec.Header().Get("Location"))
	}
	if rec.Header().Get("Cache-Control") != redirectCacheTTL {
		t.Fatalf("cache-control = %q", rec.Header().Get("Cache-Control"))
	}
	cached, outcome, err := h.cache.Get(context.Background(), "my-link", h.now)
	if err != nil || outcome != platform.CacheFresh || cached.LongURL != "https://example.com/dest" {
		t.Fatalf("cache = %+v %q %v", cached, outcome, err)
	}
	events := h.publisher.Events()
	if len(events) != 1 || events[0].ShortCode != "my-link" || events[0].EventID == "" {
		t.Fatalf("events = %+v", events)
	}
	if events[0].SourceIPHash != hashSourceIP([]byte(testPrivacyKey), testRedirectIP, h.now) {
		t.Fatalf("ip hash = %q", events[0].SourceIPHash)
	}
}

func TestRedirectFreshCacheSkipsStore(t *testing.T) {
	now := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
	cache := memory.NewCache()
	if err := cache.Put(context.Background(), platform.CacheRecord{
		ShortCode: "cached", State: platform.CacheActive, LongURL: "https://example.com/cached",
		ExpiresAt: now.Add(time.Hour), FreshUntil: now.Add(time.Minute), StaleUntil: now.Add(2 * time.Minute), Version: 1,
	}); err != nil {
		t.Fatal(err)
	}
	handler := NewRedirect(RedirectDeps{
		Store:      forbiddenGetStore{t: t},
		Cache:      cache,
		Publisher:  &memory.EventPublisher{},
		Clock:      memory.NewClock(now),
		PrivacyKey: []byte(testPrivacyKey),
	})
	rec := getRedirect(handler, "/cached")
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "https://example.com/cached" {
		t.Fatalf("status/location = %d %q", rec.Code, rec.Header().Get("Location"))
	}
}

func TestRedirectUnknownIsNotCached(t *testing.T) {
	h := newRedirectHarness(t)
	rec := getRedirect(h.handler, "/missing")
	if rec.Code != http.StatusNotFound || rec.Header().Get("Cache-Control") != cacheNoStore {
		t.Fatalf("status/cc = %d %q", rec.Code, rec.Header().Get("Cache-Control"))
	}
	if _, outcome, _ := h.cache.Get(context.Background(), "missing", h.now); outcome != platform.CacheMiss {
		t.Fatalf("outcome = %q", outcome)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestRedirectDoesNotNegativeCacheThenSeesCreate(t *testing.T) {
	h := newRedirectHarness(t)
	if rec := getRedirect(h.handler, "/later"); rec.Code != http.StatusNotFound {
		t.Fatalf("first status = %d", rec.Code)
	}
	h.seed(t, "later", "https://example.com/later", h.now.Add(time.Hour))
	rec := getRedirect(h.handler, "/later")
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "https://example.com/later" {
		t.Fatalf("status/location = %d %q", rec.Code, rec.Header().Get("Location"))
	}
	if h.gets.gets.Load() < 2 {
		t.Fatalf("store gets = %d, want at least 2", h.gets.gets.Load())
	}
}

func TestRedirectExpiredCacheDoesNotServeDestination(t *testing.T) {
	h := newRedirectHarness(t)
	h.seed(t, "stale-exp", "https://example.com/hidden", h.now)
	if err := h.cache.Put(context.Background(), platform.CacheRecord{
		ShortCode: "stale-exp", State: platform.CacheActive, LongURL: "https://example.com/hidden",
		ExpiresAt: h.now, FreshUntil: h.now.Add(-time.Minute), StaleUntil: h.now.Add(-time.Second), Version: 1,
	}); err != nil {
		t.Fatal(err)
	}
	rec := getRedirect(h.handler, "/stale-exp")
	if rec.Code != http.StatusNotFound || rec.Header().Get("Cache-Control") != cacheNoStore {
		t.Fatalf("status/cc = %d %q", rec.Code, rec.Header().Get("Cache-Control"))
	}
	if loc := rec.Header().Get("Location"); loc != "" {
		t.Fatalf("served destination %q", loc)
	}
}

func TestRedirectStaleRevalidatesAuthoritativeRecord(t *testing.T) {
	h := newRedirectHarness(t)
	h.seed(t, "reval", "https://example.com/new", h.now.Add(time.Hour))
	if err := h.cache.Put(context.Background(), platform.CacheRecord{
		ShortCode: "reval", State: platform.CacheActive, LongURL: "https://example.com/old",
		ExpiresAt: h.now.Add(time.Hour), FreshUntil: h.now.Add(-time.Minute), StaleUntil: h.now.Add(time.Minute), Version: 1,
	}); err != nil {
		t.Fatal(err)
	}
	rec := getRedirect(h.handler, "/reval")
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "https://example.com/new" {
		t.Fatalf("status/location = %d %q", rec.Code, rec.Header().Get("Location"))
	}
	if h.gets.gets.Load() != 1 {
		t.Fatalf("store gets = %d", h.gets.gets.Load())
	}
	cached, outcome, _ := h.cache.Get(context.Background(), "reval", h.now)
	if outcome != platform.CacheFresh || cached.LongURL != "https://example.com/new" {
		t.Fatalf("refreshed cache = %+v %q", cached, outcome)
	}
}

func TestRedirectStaleStoreErrorFallback(t *testing.T) {
	now := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
	cache := memory.NewCache()
	record := platform.CacheRecord{
		ShortCode: "grace", State: platform.CacheActive, LongURL: "https://example.com/grace",
		ExpiresAt: now.Add(time.Hour), FreshUntil: now.Add(-time.Minute), StaleUntil: now.Add(time.Minute), Version: 1,
	}
	if err := cache.Put(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	inside := NewRedirect(RedirectDeps{
		Store: failGetStore{err: platform.ErrDependency}, Cache: cache,
		Publisher: &memory.EventPublisher{}, Clock: memory.NewClock(now), PrivacyKey: []byte(testPrivacyKey),
	})
	rec := getRedirect(inside, "/grace")
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "https://example.com/grace" {
		t.Fatalf("inside window status/location = %d %q", rec.Code, rec.Header().Get("Location"))
	}

	outside := NewRedirect(RedirectDeps{
		Store: failGetStore{err: platform.ErrDependency}, Cache: memory.NewCache(),
		Publisher: &memory.EventPublisher{}, Clock: memory.NewClock(now), PrivacyKey: []byte(testPrivacyKey),
	})
	rec = getRedirect(outside, "/grace")
	if rec.Code != http.StatusServiceUnavailable || rec.Header().Get("Cache-Control") != cacheNoStore {
		t.Fatalf("outside window status/cc = %d %q", rec.Code, rec.Header().Get("Cache-Control"))
	}
}

func TestRedirectTombstoneIgnoresStoreError(t *testing.T) {
	now := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
	deletedAt := now
	cache := memory.NewCache()
	if err := cache.Put(context.Background(), platform.CacheRecord{
		ShortCode: "gone", State: platform.CacheDeleted, LongURL: "https://example.com/should-hide",
		Version: 2, DeletedAt: &deletedAt, StaleUntil: now.Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	handler := NewRedirect(RedirectDeps{
		Store: failGetStore{err: platform.ErrDependency}, Cache: cache,
		Publisher: &memory.EventPublisher{}, Clock: memory.NewClock(now), PrivacyKey: []byte(testPrivacyKey),
	})
	rec := getRedirect(handler, "/gone")
	if rec.Code != http.StatusNotFound || rec.Header().Get("Location") != "" {
		t.Fatalf("status/location = %d %q", rec.Code, rec.Header().Get("Location"))
	}
}

func TestRedirectPublisherFailureDoesNotChange302(t *testing.T) {
	h := newRedirectHarness(t)
	h.seed(t, "ok-link", "https://example.com", h.now.Add(time.Hour))
	publisher := &failPublisher{err: errors.New("kinesis down")}
	handler := NewRedirect(RedirectDeps{
		Store: h.gets, Cache: h.cache, Publisher: publisher,
		Clock: h.clock, PrivacyKey: []byte(testPrivacyKey),
	})
	rec := getRedirect(handler, "/ok-link")
	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d", rec.Code)
	}
	if publisher.calls.Load() != 1 {
		t.Fatalf("publish calls = %d", publisher.calls.Load())
	}
}

func TestRedirectInvalidCodeSkipsStore(t *testing.T) {
	store := forbiddenGetStore{t: t}
	handler := NewRedirect(RedirectDeps{
		Store: store, Cache: memory.NewCache(), Publisher: &memory.EventPublisher{},
		Clock: memory.NewClock(time.Now()), PrivacyKey: []byte(testPrivacyKey),
	})
	for _, path := range []string{"/ab", "/api", "/app", "/bad.code"} {
		rec := getRedirect(handler, path)
		if rec.Code != http.StatusNotFound || rec.Header().Get("Cache-Control") != cacheNoStore {
			t.Fatalf("%s status/cc = %d %q", path, rec.Code, rec.Header().Get("Cache-Control"))
		}
	}
}

func TestRedirectInactivePopulatesTombstone(t *testing.T) {
	h := newRedirectHarness(t)
	h.seed(t, "dead", "https://example.com/dead", h.now.Add(time.Hour))
	owner := platform.Principal{ActorID: "actor", Role: platform.RoleDeveloper, OwnerID: "owner"}
	if _, err := h.store.SoftDelete(context.Background(), owner, "dead", h.now, ""); err != nil {
		t.Fatal(err)
	}
	rec := getRedirect(h.handler, "/dead")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rec.Code)
	}
	_, outcome, err := h.cache.Get(context.Background(), "dead", h.now)
	if err != nil || outcome != platform.CacheHitDeleted {
		t.Fatalf("tombstone outcome = %q %v", outcome, err)
	}
}

func TestRedirectStaleDeletedAuthoritative(t *testing.T) {
	h := newRedirectHarness(t)
	h.seed(t, "flip", "https://example.com/old", h.now.Add(time.Hour))
	owner := platform.Principal{ActorID: "actor", Role: platform.RoleDeveloper, OwnerID: "owner"}
	if _, err := h.store.SoftDelete(context.Background(), owner, "flip", h.now, ""); err != nil {
		t.Fatal(err)
	}
	if err := h.cache.Put(context.Background(), platform.CacheRecord{
		ShortCode: "flip", State: platform.CacheActive, LongURL: "https://example.com/old",
		ExpiresAt: h.now.Add(time.Hour), FreshUntil: h.now.Add(-time.Minute), StaleUntil: h.now.Add(time.Minute), Version: 1,
	}); err != nil {
		t.Fatal(err)
	}
	rec := getRedirect(h.handler, "/flip")
	if rec.Code != http.StatusNotFound || rec.Header().Get("Location") != "" {
		t.Fatalf("served stale destination: %d %q", rec.Code, rec.Header().Get("Location"))
	}
}

func TestRedirectCacheErrorsDegradeToStore(t *testing.T) {
	h := newRedirectHarness(t)
	h.seed(t, "still", "https://example.com/still", h.now.Add(time.Hour))
	handler := NewRedirect(RedirectDeps{
		Store: h.gets, Cache: failCache{inner: h.cache, getErr: platform.ErrDependency, putErr: platform.ErrDependency},
		Publisher: h.publisher, Clock: h.clock, PrivacyKey: []byte(testPrivacyKey),
	})
	rec := getRedirect(handler, "/still")
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "https://example.com/still" {
		t.Fatalf("status/location = %d %q", rec.Code, rec.Header().Get("Location"))
	}
}

func TestRedirectForwardedIPAndNilPublisher(t *testing.T) {
	h := newRedirectHarness(t)
	h.seed(t, "fwd", "https://example.com/fwd", h.now.Add(time.Hour))
	req := httptest.NewRequest(http.MethodGet, "/fwd", nil)
	req.Header.Set("X-Forwarded-For", "198.51.100.1, 203.0.113.1")
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d", rec.Code)
	}
	events := h.publisher.Events()
	if len(events) != 1 || events[0].SourceIPHash != hashSourceIP([]byte(testPrivacyKey), "198.51.100.1", h.now) {
		t.Fatalf("events = %+v", events)
	}

	nilPub := NewRedirect(RedirectDeps{
		Store: h.gets, Cache: memory.NewCache(), Clock: h.clock, PrivacyKey: []byte(testPrivacyKey),
	})
	if rec := getRedirect(nilPub, "/fwd"); rec.Code != http.StatusFound {
		t.Fatalf("nil publisher status = %d", rec.Code)
	}
}

func TestRedirectStaleNotFound(t *testing.T) {
	now := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
	cache := memory.NewCache()
	if err := cache.Put(context.Background(), platform.CacheRecord{
		ShortCode: "ghost", State: platform.CacheActive, LongURL: "https://example.com/ghost",
		ExpiresAt: now.Add(time.Hour), FreshUntil: now.Add(-time.Minute), StaleUntil: now.Add(time.Minute), Version: 1,
	}); err != nil {
		t.Fatal(err)
	}
	handler := NewRedirect(RedirectDeps{
		Store: memory.NewLinkStore(), Cache: cache, Publisher: &memory.EventPublisher{},
		Clock: memory.NewClock(now), PrivacyKey: []byte(testPrivacyKey),
	})
	rec := getRedirect(handler, "/ghost")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestRedirectStaleEmptyFallbackUnavailable(t *testing.T) {
	now := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
	cache := memory.NewCache()
	if err := cache.Put(context.Background(), platform.CacheRecord{
		ShortCode: "empty", State: platform.CacheActive, LongURL: "",
		ExpiresAt: now.Add(time.Hour), FreshUntil: now.Add(-time.Minute), StaleUntil: now.Add(time.Minute), Version: 1,
	}); err != nil {
		t.Fatal(err)
	}
	handler := NewRedirect(RedirectDeps{
		Store: failGetStore{err: platform.ErrDependency}, Cache: cache,
		Publisher: &memory.EventPublisher{}, Clock: memory.NewClock(now), PrivacyKey: []byte(testPrivacyKey),
	})
	rec := getRedirect(handler, "/empty")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestActiveCacheRecordClampsPastExpiry(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	record := activeCacheRecord(platform.Link{ShortCode: "x", LongURL: "https://example.com", ExpiresAt: now.Add(-time.Hour)}, now)
	if record.FreshUntil.After(now) || record.StaleUntil.After(record.ExpiresAt) {
		t.Fatalf("record = %+v", record)
	}
}

func TestRequestIPWithoutPort(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.RemoteAddr = "192.0.2.1"
	if got := requestIP(req); got != "192.0.2.1" {
		t.Fatalf("ip = %q", got)
	}
	req.Header.Set("X-Forwarded-For", "198.51.100.8")
	if got := requestIP(req); got != "198.51.100.8" {
		t.Fatalf("forwarded ip = %q", got)
	}
}

func TestRequestIPPrefersCloudflareHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.RemoteAddr = "192.0.2.1"
	// Cloudflare overwrites any client-sent value, so it takes precedence over
	// the spoofable X-Forwarded-For a caller could set directly against
	// CloudFront.
	req.Header.Set("X-Forwarded-For", "203.0.113.1")
	req.Header.Set("CF-Connecting-IP", "198.51.100.9")
	if got := requestIP(req); got != "198.51.100.9" {
		t.Fatalf("ip = %q", got)
	}
}

func TestNewEventID(t *testing.T) {
	if id := newEventID(); len(id) != 32 {
		t.Fatalf("event id length = %d", len(id))
	}
}

type deadlineGetStore struct {
	platform.LinkStore
	deadline    time.Time
	hasDeadline bool
}

func (s *deadlineGetStore) Get(ctx context.Context, code string) (platform.Link, error) {
	s.deadline, s.hasDeadline = ctx.Deadline()
	return s.LinkStore.Get(ctx, code)
}

func TestRedirectStoreGetIsBounded(t *testing.T) {
	now := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
	store := memory.NewLinkStore()
	if _, err := store.Create(context.Background(), platform.Link{
		ShortCode: "bounded", LongURL: "https://example.com/", OwnerID: "owner",
		IsCustom: true, CreatedAt: now, ExpiresAt: now.Add(24 * time.Hour),
	}, nil); err != nil {
		t.Fatal(err)
	}
	gets := &deadlineGetStore{LinkStore: store}
	handler := NewRedirect(RedirectDeps{
		Store: gets, Cache: memory.NewCache(), Publisher: &memory.EventPublisher{},
		Clock: memory.NewClock(now), PrivacyKey: []byte(testPrivacyKey),
	})

	if rec := getRedirect(handler, "/bounded"); rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusFound)
	}
	if !gets.hasDeadline {
		t.Fatal("LinkStore.Get context had no deadline")
	}
	if budget := time.Until(gets.deadline); budget <= 0 || budget > storeTimeout {
		t.Fatalf("deadline budget = %v, want in (0, %v]", budget, storeTimeout)
	}
}
