package main

import (
	"context"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"

	"github.com/SumonMSelim/molla/internal/adapters/aws/ddbfake"
	ddb "github.com/SumonMSelim/molla/internal/adapters/aws/dynamodb"
	"github.com/SumonMSelim/molla/internal/platform"
)

type scriptedStats struct {
	failCode string
	calls    int
	last     map[string]int64
}

func (s *scriptedStats) Get(context.Context, string) (platform.Stats, error) {
	return platform.Stats{}, platform.ErrNotFound
}

func (s *scriptedStats) Increment(_ context.Context, code string, n int64, _ time.Time) error {
	s.calls++
	if s.last == nil {
		s.last = map[string]int64{}
	}
	s.last[code] += n
	if code == s.failCode {
		return platform.ErrDependency
	}
	return nil
}

func TestHandleEventPartialBatchFailure(t *testing.T) {
	bad := events.DynamoDBEventRecord{
		EventName: "INSERT",
		Change: events.DynamoDBStreamRecord{SequenceNumber: "seq-bad", NewImage: map[string]events.DynamoDBAttributeValue{
			ddb.AttrClickShortCode: events.NewStringAttribute("aaa1111"),
		}},
	}
	event := events.DynamoDBEvent{Records: []events.DynamoDBEventRecord{
		record("seq-ok", "INSERT", "aaa1111", 1),
		record("seq-ok2", "INSERT", "aaa1111", 2),
		bad,
		record("seq-fail", "INSERT", "bbb2222", 2),
		record("seq-ttl", "REMOVE", "ccc3333", 3),
	}}
	store := &scriptedStats{failCode: "bbb2222"}
	resp, err := handleEvent(context.Background(), event, store)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, f := range resp.BatchItemFailures {
		got[f.ItemIdentifier] = true
	}
	if !got["seq-bad"] || !got["seq-fail"] || got["seq-ok"] || got["seq-ok2"] || got["seq-ttl"] {
		t.Fatalf("failures = %+v", resp.BatchItemFailures)
	}
	if store.calls != 2 {
		t.Fatalf("increment calls = %d", store.calls)
	}
	if store.last["aaa1111"] != 2 || store.last["ccc3333"] != 0 {
		t.Fatalf("increments = %v", store.last)
	}
}

func record(seq, eventName, code string, at int64) events.DynamoDBEventRecord {
	return events.DynamoDBEventRecord{
		EventName: eventName,
		Change: events.DynamoDBStreamRecord{SequenceNumber: seq, NewImage: map[string]events.DynamoDBAttributeValue{
			ddb.AttrClickEventID:      events.NewStringAttribute("evt-" + seq),
			ddb.AttrClickShortCode:    events.NewStringAttribute(code),
			ddb.AttrClickOccurredAt:   events.NewNumberAttribute(strconv.FormatInt(at, 10)),
			ddb.AttrClickSourceIPHash: events.NewStringAttribute("hash"),
		}},
	}
}

func TestClickFromImageRoundTrip(t *testing.T) {
	rec := record("1", "INSERT", "abc1234", 1_700_000_000)
	click, err := clickFromImage(rec.Change.NewImage)
	if err != nil {
		t.Fatal(err)
	}
	if click.ShortCode != "abc1234" || click.EventID != "evt-1" || click.SourceIPHash != "hash" || click.OccurredAt.Unix() != 1_700_000_000 {
		t.Fatalf("click = %+v", click)
	}
	if _, err := clickFromImage(map[string]events.DynamoDBAttributeValue{
		ddb.AttrClickShortCode:  events.NewStringAttribute("abc1234"),
		ddb.AttrClickOccurredAt: events.NewStringAttribute("not-a-number"),
	}); err == nil {
		t.Fatal("expected malformed error")
	}
}

func TestNewStatsStoreUsesEndpoint(t *testing.T) {
	srv := httptest.NewServer(ddbfake.New())
	t.Cleanup(srv.Close)
	t.Setenv("MOLLA_AWS_ENDPOINT", srv.URL)
	store, err := newStatsStore()
	if err != nil || store == nil {
		t.Fatalf("store = %v err = %v", store, err)
	}
}
