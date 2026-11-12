//go:build darwin || linux

package main

import (
	"context"
	"io"
	"os"
	"os/signal"
	"runtime/trace"
	"syscall"
	"time"
)

// Logical observation units, NOT CPU operations or a new native grant. Runtime
// classification/projection is the supplied RD bound 493 (shared 20 frames,
// <=8 per transaction), not permission to add 493 more operations. The wrapper
// closes with <=19 fixed boundary units: env presence; FD admission; writer
// construction; Start admission; start annotation; native entry; native return
// classification; return annotation; Stop; writer lock; overflow sample; error
// sample; unlock; Close; close result; caller liveness; deadline liveness;
// trace qualification; report handoff. Descriptor internal checks use the
// EXISTING external trace admission allowance, not native M/P/S currencies.
const privateRDTraceClassifierUnits = 493
const privateRDTraceBoundaryUnits = 19
const privateRDTraceLogicalUnits = privateRDTraceClassifierUnits + privateRDTraceBoundaryUnits
const privateRDTraceAnnotations = 3*7 + 4 + 2

// A successful Start owns one Stop. No injected callback or native-producing
// interface exists; tests may exercise this transport object with synthetic
// trace bytes only. Start refusal must never stop another tracer.
type privateRDTracing struct {
	file    *os.File
	writer  *privatePrepareTraceWriter
	started bool
}

func privateRDStartTrace(f *os.File) (*privateRDTracing, bool) {
	t := &privateRDTracing{file: f, writer: &privatePrepareTraceWriter{dst: f, remaining: privatePrepareTraceBytes}}
	if trace.Start(t.writer) != nil {
		_ = f.Close()
		return nil, false
	}
	t.started = true
	return t, true
}
func (t *privateRDTracing) stop() privateRDTrace {
	if t == nil || !t.started {
		return privateRDTraceInvalid
	}
	t.started = false
	trace.Stop() // drains writer before sampling and before closing owned FD
	t.writer.mu.Lock()
	bad := t.writer.overflow || t.writer.err != nil
	t.writer.mu.Unlock()
	if t.file.Close() != nil {
		bad = true
	}
	if bad {
		return privateRDTraceInvalid
	}
	return privateRDTraceComplete
}
func privateRDFinishReport(ctx context.Context, w io.Writer, b *privateRDBudget, out privateRDOutcome, tr privateRDTrace) int {
	// No successful report after owned cleanup, caller cancellation or the original
	// deadline. Failures before config admission have no original deadline and may
	// publish the same single content-free refusal report.
	if ctx == nil || ctx.Err() != nil || !out.deadline.IsZero() && !time.Now().Before(out.deadline) {
		return 1
	}
	return privateRDReport(w, b, out, tr)
}
func runPrivateSourcePrepareRDTraced(args []string, stdout, stderr io.Writer) int {
	ctx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()
	outer, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var b privateRDBudget
	tr := privateRDTraceNotRequested
	var tracing *privateRDTracing
	raw, enabled := os.LookupEnv(privatePrepareTraceFDEnv)
	if enabled {
		f := privatePrepareTraceFile(raw, stdout, stderr)
		if f == nil {
			return privateRDFinishReport(outer, stdout, &b, privateRDOutcome{}, privateRDTraceInvalid)
		}
		var ok bool
		tracing, ok = privateRDStartTrace(f)
		if !ok {
			return privateRDFinishReport(outer, stdout, &b, privateRDOutcome{}, privateRDTraceInvalid)
		}
		trace.Log(outer, "private.prepare.cli", "TRACE_START")
	}
	out := privateRDNativeCLI(outer, args, &b)
	if tracing != nil {
		if out.qualified && out.cleanup && out.live {
			trace.Log(outer, "private.prepare.cli", "TRACE_RETURN_ZERO")
		} else {
			trace.Log(outer, "private.prepare.cli", "TRACE_RETURN_NONZERO")
		}
		tr = tracing.stop()
	}
	// Native outcome is never overwritten by trace writer/close errors. A requested
	// incomplete trace fails qualification and exit status, not native capture.
	return privateRDFinishReport(outer, stdout, &b, out, tr)
}
