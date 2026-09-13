package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewHandlerRequiresPermutationKey(t *testing.T) {
	t.Setenv("MOLLA_PERMUTATION_KEY", "")
	t.Setenv("MOLLA_PRIVACY_KEY", "privacy")
	if _, err := newHandler(); err == nil {
		t.Fatal("expected error")
	}
}

func TestNewHandlerRequiresPrivacyKey(t *testing.T) {
	t.Setenv("MOLLA_PERMUTATION_KEY", "molla-slice-1-fixed-test-key")
	t.Setenv("MOLLA_PRIVACY_KEY", "")
	if _, err := newHandler(); err == nil {
		t.Fatal("expected error")
	}
}

func TestLocalServerCreateAndRedirect(t *testing.T) {
	t.Setenv("MOLLA_PERMUTATION_KEY", "molla-slice-1-fixed-test-key")
	t.Setenv("MOLLA_PRIVACY_KEY", "privacy-test-key")
	t.Setenv("MOLLA_PUBLIC_BASE", "http://127.0.0.1:8080")
	t.Setenv("MOLLA_DEV_API_KEY", "dev-local-key")

	handler, err := newHandler()
	if err != nil {
		t.Fatal(err)
	}

	create := httptest.NewRequest(http.MethodPost, "/api/v1/links", strings.NewReader(`{"long_url":"https://example.com/local"}`))
	create.Header.Set("Content-Type", "application/json")
	create.Header.Set("X-Api-Key", "dev-local-key")
	create.Header.Set("Idempotency-Key", "local-one")
	created := httptest.NewRecorder()
	handler.ServeHTTP(created, create)
	if created.Code != http.StatusCreated {
		t.Fatalf("create status = %d body = %s", created.Code, created.Body.String())
	}
	var body struct {
		ShortCode string `json:"short_code"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.ShortCode == "" {
		t.Fatal("missing short_code")
	}

	redirect := httptest.NewRequest(http.MethodGet, "/"+body.ShortCode, nil)
	got := httptest.NewRecorder()
	handler.ServeHTTP(got, redirect)
	if got.Code != http.StatusFound {
		t.Fatalf("redirect status = %d body = %s", got.Code, got.Body.String())
	}
	if loc := got.Header().Get("Location"); loc != "https://example.com/local" {
		t.Fatalf("Location = %q", loc)
	}
}

func TestLocalServerUnauthorized(t *testing.T) {
	t.Setenv("MOLLA_PERMUTATION_KEY", "molla-slice-1-fixed-test-key")
	t.Setenv("MOLLA_PRIVACY_KEY", "privacy-test-key")
	handler, err := newHandler()
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/links", strings.NewReader(`{"long_url":"https://example.com"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rec.Code)
	}
	payload, _ := io.ReadAll(rec.Body)
	if !strings.Contains(string(payload), "UNAUTHORIZED") {
		t.Fatalf("body = %s", payload)
	}
}
