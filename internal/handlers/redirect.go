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
	"time"

	"github.com/SumonMSelim/molla/internal/adapters/logging"
	"github.com/SumonMSelim/molla/internal/core"
	"github.com/SumonMSelim/molla/internal/platform"
)

const (
	cacheFreshTTL   = 24 * time.Hour
	cacheStaleGrace = 15 * time.Minute
	publishTimeout  = 100 * time.Millisecond
	// storeTimeout bounds the DynamoDB read on the hot path. The redirect
	// Lambda has a 3s budget; 1s leaves room for the SDK's three attempts to
	// finish or be cut short, plus the cache write and the response.
	storeTimeout     = 1 * time.Second
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
	store      platform.LinkStore
	cache      platform.Cache
	publisher  platform.EventPublisher
	clock      platform.Clock
	privacyKey []byte
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
	log := logging.FromContext(r.Context()).With("short_code", code)
	if err != nil {
		log.Error("cache read failed", "error", err)
		outcome = platform.CacheMiss
		record = platform.CacheRecord{}
	} else {
		log.Debug("cache read", "outcome", outcome)
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

// storeGet reads the link with a budget tighter than the Lambda's own timeout,
// so a hung DynamoDB call still leaves room to fall back or fail fast.
func (h *Redirect) storeGet(r *http.Request, code string) (platform.Link, error) {
	ctx, cancel := context.WithTimeout(r.Context(), storeTimeout)
	defer cancel()
	return h.store.Get(ctx, code)
}

func (h *Redirect) revalidate(w http.ResponseWriter, r *http.Request, code string, now time.Time, stale platform.CacheRecord) {
	link, err := h.storeGet(r, code)
	if err == nil {
		h.applyStore(w, r, link, now)
		return
	}
	if errors.Is(err, platform.ErrNotFound) {
		writeNoStore(w, http.StatusNotFound)
		return
	}
	if stale.State == platform.CacheActive && now.Before(stale.StaleUntil) && now.Before(stale.ExpiresAt) && stale.LongURL != "" {
		logging.FromContext(r.Context()).Warn("serving stale cache record", "short_code", code, "error", err)
		h.found(w, r, code, stale.LongURL, now)
		return
	}
	logging.FromContext(r.Context()).Error("revalidate failed", "short_code", code, "error", err)
	writeNoStore(w, http.StatusServiceUnavailable)
}

func (h *Redirect) lookup(w http.ResponseWriter, r *http.Request, code string, now time.Time) {
	link, err := h.storeGet(r, code)
	if errors.Is(err, platform.ErrNotFound) {
		writeNoStore(w, http.StatusNotFound)
		return
	}
	if err != nil {
		logging.FromContext(r.Context()).Error("link lookup failed", "short_code", code, "error", err)
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
		if err := h.cache.Put(r.Context(), activeCacheRecord(link, now)); err != nil {
			logging.FromContext(r.Context()).Error("cache populate failed", "short_code", link.ShortCode, "error", err)
		}
		h.found(w, r, link.ShortCode, decision.LongURL, now)
		return
	}
	if !link.IsActive {
		if err := h.cache.Put(r.Context(), deletedCacheRecord(link, now)); err != nil {
			logging.FromContext(r.Context()).Error("tombstone populate failed", "short_code", link.ShortCode, "error", err)
		}
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
		logging.FromContext(r.Context()).Error("click publish failed", "short_code", code, "error", err)
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
	// mol.la is proxied through Cloudflare, which sets this to the real client
	// IP on every request it forwards; unlike X-Forwarded-For it cannot be
	// spoofed by the client (Cloudflare overwrites any client-sent value).
	if cf := strings.TrimSpace(r.Header.Get("CF-Connecting-IP")); cf != "" {
		return cf
	}
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
