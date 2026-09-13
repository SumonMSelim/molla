package dynamodb

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/SumonMSelim/molla/internal/platform"
)

// IdentityStore is the DynamoDB CredentialStore. Tokens are hashed before any request.
type IdentityStore struct {
	client clientAPI
	table  string
}

func NewIdentityStore(client clientAPI) *IdentityStore {
	return &IdentityStore{client: client, table: tableCredentials}
}

func (s *IdentityStore) Store(ctx context.Context, token string, cred platform.Credential) error {
	if token == "" || cred.ActorID == "" || cred.OwnerID == "" {
		return platform.ErrInvalidCredential
	}
	if cred.IssuedAt.IsZero() || cred.ExpiresAt.IsZero() {
		return platform.ErrInvalidCredential
	}
	if !cred.ExpiresAt.After(cred.IssuedAt) {
		return platform.ErrInvalidCredential
	}
	if cred.ExpiresAt.After(cred.IssuedAt.Add(platform.MaximumCredentialLifetime)) {
		return platform.ErrInvalidCredential
	}
	status := cred.Status
	if status == "" {
		status = platform.CredentialActive
	}
	if status != platform.CredentialActive && status != platform.CredentialRevoked {
		return platform.ErrInvalidCredential
	}
	hash := platform.HashToken(token)
	_, err := s.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(s.table),
		Item: map[string]types.AttributeValue{
			attrTokenHash: avS(hash),
			attrActorID:   avS(cred.ActorID),
			attrOwnerID:   avS(cred.OwnerID),
			attrStatus:    avS(string(status)),
			attrExpiresAt: avN(unix(cred.ExpiresAt)),
		},
	})
	return mapAWSError(err)
}

func (s *IdentityStore) Resolve(ctx context.Context, token string, now time.Time) (platform.Principal, error) {
	if token == "" {
		return platform.Principal{}, platform.ErrUnauthorized
	}
	hash := platform.HashToken(token)
	out, err := s.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName:      aws.String(s.table),
		Key:            map[string]types.AttributeValue{attrTokenHash: avS(hash)},
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return platform.Principal{}, mapAWSError(err)
	}
	if len(out.Item) == 0 {
		return platform.Principal{}, platform.ErrUnauthorized
	}
	status := platform.CredentialStatus(stringAttr(out.Item, attrStatus))
	expires := timeAttr(out.Item, attrExpiresAt)
	if status != platform.CredentialActive || expires.IsZero() || !now.Before(expires) {
		return platform.Principal{}, platform.ErrUnauthorized
	}
	return platform.Principal{
		ActorID: stringAttr(out.Item, attrActorID),
		Role:    platform.RoleDeveloper,
		OwnerID: stringAttr(out.Item, attrOwnerID),
	}, nil
}

var _ platform.CredentialStore = (*IdentityStore)(nil)
