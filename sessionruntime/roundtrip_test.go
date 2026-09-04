package sessionruntime

import (
	"context"
	"encoding/json"
	"io"
	"reflect"
	"sync"
	"testing"
	"time"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/internal/session"
)

func TestExclusiveRoundTripSurfaceExists(t *testing.T) {
	if _, ok := reflect.TypeOf((*Manager)(nil)).MethodByName("RoundTrip"); !ok {
		t.Fatal("ASSERT_ROUNDTRIP_SURFACE: missing transport-neutral RoundTrip")
	}
}

func TestRuntimeObservationSurfaceAccountsOwnedWireAndThermalPhase(t *testing.T) {
	type fieldExpectation struct {
		name string
		typ  reflect.Type
	}
	for _, subject := range []struct {
		name   string
		typ    reflect.Type
		fields []fieldExpectation
	}{
		{"round trip", reflect.TypeOf(RoundTripResult{}), []fieldExpectation{{"Duration", reflect.TypeOf(time.Duration(0))}, {"RequestMessages", reflect.TypeOf(0)}, {"RequestBytes", reflect.TypeOf(int64(0))}, {"ThermalPhase", reflect.TypeOf("")}}},
		{"readiness", reflect.TypeOf(ReadinessSnapshot{}), []fieldExpectation{{"Duration", reflect.TypeOf(time.Duration(0))}, {"RequestMessages", reflect.TypeOf(0)}, {"RequestBytes", reflect.TypeOf(int64(0))}, {"ResponseMessages", reflect.TypeOf(0)}, {"ResponseBytes", reflect.TypeOf(int64(0))}, {"ThermalPhase", reflect.TypeOf("")}}},
	} {
		for _, want := range subject.fields {
			field, ok := subject.typ.FieldByName(want.name)
			if !ok || field.Type != want.typ {
				t.Fatalf("ASSERT_RUNTIME_OBSERVATION_SURFACE: %s field %s missing or has wrong type", subject.name, want.name)
			}
		}
	}
}

type roundTripChild struct {
	input     *io.PipeReader
	stdin     *io.PipeWriter
	output    *io.PipeWriter
	stdout    *io.PipeReader
	mode      string
	mu        sync.Mutex
	requests  []lspwire.Message
	teardowns int
}

func newRoundTripChild(mode string) *roundTripChild {
	input, stdin := io.Pipe()
	stdout, output := io.Pipe()
	c := &roundTripChild{input: input, stdin: stdin, output: output, stdout: stdout, mode: mode}
	go c.serve()
	return c
}
func (c *roundTripChild) serve() {
	r := lspwire.NewReader(c.input, lspwire.DefaultLimits())
	w := lspwire.NewWriter(c.output, lspwire.DefaultLimits())
	for {
		msg, err := r.Read()
		if err != nil {
			return
		}
		c.mu.Lock()
		c.requests = append(c.requests, msg)
		c.mu.Unlock()
		if msg.Method == "$/cancelRequest" {
			continue
		}
		switch c.mode {
		case "success":
			_ = w.Write(lspwire.Message{JSONRPC: lspwire.Version, Method: "window/logMessage", Params: json.RawMessage(`{"type":3}`)})
			_ = w.Write(lspwire.Message{JSONRPC: lspwire.Version, ID: json.RawMessage(`999`), Result: json.RawMessage(`null`)})
			_ = w.Write(lspwire.Message{JSONRPC: lspwire.Version, ID: msg.ID, Result: json.RawMessage(`{"ok":true}`)})
		case "server-error":
			_ = w.Write(lspwire.Message{JSONRPC: lspwire.Version, ID: msg.ID, Error: &lspwire.RPCError{Code: -32603, Message: "boom"}})
		case "malformed":
			_, _ = io.WriteString(c.output, "Content-Length: nope\r\n\r\n{}")
		case "eof":
			_ = c.output.Close()
		case "hang":
		case "duplicate":
			_ = w.Write(lspwire.Message{JSONRPC: lspwire.Version, ID: msg.ID, Result: json.RawMessage(`1`)})
			_ = w.Write(lspwire.Message{JSONRPC: lspwire.Version, ID: msg.ID, Result: json.RawMessage(`2`)})
		}
	}
}
func (c *roundTripChild) Stdin() io.WriteCloser { return c.stdin }
func (c *roundTripChild) Stdout() io.ReadCloser { return c.stdout }
func (c *roundTripChild) Teardown(context.Context) managedprocess.TeardownObservation {
	c.mu.Lock()
	c.teardowns++
	c.mu.Unlock()
	_ = c.stdin.Close()
	_ = c.input.Close()
	_ = c.output.Close()
	return managedprocess.TeardownObservation{Death: managedprocess.DeathObservation{Kind: managedprocess.DeathExited, Reap: managedprocess.ReapObservation{Kind: managedprocess.ReapComplete}}}
}
func (c *roundTripChild) Close() managedprocess.ResourceObservation {
	_ = c.stdout.Close()
	return managedprocess.ResourceObservation{Kind: managedprocess.ResourcesClosed}
}
func (c *roundTripChild) snapshot() ([]lspwire.Message, int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]lspwire.Message(nil), c.requests...), c.teardowns
}

func (c *roundTripChild) waitForCancels(want int) ([]lspwire.Message, int) {
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		requests, teardowns := c.snapshot()
		count := 0
		for _, request := range requests {
			if request.Method == "$/cancelRequest" {
				count++
			}
		}
		if count >= want {
			return requests, teardowns
		}
		time.Sleep(time.Millisecond)
	}
	return c.snapshot()
}

func roundTripManager(t *testing.T, mode string) (*Manager, StartResult, *roundTripChild) {
	t.Helper()
	child := newRoundTripChild(mode)
	m, err := New(Config{Limits: Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 2, MaxCancels: 2, MaxTombstones: 4, MaxObservations: 64, MaxOperations: 2}, Starter: oneChildStarter{child}})
	if err != nil {
		t.Fatal(err)
	}
	s := m.Start(context.Background(), StartRequest{Profile: profile(t)})
	if got := m.ObserveInitialization(s.SessionID, s.Generation, true); got.State != session.Ready {
		t.Fatal(got)
	}
	return m, s, child
}

func TestRoundTripMatchesWithinBoundsAndRetainsInterleaving(t *testing.T) {
	m, s, _ := roundTripManager(t, "success")
	got := m.RoundTrip(context.Background(), RoundTripRequest{SessionID: s.SessionID, Generation: s.Generation, Method: "test/method", Params: json.RawMessage(`{"x":1}`), Deadline: time.Now().Add(time.Second), MaxMessages: 3, MaxBytes: 4096})
	if got.Failure != "" || string(got.Result) != `{"ok":true}` || got.Key.ID != 1 || len(got.Notifications) != 1 || len(got.Responses) != 1 || got.Messages != 3 {
		t.Fatalf("ASSERT_ROUNDTRIP_CORRELATED_BOUNDED_SUCCESS: %+v", got)
	}
	if got.RequestMessages != 1 || got.RequestBytes <= 0 || got.Bytes <= 0 || got.Duration < 0 || got.ThermalPhase != "WARM" {
		t.Fatalf("ASSERT_ROUNDTRIP_ACCOUNTING_WARM: %+v", got)
	}
	if c := m.Census(); c.Requests != 0 || c.Workers != 0 {
		t.Fatalf("ASSERT_ROUNDTRIP_OWNERSHIP_RELEASE: %+v", c)
	}
}

func TestRoundTripRejectsNonReadyStaleAndConcurrentLifecycle(t *testing.T) {
	child := newRoundTripChild("hang")
	m, _ := New(Config{Limits: Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 2, MaxCancels: 2, MaxTombstones: 4, MaxObservations: 64, MaxOperations: 2}, Starter: oneChildStarter{child}})
	s := m.Start(context.Background(), StartRequest{Profile: profile(t)})
	if got := m.RoundTrip(context.Background(), RoundTripRequest{SessionID: s.SessionID, Generation: s.Generation, Method: "x"}); got.Failure != session.LifecycleConflict {
		t.Fatalf("ASSERT_ROUNDTRIP_READY_ONLY: %+v", got)
	}
	m.ObserveInitialization(s.SessionID, s.Generation, true)
	if got := m.RoundTrip(context.Background(), RoundTripRequest{SessionID: s.SessionID, Generation: s.Generation + 1, Method: "x"}); got.Failure != session.StaleGeneration {
		t.Fatalf("ASSERT_ROUNDTRIP_EXACT_GENERATION: %+v", got)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan RoundTripResult, 1)
	go func() {
		done <- m.RoundTrip(ctx, RoundTripRequest{SessionID: s.SessionID, Generation: s.Generation, Method: "x", Deadline: time.Now().Add(time.Second), MaxMessages: 1, MaxBytes: 1024})
	}()
	deadline := time.Now().Add(time.Second)
	for m.Census().Workers == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if second := m.RoundTrip(context.Background(), RoundTripRequest{SessionID: s.SessionID, Generation: s.Generation, Method: "y"}); second.Failure != session.LifecycleConflict {
		t.Fatalf("ASSERT_ROUNDTRIP_EXCLUSIVE_REJECT: %+v", second)
	}
	if stop := m.Stop(context.Background(), s.SessionID, "caller"); stop.Failure != session.LifecycleConflict {
		t.Fatalf("ASSERT_ROUNDTRIP_LIFECYCLE_CONFLICT: %+v", stop)
	}
	cancel()
	got := <-done
	if got.Failure != session.RequestCancelled {
		t.Fatalf("ASSERT_ROUNDTRIP_CANCEL_TERMINAL: %+v", got)
	}
	requests, teardowns := child.waitForCancels(1)
	cancels := 0
	for _, r := range requests {
		if r.Method == "$/cancelRequest" {
			cancels++
		}
	}
	if cancels != 1 {
		t.Fatalf("ASSERT_ROUNDTRIP_CANCEL_EXACTLY_ONCE: count=%d requests=%+v", cancels, requests)
	}
	if teardowns != 1 || m.Census().Workers != 0 || m.Census().Requests != 0 {
		t.Fatalf("ASSERT_ROUNDTRIP_FAIL_CLOSED_RELEASE: teardown=%d census=%+v", teardowns, m.Census())
	}
}

func TestRoundTripServerAndProtocolFailures(t *testing.T) {
	m, s, _ := roundTripManager(t, "server-error")
	got := m.RoundTrip(context.Background(), RoundTripRequest{SessionID: s.SessionID, Generation: s.Generation, Method: "x", Deadline: time.Now().Add(time.Second), MaxMessages: 1, MaxBytes: 1024})
	if got.Failure != "" || got.ServerError == nil || got.ServerError.Code != -32603 {
		t.Fatalf("ASSERT_ROUNDTRIP_SERVER_ERROR: %+v", got)
	}
	for _, tc := range []struct {
		mode string
		want session.Failure
	}{{"malformed", session.SessionPoisoned}, {"eof", session.SessionCrashed}} {
		t.Run(tc.mode, func(t *testing.T) {
			m, s, _ := roundTripManager(t, tc.mode)
			got := m.RoundTrip(context.Background(), RoundTripRequest{SessionID: s.SessionID, Generation: s.Generation, Method: "x", Deadline: time.Now().Add(time.Second), MaxMessages: 1, MaxBytes: 1024})
			if got.Failure != tc.want {
				t.Fatalf("ASSERT_ROUNDTRIP_PROTOCOL_%s: %+v", tc.mode, got)
			}
		})
	}
}

func benchmarkProfile(b *testing.B) runtimeprofile.Profile {
	b.Helper()
	validated, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "benchmark", Workspace: "/workspace", Profile: "go", EnvironmentReference: "local"})
	if err != nil {
		b.Fatal(err)
	}
	return runtimeprofile.Resolve(validated)
}

func benchmarkManager(b *testing.B, child Child) (*Manager, StartResult) {
	b.Helper()
	m, err := New(Config{Limits: Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 2, MaxCancels: 2, MaxTombstones: 4, MaxObservations: 64, MaxOperations: 2}, Starter: oneChildStarter{child}})
	if err != nil {
		b.Fatal(err)
	}
	return m, m.Start(context.Background(), StartRequest{Profile: benchmarkProfile(b), Process: managedprocess.Spec{Path: "fixture"}})
}

func BenchmarkRuntimeColdReadiness(b *testing.B) {
	for i := 0; i < b.N; i++ {
		child := newReadinessChild("ready")
		m, started := benchmarkManager(b, child)
		pending := m.BeginReadiness(context.Background(), started.SessionID, started.Generation, time.Now().Add(time.Second))
		terminal, ok := m.WaitReadiness(context.Background(), pending.ID)
		if !ok || terminal.ThermalPhase != "COLD" || terminal.RequestMessages != 2 || terminal.ResponseMessages != 1 {
			b.Fatalf("cold readiness accounting: %+v", terminal)
		}
		_ = child.Teardown(context.Background())
		_ = child.Close()
	}
}

func BenchmarkRuntimeWarmRoundTrip(b *testing.B) {
	m, started, child := roundTripManagerForBenchmark(b, "success")
	b.Cleanup(func() { _ = child.Teardown(context.Background()); _ = child.Close() })
	request := RoundTripRequest{SessionID: started.SessionID, Generation: started.Generation, Method: "benchmark/method", Params: json.RawMessage(`{"x":1}`), MaxMessages: 3, MaxBytes: 4096}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		request.Deadline = time.Now().Add(time.Second)
		got := m.RoundTrip(context.Background(), request)
		if got.Failure != "" || got.ThermalPhase != "WARM" || got.RequestMessages != 1 || got.Messages != 3 {
			b.Fatalf("warm round trip accounting: %+v", got)
		}
	}
}

func BenchmarkRuntimeBusyRejection(b *testing.B) {
	m, started, child := roundTripManagerForBenchmark(b, "success")
	b.Cleanup(func() { _ = child.Teardown(context.Background()); _ = child.Close() })
	m.mu.Lock()
	m.sessions[started.SessionID].protocolOwned = true
	m.mu.Unlock()
	request := RoundTripRequest{SessionID: started.SessionID, Generation: started.Generation, Method: "benchmark/rejected"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if got := m.RoundTrip(context.Background(), request); got.Failure == "" {
			b.Fatal("busy round trip unexpectedly admitted")
		}
	}
}

func roundTripManagerForBenchmark(b *testing.B, mode string) (*Manager, StartResult, *roundTripChild) {
	b.Helper()
	child := newRoundTripChild(mode)
	m, started := benchmarkManager(b, child)
	m.ObserveInitialization(started.SessionID, started.Generation, true)
	return m, started, child
}

func TestRoundTripTimeoutCancelsOnceAndTerminalIsImmutable(t *testing.T) {
	m, s, child := roundTripManager(t, "hang")
	got := m.RoundTrip(context.Background(), RoundTripRequest{SessionID: s.SessionID, Generation: s.Generation, Method: "x", Deadline: time.Now().Add(15 * time.Millisecond), MaxMessages: 1, MaxBytes: 1024})
	if got.Failure != session.RequestTimeout {
		t.Fatalf("ASSERT_ROUNDTRIP_TIMEOUT_TERMINAL: %+v", got)
	}
	requests, _ := child.waitForCancels(1)
	cancels := 0
	for _, r := range requests {
		if r.Method == "$/cancelRequest" {
			cancels++
		}
	}
	if cancels != 1 {
		t.Fatalf("ASSERT_ROUNDTRIP_TIMEOUT_CANCEL_ONCE: %d", cancels)
	}
	obs := m.Observations()
	copyObs := m.Observations()
	if !reflect.DeepEqual(obs, copyObs) {
		t.Fatalf("ASSERT_ROUNDTRIP_TERMINAL_IMMUTABLE: first=%+v again=%+v", obs, copyObs)
	}
}
