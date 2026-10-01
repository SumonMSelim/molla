package dynamodb

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/SumonMSelim/molla/internal/platform"
)

// Click item attribute names. The aggregate Lambda reads these back from the
// table's stream NEW_IMAGE, so they are exported to keep both sides in sync.
const (
	AttrClickEventID      = "event_id"
	AttrClickShortCode    = "short_code"
	AttrClickOccurredAt   = "occurred_at"
	AttrClickSourceIPHash = "source_ip_hash"
	AttrClickTTL          = "ttl"

	// ClickRetention is how long a raw click item stays in the table. TTL
	// expiry is best-effort, so it is a retention window, not a guarantee.
	ClickRetention = 90 * 24 * time.Hour
)

// ClickStore is the DynamoDB EventPublisher: one PutItem per click into the
// Clicks table. DynamoDB Streams on that table feed the aggregate Lambda, and
// the item's TTL doubles as the raw-event retention window. Reached through the
// free gateway VPC endpoint, so the redirect Lambda needs no interface endpoint.
type ClickStore struct {
	client clientAPI
	table  string
}

func NewClickStore(client clientAPI) *ClickStore {
	return &ClickStore{client: client, table: clicksTable()}
}

func (c *ClickStore) Publish(ctx context.Context, event platform.ClickEvent) error {
	occurred := event.OccurredAt.UTC()
	_, err := c.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(c.table),
		Item: map[string]types.AttributeValue{
			AttrClickEventID:      avS(event.EventID),
			AttrClickShortCode:    avS(event.ShortCode),
			AttrClickOccurredAt:   avN(unix(occurred)),
			AttrClickSourceIPHash: avS(event.SourceIPHash),
			AttrClickTTL:          avN(unix(occurred.Add(ClickRetention))),
		},
	})
	return mapAWSError(err)
}

var _ platform.EventPublisher = (*ClickStore)(nil)
