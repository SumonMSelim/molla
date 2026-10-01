package dynamodb

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"

	awsadapter "github.com/SumonMSelim/molla/internal/adapters/aws"
	"github.com/SumonMSelim/molla/internal/adapters/aws/ddbfake"
	"github.com/SumonMSelim/molla/internal/platform"
)

func TestClickStorePublishWritesItemWithRetentionTTL(t *testing.T) {
	fake := ddbfake.New()
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)
	t.Setenv("MOLLA_CLICKS_TABLE", "clicks-test")
	client := dynamodb.NewFromConfig(awsadapter.StaticConfig("us-east-1", srv.URL, srv.Client()))
	store := NewClickStore(client)

	at := time.Unix(1_700_000_000, 0).UTC()
	err := store.Publish(context.Background(), platform.ClickEvent{
		EventID: "evt-1", ShortCode: "abc1234", OccurredAt: at, SourceIPHash: "deadbeef",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(fake.Requests) != 1 || fake.Targets[0] != "DynamoDB_20120810.PutItem" {
		t.Fatalf("requests = %d targets = %v", len(fake.Requests), fake.Targets)
	}
	var req struct {
		TableName string
		Item      map[string]map[string]string
	}
	if err := json.Unmarshal(fake.Requests[0], &req); err != nil {
		t.Fatal(err)
	}
	if req.TableName != "clicks-test" {
		t.Fatalf("table = %q", req.TableName)
	}
	if req.Item[AttrClickEventID]["S"] != "evt-1" || req.Item[AttrClickShortCode]["S"] != "abc1234" || req.Item[AttrClickSourceIPHash]["S"] != "deadbeef" {
		t.Fatalf("item = %+v", req.Item)
	}
	if req.Item[AttrClickOccurredAt]["N"] != strconv.FormatInt(at.Unix(), 10) {
		t.Fatalf("occurred_at = %v", req.Item[AttrClickOccurredAt])
	}
	if req.Item[AttrClickTTL]["N"] != strconv.FormatInt(at.Add(ClickRetention).Unix(), 10) {
		t.Fatalf("ttl = %v", req.Item[AttrClickTTL])
	}
}

func TestClickStorePublishMapsDependencyError(t *testing.T) {
	srv := httptest.NewServer(ddbfake.New())
	srv.Close()
	client := dynamodb.NewFromConfig(awsadapter.StaticConfig("us-east-1", srv.URL, srv.Client()))
	err := NewClickStore(client).Publish(context.Background(), platform.ClickEvent{EventID: "x", ShortCode: "abc1234"})
	if err == nil {
		t.Fatal("expected error")
	}
}
