package kinesis

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	awskinesis "github.com/aws/aws-sdk-go-v2/service/kinesis"

	awsadapter "github.com/SumonMSelim/molla/internal/adapters/aws"
	"github.com/SumonMSelim/molla/internal/platform"
)

func TestPublisherPartitionKeysSpreadAcross16Buckets(t *testing.T) {
	var keys []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			PartitionKey string
			Data         []byte
			StreamName   string
		}
		_ = json.Unmarshal(body, &req)
		keys = append(keys, req.PartitionKey)
		var ev struct {
			ShortCode string `json:"short_code"`
		}
		_ = json.Unmarshal(req.Data, &ev)
		if ev.ShortCode != "abc1234" {
			http.Error(w, "missing short_code", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/x-amz-json-1.1")
		_, _ = w.Write([]byte(`{"ShardId":"shardId-000","SequenceNumber":"1"}`))
	}))
	t.Cleanup(srv.Close)
	client := awskinesis.NewFromConfig(awsadapter.StaticConfig("us-east-1", srv.URL, srv.Client()), func(o *awskinesis.Options) {
		o.Retryer = awssdk.NopRetryer{}
	})
	pub := NewPublisher(client, "clicks")
	seen := map[string]bool{}
	for i := 0; i < 64; i++ {
		event := platform.ClickEvent{
			EventID:    time.Unix(int64(i), 0).Format(time.RFC3339Nano) + string(rune('a'+i%26)),
			ShortCode:  "abc1234",
			OccurredAt: time.Unix(1_700_000_000, 0).UTC(),
		}
		if err := pub.Publish(context.Background(), event); err != nil {
			t.Fatal(err)
		}
		if PartitionKey(event)[:8] != "abc1234#" {
			t.Fatalf("partition key = %q", PartitionKey(event))
		}
		seen[PartitionKey(event)] = true
	}
	if len(seen) < 2 {
		t.Fatalf("expected multiple buckets, got %v", seen)
	}
	if len(keys) != 64 {
		t.Fatalf("put count = %d", len(keys))
	}
}
