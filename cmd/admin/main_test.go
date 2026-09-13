package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	awsadapter "github.com/SumonMSelim/molla/internal/adapters/aws"
	"github.com/SumonMSelim/molla/internal/adapters/aws/ddbfake"
	"github.com/SumonMSelim/molla/internal/platform"
)

func TestRunTakedown(t *testing.T) {
	var got struct {
		actor, code, reason string
	}
	rt := adminRuntime{
		delete: func(_ context.Context, principal platform.Principal, code, reason string) error {
			got.actor, got.code, got.reason = principal.ActorID, code, reason
			if principal.Role != platform.RoleOperator {
				t.Fatalf("role = %q", principal.Role)
			}
			return nil
		},
		lookupEnv: func(string) (string, bool) { return "", false },
		stdout:    io.Discard,
	}
	var out bytes.Buffer
	rt.stdout = &out
	if err := run([]string{"takedown", "--code", "abc1234", "--reason", "malware", "--actor", "arn:ops"}, rt); err != nil {
		t.Fatal(err)
	}
	if got.actor != "arn:ops" || got.code != "abc1234" || got.reason != "malware" {
		t.Fatalf("got %+v", got)
	}
	if !strings.Contains(out.String(), "abc1234") {
		t.Fatalf("stdout = %q", out.String())
	}
}

func TestRunTakedownIdentityAndErrors(t *testing.T) {
	called := false
	rt := adminRuntime{
		delete: func(context.Context, platform.Principal, string, string) error {
			called = true
			return nil
		},
		identity: func(context.Context) (platform.Principal, error) {
			return platform.Principal{ActorID: "arn:sts", Role: platform.RoleOperator}, nil
		},
		stdout: io.Discard,
	}
	if err := run([]string{"takedown", "--code", "abc1234", "--reason", "phish"}, rt); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("delete not called")
	}
	if err := run(nil, rt); err == nil {
		t.Fatal("expected usage error")
	}
	if err := run([]string{"other"}, rt); err == nil {
		t.Fatal("expected unknown command")
	}
	if err := run([]string{"takedown", "--code", "abc1234"}, rt); err == nil {
		t.Fatal("expected missing reason")
	}
	rt.identity = nil
	if err := run([]string{"takedown", "--code", "abc1234", "--reason", "x"}, rt); err == nil {
		t.Fatal("expected missing actor")
	}
	if err := run([]string{"takedown", "--bogus"}, rt); err == nil {
		t.Fatal("expected flag parse error")
	}
	rt.identity = func(context.Context) (platform.Principal, error) {
		return platform.Principal{ActorID: "arn:sts", Role: platform.RoleOperator}, nil
	}
	rt.delete = func(context.Context, platform.Principal, string, string) error {
		return platform.ErrNotFound
	}
	if err := run([]string{"takedown", "--code", "missing", "--reason", "x"}, rt); !errors.Is(err, platform.ErrNotFound) {
		t.Fatalf("error = %v", err)
	}
}

func TestAWSRuntimeTakedown(t *testing.T) {
	fake := ddbfake.New()
	fake.Seed("Links", "takenow", ddbfake.AV{
		"short_code": {"S": "takenow"},
		"long_url":   {"S": "https://example.com"},
		"owner_id":   {"S": "owner"},
		"is_custom":  {"BOOL": false},
		"is_active":  {"BOOL": true},
		"version":    {"N": "1"},
		"created_at": {"N": "1700000000"},
		"expires_at": {"N": "1700003600"},
		"purge_at":   {"N": "1700003600"},
	})

	var invalidated []byte
	var audit bytes.Buffer
	mux := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		target := r.Header.Get("X-Amz-Target")
		switch {
		case strings.Contains(target, "DynamoDB"):
			fake.ServeHTTP(w, r)
		case strings.Contains(r.URL.Path, "invocations"):
			invalidated, _ = io.ReadAll(r.Body)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		default:
			body, _ := io.ReadAll(r.Body)
			_ = r.ParseForm()
			if strings.Contains(string(body), "GetCallerIdentity") || strings.Contains(r.Form.Encode(), "GetCallerIdentity") {
				w.Header().Set("Content-Type", "text/xml")
				_, _ = w.Write([]byte(`<GetCallerIdentityResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/"><GetCallerIdentityResult><Arn>arn:aws:iam::1:user/ops</Arn><Account>1</Account><UserId>AIDAI</UserId></GetCallerIdentityResult></GetCallerIdentityResponse>`))
				return
			}
			http.NotFound(w, r)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	cfg := awsadapter.StaticConfig("us-east-1", srv.URL, srv.Client())
	var out bytes.Buffer
	rt := newAWSRuntime(cfg, "molla-invalidate", &audit, &out)
	if err := run([]string{"takedown", "--code", "takenow", "--reason", "malware"}, rt); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "takenow") {
		t.Fatalf("stdout = %q", out.String())
	}
	var req struct {
		ShortCode string `json:"short_code"`
		Version   int64  `json:"version"`
	}
	if err := json.Unmarshal(invalidated, &req); err != nil {
		t.Fatalf("payload %q: %v", invalidated, err)
	}
	if req.ShortCode != "takenow" || req.Version != 2 {
		t.Fatalf("invalidate payload = %+v", req)
	}
	if !strings.Contains(audit.String(), "owner") || !strings.Contains(audit.String(), "arn:aws:iam::1:user/ops") {
		t.Fatalf("audit = %s", audit.String())
	}
}

func TestRunIssue(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	var stored struct {
		token string
		cred  platform.Credential
	}
	var out bytes.Buffer
	rt := adminRuntime{
		issue: func(_ context.Context, token string, cred platform.Credential) error {
			stored.token, stored.cred = token, cred
			return nil
		},
		now:       func() time.Time { return now },
		randToken: func() (string, error) { return "generated-token", nil },
		stdout:    &out,
	}
	if err := run([]string{"issue", "--owner", "acme", "--actor", "arn:ops"}, rt); err != nil {
		t.Fatal(err)
	}
	if stored.token != "generated-token" || stored.cred.OwnerID != "acme" || stored.cred.ActorID != "arn:ops" {
		t.Fatalf("stored = %+v", stored)
	}
	if stored.cred.ExpiresAt != now.Add(90*24*time.Hour) {
		t.Fatalf("expires = %s", stored.cred.ExpiresAt)
	}
	if !strings.Contains(out.String(), "generated-token") {
		t.Fatalf("stdout = %q", out.String())
	}
	if err := run([]string{"issue", "--owner", "acme", "--actor", "arn:ops", "--token", "existing-key"}, rt); err != nil {
		t.Fatal(err)
	}
	if stored.token != "existing-key" {
		t.Fatalf("token = %q", stored.token)
	}
}

func TestRunIssueErrors(t *testing.T) {
	rt := adminRuntime{stdout: io.Discard}
	if err := run([]string{"issue", "--owner", "acme"}, rt); err == nil {
		t.Fatal("expected issue not configured")
	}
	rt.issue = func(context.Context, string, platform.Credential) error { return nil }
	if err := run([]string{"issue"}, rt); err == nil {
		t.Fatal("expected owner")
	}
	if err := run([]string{"issue", "--owner", "acme", "--days", "0", "--actor", "a"}, rt); err == nil {
		t.Fatal("expected days")
	}
	if err := run([]string{"issue", "--owner", "acme", "--days", "91", "--actor", "a"}, rt); err == nil {
		t.Fatal("expected days")
	}
	if err := run([]string{"issue", "--owner", "acme"}, rt); err == nil {
		t.Fatal("expected actor")
	}
	rt.identity = func(context.Context) (platform.Principal, error) {
		return platform.Principal{}, errors.New("sts down")
	}
	if err := run([]string{"issue", "--owner", "acme"}, rt); err == nil {
		t.Fatal("expected sts")
	}
	rt.identity = nil
	rt.randToken = nil
	if err := run([]string{"issue", "--owner", "acme", "--actor", "a"}, rt); err == nil {
		t.Fatal("expected generator")
	}
	if err := run([]string{"issue", "--bogus"}, rt); err == nil {
		t.Fatal("expected flag parse")
	}
}

func TestAWSRuntimeIssue(t *testing.T) {
	fake := ddbfake.New()
	mux := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.Header.Get("X-Amz-Target"), "DynamoDB") {
			fake.ServeHTTP(w, r)
			return
		}
		http.NotFound(w, r)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	cfg := awsadapter.StaticConfig("us-east-1", srv.URL, srv.Client())
	var out bytes.Buffer
	rt := newAWSRuntime(cfg, "molla-invalidate", io.Discard, &out)
	if err := run([]string{"issue", "--owner", "acme", "--actor", "arn:ops", "--token", "raw-secret-token", "--days", "1"}, rt); err != nil {
		t.Fatal(err)
	}
	hash := platform.HashToken("raw-secret-token")
	item, ok := fake.Item("Credentials", hash)
	if !ok {
		t.Fatal("credential not stored")
	}
	if got, _ := item["owner_id"]["S"].(string); got != "acme" {
		t.Fatalf("item = %+v", item)
	}
}

func TestRandomToken(t *testing.T) {
	a, err := randomToken()
	if err != nil || len(a) != 64 {
		t.Fatalf("token %q %v", a, err)
	}
	b, err := randomToken()
	if err != nil || a == b {
		t.Fatalf("tokens not unique")
	}
}
