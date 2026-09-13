package dynamodb

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/SumonMSelim/molla/internal/platform"
)

// StatsStore is the DynamoDB implementation of platform.StatsStore.
type StatsStore struct {
	client clientAPI
	table  string
}

func NewStatsStore(client clientAPI) *StatsStore {
	return &StatsStore{client: client, table: statsTable()}
}

func (s *StatsStore) Get(ctx context.Context, code string) (platform.Stats, error) {
	out, err := s.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName:      aws.String(s.table),
		Key:            map[string]types.AttributeValue{attrShortCode: avS(code)},
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return platform.Stats{}, mapAWSError(err)
	}
	if len(out.Item) == 0 {
		return platform.Stats{}, platform.ErrNotFound
	}
	return platform.Stats{
		ShortCode:   stringAttr(out.Item, attrShortCode),
		Clicks:      intAttr(out.Item, attrClicks),
		LastClickAt: timeAttr(out.Item, attrLastClickAt),
	}, nil
}

func (s *StatsStore) Increment(ctx context.Context, code string, delta int64, lastClickAt time.Time) error {
	if code == "" || delta == 0 {
		return nil
	}
	_, err := s.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:        aws.String(s.table),
		Key:              map[string]types.AttributeValue{attrShortCode: avS(code)},
		UpdateExpression: aws.String("ADD " + attrClicks + " :n SET " + attrLastClickAt + " = :t"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":n": avN(delta),
			":t": avN(unix(lastClickAt)),
		},
	})
	return mapAWSError(err)
}

var _ platform.StatsStore = (*StatsStore)(nil)
