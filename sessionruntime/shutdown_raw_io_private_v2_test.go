package sessionruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/managedprocess"
)

type shutdownRawFlushWriter struct {
	io.WriteCloser
	flushes  int
	flushErr error
}

func (w *shutdownRawFlushWriter) Flush() error { w.flushes++; return w.flushErr }

type shutdownRawFlushChild struct {
	*shutdownLoopChild
	writer *shutdownRawFlushWriter
}

func (c shutdownRawFlushChild) Stdin() io.WriteCloser { return c.writer }
func TestShutdownRawFlushPrivateV2(t *testing.T) {
	c := newShutdownLoopChild("exact")
	w := &shutdownRawFlushWriter{WriteCloser: c.stdin}
	m, err := New(Config{Limits: Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 1, MaxCancels: 1, MaxTombstones: 2, MaxObservations: 4}, Wire: lspwire.DefaultLimits(), Starter: oneChildStarter{c}})
	if err != nil {
		t.Fatal(err)
	}
	observed := m.gracefulShutdown(shutdownRawFlushChild{c, w}, lspwire.NewPending(2), 1)
	c.Teardown(context.Background())
	c.Close()
	if !observed.acknowledged || w.flushes != 2 {
		t.Fatalf("ASSERT_RAW_PRESERVES_FLUSH: acknowledged=%v flushes=%d", observed.acknowledged, w.flushes)
	}
}

func TestShutdownRawOptionalInterfacePrivateV2(t *testing.T) {
	var b bytes.Buffer
	plain := (&shutdownCountingWriterPrivateV2{source: &b}).wireWriter()
	if _, hasFlush := plain.(interface{ Flush() error }); hasFlush {
		t.Fatal("ASSERT_RAW_NO_INVENTED_FLUSH")
	}
	sentinel := errors.New("owned flush failure")
	underlying := &shutdownRawFlushWriter{WriteCloser: nopWriteCloserPrivateV2{&b}, flushErr: sentinel}
	counter := &shutdownCountingWriterPrivateV2{source: underlying}
	writer := lspwire.NewWriter(counter.wireWriter(), lspwire.DefaultLimits())
	err := writer.Write(lspwire.Message{JSONRPC: lspwire.Version, Method: "owned"})
	if err != sentinel || underlying.flushes != 1 || counter.calls != 2 {
		t.Fatalf("ASSERT_RAW_FLUSH_ERROR_UNCHANGED: sameError=%v flushes=%d calls=%d", err == sentinel, underlying.flushes, counter.calls)
	}
}

type nopWriteCloserPrivateV2 struct{ io.Writer }

func (nopWriteCloserPrivateV2) Close() error { return nil }

func TestShutdownRawProcessMetadataPrivateV2(t *testing.T) {
	f := cleanupObservationPrivateV2(LifecycleCleanupObservationPrivateV2{}, managedprocess.TeardownObservation{Death: managedprocess.DeathObservation{
		Stderr:         managedprocess.StderrObservation{Bytes: []byte("WITHHELD_FIXTURE_TEXT"), Truncated: true},
		StdoutDelivery: managedprocess.StreamDeliveryObservation{Err: errors.New("WITHHELD_FIXTURE_TEXT"), Category: "WITHHELD_FIXTURE_TEXT"},
		StderrDelivery: managedprocess.StreamDeliveryObservation{Err: errors.New("WITHHELD_FIXTURE_TEXT"), Category: "CLOSED"},
	}}, managedprocess.ResourceObservation{}, false)
	raw, _ := json.Marshal(f)
	if bytes.Contains(raw, []byte("WITHHELD_FIXTURE_TEXT")) || f.StderrCapturedBytes != 21 || !f.StderrTruncated || !f.StdoutDeliveryError || !f.StderrDeliveryError || f.StdoutDeliveryCategory != "" || f.StderrDeliveryCategory != "CLOSED" || f.ProcessIOPrivateV2Availability != "OBSERVED_FIELDS_ONLY" || !f.StderrDrainAssumed {
		t.Fatalf("ASSERT_RAW_PROCESS_CONTENT_FREE: %s", raw)
	}
}

func TestShutdownRawCounterPrivateV2(t *testing.T) {
	n, overflow := ^uint64(0)-1, false
	shutdownAddPrivateV2(&n, 1, &overflow)
	if n != ^uint64(0) || overflow {
		t.Fatal("ASSERT_RAW_COUNTER_EXACT_MAX")
	}
	shutdownAddPrivateV2(&n, 1, &overflow)
	if n != ^uint64(0) || !overflow {
		t.Fatal("ASSERT_RAW_COUNTER_SATURATES")
	}
}

func TestShutdownRawIOPrivateV2(t *testing.T) {
	for _, mode := range []string{"exact", "coalesced", "malformed", "malformed-json", "timeout", "partial-timeout", "request"} {
		t.Run(mode, func(t *testing.T) {
			c := newShutdownLoopChild(mode)
			op, _ := shutdownLoopRun(t, c)
			f, ok := op.CleanupObservationPrivateV2()
			raw, _ := json.Marshal(f)
			var fields map[string]any
			_ = json.Unmarshal(raw, &fields)
			// This assertion intentionally executes on the pre-instrumentation artifact.
			if !ok || fields["RawIOFinalAfterJoin"] != true || fields["ReaderJoined"] != true || fields["RawWriteBytes"] == nil || fields["RawWriteBytes"].(float64) <= 0 {
				t.Fatalf("ASSERT_RAW_IO_JOINED: mode=%s fields=%s", mode, raw)
			}
			if f.ShutdownRequestID == "" || f.ShutdownBodyBytes <= 0 || f.ShutdownBodyBytes != f.ShutdownBodyWrittenBytes || f.RawWriteBytes != uint64(f.ShutdownHeaderWrittenBytes+f.ShutdownBodyWrittenBytes) {
				t.Fatal("ASSERT_RAW_OWN_FRAME_METADATA")
			}
			if fields["RawReadCalls"].(float64) < 1 {
				t.Fatal("ASSERT_RAW_READ_CALLS")
			}
			if mode == "malformed" || mode == "malformed-json" {
				if fields["RawReadBytes"].(float64) <= 0 || f.ReadFrames != 0 || f.ReadBytes != 0 {
					t.Fatal("ASSERT_RAW_NOT_VALIDATED")
				}
			}
			if (mode == "timeout" || mode == "partial-timeout") && (fields["ReaderActiveAtDeadline"] != true || f.ReadFrames != 0) {
				t.Fatal("ASSERT_RAW_TIMEOUT_ACTIVE")
			}
			if mode == "partial-timeout" && (f.RawReadBytes == 0 || f.RawReadBytesAtDeadline == 0 || f.RawReadErrors == 0 || f.ReadBytes != 0 || f.ShutdownRead != ShutdownTimeoutPrivateV2) {
				t.Fatal("ASSERT_RAW_PARTIAL_TIMEOUT_DISTINCT")
			}
			if mode == "malformed-json" && (f.BodyReadEvents != 1 || f.BodyReadBytes != 1 || f.DecodeAttempts != 1 || f.RejectedDecodes != 1) {
				t.Fatal("ASSERT_RAW_COMPLETE_MALFORMED_DISTINCT")
			}
			if mode == "partial-timeout" && (f.BodyReadEvents != 1 || f.BodyReadBytes != 1 || f.ClosedBodyReadEvents != 1 || f.DecodeAttempts != 0) {
				t.Fatal("ASSERT_RAW_PARTIAL_BODY_NO_DECODE")
			}
			before := f
			f.RawReadBytes = ^uint64(0)
			again, _ := op.CleanupObservationPrivateV2()
			if again != before {
				t.Fatal("ASSERT_RAW_IMMUTABLE_COPY")
			}
			if fields["RawCounterOverflow"] != false {
				t.Fatal("ASSERT_RAW_COUNTER_OVERFLOW")
			}
			public, _ := json.Marshal(op)
			var publicFields map[string]any
			_ = json.Unmarshal(public, &publicFields)
			if publicFields["RawReadBytes"] != nil {
				t.Fatal("ASSERT_RAW_PUBLIC_EXCLUDED")
			}
			t.Logf("ASSERT_RAW_IO_JOINED[%s]: PASS", mode)
		})
	}
}
