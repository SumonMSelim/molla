package handlers

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/SumonMSelim/molla/internal/adapters/memory"
	"github.com/SumonMSelim/molla/internal/core"
	"github.com/SumonMSelim/molla/internal/platform"
)

func TestCreateGeneratedCodeAndDefaultExpiry(t *testing.T) {
	h := newHarness(t, "")
	rec := postCreate(h.handler, testAPIToken, "", `{"long_url":"https://example.com/very/long/path"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	got := decodeCreate(t, rec)
	if got.ShortCode != "UIiAFaQ" || len(got.ShortCode) != core.CodeLength {
		t.Fatalf("short_code = %q", got.ShortCode)
	}
	if got.ShortURL != "https://mol.la/UIiAFaQ" {
		t.Fatalf("short_url = %q", got.ShortURL)
	}
	if got.LongURL != "https://example.com/very/long/path" {
		t.Fatalf("long_url = %q", got.LongURL)
	}
	if got.CreatedAt != "2026-09-12T00:00:00Z" {
		t.Fatalf("created_at = %q", got.CreatedAt)
	}
	if got.ExpiresAt != "2031-09-11T00:00:00Z" {
		t.Fatalf("expires_at = %q", got.ExpiresAt)
	}
	created, err := time.Parse(time.RFC3339, got.CreatedAt)
	if err != nil {
		t.Fatal(err)
	}
	expires, err := time.Parse(time.RFC3339, got.ExpiresAt)
	if err != nil {
		t.Fatal(err)
	}
	if expires.Sub(created) != time.Duration(core.DefaultExpirySeconds)*time.Second {
		t.Fatalf("expiry delta = %s", expires.Sub(created))
	}
}

func TestCreateCustomAliasAndPublicBaseTrim(t *testing.T) {
	h := newHarness(t, "https://mol.la/")
	rec := postCreate(h.handler, testAPIToken, "", `{"long_url":"https://example.com","alias":"my-link","expires_in":60}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	got := decodeCreate(t, rec)
	if got.ShortCode != "my-link" || got.ShortURL != "https://mol.la/my-link" {
		t.Fatalf("response = %+v", got)
	}
	stored, err := h.store.Get(context.Background(), "my-link")
	if err != nil || !stored.IsCustom || stored.OwnerID != testOwner {
		t.Fatalf("stored = %+v, err = %v", stored, err)
	}
}

func TestCreateAliasTaken(t *testing.T) {
	h := newHarness(t, "")
	if rec := postCreate(h.handler, testAPIToken, "", `{"long_url":"https://example.com","alias":"taken"}`); rec.Code != http.StatusCreated {
		t.Fatalf("seed status = %d", rec.Code)
	}
	rec := postCreate(h.handler, testOtherToken, "", `{"long_url":"https://example.com/other","alias":"taken"}`)
	if rec.Code != http.StatusConflict || decodeError(t, rec) != "ALIAS_TAKEN" {
		t.Fatalf("status/body = %d %s", rec.Code, rec.Body.String())
	}
}

func TestCreateRejectsInvalidURLWithoutStoreWrite(t *testing.T) {
	h := newHarness(t, "")
	rec := postCreate(h.handler, testAPIToken, "", `{"long_url":"javascript:alert(1)"}`)
	if rec.Code != http.StatusBadRequest || decodeError(t, rec) != "INVALID_URL" {
		t.Fatalf("status/body = %d %s", rec.Code, rec.Body.String())
	}
	if h.counted.creates.Load() != 0 {
		t.Fatalf("Create called %d times", h.counted.creates.Load())
	}
}

func TestCreateExpiryBounds(t *testing.T) {
	h := newHarness(t, "")
	tests := []struct {
		name       string
		expiresIn  string
		wantStatus int
		wantError  string
	}{
		{name: "below min", expiresIn: "59", wantStatus: http.StatusBadRequest, wantError: "INVALID_EXPIRY"},
		{name: "min", expiresIn: "60", wantStatus: http.StatusCreated},
		{name: "max", expiresIn: "157680000", wantStatus: http.StatusCreated},
		{name: "above max", expiresIn: "157680001", wantStatus: http.StatusBadRequest, wantError: "INVALID_EXPIRY"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			body := `{"long_url":"https://example.com","alias":"` + strings.ReplaceAll(test.name, " ", "-") + `","expires_in":` + test.expiresIn + `}`
			rec := postCreate(h.handler, testAPIToken, "", body)
			if rec.Code != test.wantStatus {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if test.wantError != "" && decodeError(t, rec) != test.wantError {
				t.Fatalf("error = %s", rec.Body.String())
			}
		})
	}
}

func TestCreateRejectsOversizedBodyWithoutReading(t *testing.T) {
	h := newHarness(t, "")
	rec := postCreateReader(h.handler, testAPIToken, "", failReader{t: t}, maxCreateBody+1)
	if rec.Code != http.StatusBadRequest || decodeError(t, rec) != "INVALID_REQUEST" {
		t.Fatalf("status/body = %d %s", rec.Code, rec.Body.String())
	}
	if h.counted.creates.Load() != 0 {
		t.Fatalf("Create called %d times", h.counted.creates.Load())
	}
}

func TestCreateRejectsOversizedUnknownLengthBody(t *testing.T) {
	h := newHarness(t, "")
	body := strings.Repeat("a", maxCreateBody+1)
	rec := postCreateReader(h.handler, testAPIToken, "", strings.NewReader(body), -1)
	if rec.Code != http.StatusBadRequest || decodeError(t, rec) != "INVALID_REQUEST" {
		t.Fatalf("status/body = %d %s", rec.Code, rec.Body.String())
	}
}

func TestCreateIdempotentReplay(t *testing.T) {
	h := newHarness(t, "")
	body := `{"long_url":"https://example.com"}`
	first := postCreate(h.handler, testAPIToken, "retry-key", body)
	second := postCreate(h.handler, testAPIToken, "retry-key", body)
	if first.Code != http.StatusCreated || second.Code != http.StatusCreated {
		t.Fatalf("status %d / %d", first.Code, second.Code)
	}
	if decodeCreate(t, first).ShortCode != decodeCreate(t, second).ShortCode {
		t.Fatal("replay minted a different code")
	}
	if _, err := h.store.Get(context.Background(), "ReHEeU7"); !errors.Is(err, platform.ErrNotFound) {
		t.Fatal("second generated code was stored")
	}
}

func TestCreateConcurrentIdempotentReplay(t *testing.T) {
	h := newHarness(t, "")
	start := make(chan struct{})
	results := make(chan *httptest.ResponseRecorder, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results <- postCreate(h.handler, testAPIToken, "concurrent", `{"long_url":"https://example.com/concurrent"}`)
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	codes := map[string]struct{}{}
	for rec := range results {
		if rec.Code != http.StatusCreated {
			t.Fatalf("status = %d %s", rec.Code, rec.Body.String())
		}
		codes[decodeCreate(t, rec).ShortCode] = struct{}{}
	}
	if len(codes) != 1 {
		t.Fatalf("unique codes = %d", len(codes))
	}
}

func TestCreateIdempotencyConflict(t *testing.T) {
	h := newHarness(t, "")
	if rec := postCreate(h.handler, testAPIToken, "same", `{"long_url":"https://example.com/one"}`); rec.Code != http.StatusCreated {
		t.Fatalf("seed = %d %s", rec.Code, rec.Body.String())
	}
	rec := postCreate(h.handler, testAPIToken, "same", `{"long_url":"https://example.com/two"}`)
	if rec.Code != http.StatusConflict || decodeError(t, rec) != "IDEMPOTENCY_CONFLICT" {
		t.Fatalf("status/body = %d %s", rec.Code, rec.Body.String())
	}
}

func TestCreateIdempotencyIsOwnerScoped(t *testing.T) {
	h := newHarness(t, "")
	first := postCreate(h.handler, testAPIToken, "shared", `{"long_url":"https://example.com/a"}`)
	second := postCreate(h.handler, testOtherToken, "shared", `{"long_url":"https://example.com/b"}`)
	if first.Code != http.StatusCreated || second.Code != http.StatusCreated {
		t.Fatalf("status %d / %d", first.Code, second.Code)
	}
	if decodeCreate(t, first).ShortCode == decodeCreate(t, second).ShortCode {
		t.Fatal("owners shared a short code")
	}
}

func TestCreateCanonicalHashIgnoresJSONLayout(t *testing.T) {
	h := newHarness(t, "")
	first := postCreate(h.handler, testAPIToken, "canon", "{\n  \"expires_in\": 157680000,\n  \"long_url\": \"https://example.com\" \n}")
	second := postCreate(h.handler, testAPIToken, "canon", `{"long_url":"https://example.com","alias":""}`)
	if first.Code != http.StatusCreated || second.Code != http.StatusCreated {
		t.Fatalf("status %d / %d bodies %s / %s", first.Code, second.Code, first.Body.String(), second.Body.String())
	}
	if decodeCreate(t, first).ShortCode != decodeCreate(t, second).ShortCode {
		t.Fatal("canonical hash treated equivalent bodies as different")
	}

	hashDefault := canonicalCreateHash("https://example.com", "", core.DefaultExpirySeconds)
	hashExplicit := canonicalCreateHash("https://example.com", "", 157680000)
	if hashDefault != hashExplicit {
		t.Fatalf("hashes %s vs %s", hashDefault, hashExplicit)
	}
}

func TestCreateGeneratedCodeRetriesAliasCollision(t *testing.T) {
	h := newHarness(t, "")
	if rec := postCreate(h.handler, testAPIToken, "", `{"long_url":"https://example.com","alias":"UIiAFaQ"}`); rec.Code != http.StatusCreated {
		t.Fatalf("seed = %d %s", rec.Code, rec.Body.String())
	}
	rec := postCreate(h.handler, testAPIToken, "", `{"long_url":"https://example.com/generated"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d %s", rec.Code, rec.Body.String())
	}
	if got := decodeCreate(t, rec).ShortCode; got != "ReHEeU7" {
		t.Fatalf("short_code = %q, want next generated code", got)
	}
}

func TestCreateCollisionDoesNotLeaveIdempotencyRecord(t *testing.T) {
	h := newHarness(t, "")
	if rec := postCreate(h.handler, testAPIToken, "", `{"long_url":"https://example.com","alias":"taken"}`); rec.Code != http.StatusCreated {
		t.Fatal(rec.Body.String())
	}
	rec := postCreate(h.handler, testAPIToken, "orphan", `{"long_url":"https://example.com/new","alias":"taken"}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d", rec.Code)
	}
	retry := postCreate(h.handler, testAPIToken, "orphan", `{"long_url":"https://example.com/retry","alias":"fresh"}`)
	if retry.Code != http.StatusCreated {
		t.Fatalf("retry status = %d %s", retry.Code, rec.Body.String())
	}
	if decodeCreate(t, retry).ShortCode != "fresh" {
		t.Fatal("orphaned idempotency record reused")
	}
}

func TestCreateValidationErrors(t *testing.T) {
	h := newHarness(t, "")
	tests := []struct {
		name       string
		idem       string
		body       string
		wantStatus int
		wantError  string
	}{
		{name: "malformed json", body: `{"long_url":`, wantStatus: http.StatusBadRequest, wantError: "INVALID_REQUEST"},
		{name: "invalid alias", body: `{"long_url":"https://example.com","alias":"ab"}`, wantStatus: http.StatusBadRequest, wantError: "INVALID_ALIAS"},
		{name: "reserved alias", body: `{"long_url":"https://example.com","alias":"api"}`, wantStatus: http.StatusBadRequest, wantError: "INVALID_ALIAS"},
		{name: "empty idempotency key", idem: " ", body: `{"long_url":"https://example.com"}`, wantStatus: http.StatusBadRequest, wantError: "INVALID_REQUEST"},
		{name: "too long idempotency key", idem: strings.Repeat("a", 129), body: `{"long_url":"https://example.com"}`, wantStatus: http.StatusBadRequest, wantError: "INVALID_REQUEST"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rec := postCreate(h.handler, testAPIToken, test.idem, test.body)
			if rec.Code != test.wantStatus || decodeError(t, rec) != test.wantError {
				t.Fatalf("status/body = %d %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestCreateEmptyIdempotencyHeader(t *testing.T) {
	h := newHarness(t, "")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/links", strings.NewReader(`{"long_url":"https://example.com"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", testAPIToken)
	req.Header["Idempotency-Key"] = []string{""}
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || decodeError(t, rec) != "INVALID_REQUEST" {
		t.Fatalf("status/body = %d %s", rec.Code, rec.Body.String())
	}
}

func TestCreateWrongMethod(t *testing.T) {
	h := newHarness(t, "")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/links", nil)
	req.Header.Set("X-Api-Key", testAPIToken)
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestCreateMissingPrincipal(t *testing.T) {
	api := &API{}
	rec := httptest.NewRecorder()
	api.create(rec, httptest.NewRequest(http.MethodPost, "/api/v1/links", strings.NewReader(`{}`)))
	if rec.Code != http.StatusUnauthorized || decodeError(t, rec) != "UNAUTHORIZED" {
		t.Fatalf("status/body = %d %s", rec.Code, rec.Body.String())
	}
}

func TestCreateAllocatorAndStoreFailures(t *testing.T) {
	now := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
	permuter, err := core.NewPermuter([]byte(testPermuteKey))
	if err != nil {
		t.Fatal(err)
	}
	creds := newTestCredentials(t)
	t.Run("allocator", func(t *testing.T) {
		h := New(Deps{
			Store:       &countingStore{LinkStore: &memoryLinkStub{}},
			Allocator:   failAllocator{err: platform.ErrDependency},
			Clock:       memory.NewClock(now),
			Credentials: creds,
			Permuter:    permuter,
		})
		rec := postCreate(h, testAPIToken, "", `{"long_url":"https://example.com"}`)
		if rec.Code != http.StatusServiceUnavailable || decodeError(t, rec) != "TEMPORARILY_UNAVAILABLE" {
			t.Fatalf("status/body = %d %s", rec.Code, rec.Body.String())
		}
	})
	t.Run("store", func(t *testing.T) {
		h := New(Deps{
			Store:       failStore{err: platform.ErrDependency},
			Allocator:   &leaseAllocator{},
			Clock:       memory.NewClock(now),
			Credentials: creds,
			Permuter:    permuter,
		})
		rec := postCreate(h, testAPIToken, "", `{"long_url":"https://example.com"}`)
		if rec.Code != http.StatusServiceUnavailable || decodeError(t, rec) != "TEMPORARILY_UNAVAILABLE" {
			t.Fatalf("status/body = %d %s", rec.Code, rec.Body.String())
		}
	})
	t.Run("negative id", func(t *testing.T) {
		h := New(Deps{
			Store:       failStore{err: errors.New("unused")},
			Allocator:   failAllocator{id: -1},
			Clock:       memory.NewClock(now),
			Credentials: creds,
			Permuter:    permuter,
		})
		rec := postCreate(h, testAPIToken, "", `{"long_url":"https://example.com"}`)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d", rec.Code)
		}
	})
	t.Run("collision exhausted", func(t *testing.T) {
		h := New(Deps{
			Store:       failStore{err: platform.ErrCollision},
			Allocator:   &leaseAllocator{},
			Clock:       memory.NewClock(now),
			Credentials: creds,
			Permuter:    permuter,
		})
		rec := postCreate(h, testAPIToken, "", `{"long_url":"https://example.com"}`)
		if rec.Code != http.StatusServiceUnavailable || decodeError(t, rec) != "TEMPORARILY_UNAVAILABLE" {
			t.Fatalf("status/body = %d %s", rec.Code, rec.Body.String())
		}
	})
	t.Run("permute out of range", func(t *testing.T) {
		h := New(Deps{
			Store:       failStore{err: errors.New("unused")},
			Allocator:   failAllocator{id: int64(core.Base62Limit)},
			Clock:       memory.NewClock(now),
			Credentials: creds,
			Permuter:    permuter,
		})
		rec := postCreate(h, testAPIToken, "", `{"long_url":"https://example.com"}`)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d %s", rec.Code, rec.Body.String())
		}
	})
}

func TestWriteJSONMarshalError(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(rec, http.StatusOK, make(chan int))
	if rec.Code != http.StatusServiceUnavailable || decodeError(t, rec) != "TEMPORARILY_UNAVAILABLE" {
		t.Fatalf("status/body = %d %s", rec.Code, rec.Body.String())
	}
}

func TestValidIdempotencyKey(t *testing.T) {
	tests := []struct {
		key  string
		want bool
	}{
		{key: "a", want: true},
		{key: strings.Repeat("a", 128), want: true},
		{key: "", want: false},
		{key: strings.Repeat("a", 129), want: false},
		{key: "has space", want: false},
		{key: "tab\tkey", want: false},
		{key: "héllo", want: false},
	}
	for _, test := range tests {
		if got := validIdempotencyKey(test.key); got != test.want {
			t.Fatalf("validIdempotencyKey(%q) = %v", test.key, got)
		}
	}
}

type failReader struct{ t *testing.T }

func (f failReader) Read([]byte) (int, error) {
	f.t.Fatal("body was read")
	return 0, io.EOF
}

type failAllocator struct {
	id  int64
	err error
}

func (f failAllocator) Lease(context.Context) (int64, error) {
	if f.err != nil {
		return 0, f.err
	}
	return f.id, nil
}

type leaseAllocator struct{ n int64 }

func (a *leaseAllocator) Lease(context.Context) (int64, error) {
	id := a.n
	a.n++
	return id, nil
}

type failStore struct{ err error }

func (s failStore) Create(context.Context, platform.Link, *platform.Idempotency) (platform.Link, error) {
	return platform.Link{}, s.err
}
func (s failStore) Get(context.Context, string) (platform.Link, error) {
	return platform.Link{}, platform.ErrNotFound
}
func (s failStore) SoftDelete(context.Context, platform.Principal, string, time.Time, string) (platform.Deletion, error) {
	return platform.Deletion{}, platform.ErrNotFound
}

type memoryLinkStub struct{}

func (memoryLinkStub) Create(context.Context, platform.Link, *platform.Idempotency) (platform.Link, error) {
	return platform.Link{}, nil
}
func (memoryLinkStub) Get(context.Context, string) (platform.Link, error) {
	return platform.Link{}, platform.ErrNotFound
}
func (memoryLinkStub) SoftDelete(context.Context, platform.Principal, string, time.Time, string) (platform.Deletion, error) {
	return platform.Deletion{}, platform.ErrNotFound
}

func newTestCredentials(t *testing.T) *credentialStub {
	t.Helper()
	return &credentialStub{principal: platform.Principal{ActorID: testActor, Role: platform.RoleDeveloper, OwnerID: testOwner}}
}

type credentialStub struct {
	principal platform.Principal
	err       error
}

func (c *credentialStub) Store(context.Context, string, platform.Credential) error { return nil }
func (c *credentialStub) Resolve(context.Context, string, time.Time) (platform.Principal, error) {
	if c.err != nil {
		return platform.Principal{}, c.err
	}
	return c.principal, nil
}

func (c *credentialStub) Revoke(context.Context, string) error { return c.err }
