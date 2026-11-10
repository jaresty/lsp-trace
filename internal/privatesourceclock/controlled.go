//go:build lsp_trace_controlled_clock

package privatesourceclock

import (
	"context"
	"time"
)

type key struct{}

// WithControlledClock is absent from production builds and all CLI schemas.
func WithControlledClock(ctx context.Context, f func(string) time.Time) context.Context {
	return context.WithValue(ctx, key{}, f)
}
func Now(ctx context.Context, boundary string) time.Time {
	if ctx != nil {
		if f, ok := ctx.Value(key{}).(func(string) time.Time); ok && f != nil {
			return f(boundary)
		}
	}
	return time.Now()
}
