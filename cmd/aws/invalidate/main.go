// Command invalidate is the VPC Lambda that writes a versioned Redis tombstone.
package main

import (
	"context"
	"errors"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-lambda-go/lambdacontext"
	goredis "github.com/redis/go-redis/v9"

	awslambda "github.com/SumonMSelim/molla/internal/adapters/aws/lambda"
	"github.com/SumonMSelim/molla/internal/adapters/logging"
	redisadapter "github.com/SumonMSelim/molla/internal/adapters/redis"
)

func handle(ctx context.Context, req awslambda.InvalidateRequest) error {
	log := logging.NewLogger()
	if lc, ok := lambdacontext.FromContext(ctx); ok {
		log = log.With("request_id", lc.AwsRequestID)
	}
	err := handleWith(ctx, req, os.Getenv("MOLLA_REDIS_ADDR"))
	if err != nil {
		log.Error("invalidate failed", "short_code", req.ShortCode, "version", req.Version, "error", err)
	}
	return err
}

func handleWith(ctx context.Context, req awslambda.InvalidateRequest, redisAddr string) error {
	if redisAddr == "" {
		return errors.New("MOLLA_REDIS_ADDR required")
	}
	client := goredis.NewClient(redisadapter.OptionsFromEnv(redisAddr))
	defer client.Close()
	return awslambda.Apply(ctx, redisadapter.NewCache(client), req)
}

func main() {
	lambda.Start(handle)
}
