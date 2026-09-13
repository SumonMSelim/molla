package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/SumonMSelim/molla/internal/adapters/memory"
	"github.com/SumonMSelim/molla/internal/core"
	"github.com/SumonMSelim/molla/internal/platform"
)

func TestDeleteOwnerThenRedirectNotFound(t *testing.T) {
	h := newHarness(t, "")
	rec := postCreate(h.handler, testAPIToken, "", `{"long_url":"https://example.com/delete-me","alias":"delcode"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create = %d %s", rec.Code, rec.Body.String())
	}
	deleted := deleteLink(h.handler, testAPIToken, "delcode")
	if deleted.Code != http.StatusNoContent {
		t.Fatalf("delete = %d %s", deleted.Code, deleted.Body.String())
	}
	if deleted.Body.Len() != 0 {
		t.Fatalf("body = %q", deleted.Body.String())
	}
	link, err := h.store.Get(context.Background(), "delcode")
	if err != nil || link.IsActive {
		t.Fatalf("stored = %+v err=%v", link, err)
	}
	events := h.audit.Events()
	if len(events) != 1 || events[0].Outcome != auditDeleted || events[0].ActorID != testActor || events[0].OwnerID != testOwner || events[0].ShortCode != "delcode" {
		t.Fatalf("audit = %+v", events)
	}
	redirect := NewRedirect(RedirectDeps{
		Store:      h.store,
		Cache:      h.cache,
		Publisher:  &memory.EventPublisher{},
		Clock:      h.clock,
		PrivacyKey: []byte(testPrivacyKey),
	})
	req := httptest.NewRequest(http.MethodGet, "/delcode", nil)
	out := httptest.NewRecorder()
	redirect.ServeHTTP(out, req)
	if out.Code != http.StatusNotFound {
		t.Fatalf("redirect status = %d", out.Code)
	}
}

func TestDeleteForbiddenAndNotFound(t *testing.T) {
	h := newHarness(t, "")
	if rec := postCreate(h.handler, testAPIToken, "", `{"long_url":"https://example.com","alias":"keepme1"}`); rec.Code != http.StatusCreated {
		t.Fatal(rec.Body.String())
	}
	forbidden := deleteLink(h.handler, testOtherToken, "keepme1")
	if forbidden.Code != http.StatusForbidden || decodeError(t, forbidden) != "FORBIDDEN" {
		t.Fatalf("forbidden = %d %s", forbidden.Code, forbidden.Body.String())
	}
	link, err := h.store.Get(context.Background(), "keepme1")
	if err != nil || !link.IsActive {
		t.Fatalf("link mutated: %+v %v", link, err)
	}
	missing := deleteLink(h.handler, testAPIToken, "noexist")
	if missing.Code != http.StatusNotFound || decodeError(t, missing) != "NOT_FOUND" {
		t.Fatalf("missing = %d %s", missing.Code, missing.Body.String())
	}
	invalid := deleteLink(h.handler, testAPIToken, "ab")
	if invalid.Code != http.StatusNotFound {
		t.Fatalf("invalid = %d", invalid.Code)
	}
	unauth := deleteLink(h.handler, "", "keepme1")
	if unauth.Code != http.StatusUnauthorized {
		t.Fatalf("unauth = %d", unauth.Code)
	}
}

func TestDeleteInvalidateFailureThenRetry(t *testing.T) {
	h := newHarness(t, "")
	if rec := postCreate(h.handler, testAPIToken, "", `{"long_url":"https://example.com","alias":"retryxx"}`); rec.Code != http.StatusCreated {
		t.Fatal(rec.Body.String())
	}
	before, err := h.store.Get(context.Background(), "retryxx")
	if err != nil {
		t.Fatal(err)
	}
	inv := &failOnceInvalidator{next: memory.NewCacheInvalidator(h.cache), err: errors.New("redis down")}
	permuter, err := core.NewPermuter([]byte(testPermuteKey))
	if err != nil {
		t.Fatal(err)
	}
	handler := New(Deps{
		Store:       h.store,
		Allocator:   h.alloc,
		Clock:       h.clock,
		Credentials: h.creds,
		Permuter:    permuter,
		Stats:       h.stats,
		Invalidator: inv,
		Audit:       h.audit,
	})
	first := deleteLink(handler, testAPIToken, "retryxx")
	if first.Code != http.StatusServiceUnavailable || decodeError(t, first) != "TEMPORARILY_UNAVAILABLE" {
		t.Fatalf("first = %d %s", first.Code, first.Body.String())
	}
	after, err := h.store.Get(context.Background(), "retryxx")
	if err != nil || after.IsActive || after.Version != before.Version+1 {
		t.Fatalf("after first delete = %+v err=%v", after, err)
	}
	events := h.audit.Events()
	if len(events) != 1 || events[0].Outcome != auditInvalidateFailed {
		t.Fatalf("audit = %+v", events)
	}

	second := deleteLink(handler, testAPIToken, "retryxx")
	if second.Code != http.StatusNoContent {
		t.Fatalf("retry = %d %s", second.Code, second.Body.String())
	}
	retryLink, err := h.store.Get(context.Background(), "retryxx")
	if err != nil || retryLink.Version != after.Version {
		t.Fatalf("retry mutated version: %+v", retryLink)
	}
	events = h.audit.Events()
	if len(events) != 2 || events[1].Outcome != auditDeleted {
		t.Fatalf("retry audit = %+v", events)
	}
}

func TestDeleteOperatorTakedownAndAuditFailure(t *testing.T) {
	store := memory.NewLinkStore()
	now := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
	link := platform.Link{
		ShortCode: "takenow", LongURL: "https://example.com", OwnerID: testOwner,
		CreatedAt: now, ExpiresAt: now.Add(time.Hour),
	}
	if _, err := store.Create(context.Background(), link, nil); err != nil {
		t.Fatal(err)
	}
	cache := memory.NewCache()
	audit := &memory.AuditSink{}
	clock := memory.NewClock(now)
	deleter := Deleter{
		Store:       store,
		Invalidator: memory.NewCacheInvalidator(cache),
		Audit:       audit,
		Clock:       clock,
	}
	operator := platform.Principal{ActorID: "arn:aws:iam::1:user/ops", Role: platform.RoleOperator}
	if err := deleter.Delete(context.Background(), operator, "takenow", ""); !errors.Is(err, platform.ErrForbidden) {
		t.Fatalf("empty reason error = %v", err)
	}
	if err := deleter.Delete(context.Background(), operator, "takenow", "malware"); err != nil {
		t.Fatal(err)
	}
	events := audit.Events()
	if len(events) != 1 {
		t.Fatalf("events = %+v", events)
	}
	got := events[0]
	if got.ActorID != operator.ActorID || got.Role != platform.RoleOperator || got.OwnerID != testOwner || got.ShortCode != "takenow" || got.Reason != "malware" || got.Outcome != auditDeleted || !got.Timestamp.Equal(now) {
		t.Fatalf("audit event = %+v", got)
	}

	deleter.Audit = failAudit{err: errors.New("sink down")}
	if err := deleter.Delete(context.Background(), operator, "takenow", "malware"); err != nil {
		t.Fatal(err)
	}

	api := &API{deleter: deleter}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/links/takenow", nil)
	req.SetPathValue("short_code", "takenow")
	api.delete(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("missing principal = %d", rec.Code)
	}
}

func TestDeleteStoreDependency(t *testing.T) {
	now := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
	permuter, err := core.NewPermuter([]byte(testPermuteKey))
	if err != nil {
		t.Fatal(err)
	}
	h := New(Deps{
		Store:       failLinkStore{err: platform.ErrDependency},
		Allocator:   &leaseAllocator{},
		Clock:       memory.NewClock(now),
		Credentials: newTestCredentials(t),
		Permuter:    permuter,
		Stats:       memory.NewStatsStore(),
		Invalidator: memory.NewCacheInvalidator(memory.NewCache()),
		Audit:       &memory.AuditSink{},
	})
	rec := deleteLink(h, testAPIToken, "statzzz")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d %s", rec.Code, rec.Body.String())
	}
}

type failOnceInvalidator struct {
	next  platform.CacheInvalidator
	err   error
	calls int
}

func (f *failOnceInvalidator) Invalidate(ctx context.Context, deletion platform.Deletion) error {
	f.calls++
	if f.calls == 1 && f.err != nil {
		return f.err
	}
	return f.next.Invalidate(ctx, deletion)
}

type failAudit struct{ err error }

func (f failAudit) Record(context.Context, platform.AuditEvent) error { return f.err }
