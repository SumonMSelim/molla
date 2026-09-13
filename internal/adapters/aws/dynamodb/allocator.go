package dynamodb

import (
	"context"
	"strconv"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/SumonMSelim/molla/internal/platform"
)

// IDAllocator leases monotonic IDs in blocks from the Counters table.
type IDAllocator struct {
	client    clientAPI
	table     string
	region    string
	regionID  int64
	blockSize int64
	mu        sync.Mutex
	next      int64
	end       int64
}

func NewIDAllocator(client clientAPI, region string, blockSize int64) *IDAllocator {
	if region == "" {
		region = defaultRegion
	}
	if blockSize <= 0 {
		blockSize = defaultBlockSize
	}
	return &IDAllocator{client: client, table: tableCounters, region: region, blockSize: blockSize}
}

func (a *IDAllocator) Lease(ctx context.Context) (int64, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.next >= a.end {
		end, err := a.leaseBlock(ctx)
		if err != nil {
			return 0, err
		}
		a.end = end
		a.next = end - a.blockSize
	}
	local := a.next
	a.next++
	return a.regionID<<38 | local, nil
}

func (a *IDAllocator) leaseBlock(ctx context.Context) (int64, error) {
	out, err := a.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:        aws.String(a.table),
		Key:              map[string]types.AttributeValue{attrRegion: avS(a.region)},
		UpdateExpression: aws.String("ADD " + attrCounter + " :n"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":n": avN(a.blockSize),
		},
		ReturnValues: types.ReturnValueAllNew,
	})
	if err != nil {
		return 0, mapAWSError(err)
	}
	end, err := strconv.ParseInt(out.Attributes[attrCounter].(*types.AttributeValueMemberN).Value, 10, 64)
	if err != nil {
		return 0, platform.ErrDependency
	}
	return end, nil
}

var _ platform.IDAllocator = (*IDAllocator)(nil)
