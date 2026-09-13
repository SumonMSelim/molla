package aws

import (
	"context"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
)

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
