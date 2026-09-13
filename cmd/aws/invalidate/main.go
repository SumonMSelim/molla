// Command invalidate is the VPC Lambda that writes a versioned Redis tombstone.
package main

import (
	"context"
	"errors"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	goredis "github.com/redis/go-redis/v9"

	awslambda "github.com/SumonMSelim/molla/internal/adapters/aws/lambda"
	redisadapter "github.com/SumonMSelim/molla/internal/adapters/redis"
)

func handle(ctx context.Context, req awslambda.InvalidateRequest) error {
	return handleWith(ctx, req, os.Getenv("MOLLA_REDIS_ADDR"))
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
