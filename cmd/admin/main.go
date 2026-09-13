// Command admin is the operator CLI for abuse takedown. Slice 5 authenticates
// with a local operator principal (--actor or MOLLA_ADMIN_ACTOR); Slice 6
// replaces that with STS-derived identity and production adapters.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/SumonMSelim/molla/internal/adapters/memory"
	"github.com/SumonMSelim/molla/internal/handlers"
	"github.com/SumonMSelim/molla/internal/platform"
)

type liveClock struct{}

func (liveClock) Now() time.Time { return time.Now().UTC() }

type adminRuntime struct {
	delete    func(context.Context, platform.Principal, string, string) error
	lookupEnv func(string) (string, bool)
	stdout    io.Writer
}

func newRuntime() adminRuntime {
	cache := memory.NewCache()
	return adminRuntime{
		delete: handlers.Deleter{
			Store:       memory.NewLinkStore(),
			Invalidator: memory.NewCacheInvalidator(cache),
			Audit:       &memory.AuditSink{},
			Clock:       liveClock{},
		}.Delete,
		lookupEnv: os.LookupEnv,
		stdout:    os.Stdout,
	}
}

func run(args []string, rt adminRuntime) error {
	if len(args) < 1 {
		return errors.New("usage: admin takedown --code CODE --reason REASON [--actor ACTOR]")
	}
	switch args[0] {
	case "takedown":
		return runTakedown(args[1:], rt)
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runTakedown(args []string, rt adminRuntime) error {
	fs := flag.NewFlagSet("takedown", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	code := fs.String("code", "", "")
	reason := fs.String("reason", "", "")
	actor := fs.String("actor", "", "")
	if err := fs.Parse(args); err != nil {
		return err
	}
	actorID := strings.TrimSpace(*actor)
	if actorID == "" {
		if value, ok := rt.lookupEnv("MOLLA_ADMIN_ACTOR"); ok {
			actorID = strings.TrimSpace(value)
		}
	}
	if actorID == "" {
		return errors.New("actor required via --actor or MOLLA_ADMIN_ACTOR")
	}
	if strings.TrimSpace(*code) == "" || strings.TrimSpace(*reason) == "" {
		return errors.New("takedown requires --code and --reason")
	}
	principal := platform.Principal{ActorID: actorID, Role: platform.RoleOperator}
	if err := rt.delete(context.Background(), principal, strings.TrimSpace(*code), strings.TrimSpace(*reason)); err != nil {
		return err
	}
	_, err := fmt.Fprintf(rt.stdout, "takedown %s ok\n", strings.TrimSpace(*code))
	return err
}

func main() {
	if err := run(os.Args[1:], newRuntime()); err != nil {
		log.Fatal(err)
	}
}
