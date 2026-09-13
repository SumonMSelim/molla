package logging

import (
	"context"
	"encoding/json"
	"io"

	"github.com/SumonMSelim/molla/internal/platform"
)

// Sink writes audit events as one JSON object per line.
type Sink struct {
	W io.Writer
}

func (s Sink) Record(_ context.Context, event platform.AuditEvent) error {
	return json.NewEncoder(s.W).Encode(event)
}

var _ platform.AuditSink = Sink{}
