package dynamodb

import (
	"context"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/SumonMSelim/molla/internal/platform"
)

const (
	tableLinks       = "Links"
	tableStats       = "LinkStats"
	tableCounters    = "Counters"
	tableIdempotency = "Idempotency"
	tableCredentials = "Credentials"
	defaultRegion    = "us-east-1"
	defaultBlockSize = int64(1000)
	attrShortCode    = "short_code"
	attrLongURL      = "long_url"
	attrOwnerID      = "owner_id"
	attrIsCustom     = "is_custom"
	attrIsActive     = "is_active"
	attrVersion      = "version"
	attrCreatedAt    = "created_at"
	attrExpiresAt    = "expires_at"
	attrDeletedAt    = "deleted_at"
	attrPurgeAt      = "purge_at"
	attrDeletedBy    = "deleted_by"
	attrDeleteRole   = "delete_role"
	attrDeleteReason = "delete_reason"
	attrOwnerKey     = "owner_key"
	attrRequestHash  = "request_hash"
	attrTTL          = "ttl"
	attrRegion       = "region"
	attrCounter      = "counter"
	attrClicks       = "clicks"
	attrLastClickAt  = "last_click_at"
	attrTokenHash    = "token_hash"
	attrActorID      = "actor_id"
	attrStatus       = "status"
)

type clientAPI interface {
	GetItem(context.Context, *dynamodb.GetItemInput, ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
	PutItem(context.Context, *dynamodb.PutItemInput, ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error)
	UpdateItem(context.Context, *dynamodb.UpdateItemInput, ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error)
	TransactWriteItems(context.Context, *dynamodb.TransactWriteItemsInput, ...func(*dynamodb.Options)) (*dynamodb.TransactWriteItemsOutput, error)
}

func avS(v string) types.AttributeValue {
	return &types.AttributeValueMemberS{Value: v}
}

func avN(v int64) types.AttributeValue {
	return &types.AttributeValueMemberN{Value: strconv.FormatInt(v, 10)}
}

func avB(v bool) types.AttributeValue {
	return &types.AttributeValueMemberBOOL{Value: v}
}

func unix(t time.Time) int64 {
	return t.UTC().Unix()
}

func fromUnix(v int64) time.Time {
	return time.Unix(v, 0).UTC()
}

func stringAttr(item map[string]types.AttributeValue, key string) string {
	if av, ok := item[key].(*types.AttributeValueMemberS); ok {
		return av.Value
	}
	return ""
}

func boolAttr(item map[string]types.AttributeValue, key string) bool {
	if av, ok := item[key].(*types.AttributeValueMemberBOOL); ok {
		return av.Value
	}
	return false
}

func intAttr(item map[string]types.AttributeValue, key string) int64 {
	if av, ok := item[key].(*types.AttributeValueMemberN); ok {
		n, _ := strconv.ParseInt(av.Value, 10, 64)
		return n
	}
	return 0
}

func timeAttr(item map[string]types.AttributeValue, key string) time.Time {
	if _, ok := item[key]; !ok {
		return time.Time{}
	}
	return fromUnix(intAttr(item, key))
}

func linkFromItem(item map[string]types.AttributeValue) platform.Link {
	link := platform.Link{
		ShortCode:    stringAttr(item, attrShortCode),
		LongURL:      stringAttr(item, attrLongURL),
		OwnerID:      stringAttr(item, attrOwnerID),
		IsCustom:     boolAttr(item, attrIsCustom),
		IsActive:     boolAttr(item, attrIsActive),
		Version:      intAttr(item, attrVersion),
		CreatedAt:    timeAttr(item, attrCreatedAt),
		ExpiresAt:    timeAttr(item, attrExpiresAt),
		PurgeAt:      timeAttr(item, attrPurgeAt),
		DeletedBy:    stringAttr(item, attrDeletedBy),
		DeleteRole:   platform.Role(stringAttr(item, attrDeleteRole)),
		DeleteReason: stringAttr(item, attrDeleteReason),
	}
	if _, ok := item[attrDeletedAt]; ok {
		t := timeAttr(item, attrDeletedAt)
		link.DeletedAt = &t
	}
	return link
}

func itemFromLink(link platform.Link) map[string]types.AttributeValue {
	item := map[string]types.AttributeValue{
		attrShortCode: avS(link.ShortCode),
		attrLongURL:   avS(link.LongURL),
		attrOwnerID:   avS(link.OwnerID),
		attrIsCustom:  avB(link.IsCustom),
		attrIsActive:  avB(link.IsActive),
		attrVersion:   avN(link.Version),
		attrCreatedAt: avN(unix(link.CreatedAt)),
		attrExpiresAt: avN(unix(link.ExpiresAt)),
		attrPurgeAt:   avN(unix(link.PurgeAt)),
	}
	if link.DeletedAt != nil {
		item[attrDeletedAt] = avN(unix(*link.DeletedAt))
	}
	if link.DeletedBy != "" {
		item[attrDeletedBy] = avS(link.DeletedBy)
	}
	if link.DeleteRole != "" {
		item[attrDeleteRole] = avS(string(link.DeleteRole))
	}
	if link.DeleteReason != "" {
		item[attrDeleteReason] = avS(link.DeleteReason)
	}
	return item
}

func mapAWSError(err error) error {
	if err == nil {
		return nil
	}
	return platform.ErrDependency
}
