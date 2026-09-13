package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"
)

func TestFromContextReturnsAttachedLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil)).With("request_id", "req-1")

	FromContext(WithLogger(context.Background(), logger)).Error("boom", "short_code", "abc1234")

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["request_id"] != "req-1" || got["short_code"] != "abc1234" || got["msg"] != "boom" {
		t.Fatalf("got %v", got)
	}
}

func TestFromContextFallsBackToDefault(t *testing.T) {
	if FromContext(context.Background()) != slog.Default() {
		t.Fatal("want default logger when none attached")
	}
}
