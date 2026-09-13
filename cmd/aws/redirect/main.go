// Command redirect is the AWS Lambda entrypoint for GET /{short_code}. It
// wires adapters into the redirect handler from internal/handlers and holds
// no logic itself. It is its own function so the hot path gets provisioned
// concurrency and a read-only role.
package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	awskinesis "github.com/aws/aws-sdk-go-v2/service/kinesis"
	"github.com/awslabs/aws-lambda-go-api-proxy/httpadapter"
	goredis "github.com/redis/go-redis/v9"

	awsadapter "github.com/SumonMSelim/molla/internal/adapters/aws"
	ddb "github.com/SumonMSelim/molla/internal/adapters/aws/dynamodb"
	"github.com/SumonMSelim/molla/internal/adapters/aws/kinesis"
	"github.com/SumonMSelim/molla/internal/adapters/logging"
	redisadapter "github.com/SumonMSelim/molla/internal/adapters/redis"
	"github.com/SumonMSelim/molla/internal/handlers"
)

type liveClock struct{}

func (liveClock) Now() time.Time { return time.Now().UTC() }

func newHandler() (http.Handler, error) {
	key := os.Getenv("MOLLA_PRIVACY_KEY")
	if key == "" {
		return nil, errors.New("MOLLA_PRIVACY_KEY required")
	}
	addr := os.Getenv("MOLLA_REDIS_ADDR")
	if addr == "" {
		return nil, errors.New("MOLLA_REDIS_ADDR required")
	}
	stream := os.Getenv("MOLLA_CLICK_STREAM")
	if stream == "" {
		return nil, errors.New("MOLLA_CLICK_STREAM required")
	}
	cfg, err := awsadapter.RuntimeConfig(context.Background())
	if err != nil {
		return nil, err
	}
	ddbClient := dynamodb.NewFromConfig(cfg, func(o *dynamodb.Options) { o.Retryer = awsadapter.Retryer() })
	kinesisClient := awskinesis.NewFromConfig(cfg, func(o *awskinesis.Options) { o.Retryer = awsadapter.Retryer() })
	cache := redisadapter.NewCache(goredis.NewClient(redisadapter.OptionsFromEnv(addr)))
	return handlers.NewRedirect(handlers.RedirectDeps{
		Store:      ddb.NewLinkStore(ddbClient),
		Cache:      cache,
		Publisher:  kinesis.NewPublisher(kinesisClient, stream),
		Clock:      liveClock{},
		PrivacyKey: []byte(key),
	}), nil
}

func runRedirect(start func(any)) error {
	handler, err := newHandler()
	if err != nil {
		return err
	}
	start(httpadapter.NewV2(withRequestLogger(handler)).ProxyWithContext)
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
	if err := runRedirect(lambda.Start); err != nil {
		log.Fatal(err)
	}
}
