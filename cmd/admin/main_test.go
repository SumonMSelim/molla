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
