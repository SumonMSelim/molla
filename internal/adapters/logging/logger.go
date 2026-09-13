package logging

import (
	"context"
	"log/slog"
	"os"
)

type loggerContextKey struct{}

// NewLogger returns the root structured logger writing JSON lines to stderr.
func NewLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stderr, nil))
}

// WithLogger returns a context carrying logger for the current invocation.
func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerContextKey{}, logger)
}

// FromContext returns the invocation logger, or the default logger when none
// was attached, so callers never need a nil check.
func FromContext(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(loggerContextKey{}).(*slog.Logger); ok && logger != nil {
		return logger
	}
	return slog.Default()
}
