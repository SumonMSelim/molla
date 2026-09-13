package aws

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStaticConfigUsesProvidedEndpoint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	cfg := StaticConfig("eu-west-1", srv.URL, srv.Client())
	if cfg.Region != "eu-west-1" {
		t.Fatalf("region = %q", cfg.Region)
	}
	if cfg.BaseEndpoint == nil || *cfg.BaseEndpoint != srv.URL {
		t.Fatalf("endpoint = %v", cfg.BaseEndpoint)
	}
	if cfg.RetryMaxAttempts != 1 {
		t.Fatalf("retries = %d", cfg.RetryMaxAttempts)
	}
}

func TestStaticConfigDefaults(t *testing.T) {
	cfg := StaticConfig("", "http://127.0.0.1:1", nil)
	if cfg.Region != "us-east-1" {
		t.Fatalf("region = %q", cfg.Region)
	}
	if cfg.HTTPClient == nil {
		t.Fatal("nil HTTP client")
	}
}

func TestRuntimeConfigUsesEndpoint(t *testing.T) {
	t.Setenv("MOLLA_AWS_ENDPOINT", "http://127.0.0.1:1")
	t.Setenv("AWS_REGION", "eu-central-1")
	cfg, err := RuntimeConfig(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Region != "eu-central-1" {
		t.Fatalf("region = %q", cfg.Region)
	}
	if cfg.BaseEndpoint == nil || *cfg.BaseEndpoint != "http://127.0.0.1:1" {
		t.Fatalf("endpoint = %v", cfg.BaseEndpoint)
	}
}
