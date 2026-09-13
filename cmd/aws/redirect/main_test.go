package main

import (
	"context"
	"net/http"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/awslabs/aws-lambda-go-api-proxy/httpadapter"
)

func TestNewHandlerRequiresPrivacyKey(t *testing.T) {
	t.Setenv("MOLLA_PRIVACY_KEY", "")
	if _, err := newHandler(); err == nil {
		t.Fatal("expected error")
	}
}

func TestLambdaAdapterUnknownCode(t *testing.T) {
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
