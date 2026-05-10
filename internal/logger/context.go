package logger

import (
	"context"

	"github.com/rs/zerolog"
)

// WithContext stores an enriched logger in ctx.
func WithContext(ctx context.Context, l *zerolog.Logger) context.Context {
	return l.WithContext(ctx)
}

// FromContext retrieves the logger from ctx, falling back to the global logger.
func FromContext(ctx context.Context) *zerolog.Logger {
	l := zerolog.Ctx(ctx)
	if l == nil || l.GetLevel() == zerolog.Disabled {
		return &global
	}
	return l
}
