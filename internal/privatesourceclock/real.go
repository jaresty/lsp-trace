//go:build !lsp_trace_controlled_clock

// Package privatesourceclock keeps production clocks non-configurable. The
// alternate implementation exists only in explicitly tagged controlled tests.
package privatesourceclock

import (
	"context"
	"time"
)

func Now(_ context.Context, _ string) time.Time { return time.Now() }
