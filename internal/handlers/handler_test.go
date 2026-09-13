package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SumonMSelim/molla/internal/adapters/memory"
	"github.com/SumonMSelim/molla/internal/core"
	"github.com/SumonMSelim/molla/internal/platform"
)

const (
	testPermuteKey = "molla-slice-1-fixed-test-key"
)

type countingStore struct {
	platform.LinkStore
	creates atomic.Int32
}

func (s *countingStore) Create(ctx context.Context, link platform.Link, idem *platform.Idempotency) (platform.Link, error) {
	s.creates.Add(1)
	return s.LinkStore.Create(ctx, link, idem)
}

type harness struct {
	handler http.Handler
	store   *memory.LinkStore
	counted *countingStore
	clock   *memory.Clock
	alloc   *memory.IDAllocator
	cache   *memory.Cache
	stats   *memory.StatsStore
	now     time.Time
}

func newHarness(t *testing.T, publicBase string) *harness {
	t.Helper()
	now := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
	store := memory.NewLinkStore()
	counted := &countingStore{LinkStore: store}
	clock := memory.NewClock(now)
	alloc := memory.NewIDAllocator(0)
	cache := memory.NewCache()
	stats := memory.NewStatsStore()
	permuter, err := core.NewPermuter([]byte(testPermuteKey))
	if err != nil {
		t.Fatal(err)
	}
	return &harness{
		handler: New(Deps{
			Store:      counted,
			Allocator:  alloc,
			Clock:      clock,
			Permuter:   permuter,
			PublicBase: publicBase,
			Stats:      stats,
		}),
		store:   store,
		counted: counted,
		clock:   clock,
		alloc:   alloc,
		cache:   cache,
		stats:   stats,
		now:     now,
	}
}

func postCreate(h http.Handler, idem, body string) *httptest.ResponseRecorder {
	return postCreateReader(h, idem, strings.NewReader(body), int64(len(body)))
}

func postCreateReader(h http.Handler, idem string, body io.Reader, contentLength int64) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/links", body)
	req.Header.Set("Content-Type", "application/json")
	if idem != "" {
		req.Header.Set("Idempotency-Key", idem)
	}
	req.ContentLength = contentLength
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decodeError(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body errorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body: %v (%s)", err, rec.Body.String())
	}
	return body.Error
}

func decodeCreate(t *testing.T, rec *httptest.ResponseRecorder) createResponse {
	t.Helper()
	var body createResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode create body: %v (%s)", err, rec.Body.String())
	}
	return body
}

func getStats(h http.Handler, code string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/links/"+code+"/stats", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decodeStats(t *testing.T, rec *httptest.ResponseRecorder) statsResponse {
	t.Helper()
	var body statsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode stats body: %v (%s)", err, rec.Body.String())
	}
	return body
}
