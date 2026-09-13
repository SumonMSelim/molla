package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

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

func TestRunTakedownActorFromEnvAndErrors(t *testing.T) {
	called := false
	rt := adminRuntime{
		delete: func(context.Context, platform.Principal, string, string) error {
			called = true
			return nil
		},
		lookupEnv: func(key string) (string, bool) {
			if key == "MOLLA_ADMIN_ACTOR" {
				return "env-actor", true
			}
			return "", false
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
	rt.lookupEnv = func(string) (string, bool) { return "", false }
	if err := run([]string{"takedown", "--code", "abc1234", "--reason", "x"}, rt); err == nil {
		t.Fatal("expected missing actor")
	}
	if err := run([]string{"takedown", "--bogus"}, rt); err == nil {
		t.Fatal("expected flag parse error")
	}
	rt.lookupEnv = func(string) (string, bool) { return "actor", true }
	rt.delete = func(context.Context, platform.Principal, string, string) error {
		return platform.ErrNotFound
	}
	if err := run([]string{"takedown", "--code", "missing", "--reason", "x"}, rt); !errors.Is(err, platform.ErrNotFound) {
		t.Fatalf("error = %v", err)
	}
}

func TestNewRuntime(t *testing.T) {
	rt := newRuntime()
	if rt.delete == nil || rt.lookupEnv == nil || rt.stdout == nil {
		t.Fatal("incomplete runtime")
	}
}
