package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/SumonMSelim/molla/internal/adapters/memory"
	"github.com/SumonMSelim/molla/internal/platform"
)

func TestDeleterOperatorTakedownThenRedirectNotFound(t *testing.T) {
	h := newHarness(t, "")
	rec := postCreate(h.handler, "", `{"long_url":"https://example.com/delete-me","alias":"delcode"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create = %d %s", rec.Code, rec.Body.String())
	}
	deleter := Deleter{
		Store:       h.store,
		Invalidator: memory.NewCacheInvalidator(h.cache),
		Audit:       &memory.AuditSink{},
		Clock:       h.clock,
	}
	operator := platform.Principal{ActorID: "arn:aws:iam::1:user/ops", Role: platform.RoleOperator}
	if err := deleter.Delete(context.Background(), operator, "delcode", "malware"); err != nil {
		t.Fatal(err)
	}
	link, err := h.store.Get(context.Background(), "delcode")
	if err != nil || link.IsActive {
		t.Fatalf("stored = %+v err=%v", link, err)
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

func TestDeleteInvalidateFailureThenRetry(t *testing.T) {
	h := newHarness(t, "")
	if rec := postCreate(h.handler, "", `{"long_url":"https://example.com","alias":"retryxx"}`); rec.Code != http.StatusCreated {
		t.Fatal(rec.Body.String())
	}
	before, err := h.store.Get(context.Background(), "retryxx")
	if err != nil {
		t.Fatal(err)
	}
	audit := &memory.AuditSink{}
	inv := &failOnceInvalidator{next: memory.NewCacheInvalidator(h.cache), err: errors.New("redis down")}
	deleter := Deleter{
		Store:       h.store,
		Invalidator: inv,
		Audit:       audit,
		Clock:       h.clock,
	}
	operator := platform.Principal{ActorID: "arn:aws:iam::1:user/ops", Role: platform.RoleOperator}

	if err := deleter.Delete(context.Background(), operator, "retryxx", "malware"); !errors.Is(err, platform.ErrDependency) {
		t.Fatalf("first delete err = %v", err)
	}
	after, err := h.store.Get(context.Background(), "retryxx")
	if err != nil || after.IsActive || after.Version != before.Version+1 {
		t.Fatalf("after first delete = %+v err=%v", after, err)
	}
	events := audit.Events()
	if len(events) != 1 || events[0].Outcome != auditInvalidateFailed {
		t.Fatalf("audit = %+v", events)
	}

	if err := deleter.Delete(context.Background(), operator, "retryxx", "malware"); err != nil {
		t.Fatalf("retry err = %v", err)
	}
	retryLink, err := h.store.Get(context.Background(), "retryxx")
	if err != nil || retryLink.Version != after.Version {
		t.Fatalf("retry mutated version: %+v", retryLink)
	}
	events = audit.Events()
	if len(events) != 2 || events[1].Outcome != auditDeleted {
		t.Fatalf("retry audit = %+v", events)
	}
}

func TestDeleterOperatorTakedownAndAuditFailure(t *testing.T) {
	store := memory.NewLinkStore()
	now := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
	link := platform.Link{
		ShortCode: "takenow", LongURL: "https://example.com",
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
	if got.ActorID != operator.ActorID || got.Role != platform.RoleOperator || got.ShortCode != "takenow" || got.Reason != "malware" || got.Outcome != auditDeleted || !got.Timestamp.Equal(now) {
		t.Fatalf("audit event = %+v", got)
	}

	deleter.Audit = failAudit{err: errors.New("sink down")}
	if err := deleter.Delete(context.Background(), operator, "takenow", "malware"); err != nil {
		t.Fatal(err)
	}
}

func TestDeleterStoreDependency(t *testing.T) {
	now := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
	deleter := Deleter{
		Store:       failLinkStore{err: platform.ErrDependency},
		Invalidator: memory.NewCacheInvalidator(memory.NewCache()),
		Audit:       &memory.AuditSink{},
		Clock:       memory.NewClock(now),
	}
	operator := platform.Principal{ActorID: "arn:aws:iam::1:user/ops", Role: platform.RoleOperator}
	if err := deleter.Delete(context.Background(), operator, "statzzz", "malware"); !errors.Is(err, platform.ErrDependency) {
		t.Fatalf("err = %v", err)
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
