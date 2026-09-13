package main

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"

	"github.com/SumonMSelim/molla/internal/adapters/aws/ddbfake"
	"github.com/SumonMSelim/molla/internal/platform"
)

type scriptedStats struct {
	failCode string
	calls    int
}

func (s *scriptedStats) Get(context.Context, string) (platform.Stats, error) {
	return platform.Stats{}, platform.ErrNotFound
}

func (s *scriptedStats) Increment(_ context.Context, code string, _ int64, _ time.Time) error {
	s.calls++
	if code == s.failCode {
		return platform.ErrDependency
	}
	return nil
}

func TestHandleEventPartialBatchFailure(t *testing.T) {
	ok, _ := json.Marshal(wireEvent{EventID: "1", ShortCode: "aaa1111", OccurredAt: time.Unix(1, 0).UTC()})
	bad := []byte(`{`)
	fail, _ := json.Marshal(wireEvent{EventID: "3", ShortCode: "bbb2222", OccurredAt: time.Unix(2, 0).UTC()})
	event := events.KinesisEvent{Records: []events.KinesisEventRecord{
		{EventID: "e1", Kinesis: events.KinesisRecord{SequenceNumber: "seq-ok", Data: ok}},
		{EventID: "e2", Kinesis: events.KinesisRecord{SequenceNumber: "seq-bad", Data: bad}},
		{EventID: "e3", Kinesis: events.KinesisRecord{SequenceNumber: "seq-fail", Data: fail}},
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
	if !got["seq-bad"] || !got["seq-fail"] || got["seq-ok"] {
		t.Fatalf("failures = %+v", resp.BatchItemFailures)
	}
	if store.calls != 2 {
		t.Fatalf("increment calls = %d", store.calls)
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
