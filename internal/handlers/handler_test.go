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
	testAPIToken   = "dev-test-token"
	testOtherToken = "dev-other-token"
	testPermuteKey = "molla-slice-1-fixed-test-key"
	testActor      = "actor"
	testOwner      = "owner"
	testOtherOwner = "other-owner"
	testOtherActor = "other-actor"
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
	creds   *memory.CredentialStore
	alloc   *memory.IDAllocator
	now     time.Time
}

func newHarness(t *testing.T, publicBase string) *harness {
	t.Helper()
	now := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
	store := memory.NewLinkStore()
	counted := &countingStore{LinkStore: store}
	clock := memory.NewClock(now)
	creds := memory.NewCredentialStore()
	alloc := memory.NewIDAllocator(0)
	permuter, err := core.NewPermuter([]byte(testPermuteKey))
	if err != nil {
		t.Fatal(err)
	}
	seedCredential(t, creds, testAPIToken, testActor, testOwner, now, now.Add(24*time.Hour), platform.CredentialActive)
	seedCredential(t, creds, testOtherToken, testOtherActor, testOtherOwner, now, now.Add(24*time.Hour), platform.CredentialActive)
	return &harness{
		handler: New(Deps{
			Store:       counted,
			Allocator:   alloc,
			Clock:       clock,
			Credentials: creds,
			Permuter:    permuter,
			PublicBase:  publicBase,
		}),
		store:   store,
		counted: counted,
		clock:   clock,
		creds:   creds,
		alloc:   alloc,
		now:     now,
	}
}

func seedCredential(t *testing.T, store *memory.CredentialStore, token, actor, owner string, issued, expires time.Time, status platform.CredentialStatus) {
	t.Helper()
	err := store.Store(context.Background(), token, platform.Credential{
		ActorID: actor, OwnerID: owner, Status: status, IssuedAt: issued, ExpiresAt: expires,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func postCreate(h http.Handler, apiKey, idem, body string) *httptest.ResponseRecorder {
	return postCreateReader(h, apiKey, idem, strings.NewReader(body), int64(len(body)))
}

func postCreateReader(h http.Handler, apiKey, idem string, body io.Reader, contentLength int64) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/links", body)
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("X-Api-Key", apiKey)
	}
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
