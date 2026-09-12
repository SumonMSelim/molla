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

func TestAuthenticateAPIKey(t *testing.T) {
	h := newHarness(t, "")
	body := `{"long_url":"https://example.com"}`

	t.Run("missing", func(t *testing.T) {
		rec := postCreate(h.handler, "", "", body)
		if rec.Code != http.StatusUnauthorized || decodeError(t, rec) != "UNAUTHORIZED" {
			t.Fatalf("status/body = %d %s", rec.Code, rec.Body.String())
		}
		if h.counted.creates.Load() != 0 {
			t.Fatalf("Create called %d times", h.counted.creates.Load())
		}
	})
	t.Run("unknown", func(t *testing.T) {
		rec := postCreate(h.handler, "unknown-token", "", body)
		if rec.Code != http.StatusUnauthorized || decodeError(t, rec) != "UNAUTHORIZED" {
			t.Fatalf("status/body = %d %s", rec.Code, rec.Body.String())
		}
		if h.counted.creates.Load() != 0 {
			t.Fatalf("Create called %d times", h.counted.creates.Load())
		}
	})
	t.Run("expired", func(t *testing.T) {
		h.clock.Advance(24 * time.Hour)
		rec := postCreate(h.handler, testAPIToken, "", body)
		if rec.Code != http.StatusUnauthorized || decodeError(t, rec) != "UNAUTHORIZED" {
			t.Fatalf("status/body = %d %s", rec.Code, rec.Body.String())
		}
		h.clock.Set(h.now)
	})
	t.Run("revoked", func(t *testing.T) {
		seedCredential(t, h.creds, "revoked-token", "revoked-actor", "revoked-owner", h.now, h.now.Add(time.Hour), platform.CredentialRevoked)
		rec := postCreate(h.handler, "revoked-token", "", body)
		if rec.Code != http.StatusUnauthorized || decodeError(t, rec) != "UNAUTHORIZED" {
			t.Fatalf("status/body = %d %s", rec.Code, rec.Body.String())
		}
		if h.counted.creates.Load() != 0 {
			t.Fatalf("Create called %d times", h.counted.creates.Load())
		}
	})
}

func TestAuthenticateDependencyFailure(t *testing.T) {
	now := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
	permuter, err := core.NewPermuter([]byte(testPermuteKey))
	if err != nil {
		t.Fatal(err)
	}
	h := New(Deps{
		Store:       failStore{err: platform.ErrDependency},
		Allocator:   &leaseAllocator{},
		Clock:       memory.NewClock(now),
		Credentials: &credentialStub{err: platform.ErrDependency},
		Permuter:    permuter,
	})
	rec := postCreate(h, "any-token", "", `{"long_url":"https://example.com"}`)
	if rec.Code != http.StatusServiceUnavailable || decodeError(t, rec) != "TEMPORARILY_UNAVAILABLE" {
		t.Fatalf("status/body = %d %s", rec.Code, rec.Body.String())
	}
}

func TestAuthenticateInjectsPrincipal(t *testing.T) {
	h := newHarness(t, "")
	rec := postCreate(h.handler, testAPIToken, "", `{"long_url":"https://example.com/owned"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d %s", rec.Code, rec.Body.String())
	}
	code := decodeCreate(t, rec).ShortCode
	stored, err := h.store.Get(context.Background(), code)
	if err != nil {
		t.Fatal(err)
	}
	if stored.OwnerID != testOwner {
		t.Fatalf("owner = %q", stored.OwnerID)
	}
}

func TestEmptyAPIKeyHeader(t *testing.T) {
	h := newHarness(t, "")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/links", nil)
	req.Header["X-Api-Key"] = []string{""}
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rec.Code)
	}
}
