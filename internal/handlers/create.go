package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/SumonMSelim/molla/internal/adapters/logging"
	"github.com/SumonMSelim/molla/internal/core"
	"github.com/SumonMSelim/molla/internal/platform"
)

type createRequest struct {
	LongURL   string `json:"long_url"`
	Alias     string `json:"alias"`
	ExpiresIn *int64 `json:"expires_in"`
}

type canonicalCreateRequest struct {
	LongURL   string `json:"long_url"`
	Alias     string `json:"alias"`
	ExpiresIn int64  `json:"expires_in"`
}

type createResponse struct {
	ShortCode string `json:"short_code"`
	ShortURL  string `json:"short_url"`
	LongURL   string `json:"long_url"`
	CreatedAt string `json:"created_at"`
	ExpiresAt string `json:"expires_at"`
}

func (a *API) create(w http.ResponseWriter, r *http.Request) {
	if r.ContentLength > maxCreateBody {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxCreateBody)

	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}

	if err := core.ValidateURL(req.LongURL); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_URL")
		return
	}
	if req.Alias != "" {
		if err := core.ValidateAlias(req.Alias); err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_ALIAS")
			return
		}
	}
	expiresIn := core.DefaultExpirySeconds
	if req.ExpiresIn != nil {
		expiresIn = *req.ExpiresIn
	}
	if err := core.ValidateExpiry(expiresIn); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_EXPIRY")
		return
	}

	idem, err := a.idempotency(r, req.LongURL, req.Alias, expiresIn)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST")
		return
	}

	now := a.clock.Now().UTC().Truncate(time.Second)
	link := platform.Link{
		LongURL:   req.LongURL,
		IsCustom:  req.Alias != "",
		CreatedAt: now,
		ExpiresAt: now.Add(time.Duration(expiresIn) * time.Second),
	}

	created, err := a.persist(r.Context(), link, req.Alias, idem)
	if err != nil {
		if errors.Is(err, platform.ErrDependency) {
			logging.FromContext(r.Context()).Error("link create failed", "custom_alias", req.Alias != "", "error", err)
		}
		writeCreateError(w, err, req.Alias != "")
		return
	}
	writeJSON(w, http.StatusCreated, createResponse{
		ShortCode: created.ShortCode,
		ShortURL:  a.publicBase + "/" + created.ShortCode,
		LongURL:   created.LongURL,
		CreatedAt: created.CreatedAt.UTC().Format(time.RFC3339),
		ExpiresAt: created.ExpiresAt.UTC().Format(time.RFC3339),
	})
}

func (a *API) persist(ctx context.Context, link platform.Link, alias string, idem *platform.Idempotency) (platform.Link, error) {
	if alias != "" {
		link.ShortCode = alias
		return a.store.Create(ctx, link, idem)
	}
	var last error
	for range maxCodeAttempts {
		code, err := a.generatedCode(ctx)
		if err != nil {
			return platform.Link{}, err
		}
		link.ShortCode = code
		created, err := a.store.Create(ctx, link, idem)
		if err == nil {
			return created, nil
		}
		if !errors.Is(err, platform.ErrCollision) {
			return platform.Link{}, err
		}
		last = err
	}
	return platform.Link{}, last
}

func (a *API) generatedCode(ctx context.Context) (string, error) {
	id, err := a.allocator.Lease(ctx)
	if err != nil {
		return "", err
	}
	if id < 0 {
		return "", platform.ErrDependency
	}
	permuted, err := a.permuter.Permute(uint64(id))
	if err != nil {
		return "", err
	}
	return core.EncodeBase62(permuted)
}

func (a *API) idempotency(r *http.Request, longURL, alias string, expiresIn int64) (*platform.Idempotency, error) {
	if _, ok := r.Header[http.CanonicalHeaderKey("Idempotency-Key")]; !ok {
		return nil, nil
	}
	key := r.Header.Get("Idempotency-Key")
	if !validIdempotencyKey(key) {
		return nil, errInvalidIdempotencyKey
	}
	return &platform.Idempotency{
		Key:         key,
		RequestHash: canonicalCreateHash(longURL, alias, expiresIn),
		ExpiresAt:   a.clock.Now().UTC().Add(idempotencyTTL),
	}, nil
}

var errInvalidIdempotencyKey = errors.New("invalid idempotency key")

func validIdempotencyKey(key string) bool {
	if len(key) < 1 || len(key) > 128 {
		return false
	}
	for i := 0; i < len(key); i++ {
		if key[i] < 33 || key[i] > 126 {
			return false
		}
	}
	return true
}

func canonicalCreateHash(longURL, alias string, expiresIn int64) string {
	payload, _ := json.Marshal(canonicalCreateRequest{
		LongURL:   longURL,
		Alias:     alias,
		ExpiresIn: expiresIn,
	})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func writeCreateError(w http.ResponseWriter, err error, customAlias bool) {
	switch {
	case errors.Is(err, platform.ErrCollision) && customAlias:
		writeError(w, http.StatusConflict, "ALIAS_TAKEN")
	case errors.Is(err, platform.ErrIdempotencyConflict):
		writeError(w, http.StatusConflict, "IDEMPOTENCY_CONFLICT")
	default:
		writeError(w, http.StatusServiceUnavailable, "TEMPORARILY_UNAVAILABLE")
	}
}
