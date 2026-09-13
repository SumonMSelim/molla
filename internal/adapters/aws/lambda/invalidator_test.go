package lambda

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	awslambda "github.com/aws/aws-sdk-go-v2/service/lambda"

	awsadapter "github.com/SumonMSelim/molla/internal/adapters/aws"
	"github.com/SumonMSelim/molla/internal/platform"
)

func TestInvalidatorSuccessAndErrorPayload(t *testing.T) {
	var got []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, _ = io.ReadAll(r.Body)
		if r.URL.Query().Get("mode") == "err" || r.Header.Get("X-Test-Mode") == "err" {
			w.Header().Set("X-Amz-Function-Error", "Unhandled")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"errorMessage":"boom"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)
	client := awslambda.NewFromConfig(awsadapter.StaticConfig("us-east-1", srv.URL, srv.Client()), func(o *awslambda.Options) {
		o.Retryer = awssdk.NopRetryer{}
	})
	inv := NewInvalidator(client, "molla-invalidate")
	err := inv.Invalidate(context.Background(), platform.Deletion{
		ShortCode: "abc1234", Version: 2, DeletedAt: time.Unix(1_700_000_000, 0).UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	var req InvalidateRequest
	if err := json.Unmarshal(got, &req); err != nil {
		t.Fatal(err)
	}
	if req.ShortCode != "abc1234" || req.Version != 2 {
		t.Fatalf("payload = %+v", req)
	}
}

type errInvoker struct {
	out *awslambda.InvokeOutput
	err error
}

func (e errInvoker) Invoke(context.Context, *awslambda.InvokeInput, ...func(*awslambda.Options)) (*awslambda.InvokeOutput, error) {
	return e.out, e.err
}

func TestInvalidatorMapsFailures(t *testing.T) {
	unhandled := "Unhandled"
	inv := NewInvalidator(errInvoker{out: &awslambda.InvokeOutput{FunctionError: &unhandled, StatusCode: 200}}, "fn")
	if err := inv.Invalidate(context.Background(), platform.Deletion{ShortCode: "x", Version: 1, DeletedAt: time.Now()}); err == nil {
		t.Fatal("expected function error")
	}
	inv = NewInvalidator(errInvoker{err: errors.New("timeout")}, "fn")
	if err := inv.Invalidate(context.Background(), platform.Deletion{ShortCode: "x", Version: 1, DeletedAt: time.Now()}); err == nil {
		t.Fatal("expected timeout error")
	}
	inv = NewInvalidator(errInvoker{out: &awslambda.InvokeOutput{StatusCode: 500}}, "fn")
	if err := inv.Invalidate(context.Background(), platform.Deletion{ShortCode: "x", Version: 1, DeletedAt: time.Now()}); err == nil {
		t.Fatal("expected status error")
	}
}
