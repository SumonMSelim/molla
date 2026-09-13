// Command api is the AWS Lambda entrypoint for the JSON API (create, stats,
// delete) behind API Gateway. It wires adapters into the mux from
// internal/handlers and holds no logic itself.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	awslambda "github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/awslabs/aws-lambda-go-api-proxy/httpadapter"

	awsadapter "github.com/SumonMSelim/molla/internal/adapters/aws"
	ddb "github.com/SumonMSelim/molla/internal/adapters/aws/dynamodb"
	lam "github.com/SumonMSelim/molla/internal/adapters/aws/lambda"
	"github.com/SumonMSelim/molla/internal/adapters/logging"
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
	cfg, err := awsadapter.RuntimeConfig(context.Background())
	if err != nil {
		return nil, err
	}
	ddbClient := dynamodb.NewFromConfig(cfg, func(o *dynamodb.Options) { o.Retryer = awssdk.NopRetryer{} })
	lambdaClient := awslambda.NewFromConfig(cfg, func(o *awslambda.Options) { o.Retryer = awssdk.NopRetryer{} })
	function := os.Getenv("MOLLA_INVALIDATE_FUNCTION")
	if function == "" {
		function = "molla-invalidate"
	}
	return handlers.New(handlers.Deps{
		Store:       ddb.NewLinkStore(ddbClient),
		Allocator:   ddb.NewIDAllocator(ddbClient, os.Getenv("AWS_REGION"), 0),
		Clock:       liveClock{},
		Credentials: ddb.NewIdentityStore(ddbClient),
		Permuter:    permuter,
		PublicBase:  os.Getenv("MOLLA_PUBLIC_BASE"),
		Stats:       ddb.NewStatsStore(ddbClient),
		Invalidator: lam.NewInvalidator(lambdaClient, function),
		Audit:       logging.Sink{W: os.Stderr},
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
