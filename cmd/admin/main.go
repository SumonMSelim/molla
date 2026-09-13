// Command admin is the operator CLI for abuse takedown and credential issue.
// Identity comes from STS; adapters talk to DynamoDB, the invalidation Lambda,
// and structured logs.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
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
	issue     func(context.Context, string, platform.Credential) error
	identity  func(context.Context) (platform.Principal, error)
	now       func() time.Time
	randToken func() (string, error)
	lookupEnv func(string) (string, bool)
	stdout    io.Writer
}

func run(args []string, rt adminRuntime) error {
	if len(args) < 1 {
		return errors.New("usage: admin takedown|issue ...")
	}
	switch args[0] {
	case "takedown":
		return runTakedown(args[1:], rt)
	case "issue":
		return runIssue(args[1:], rt)
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

func runIssue(args []string, rt adminRuntime) error {
	if rt.issue == nil {
		return errors.New("issue not configured")
	}
	fs := flag.NewFlagSet("issue", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	owner := fs.String("owner", "", "")
	actor := fs.String("actor", "", "")
	token := fs.String("token", "", "")
	days := fs.Int("days", 90, "")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*owner) == "" {
		return errors.New("issue requires --owner")
	}
	if *days < 1 || time.Duration(*days)*24*time.Hour > platform.MaximumCredentialLifetime {
		return errors.New("issue --days must be 1..90")
	}
	actorID := strings.TrimSpace(*actor)
	if actorID == "" && rt.identity != nil {
		got, err := rt.identity(context.Background())
		if err != nil {
			return err
		}
		actorID = got.ActorID
	}
	if actorID == "" {
		return errors.New("actor required via --actor or STS caller identity")
	}
	raw := strings.TrimSpace(*token)
	if raw == "" {
		if rt.randToken == nil {
			return errors.New("token generator required")
		}
		generated, err := rt.randToken()
		if err != nil {
			return err
		}
		raw = generated
	}
	now := time.Now().UTC()
	if rt.now != nil {
		now = rt.now()
	}
	cred := platform.Credential{
		ActorID:   actorID,
		OwnerID:   strings.TrimSpace(*owner),
		Status:    platform.CredentialActive,
		IssuedAt:  now,
		ExpiresAt: now.Add(time.Duration(*days) * 24 * time.Hour),
	}
	if err := rt.issue(context.Background(), raw, cred); err != nil {
		return err
	}
	_, err := fmt.Fprintf(rt.stdout, "issued owner=%s actor=%s\n%s\n", cred.OwnerID, cred.ActorID, raw)
	return err
}

func randomToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
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
	idents := ddb.NewIdentityStore(ddbClient)
	deleter := handlers.Deleter{
		Store:       ddb.NewLinkStore(ddbClient),
		Invalidator: lam.NewInvalidator(lambdaClient, function),
		Audit:       logging.Sink{W: audit},
		Clock:       liveClock{},
	}
	return adminRuntime{
		delete:    deleter.Delete,
		issue:     idents.Store,
		identity:  stsIdentity(stsClient),
		now:       func() time.Time { return time.Now().UTC() },
		randToken: randomToken,
		lookupEnv: os.LookupEnv,
		stdout:    stdout,
	}
}

func newRuntime() (adminRuntime, error) {
	cfg, err := awsadapter.RuntimeConfig(context.Background())
	if err != nil {
		return adminRuntime{}, err
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
