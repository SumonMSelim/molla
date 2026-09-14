package dynamodb

import (
	"context"
	"errors"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/SumonMSelim/molla/internal/platform"
)

// LinkStore is the DynamoDB implementation of platform.LinkStore.
type LinkStore struct {
	client      clientAPI
	links       string
	idempotency string
}

func NewLinkStore(client clientAPI) *LinkStore {
	return &LinkStore{client: client, links: linksTable(), idempotency: idempotencyTable()}
}

func (s *LinkStore) Get(ctx context.Context, code string) (platform.Link, error) {
	out, err := s.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName:      aws.String(s.links),
		Key:            map[string]types.AttributeValue{attrShortCode: avS(code)},
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return platform.Link{}, mapAWSError(err)
	}
	if len(out.Item) == 0 {
		return platform.Link{}, platform.ErrNotFound
	}
	return linkFromItem(out.Item), nil
}

func (s *LinkStore) Create(ctx context.Context, link platform.Link, idem *platform.Idempotency) (platform.Link, error) {
	link.Version = 1
	link.IsActive = true
	link.DeletedAt = nil
	link.DeletedBy = ""
	link.DeleteRole = ""
	link.DeleteReason = ""
	link.PurgeAt = link.ExpiresAt
	items := []types.TransactWriteItem{
		{Put: &types.Put{
			TableName:           aws.String(s.links),
			Item:                itemFromLink(link),
			ConditionExpression: aws.String("attribute_not_exists(" + attrShortCode + ")"),
		}},
	}
	if idem != nil {
		items = append(items, types.TransactWriteItem{Put: &types.Put{
			TableName: aws.String(s.idempotency),
			Item: map[string]types.AttributeValue{
				attrOwnerKey:    avS(link.OwnerID + "#" + idem.Key),
				attrShortCode:   avS(link.ShortCode),
				attrRequestHash: avS(idem.RequestHash),
				attrTTL:         avN(unix(idem.ExpiresAt)),
			},
			ConditionExpression: aws.String("attribute_not_exists(" + attrOwnerKey + ")"),
		}})
	}
	_, err := s.client.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{TransactItems: items})
	if err == nil {
		return link, nil
	}
	var canceled *types.TransactionCanceledException
	if !errors.As(err, &canceled) {
		return platform.Link{}, mapAWSError(err)
	}
	return s.mapCancel(ctx, link, idem, canceled.CancellationReasons)
}

func (s *LinkStore) mapCancel(ctx context.Context, link platform.Link, idem *platform.Idempotency, reasons []types.CancellationReason) (platform.Link, error) {
	if idem != nil {
		replayed, err := s.replayIdempotency(ctx, link.OwnerID, idem)
		if err == nil {
			return replayed, nil
		}
		if errors.Is(err, platform.ErrIdempotencyConflict) {
			return platform.Link{}, err
		}
	}
	linkFailed := len(reasons) > 0 && aws.ToString(reasons[0].Code) == "ConditionalCheckFailed"
	if linkFailed {
		return platform.Link{}, platform.ErrCollision
	}
	return platform.Link{}, platform.ErrDependency
}

func (s *LinkStore) replayIdempotency(ctx context.Context, ownerID string, idem *platform.Idempotency) (platform.Link, error) {
	out, err := s.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName:      aws.String(s.idempotency),
		Key:            map[string]types.AttributeValue{attrOwnerKey: avS(ownerID + "#" + idem.Key)},
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return platform.Link{}, mapAWSError(err)
	}
	if len(out.Item) == 0 {
		return platform.Link{}, platform.ErrCollision
	}
	if stringAttr(out.Item, attrRequestHash) != idem.RequestHash {
		return platform.Link{}, platform.ErrIdempotencyConflict
	}
	return s.Get(ctx, stringAttr(out.Item, attrShortCode))
}

func (s *LinkStore) SoftDelete(ctx context.Context, principal platform.Principal, code string, now time.Time, reason string) (platform.Deletion, error) {
	link, err := s.Get(ctx, code)
	if err != nil {
		return platform.Deletion{}, err
	}
	if !authorized(principal, link, reason) {
		return platform.Deletion{}, platform.ErrForbidden
	}
	if !link.IsActive {
		if link.DeletedBy != principal.ActorID || link.DeleteRole != principal.Role {
			return platform.Deletion{}, platform.ErrForbidden
		}
		return deletionFrom(link), nil
	}
	now = now.UTC().Truncate(time.Second)
	purge := now.Add(platform.DeleteRetention)
	_, err = s.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(s.links),
		Key:       map[string]types.AttributeValue{attrShortCode: avS(code)},
		UpdateExpression: aws.String(
			"SET #active = :false, #ver = :ver, " +
				"#deleted_at = :now, #purge_at = :purge, " +
				"#deleted_by = :actor, #delete_role = :role, " +
				"#delete_reason = :reason",
		),
		ConditionExpression: aws.String("#active = :true AND #ver = :old"),
		ExpressionAttributeNames: map[string]string{
			"#active":        attrIsActive,
			"#ver":           attrVersion,
			"#deleted_at":    attrDeletedAt,
			"#purge_at":      attrPurgeAt,
			"#deleted_by":    attrDeletedBy,
			"#delete_role":   attrDeleteRole,
			"#delete_reason": attrDeleteReason,
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":false":  avB(false),
			":true":   avB(true),
			":ver":    avN(link.Version + 1),
			":old":    avN(link.Version),
			":now":    avN(unix(now)),
			":purge":  avN(unix(purge)),
			":actor":  avS(principal.ActorID),
			":role":   avS(string(principal.Role)),
			":reason": avS(reason),
		},
	})
	if err != nil {
		return platform.Deletion{}, mapAWSError(err)
	}
	link.IsActive = false
	link.Version++
	link.DeletedAt = &now
	link.PurgeAt = purge
	link.DeletedBy = principal.ActorID
	link.DeleteRole = principal.Role
	link.DeleteReason = reason
	return deletionFrom(link), nil
}

func authorized(principal platform.Principal, link platform.Link, reason string) bool {
	if principal.ActorID == "" {
		return false
	}
	switch principal.Role {
	case platform.RoleDeveloper:
		return principal.OwnerID != "" && principal.OwnerID == link.OwnerID
	case platform.RoleOperator:
		return reason != ""
	default:
		return false
	}
}

func deletionFrom(link platform.Link) platform.Deletion {
	return platform.Deletion{
		ShortCode: link.ShortCode,
		OwnerID:   link.OwnerID,
		Version:   link.Version,
		DeletedAt: *link.DeletedAt,
		PurgeAt:   link.PurgeAt,
	}
}

var _ platform.LinkStore = (*LinkStore)(nil)
