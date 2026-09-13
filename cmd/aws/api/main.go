// Command api is the AWS Lambda entrypoint for the JSON API (create, stats,
// delete) behind API Gateway. It wires adapters into the mux from
// internal/handlers and holds no logic itself.
package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-lambda-go/lambdacontext"
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
	ddbClient := dynamodb.NewFromConfig(cfg, func(o *dynamodb.Options) { o.Retryer = awsadapter.Retryer() })
	lambdaClient := awslambda.NewFromConfig(cfg, func(o *awslambda.Options) { o.Retryer = awsadapter.Retryer() })
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
	proxy := httpadapter.New(withRequestLogger(handler)).ProxyWithContext
	start(proxy)
	return nil
}

// withRequestLogger attaches the invocation logger, tagged with the Lambda
// request ID, so every log line from one invocation correlates.
func withRequestLogger(next http.Handler) http.Handler {
	logger := logging.NewLogger()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(logging.WithLogger(r.Context(), requestLogger(r.Context(), logger))))
	})
}

func requestLogger(ctx context.Context, logger *slog.Logger) *slog.Logger {
	if lc, ok := lambdacontext.FromContext(ctx); ok {
		return logger.With("request_id", lc.AwsRequestID)
	}
	return logger
}

func main() {
	if err := runAPI(lambda.Start); err != nil {
		log.Fatal(err)
	}
}
