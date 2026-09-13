package dynamodb

import (
	"errors"
	"strings"
	"testing"

	"github.com/SumonMSelim/molla/internal/platform"
)

func TestMapAWSErrorPreservesCause(t *testing.T) {
	cause := errors.New("ProvisionedThroughputExceededException")
	err := mapAWSError(cause)

	if !errors.Is(err, platform.ErrDependency) {
		t.Fatalf("errors.Is(err, ErrDependency) = false, want true")
	}
	if !errors.Is(err, cause) {
		t.Fatalf("errors.Is(err, cause) = false, want true")
	}
	if !strings.Contains(err.Error(), cause.Error()) {
		t.Fatalf("error string %q does not contain cause", err.Error())
	}
}

func TestMapAWSErrorNil(t *testing.T) {
	if err := mapAWSError(nil); err != nil {
		t.Fatalf("mapAWSError(nil) = %v, want nil", err)
	}
}
