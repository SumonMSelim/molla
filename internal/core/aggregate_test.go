package core

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/SumonMSelim/molla/internal/platform"
)

type incrementCall struct {
	code string
	n    int64
	at   time.Time
}

type recordingStats struct {
	calls []incrementCall
	err   error
}

func (s *recordingStats) Get(context.Context, string) (platform.Stats, error) {
	return platform.Stats{}, platform.ErrNotFound
}

func (s *recordingStats) Increment(_ context.Context, code string, delta int64, lastClickAt time.Time) error {
	s.calls = append(s.calls, incrementCall{code: code, n: delta, at: lastClickAt})
	return s.err
}

func TestAggregateClicksBatchesByCode(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	events := make([]platform.ClickEvent, 0, 100)
	codes := []string{"aaa1111", "bbb2222", "ccc3333"}
	counts := map[string]int64{"aaa1111": 50, "bbb2222": 30, "ccc3333": 20}
	for _, code := range codes {
		for i := int64(0); i < counts[code]; i++ {
			events = append(events, platform.ClickEvent{
				EventID:    code + string(rune('a'+i%26)),
				ShortCode:  code,
				OccurredAt: now.Add(time.Duration(i) * time.Second),
			})
		}
	}
	events = append(events, platform.ClickEvent{EventID: "skip", OccurredAt: now})

	store := &recordingStats{}
	if err := AggregateClicks(context.Background(), store, events); err != nil {
		t.Fatal(err)
	}
	if len(store.calls) != 3 {
		t.Fatalf("Increment calls = %d, want 3", len(store.calls))
	}
	got := map[string]incrementCall{}
	for _, call := range store.calls {
		got[call.code] = call
	}
	for code, want := range counts {
		call, ok := got[code]
		if !ok || call.n != want {
			t.Fatalf("code %s increment = %+v, want n=%d", code, call, want)
		}
		if !call.at.Equal(now.Add(time.Duration(want-1) * time.Second)) {
			t.Fatalf("code %s last_click_at = %s", code, call.at)
		}
	}
}

func TestAggregateClicksEmptyAndError(t *testing.T) {
	store := &recordingStats{}
	if err := AggregateClicks(context.Background(), store, nil); err != nil {
		t.Fatal(err)
	}
	if len(store.calls) != 0 {
		t.Fatalf("calls = %d", len(store.calls))
	}
	store.err = platform.ErrDependency
	err := AggregateClicks(context.Background(), store, []platform.ClickEvent{{ShortCode: "abc1234"}})
	if !errors.Is(err, platform.ErrDependency) {
		t.Fatalf("error = %v", err)
	}
}
