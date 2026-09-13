package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/SumonMSelim/molla/internal/core"
	"github.com/SumonMSelim/molla/internal/platform"
)

const (
	cacheFreshTTL    = 24 * time.Hour
	cacheStaleGrace  = 15 * time.Minute
	publishTimeout   = 100 * time.Millisecond
	redirectCacheTTL = "public, max-age=5"
	cacheNoStore     = "no-store"
)

// RedirectDeps are the ports the unauthenticated redirect origin needs.
type RedirectDeps struct {
	Store      platform.LinkStore
	Cache      platform.Cache
	Publisher  platform.EventPublisher
	Clock      platform.Clock
	PrivacyKey []byte
}

type Redirect struct {
	store           platform.LinkStore
	cache           platform.Cache
	publisher       platform.EventPublisher
	clock           platform.Clock
	privacyKey      []byte
	publishFailures atomic.Uint64
}

// NewRedirect returns the GET /{code} mux for the redirect origin.
func NewRedirect(deps RedirectDeps) http.Handler {
	redirect := &Redirect{
		store:      deps.Store,
		cache:      deps.Cache,
		publisher:  deps.Publisher,
		clock:      deps.Clock,
		privacyKey: append([]byte(nil), deps.PrivacyKey...),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{code}", redirect.serve)
	return mux
}

func (h *Redirect) serve(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	if core.ValidateAlias(code) != nil {
		writeNoStore(w, http.StatusNotFound)
		return
	}

	now := h.clock.Now().UTC()
	record, outcome, err := h.cache.Get(r.Context(), code, now)
	if err != nil {
		outcome = platform.CacheMiss
		record = platform.CacheRecord{}
	}

	switch outcome {
	case platform.CacheFresh:
		if record.State == platform.CacheActive && now.Before(record.ExpiresAt) && record.LongURL != "" {
			h.found(w, r, code, record.LongURL, now)
			return
		}
	case platform.CacheHitDeleted:
		writeNoStore(w, http.StatusNotFound)
		return
	case platform.CacheStale:
		h.revalidate(w, r, code, now, record)
		return
	}

	h.lookup(w, r, code, now)
}

func (h *Redirect) revalidate(w http.ResponseWriter, r *http.Request, code string, now time.Time, stale platform.CacheRecord) {
	link, err := h.store.Get(r.Context(), code)
	if err == nil {
		h.applyStore(w, r, link, now)
		return
	}
	if errors.Is(err, platform.ErrNotFound) {
		writeNoStore(w, http.StatusNotFound)
		return
	}
	if stale.State == platform.CacheActive && now.Before(stale.StaleUntil) && now.Before(stale.ExpiresAt) && stale.LongURL != "" {
		h.found(w, r, code, stale.LongURL, now)
		return
	}
	writeNoStore(w, http.StatusServiceUnavailable)
}

func (h *Redirect) lookup(w http.ResponseWriter, r *http.Request, code string, now time.Time) {
	link, err := h.store.Get(r.Context(), code)
	if errors.Is(err, platform.ErrNotFound) {
		writeNoStore(w, http.StatusNotFound)
		return
	}
	if err != nil {
		writeNoStore(w, http.StatusServiceUnavailable)
		return
	}
	h.applyStore(w, r, link, now)
}

func (h *Redirect) applyStore(w http.ResponseWriter, r *http.Request, link platform.Link, now time.Time) {
	decision := core.DecideRedirect(&core.RedirectLink{
		LongURL:   link.LongURL,
		IsActive:  link.IsActive,
		ExpiresAt: link.ExpiresAt,
	}, now)
	if decision.Found {
		_ = h.cache.Put(r.Context(), activeCacheRecord(link, now))
		h.found(w, r, link.ShortCode, decision.LongURL, now)
		return
	}
	if !link.IsActive {
		_ = h.cache.Put(r.Context(), deletedCacheRecord(link, now))
	}
	writeNoStore(w, http.StatusNotFound)
}

func (h *Redirect) found(w http.ResponseWriter, r *http.Request, code, location string, now time.Time) {
	w.Header().Set("Location", location)
	w.Header().Set("Cache-Control", redirectCacheTTL)
	w.WriteHeader(http.StatusFound)
	h.publish(r, code, now)
}

func (h *Redirect) publish(r *http.Request, code string, now time.Time) {
	if h.publisher == nil {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), publishTimeout)
	defer cancel()
	err := h.publisher.Publish(ctx, platform.ClickEvent{
		EventID:      newEventID(),
		ShortCode:    code,
		OccurredAt:   now,
		SourceIPHash: hashSourceIP(h.privacyKey, requestIP(r), now),
	})
	if err != nil {
		h.publishFailures.Add(1)
	}
}

func activeCacheRecord(link platform.Link, now time.Time) platform.CacheRecord {
	remaining := link.ExpiresAt.Sub(now)
	fresh := cacheFreshTTL
	if remaining < fresh {
		fresh = remaining
	}
	if fresh < 0 {
		fresh = 0
	}
	freshUntil := now.Add(fresh)
	staleUntil := freshUntil.Add(cacheStaleGrace)
	if staleUntil.After(link.ExpiresAt) {
		staleUntil = link.ExpiresAt
	}
	return platform.CacheRecord{
		ShortCode:  link.ShortCode,
		State:      platform.CacheActive,
		LongURL:    link.LongURL,
		ExpiresAt:  link.ExpiresAt,
		FreshUntil: freshUntil,
		StaleUntil: staleUntil,
		Version:    link.Version,
	}
}

func deletedCacheRecord(link platform.Link, now time.Time) platform.CacheRecord {
	deletedAt := now
	if link.DeletedAt != nil {
		deletedAt = *link.DeletedAt
	}
	return platform.CacheRecord{
		ShortCode:  link.ShortCode,
		State:      platform.CacheDeleted,
		Version:    link.Version,
		StaleUntil: deletedAt.Add(platform.DeleteTombstoneTTL),
		DeletedAt:  &deletedAt,
	}
}

func writeNoStore(w http.ResponseWriter, status int) {
	w.Header().Set("Cache-Control", cacheNoStore)
	w.WriteHeader(status)
}

func hashSourceIP(key []byte, ip string, now time.Time) string {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(now.UTC().Format("2006-01-02")))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte(ip))
	return hex.EncodeToString(mac.Sum(nil))
}

func requestIP(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		if i := strings.IndexByte(forwarded, ','); i >= 0 {
			forwarded = forwarded[:i]
		}
		return strings.TrimSpace(forwarded)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func newEventID() string {
	var raw [16]byte
	_, _ = rand.Read(raw[:])
	return hex.EncodeToString(raw[:])
}
