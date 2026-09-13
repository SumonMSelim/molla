// Command redirect is the AWS Lambda entrypoint for GET /{short_code}. It
// wires adapters into the redirect handler from internal/handlers and holds
// no logic itself. It is its own function so the hot path gets provisioned
// concurrency and a read-only role.
package main

import (
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/awslabs/aws-lambda-go-api-proxy/httpadapter"

	"github.com/SumonMSelim/molla/internal/adapters/memory"
	"github.com/SumonMSelim/molla/internal/handlers"
)

type liveClock struct{}

func (liveClock) Now() time.Time { return time.Now().UTC() }

func newHandler() (http.Handler, error) {
	key := os.Getenv("MOLLA_PRIVACY_KEY")
	if key == "" {
		return nil, errors.New("MOLLA_PRIVACY_KEY required")
	}
	return handlers.NewRedirect(handlers.RedirectDeps{
		Store:      memory.NewLinkStore(),
		Cache:      memory.NewCache(),
		Publisher:  &memory.EventPublisher{},
		Clock:      liveClock{},
		PrivacyKey: []byte(key),
	}), nil
}

func runRedirect(start func(any)) error {
	handler, err := newHandler()
	if err != nil {
		return err
	}
	start(httpadapter.NewV2(handler).ProxyWithContext)
	return nil
}

func main() {
	if err := runRedirect(lambda.Start); err != nil {
		log.Fatal(err)
	}
}
