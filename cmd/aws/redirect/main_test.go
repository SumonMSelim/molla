package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/aws/aws-lambda-go/events"
	"github.com/awslabs/aws-lambda-go-api-proxy/httpadapter"

	"github.com/SumonMSelim/molla/internal/adapters/aws/ddbfake"
)

func withRedirectEnv(t *testing.T) {
	t.Helper()
	srv := httptest.NewServer(ddbfake.New())
	t.Cleanup(srv.Close)
	mini := miniredis.RunT(t)
	t.Setenv("MOLLA_AWS_ENDPOINT", srv.URL)
	t.Setenv("MOLLA_REDIS_ADDR", mini.Addr())
	t.Setenv("MOLLA_CLICKS_TABLE", "clicks")
}

func TestNewHandlerRequiresPrivacyKey(t *testing.T) {
	t.Setenv("MOLLA_PRIVACY_KEY", "")
	if _, err := newHandler(); err == nil {
		t.Fatal("expected error")
	}
}

func TestNewHandlerRequiresRedisAndClicksTable(t *testing.T) {
	t.Setenv("MOLLA_PRIVACY_KEY", "privacy-test-key")
	t.Setenv("MOLLA_REDIS_ADDR", "")
	t.Setenv("MOLLA_CLICKS_TABLE", "clicks")
	if _, err := newHandler(); err == nil {
		t.Fatal("expected redis error")
	}
	t.Setenv("MOLLA_REDIS_ADDR", "127.0.0.1:6379")
	t.Setenv("MOLLA_CLICKS_TABLE", "")
	if _, err := newHandler(); err == nil {
		t.Fatal("expected clicks table error")
	}
}

func TestLambdaAdapterUnknownCode(t *testing.T) {
	withRedirectEnv(t)
	t.Setenv("MOLLA_PRIVACY_KEY", "privacy-test-key")
	handler, err := newHandler()
	if err != nil {
		t.Fatal(err)
	}
	resp, err := httpadapter.NewV2(handler).ProxyWithContext(context.Background(), events.APIGatewayV2HTTPRequest{
		RawPath: "/missing",
		RequestContext: events.APIGatewayV2HTTPRequestContext{
			HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{
				Method: http.MethodGet,
				Path:   "/missing",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestRunRedirect(t *testing.T) {
	withRedirectEnv(t)
	t.Setenv("MOLLA_PRIVACY_KEY", "privacy-test-key")
	started := false
	if err := runRedirect(func(any) { started = true }); err != nil {
		t.Fatal(err)
	}
	if !started {
		t.Fatal("lambda start not invoked")
	}
}

func TestRunRedirectRequiresPrivacyKey(t *testing.T) {
	t.Setenv("MOLLA_PRIVACY_KEY", "")
	if err := runRedirect(func(any) {}); err == nil {
		t.Fatal("expected error")
	}
}
