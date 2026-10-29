package sessionruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"
	"time"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/managedprocess"
)

func TestCleanupPrivateV2ResourcesDeathReap(t *testing.T) {
	for _, mode := range []string{"success", "resource-failed", "resource-unknown", "death-unknown", "reap-failed", "reap-unknown"} {
		t.Run(mode, func(t *testing.T) {
			c := newShutdownLoopChild("exact")
			switch mode {
			case "resource-failed":
				c.resource = managedprocess.ResourcesCloseFailed
			case "resource-unknown":
				c.resource = "UNTRUSTED_RESOURCE_TEXT"
			case "death-unknown":
				c.death = "UNTRUSTED_DEATH_TEXT"
			case "reap-failed":
				c.reap = managedprocess.ReapFailed
			case "reap-unknown":
				c.reap = "UNTRUSTED_REAP_TEXT"
			}
			op, _ := shutdownLoopRun(t, c)
			f, ok := op.CleanupObservationPrivateV2()
			if !ok || !f.TeardownAttempted || !f.ResourceCloseAttempted || f.RetiredTransportReused || !f.StderrDrainAssumed {
				t.Fatalf("ASSERT_CLEANUP_FACTS: %+v", f)
			}
			switch mode {
			case "success":
				if f.ResourceClose != managedprocess.ResourcesClosed || f.Death != managedprocess.DeathExited || f.Reap != managedprocess.ReapComplete {
					t.Fatal(f)
				}
			case "resource-failed":
				if f.ResourceClose != managedprocess.ResourcesCloseFailed {
					t.Fatal(f)
				}
			case "resource-unknown":
				if f.ResourceClose != "" {
					t.Fatal(f)
				}
			case "death-unknown":
				if f.Death != "" {
					t.Fatal(f)
				}
			case "reap-failed":
				if f.Reap != managedprocess.ReapFailed {
					t.Fatal(f)
				}
			case "reap-unknown":
				if f.Reap != "" {
					t.Fatal(f)
				}
			}
			raw, _ := json.Marshal(f)
			if bytes.Contains(raw, []byte("UNTRUSTED")) {
				t.Fatal("provider controlled kind leaked")
			}
			if mode != "success" && mode != "death-unknown" && op.State != OperationFailed {
				t.Fatal("resource/reap gate relaxed", op)
			}
		})
	}
}

func TestCleanupPrivateV2RetiredAndAbsent(t *testing.T) {
	if _, ok := (OperationSnapshot{}).CleanupObservationPrivateV2(); ok {
		t.Fatal("absent metadata manufactured")
	}
	c := newShutdownLoopChild("exact")
	// Already retired transport: close fixture once, then install only its retained
	// observations. Stop must not try protocol IO or repeat teardown/close.
	teardown := c.Teardown(context.Background())
	resources := c.Close()
	m, err := New(Config{Limits: Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 1, MaxCancels: 1, MaxTombstones: 1, MaxObservations: 4}, Starter: oneChildStarter{c}})
	if err != nil {
		t.Fatal(err)
	}
	s := m.Start(context.Background(), StartRequest{Profile: profile(t)})
	m.sessions[s.SessionID].retired = &ownedTransport{child: c, teardown: teardown, resources: resources}
	op := m.Stop(context.Background(), s.SessionID, "retired")
	if err := m.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	snap, _ := m.Operation(op.IntentID)
	f, ok := snap.CleanupObservationPrivateV2()
	if !ok || !f.RetiredTransportReused || f.TeardownAttempted || f.ResourceCloseAttempted || f.ShutdownWrite != "" || f.ShutdownRead != "" || f.ExitWrite != "" || c.teardowns.Load() != 1 || c.closes.Load() != 1 || c.shutdowns.Load() != 0 {
		t.Fatalf("ASSERT_RETIRED_TRANSPORT_REUSED_NOT_ACK: %+v", f)
	}
}

func TestCleanupPrivateV2ShutdownWriteFailure(t *testing.T) {
	c := newShutdownLoopChild("exact")
	_ = c.stdin.Close()
	op, _ := shutdownLoopRun(t, c)
	f, ok := op.CleanupObservationPrivateV2()
	if !ok || op.State != OperationFailed || f.ShutdownWrite != CleanupFailedPrivateV2 || f.ShutdownRead != "" || f.ExitWrite != "" {
		t.Fatalf("ASSERT_SHUTDOWN_WRITE_FAILURE: %+v", f)
	}
}

// Return from the underlying interrupted Read is deliberately held. This proves
// terminal publication joins the reader, not merely that Close was requested.
type heldShutdownReader struct {
	io.ReadCloser
	interrupted, release chan struct{}
}

func (r *heldShutdownReader) Read(b []byte) (int, error) {
	n, err := r.ReadCloser.Read(b)
	if err != nil {
		close(r.interrupted)
		<-r.release
	}
	return n, err
}

type heldShutdownChild struct {
	*shutdownLoopChild
	reader *heldShutdownReader
}

func (c *heldShutdownChild) Stdout() io.ReadCloser { return c.reader }
func TestCleanupPrivateV2TimeoutReaderJoined(t *testing.T) {
	c := newShutdownLoopChild("timeout")
	r := &heldShutdownReader{ReadCloser: c.stdout, interrupted: make(chan struct{}), release: make(chan struct{})}
	m, err := New(Config{Limits: Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 1, MaxCancels: 1, MaxTombstones: 1, MaxObservations: 4}, Starter: oneChildStarter{&heldShutdownChild{c, r}}})
	if err != nil {
		t.Fatal(err)
	}
	s := m.Start(context.Background(), StartRequest{Profile: profile(t)})
	stop := m.Stop(context.Background(), s.SessionID, "held-reader")
	joined := make(chan error, 1)
	go func() { joined <- m.Shutdown(context.Background()) }()
	select {
	case <-r.interrupted:
	case <-time.After(3 * time.Second):
		t.Fatal("reader not interrupted by existing teardown")
	}
	select {
	case <-joined:
		t.Fatal("ASSERT_TIMEOUT_READER_JOIN: returned before reader released")
	default:
	}
	close(r.release)
	select {
	case err := <-joined:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("reader join did not complete")
	}
	op, _ := m.Operation(stop.IntentID)
	f, ok := op.CleanupObservationPrivateV2()
	if !ok || f.ShutdownRead != ShutdownTimeoutPrivateV2 || op.State != OperationFailed || c.teardowns.Load() != 1 || c.closes.Load() != 1 {
		t.Fatalf("ASSERT_TIMEOUT_READER_JOIN: %+v %+v", op, f)
	}
}

func TestCleanupPrivateV2PendingAcceptanceNotBypassed(t *testing.T) {
	c := newShutdownLoopChild("exact")
	p := lspwire.NewPendingObserved(2, func(e lspwire.Event) {})
	// Consume the exact key before the response is accepted, using the pending
	// observer at begin. A duplicate response must not authorize exit.
	p = lspwire.NewPendingObserved(2, func(e lspwire.Event) {
		if e.Stage == lspwire.EventPendingBegin {
			p.Accept(lspwire.ResponseKey{Generation: 1, ID: 1})
		}
	})
	m := &Manager{limits: Limits{MaxObservations: 4}, wire: lspwire.DefaultLimits()}
	observed := m.gracefulShutdown(c, p, 1)
	_ = c.Teardown(context.Background())
	_ = c.Close()
	if observed.acknowledged || observed.exitNotified || observed.facts.ShutdownRead != ShutdownUnexpectedPrivateV2 {
		t.Fatal("pending acceptance bypassed", observed)
	}
}

func TestCleanupPrivateV2ErrorsAreContentFree(t *testing.T) {
	secret := errors.New("raw body method comment path error must not survive")
	f := cleanupObservationPrivateV2(LifecycleCleanupObservationPrivateV2{}, managedprocess.TeardownObservation{Death: managedprocess.DeathObservation{Kind: managedprocess.DeathUnknown, Err: secret, Reap: managedprocess.ReapObservation{Kind: managedprocess.ReapFailed, Err: secret}}}, managedprocess.ResourceObservation{Kind: managedprocess.ResourcesCloseFailed, Err: secret}, false)
	raw, _ := json.Marshal(f)
	if !f.ResourceError || !f.DeathError || !f.ReapError || bytes.Contains(raw, []byte(secret.Error())) {
		t.Fatalf("ASSERT_CONTENT_FREE_CLEANUP: %s", raw)
	}
}
