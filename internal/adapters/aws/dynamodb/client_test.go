package dynamodb

import (
	"net/http/httptest"
	"testing"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"

	awsadapter "github.com/SumonMSelim/molla/internal/adapters/aws"
	"github.com/SumonMSelim/molla/internal/adapters/aws/ddbfake"
)

func testClient(t *testing.T) (*dynamodb.Client, *ddbfake.Server) {
	t.Helper()
	fake := ddbfake.New()
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)
	cfg := awsadapter.StaticConfig("us-east-1", srv.URL, srv.Client())
	client := dynamodb.NewFromConfig(cfg, func(o *dynamodb.Options) {
		o.Retryer = awssdk.NopRetryer{}
	})
	return client, fake
}
