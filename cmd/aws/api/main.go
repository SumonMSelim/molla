// Command api is the AWS Lambda entrypoint for the JSON API (create, stats,
// delete) behind API Gateway. It wires adapters into the mux from
// internal/handlers and holds no logic itself.
package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/awslabs/aws-lambda-go-api-proxy/httpadapter"

	"github.com/SumonMSelim/molla/internal/adapters/memory"
	"github.com/SumonMSelim/molla/internal/core"
	"github.com/SumonMSelim/molla/internal/handlers"
)

type liveClock struct{}

func (liveClock) Now() time.Time { return time.Now().UTC() }

func newHandler() (http.Handler, error) {
	permuter, err := core.NewPermuter([]byte(os.Getenv("MOLLA_PERMUTATION_KEY")))
	if err != nil {
		return nil, err
	}
	cache := memory.NewCache()
	return handlers.New(handlers.Deps{
		Store:       memory.NewLinkStore(),
		Allocator:   memory.NewIDAllocator(0),
		Clock:       liveClock{},
		Credentials: memory.NewCredentialStore(),
		Permuter:    permuter,
		PublicBase:  os.Getenv("MOLLA_PUBLIC_BASE"),
		Stats:       memory.NewStatsStore(),
		Invalidator: memory.NewCacheInvalidator(cache),
		Audit:       &memory.AuditSink{},
	}), nil
}

func runAPI(start func(any)) error {
	handler, err := newHandler()
	if err != nil {
		return err
	}
	start(httpadapter.New(handler).ProxyWithContext)
	return nil
}

func main() {
	if err := runAPI(lambda.Start); err != nil {
		log.Fatal(err)
	}
}
