// Command aggregate is the Kinesis Lambda that folds click events into StatsStore increments.
package main

import (
	"context"
	"encoding/json"
	"log"
	"log/slog"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"

	awsadapter "github.com/SumonMSelim/molla/internal/adapters/aws"
	ddb "github.com/SumonMSelim/molla/internal/adapters/aws/dynamodb"
	"github.com/SumonMSelim/molla/internal/adapters/logging"
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
	log := logging.FromContext(ctx)
	groups := map[string][]platform.ClickEvent{}
	records := map[string][]events.KinesisEventRecord{}
	var resp events.KinesisEventResponse
	for _, rec := range event.Records {
		var wire wireEvent
		if err := json.Unmarshal(rec.Kinesis.Data, &wire); err != nil || wire.ShortCode == "" {
			log.Error("malformed click record", "sequence_number", rec.Kinesis.SequenceNumber, "error", err)
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
			log.Error("click aggregation failed", "short_code", code, "records", len(batch), "error", err)
			for _, rec := range records[code] {
				resp.BatchItemFailures = append(resp.BatchItemFailures, events.KinesisBatchItemFailure{ItemIdentifier: rec.Kinesis.SequenceNumber})
			}
		}
	}
	return resp, nil
}

func requestLogger(ctx context.Context, logger *slog.Logger) *slog.Logger {
	if lc, ok := lambdacontext.FromContext(ctx); ok {
		return logger.With("request_id", lc.AwsRequestID)
	}
	return logger
}

func newStatsStore() (platform.StatsStore, error) {
	cfg, err := awsadapter.RuntimeConfig(context.Background())
	if err != nil {
		return nil, err
	}
	client := dynamodb.NewFromConfig(cfg, func(o *dynamodb.Options) { o.Retryer = awsadapter.Retryer() })
	return ddb.NewStatsStore(client), nil
}

func main() {
	store, err := newStatsStore()
	if err != nil {
		log.Fatal(err)
	}
	logger := logging.NewLogger()
	lambda.Start(func(ctx context.Context, event events.KinesisEvent) (events.KinesisEventResponse, error) {
		return handleEvent(logging.WithLogger(ctx, requestLogger(ctx, logger)), event, store)
	})
}
