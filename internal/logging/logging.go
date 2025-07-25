package logging

import (
	"context"
	"log/slog"
)


type contextKey struct{}

var loggerKey = contextKey{}
var defaultLogger = slog.Default()

func WithContext(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

func FromContext(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(loggerKey).(*slog.Logger); ok {
		return logger
	}

	return defaultLogger
}
