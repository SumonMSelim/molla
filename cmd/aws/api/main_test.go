package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/awslabs/aws-lambda-go-api-proxy/httpadapter"

	"github.com/SumonMSelim/molla/internal/adapters/aws/ddbfake"
)

func withFakeAWS(t *testing.T) {
	t.Helper()
	srv := httptest.NewServer(ddbfake.New())
	t.Cleanup(srv.Close)
	t.Setenv("MOLLA_AWS_ENDPOINT", srv.URL)
	t.Setenv("MOLLA_INVALIDATE_FUNCTION", "molla-invalidate")
}

func TestNewHandlerRequiresPermutationKey(t *testing.T) {
	t.Setenv("MOLLA_PERMUTATION_KEY", "")
	if _, err := newHandler(); err == nil {
		t.Fatal("expected error")
	}
}

func TestLambdaAdapterUnauthorized(t *testing.T) {
	withFakeAWS(t)
	t.Setenv("MOLLA_PERMUTATION_KEY", "molla-slice-1-fixed-test-key")
	t.Setenv("MOLLA_PUBLIC_BASE", "https://mol.la")
	handler, err := newHandler()
	if err != nil {
		t.Fatal(err)
	}
	resp, err := httpadapter.New(handler).ProxyWithContext(context.Background(), events.APIGatewayProxyRequest{
		HTTPMethod: http.MethodPost,
		Path:       "/api/v1/links",
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       `{"long_url":"https://example.com"}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, resp.Body)
	}
	var body struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(resp.Body), &body); err != nil {
		t.Fatal(err)
	}
	if body.Error != "UNAUTHORIZED" {
		t.Fatalf("error = %q", body.Error)
	}
}

func TestLambdaAdapterInvalidKey(t *testing.T) {
	withFakeAWS(t)
	t.Setenv("MOLLA_PERMUTATION_KEY", "molla-slice-1-fixed-test-key")
	handler, err := newHandler()
	if err != nil {
		t.Fatal(err)
	}
	resp, err := httpadapter.New(handler).ProxyWithContext(context.Background(), events.APIGatewayProxyRequest{
		HTTPMethod: http.MethodPost,
		Path:       "/api/v1/links",
		Headers:    map[string]string{"Content-Type": "application/json", "X-Api-Key": "nope"},
		Body:       `{"long_url":"https://example.com"}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, resp.Body)
	}
}

func TestRunAPI(t *testing.T) {
	withFakeAWS(t)
	t.Setenv("MOLLA_PERMUTATION_KEY", "molla-slice-1-fixed-test-key")
	started := false
	if err := runAPI(func(any) { started = true }); err != nil {
		t.Fatal(err)
	}
	if !started {
		t.Fatal("lambda start not invoked")
	}
}

func TestRunAPIRequiresPermutationKey(t *testing.T) {
	t.Setenv("MOLLA_PERMUTATION_KEY", "")
	if err := runAPI(func(any) {}); err == nil {
		t.Fatal("expected error")
	}
}
