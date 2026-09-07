package sessionruntime

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/session"
)

type ownedPipeChild struct {
	input  *io.PipeReader
	stdin  *io.PipeWriter
	stdout *io.PipeReader
	output *io.PipeWriter
	once   sync.Once
	closes atomic.Int32
	tears  atomic.Int32
}

func newOwnedPipeChild() *ownedPipeChild {
	input, stdin := io.Pipe()
	stdout, output := io.Pipe()
	return &ownedPipeChild{input: input, stdin: stdin, stdout: stdout, output: output}
}
func (c *ownedPipeChild) Stdin() io.WriteCloser { return c.stdin }
func (c *ownedPipeChild) Stdout() io.ReadCloser { return c.stdout }
func (c *ownedPipeChild) Close() managedprocess.ResourceObservation {
	c.once.Do(func() { c.closes.Add(1); c.stdin.Close(); c.input.Close(); c.stdout.Close(); c.output.Close() })
	return managedprocess.ResourceObservation{Kind: managedprocess.ResourcesClosed}
}
func (c *ownedPipeChild) Teardown(context.Context) managedprocess.TeardownObservation {
	c.tears.Add(1)
	c.Close()
	return managedprocess.TeardownObservation{Death: managedprocess.DeathObservation{Kind: managedprocess.DeathExited, Reap: managedprocess.ReapObservation{Kind: managedprocess.ReapComplete}}}
}

type nextOwnedChild struct{ child *ownedPipeChild }

func (s nextOwnedChild) Start(context.Context, managedprocess.Spec) (Child, managedprocess.StartObservation) {
	return s.child, managedprocess.StartObservation{Kind: managedprocess.StartStarted}
}

func TestSupplyCancelledBeforeOpen(t *testing.T) {
	m, req, _, writer := supplyFixture(t, []byte("package p\n"))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got := m.PrepareDocument(ctx, req)
	if got.Failure != session.RequestCancelled || got.Supply != nil || got.Version != 0 || writer.Len() != 0 || len(m.sessions[req.SessionID].documents) != 0 {
		t.Fatalf("ASSERT_SUPPLY_PRE_CANCEL: %+v bytes=%d", got, writer.Len())
	}
	t.Log("ASSERT_SUPPLY_PRE_CANCEL: PASS")
	if m.Census().Workers != 0 {
		t.Fatal("worker leaked")
	}
}

// A short write with an error is admissible even for conforming io.Writers.
type partialSupplyWriter struct {
	calls    int
	nilError bool
}

func (w *partialSupplyWriter) Write(p []byte) (int, error) {
	w.calls++
	if w.nilError {
		return len(p) / 2, nil
	}
	return len(p) / 2, errors.New("partial notification")
}
func (*partialSupplyWriter) Close() error { return nil }

type partialSupplyChild struct {
	*partialSupplyWriter
	closed atomic.Bool
}

func (c *partialSupplyChild) Stdin() io.WriteCloser { return c.partialSupplyWriter }
func (*partialSupplyChild) Stdout() io.ReadCloser   { return io.NopCloser(&emptyReader{}) }
func (c *partialSupplyChild) Close() managedprocess.ResourceObservation {
	c.closed.Store(true)
	return managedprocess.ResourceObservation{Kind: managedprocess.ResourcesClosed}
}
func (c *partialSupplyChild) Teardown(context.Context) managedprocess.TeardownObservation {
	return managedprocess.TeardownObservation{Death: managedprocess.DeathObservation{Reap: managedprocess.ReapObservation{Kind: managedprocess.ReapComplete}}}
}

type emptyReader struct{}

func (*emptyReader) Read([]byte) (int, error) { return 0, io.EOF }
func TestSupplyPartialWriteRetiresWithoutObservation(t *testing.T) {
	for _, nilError := range []bool{false, true} {
		t.Run(map[bool]string{false: "error", true: "short-without-error"}[nilError], func(t *testing.T) {
			m, req, _, _ := supplyFixture(t, []byte("package p\n"))
			child := &partialSupplyChild{partialSupplyWriter: &partialSupplyWriter{nilError: nilError}}
			m.sessions[req.SessionID].process = child
			got := m.PrepareDocument(context.Background(), req)
			if got.Failure != session.SessionPoisoned || got.Supply != nil || got.Version != 0 || len(m.sessions[req.SessionID].documents) != 0 || !child.closed.Load() {
				t.Fatalf("ASSERT_SUPPLY_PARTIAL: result=%+v closed=%t", got, child.closed.Load())
			}
			t.Log("ASSERT_SUPPLY_PARTIAL: PASS")
		})
	}
}

func TestSupplyInterruptedChangeAndConcurrentLifecycle(t *testing.T) {
	m, req, file, _ := supplyFixture(t, []byte("package p\n"))
	first := m.PrepareDocument(context.Background(), req)
	if first.Supply == nil || first.Version != 1 {
		t.Fatal(first)
	}
	if err := os.WriteFile(file, []byte("package changed\n"), 0600); err != nil {
		t.Fatal(err)
	}
	child := newOwnedPipeChild()
	defer child.Close()
	m.sessions[req.SessionID].process = child
	started := make(chan struct{})
	readerDone := make(chan struct{})
	go func() { defer close(readerDone); b := make([]byte, 1); _, _ = child.input.Read(b); close(started) }()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan DocumentResult, 1)
	go func() { done <- m.PrepareDocument(ctx, req) }()
	select {
	case <-started:
	case <-time.After(time.Second):
		child.Close()
		t.Fatal("write not started")
	}
	// The first byte has been accepted, but the frame has not completed.
	conflicts := make(chan session.Failure, 4)
	go func() { conflicts <- m.Stop(context.Background(), req.SessionID, "stop").Failure }()
	go func() { conflicts <- m.Restart(context.Background(), req.SessionID, "restart").Failure }()
	go func() {
		conflicts <- m.RoundTrip(ctx, RoundTripRequest{SessionID: req.SessionID, Generation: req.Generation, Method: "x"}).Failure
	}()
	go func() { conflicts <- m.PrepareDocument(ctx, req).Failure }()
	for range 4 {
		select {
		case failure := <-conflicts:
			if failure != session.LifecycleConflict {
				t.Errorf("ASSERT_SUPPLY_EXCLUSIVE: %s", failure)
			}
		case <-time.After(time.Second):
			child.Close()
			t.Fatal("ASSERT_SUPPLY_EXCLUSIVE: contender blocked on notification lock")
		}
	}
	if _, err := m.CancelRequest(req.SessionID, lspwire.RequestKey{Generation: req.Generation}); err == nil {
		t.Error("ASSERT_SUPPLY_EXCLUSIVE: manual cancellation accessed the owned stream")
	}
	t.Log("ASSERT_SUPPLY_EXCLUSIVE: PASS")
	cancel()
	select {
	case got := <-done:
		if got.Failure != session.RequestCancelled || got.Version != 0 || got.Supply != nil || m.sessions[req.SessionID].documents[req.URI].version != 1 || first.Supply.DocumentVersion != 1 {
			t.Fatalf("ASSERT_SUPPLY_CHANGE: %+v", got)
		}
		t.Log("ASSERT_SUPPLY_CHANGE: PASS")
	case <-time.After(time.Second):
		child.Close()
		t.Fatal("ASSERT_SUPPLY_CHANGE: cancellation did not join writer")
	}
	<-readerDone
	if m.Census().Workers != 0 || child.closes.Load() != 1 || child.tears.Load() != 1 {
		t.Fatal("ASSERT_SUPPLY_RELEASE: owned work or teardown leaked")
	}
	if got := m.PrepareDocument(context.Background(), req); got.Failure != session.LifecycleConflict {
		t.Fatalf("ASSERT_SUPPLY_RETIRED: %+v", got)
	}
	// Restart only after the old operation has joined; new generation must survive.
	next := newOwnedPipeChild()
	defer next.Close()
	m.starter = nextOwnedChild{next}
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		reader := lspwire.NewReader(next.input, lspwire.DefaultLimits())
		writer := lspwire.NewWriter(next.output, lspwire.DefaultLimits())
		for {
			msg, err := reader.Read()
			if err != nil {
				return
			}
			if len(msg.ID) == 0 {
				continue
			}
			raw := json.RawMessage(`null`)
			if msg.Method == "initialize" {
				raw = json.RawMessage(`{"capabilities":{"callHierarchyProvider":true}}`)
			}
			if writer.Write(lspwire.Message{JSONRPC: lspwire.Version, ID: msg.ID, Result: raw}) != nil {
				return
			}
		}
	}()
	defer func() { next.Close(); <-serverDone }()
	restart := m.Restart(context.Background(), req.SessionID, "after-cancel")
	if restart.Failure != "" {
		t.Fatal(restart)
	}
	deadline := time.Now().Add(time.Second)
	for {
		op, _ := m.Operation(restart.IntentID)
		if op.State != OperationPending {
			if op.State != OperationComplete {
				t.Fatal(op)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("restart did not complete")
		}
		time.Sleep(time.Millisecond)
	}
	if next.closes.Load() != 0 {
		t.Fatal("ASSERT_SUPPLY_NEW_GENERATION: replacement was closed")
	}
	if got := m.RoundTrip(context.Background(), RoundTripRequest{SessionID: req.SessionID, Generation: req.Generation, Method: "old"}); got.Failure != session.StaleGeneration {
		t.Fatal(got)
	}
	req.Generation++
	got := m.PrepareDocument(context.Background(), req)
	if got.Failure != "" || got.Version != 1 || got.Supply == nil {
		t.Fatalf("ASSERT_SUPPLY_NEW_GENERATION: %+v", got)
	}
	t.Log("ASSERT_SUPPLY_NEW_GENERATION: PASS")
}

func TestSupplyRetiredStop(t *testing.T) {
	m, req, _, _ := supplyFixture(t, []byte("package p\n"))
	child := &partialSupplyChild{partialSupplyWriter: &partialSupplyWriter{}}
	m.sessions[req.SessionID].process = child
	if got := m.PrepareDocument(context.Background(), req); got.Failure != session.SessionPoisoned {
		t.Fatal(got)
	}
	stop := m.Stop(context.Background(), req.SessionID, "retired-stop")
	if stop.Failure != "" {
		t.Fatal(stop)
	}
	if op := waitOperation(t, m, stop.IntentID, OperationComplete); op.Failure != "" {
		t.Fatal(op)
	}
	if c := m.Census(); c.Workers != 0 || c.Sessions != 0 {
		t.Fatalf("ASSERT_SUPPLY_RETIRED_STOP: %+v", c)
	}
	t.Log("ASSERT_SUPPLY_RETIRED_STOP: PASS")
}

func TestOwnedRoundTripStalledIO(t *testing.T) {
	for _, mode := range []string{"write", "read-and-cancel-write"} {
		t.Run(mode, func(t *testing.T) {
			m, req, _, _ := supplyFixture(t, []byte("package p\n"))
			child := newOwnedPipeChild()
			defer child.Close()
			m.sessions[req.SessionID].process = child
			served := make(chan struct{})
			if mode == "read-and-cancel-write" {
				go func() { defer close(served); _, _ = lspwire.NewReader(child.input, lspwire.DefaultLimits()).Read() }()
			} else {
				close(served)
			}
			done := make(chan RoundTripResult, 1)
			go func() {
				done <- m.RoundTrip(context.Background(), RoundTripRequest{SessionID: req.SessionID, Generation: req.Generation, Method: "x", Deadline: time.Now().Add(5 * time.Millisecond)})
			}()
			select {
			case got := <-done:
				if got.Failure != session.RequestTimeout || m.Census().Workers != 0 || m.Census().Requests != 0 || child.tears.Load() != 1 {
					t.Fatalf("ASSERT_ROUNDTRIP_STALL: %+v %+v teardown=%d", got, m.Census(), child.tears.Load())
				}
				t.Log("ASSERT_ROUNDTRIP_STALL: PASS")
			case <-time.After(500 * time.Millisecond):
				child.Close()
				<-done
				t.Fatal("ASSERT_ROUNDTRIP_STALL: deadline failed to interrupt owned I/O")
			}
			<-served
		})
	}
}
