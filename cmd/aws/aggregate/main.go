// Command aggregate is the DynamoDB Streams Lambda that folds click items
// from the Clicks table into StatsStore increments.
package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"strconv"
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

var errMalformed = errors.New("malformed click item")

// clickFromImage decodes a stream NEW_IMAGE written by dynamodb.ClickStore.
func clickFromImage(image map[string]events.DynamoDBAttributeValue) (platform.ClickEvent, error) {
	code, ok := image[ddb.AttrClickShortCode]
	if !ok || code.DataType() != events.DataTypeString || code.String() == "" {
		return platform.ClickEvent{}, errMalformed
	}
	at, ok := image[ddb.AttrClickOccurredAt]
	if !ok || at.DataType() != events.DataTypeNumber {
		return platform.ClickEvent{}, errMalformed
	}
	unix, err := strconv.ParseInt(at.Number(), 10, 64)
	if err != nil {
		return platform.ClickEvent{}, errMalformed
	}
	event := platform.ClickEvent{ShortCode: code.String(), OccurredAt: time.Unix(unix, 0).UTC()}
	if id, ok := image[ddb.AttrClickEventID]; ok && id.DataType() == events.DataTypeString {
		event.EventID = id.String()
	}
	if h, ok := image[ddb.AttrClickSourceIPHash]; ok && h.DataType() == events.DataTypeString {
		event.SourceIPHash = h.String()
	}
	return event, nil
}

// handleEvent counts INSERT records only. MODIFY never happens for click items
// and REMOVE is the TTL sweep deleting old raw events, which must not count.
func handleEvent(ctx context.Context, event events.DynamoDBEvent, store platform.StatsStore) (events.DynamoDBEventResponse, error) {
	log := logging.FromContext(ctx)
	groups := map[string][]platform.ClickEvent{}
	records := map[string][]events.DynamoDBEventRecord{}
	var resp events.DynamoDBEventResponse
	for _, rec := range event.Records {
		if rec.EventName != string(events.DynamoDBOperationTypeInsert) {
			continue
		}
		click, err := clickFromImage(rec.Change.NewImage)
		if err != nil {
			log.Error("malformed click record", "sequence_number", rec.Change.SequenceNumber, "error", err)
			resp.BatchItemFailures = append(resp.BatchItemFailures, events.DynamoDBBatchItemFailure{ItemIdentifier: rec.Change.SequenceNumber})
			continue
		}
		groups[click.ShortCode] = append(groups[click.ShortCode], click)
		records[click.ShortCode] = append(records[click.ShortCode], rec)
	}
	for code, batch := range groups {
		if err := core.AggregateClicks(ctx, store, batch); err != nil {
			log.Error("click aggregation failed", "short_code", code, "records", len(batch), "error", err)
			for _, rec := range records[code] {
				resp.BatchItemFailures = append(resp.BatchItemFailures, events.DynamoDBBatchItemFailure{ItemIdentifier: rec.Change.SequenceNumber})
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
	lambda.Start(func(ctx context.Context, event events.DynamoDBEvent) (events.DynamoDBEventResponse, error) {
		return handleEvent(logging.WithLogger(ctx, requestLogger(ctx, logger)), event, store)
	})
}
