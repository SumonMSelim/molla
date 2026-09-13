package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/SumonMSelim/molla/internal/platform"
)

func TestSinkWritesJSONLine(t *testing.T) {
	var buf bytes.Buffer
	event := platform.AuditEvent{
		ActorID: "arn:ops", Role: platform.RoleOperator, OwnerID: "owner",
		ShortCode: "abc1234", Reason: "malware", Outcome: "deleted",
		Timestamp: time.Unix(1_700_000_000, 0).UTC(),
	}
	if err := (Sink{W: &buf}).Record(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	var got platform.AuditEvent
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.ActorID != event.ActorID || got.OwnerID != event.OwnerID || got.Outcome != "deleted" {
		t.Fatalf("got %+v", got)
	}
}
