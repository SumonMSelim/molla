package kinesis

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awskinesis "github.com/aws/aws-sdk-go-v2/service/kinesis"

	"github.com/SumonMSelim/molla/internal/platform"
)

type wireEvent struct {
	EventID      string    `json:"event_id"`
	ShortCode    string    `json:"short_code"`
	OccurredAt   time.Time `json:"occurred_at"`
	SourceIPHash string    `json:"source_ip_hash"`
}

type putter interface {
	PutRecord(context.Context, *awskinesis.PutRecordInput, ...func(*awskinesis.Options)) (*awskinesis.PutRecordOutput, error)
}

// Publisher is the Kinesis EventPublisher.
type Publisher struct {
	client putter
	stream string
}

func NewPublisher(client putter, stream string) *Publisher {
	return &Publisher{client: client, stream: stream}
}

func PartitionKey(event platform.ClickEvent) string {
	sum := sha256.Sum256([]byte(event.EventID))
	bucket := sum[0] & 0x0f
	return fmt.Sprintf("%s#%d", event.ShortCode, bucket)
}

func (p *Publisher) Publish(ctx context.Context, event platform.ClickEvent) error {
	body, err := json.Marshal(wireEvent{
		EventID:      event.EventID,
		ShortCode:    event.ShortCode,
		OccurredAt:   event.OccurredAt.UTC(),
		SourceIPHash: event.SourceIPHash,
	})
	if err != nil {
		return err
	}
	_, err = p.client.PutRecord(ctx, &awskinesis.PutRecordInput{
		StreamName:   aws.String(p.stream),
		PartitionKey: aws.String(PartitionKey(event)),
		Data:         body,
	})
	return err
}

var _ platform.EventPublisher = (*Publisher)(nil)
