// Command aggregate is the Kinesis Lambda that folds click events into StatsStore increments.
package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"

	awsadapter "github.com/SumonMSelim/molla/internal/adapters/aws"
	ddb "github.com/SumonMSelim/molla/internal/adapters/aws/dynamodb"
	"github.com/SumonMSelim/molla/internal/core"
	"github.com/SumonMSelim/molla/internal/platform"
)

type wireEvent struct {
	EventID      string    `json:"event_id"`
	ShortCode    string    `json:"short_code"`
	OccurredAt   time.Time `json:"occurred_at"`
	SourceIPHash string    `json:"source_ip_hash"`
}

func handleEvent(ctx context.Context, event events.KinesisEvent, store platform.StatsStore) (events.KinesisEventResponse, error) {
	groups := map[string][]platform.ClickEvent{}
	records := map[string][]events.KinesisEventRecord{}
	var resp events.KinesisEventResponse
	for _, rec := range event.Records {
		var wire wireEvent
		if err := json.Unmarshal(rec.Kinesis.Data, &wire); err != nil || wire.ShortCode == "" {
			resp.BatchItemFailures = append(resp.BatchItemFailures, events.KinesisBatchItemFailure{ItemIdentifier: rec.Kinesis.SequenceNumber})
			continue
		}
		groups[wire.ShortCode] = append(groups[wire.ShortCode], platform.ClickEvent{
			EventID:      wire.EventID,
			ShortCode:    wire.ShortCode,
			OccurredAt:   wire.OccurredAt,
			SourceIPHash: wire.SourceIPHash,
		})
		records[wire.ShortCode] = append(records[wire.ShortCode], rec)
	}
	for code, batch := range groups {
		if err := core.AggregateClicks(ctx, store, batch); err != nil {
			for _, rec := range records[code] {
				resp.BatchItemFailures = append(resp.BatchItemFailures, events.KinesisBatchItemFailure{ItemIdentifier: rec.Kinesis.SequenceNumber})
			}
		}
	}
	return resp, nil
}

func newStatsStore() (platform.StatsStore, error) {
	endpoint := os.Getenv("MOLLA_AWS_ENDPOINT")
	var cfg awssdk.Config
	if endpoint != "" {
		cfg = awsadapter.StaticConfig(os.Getenv("AWS_REGION"), endpoint, nil)
	} else {
		loaded, err := awsadapter.Load(context.Background())
		if err != nil {
			return nil, err
		}
		cfg = loaded
	}
	client := dynamodb.NewFromConfig(cfg, func(o *dynamodb.Options) { o.Retryer = awssdk.NopRetryer{} })
	return ddb.NewStatsStore(client), nil
}

func main() {
	store, err := newStatsStore()
	if err != nil {
		log.Fatal(err)
	}
	lambda.Start(func(ctx context.Context, event events.KinesisEvent) (events.KinesisEventResponse, error) {
		return handleEvent(ctx, event, store)
	})
}
