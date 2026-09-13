package lambda

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awslambda "github.com/aws/aws-sdk-go-v2/service/lambda"

	"github.com/SumonMSelim/molla/internal/platform"
)

// InvalidateRequest is the internal payload sent to the VPC invalidation function.
type InvalidateRequest struct {
	ShortCode string    `json:"short_code"`
	Version   int64     `json:"version"`
	DeletedAt time.Time `json:"deleted_at"`
}

type invoker interface {
	Invoke(context.Context, *awslambda.InvokeInput, ...func(*awslambda.Options)) (*awslambda.InvokeOutput, error)
}

// Invalidator invokes the cache-invalidation Lambda synchronously.
type Invalidator struct {
	client   invoker
	function string
}

func NewInvalidator(client invoker, function string) *Invalidator {
	return &Invalidator{client: client, function: function}
}

func (i *Invalidator) Invalidate(ctx context.Context, deletion platform.Deletion) error {
	payload, err := json.Marshal(InvalidateRequest{
		ShortCode: deletion.ShortCode,
		Version:   deletion.Version,
		DeletedAt: deletion.DeletedAt.UTC(),
	})
	if err != nil {
		return err
	}
	out, err := i.client.Invoke(ctx, &awslambda.InvokeInput{
		FunctionName:   aws.String(i.function),
		InvocationType: "RequestResponse",
		Payload:        payload,
	})
	if err != nil {
		return err
	}
	if out.FunctionError != nil && aws.ToString(out.FunctionError) != "" {
		return errors.New(aws.ToString(out.FunctionError))
	}
	if out.StatusCode != 0 && (out.StatusCode < 200 || out.StatusCode >= 300) {
		return errors.New("invalidation invoke failed")
	}
	return nil
}

var _ platform.CacheInvalidator = (*Invalidator)(nil)
