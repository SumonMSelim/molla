// Command admin is the operator CLI for abuse takedown. Identity comes from
// STS; adapters talk to DynamoDB, the invalidation Lambda, and structured logs.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	awslambda "github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	awsadapter "github.com/SumonMSelim/molla/internal/adapters/aws"
	ddb "github.com/SumonMSelim/molla/internal/adapters/aws/dynamodb"
	lam "github.com/SumonMSelim/molla/internal/adapters/aws/lambda"
	"github.com/SumonMSelim/molla/internal/adapters/logging"
	"github.com/SumonMSelim/molla/internal/handlers"
	"github.com/SumonMSelim/molla/internal/platform"
)

type liveClock struct{}

func (liveClock) Now() time.Time { return time.Now().UTC() }

type adminRuntime struct {
	delete    func(context.Context, platform.Principal, string, string) error
	identity  func(context.Context) (platform.Principal, error)
	lookupEnv func(string) (string, bool)
	stdout    io.Writer
}

func run(args []string, rt adminRuntime) error {
	if len(args) < 1 {
		return errors.New("usage: admin takedown --code CODE --reason REASON [--actor ACTOR]")
	}
	switch args[0] {
	case "takedown":
		return runTakedown(args[1:], rt)
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runTakedown(args []string, rt adminRuntime) error {
	fs := flag.NewFlagSet("takedown", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	code := fs.String("code", "", "")
	reason := fs.String("reason", "", "")
	actor := fs.String("actor", "", "")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*code) == "" || strings.TrimSpace(*reason) == "" {
		return errors.New("takedown requires --code and --reason")
	}
	principal := platform.Principal{Role: platform.RoleOperator, ActorID: strings.TrimSpace(*actor)}
	if principal.ActorID == "" && rt.identity != nil {
		got, err := rt.identity(context.Background())
		if err != nil {
			return err
		}
		principal = got
		principal.Role = platform.RoleOperator
	}
	if principal.ActorID == "" {
		return errors.New("actor required via --actor or STS caller identity")
	}
	if err := rt.delete(context.Background(), principal, strings.TrimSpace(*code), strings.TrimSpace(*reason)); err != nil {
		return err
	}
	_, err := fmt.Fprintf(rt.stdout, "takedown %s ok\n", strings.TrimSpace(*code))
	return err
}

func stsIdentity(client *sts.Client) func(context.Context) (platform.Principal, error) {
	return func(ctx context.Context) (platform.Principal, error) {
		out, err := client.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
		if err != nil {
			return platform.Principal{}, err
		}
		return platform.Principal{ActorID: awssdk.ToString(out.Arn), Role: platform.RoleOperator}, nil
	}
}

func newAWSRuntime(cfg awssdk.Config, function string, audit, stdout io.Writer) adminRuntime {
	ddbClient := dynamodb.NewFromConfig(cfg, func(o *dynamodb.Options) { o.Retryer = awssdk.NopRetryer{} })
	lambdaClient := awslambda.NewFromConfig(cfg, func(o *awslambda.Options) { o.Retryer = awssdk.NopRetryer{} })
	stsClient := sts.NewFromConfig(cfg, func(o *sts.Options) { o.Retryer = awssdk.NopRetryer{} })
	deleter := handlers.Deleter{
		Store:       ddb.NewLinkStore(ddbClient),
		Invalidator: lam.NewInvalidator(lambdaClient, function),
		Audit:       logging.Sink{W: audit},
		Clock:       liveClock{},
	}
	return adminRuntime{
		delete:    deleter.Delete,
		identity:  stsIdentity(stsClient),
		lookupEnv: os.LookupEnv,
		stdout:    stdout,
	}
}

func newRuntime() (adminRuntime, error) {
	ctx := context.Background()
	endpoint := os.Getenv("MOLLA_AWS_ENDPOINT")
	var cfg awssdk.Config
	if endpoint != "" {
		cfg = awsadapter.StaticConfig(os.Getenv("AWS_REGION"), endpoint, nil)
	} else {
		loaded, err := awsadapter.Load(ctx)
		if err != nil {
			return adminRuntime{}, err
		}
		cfg = loaded
	}
	function := os.Getenv("MOLLA_INVALIDATE_FUNCTION")
	if function == "" {
		function = "molla-invalidate"
	}
	return newAWSRuntime(cfg, function, os.Stderr, os.Stdout), nil
}

func main() {
	rt, err := newRuntime()
	if err != nil {
		log.Fatal(err)
	}
	if err := run(os.Args[1:], rt); err != nil {
		log.Fatal(err)
	}
}
