package aws

import (
	"context"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
)

// RuntimeConfig uses MOLLA_AWS_ENDPOINT for tests/local fakes; otherwise the default SDK chain.
func RuntimeConfig(ctx context.Context) (aws.Config, error) {
	if ep := os.Getenv("MOLLA_AWS_ENDPOINT"); ep != "" {
		return StaticConfig(os.Getenv("AWS_REGION"), ep, nil), nil
	}
	return Load(ctx)
}

// Load returns the default AWS SDK config for production runtimes.
func Load(ctx context.Context, optFns ...func(*config.LoadOptions) error) (aws.Config, error) {
	return config.LoadDefaultConfig(ctx, optFns...)
}

// StaticConfig builds a config that never reads shared credentials files or IMDS.
// Tests point BaseEndpoint at httptest.Server.
func StaticConfig(region, endpoint string, client aws.HTTPClient) aws.Config {
	if region == "" {
		region = "us-east-1"
	}
	if client == nil {
		client = http.DefaultClient
	}
	return aws.Config{
		Region:           region,
		Credentials:      credentials.NewStaticCredentialsProvider("test", "test", ""),
		BaseEndpoint:     aws.String(endpoint),
		HTTPClient:       client,
		RetryMaxAttempts: 1,
	}
}

// Retryer is the retry policy every AWS client in this repo uses: bounded
// attempts with the SDK's default exponential backoff and jitter, so a single
// throttle or transient network blip does not surface as a request error.
func Retryer() aws.Retryer {
	return retry.NewStandard(func(o *retry.StandardOptions) { o.MaxAttempts = 3 })
}
