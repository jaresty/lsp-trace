package sessionruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/session"
)

// cFrameProbe is bounded and safe for callbacks from the reader goroutine.
// Overflow fails the test, rather than silently presenting a zero count.
type cFrameProbe struct {
	mu                               sync.Mutex
	entry, decoded, retained, events int
	overflow                         bool
}

func (p *cFrameProbe) addEntry() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.events >= 16 {
		p.overflow = true
		return
	}
	p.events++
	p.entry++
}
func (p *cFrameProbe) addRetain() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.events >= 16 {
		p.overflow = true
		return
	}
	p.events++
	p.retained++
}
func (p *cFrameProbe) addWire(e lspwire.Event) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.events >= 16 {
		p.overflow = true
		return
	}
	p.events++
	if e.Stage == lspwire.EventDecode {
		p.decoded++
	}
}
func (p *cFrameProbe) snapshot() (entry, decoded, retained int, overflow bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.entry, p.decoded, p.retained, p.overflow
}

// The pinned manager-backed B4 path, not only ordinary RoundTrip, must use the
// reader built with the optional observer. No C admission guard is tested here.
func TestCFramePrivateB4ManagerReaderObservation(t *testing.T) {
	f := b4LeaseFixture(t)
	for _, name := range []string{"A", "B"} {
		t.Run(name, func(t *testing.T) {
			b4LeaseExpectation(t, f, name)
			m, s, req, owner, child := b4LeaseManager(t, f, name)
			probe := &cFrameProbe{}
			// The fixture builder has constructed but has not yet issued a request.
			m.cDecodeEntry, m.cWireObserver, m.cRetainEntry = probe.addEntry, probe.addWire, probe.addRetain
			result, lease := m.RoundTripPrivateB4(context.Background(), req, owner)
			select {
			case err := <-child.observed:
				if err != nil {
					t.Fatalf("BLOCKED_NOT_RED: exact WRITE: %v", err)
				}
			case <-time.After(time.Second):
				t.Fatal("BLOCKED_NOT_RED: no exact WRITE observation")
			}
			frame, ok := result.CompletedDefinitionResponseFrame()
			if result.Failure != "" || result.ServerError != nil || lease == (B4DefinitionLease{}) || s.Generation != 1 || !ok || !bytes.Equal(frame, b4LeaseGet(t, f, name+"/response.frame")) {
				t.Fatalf("BLOCKED_NOT_RED: %s pinned selected READ: %+v", name, result)
			}
			entry, decoded, retained, overflow := probe.snapshot()
			if overflow || entry != 1 || decoded != 1 || retained == 0 {
				t.Fatalf("OBSERVATION_B4_READER: entry=%d decoded=%d retained=%d overflow=%v", entry, decoded, retained, overflow)
			}
		})
	}
}

func TestCFrameManagerNilObserverSuccessAndFailureCharacterization(t *testing.T) {
	for _, tc := range []struct {
		mode        string
		maxMessages int
		wantFailure session.Failure
		wantEntries int
	}{
		{"references-empty", 1, "", 1},
		{"malformed", 1, session.SessionPoisoned, 0},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			run := func(withObserver bool) (RoundTripResult, int, int, bool) {
				t.Helper()
				child := newRoundTripChild(tc.mode)
				probe := &cFrameProbe{}
				cfg := Config{Limits: Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 2, MaxCancels: 2, MaxTombstones: 4, MaxObservations: 64, MaxOperations: 2}, Starter: oneChildStarter{child}}
				if withObserver {
					cfg.cDecodeEntry = probe.addEntry
					cfg.cWireObserver = probe.addWire
					cfg.cRetainEntry = probe.addRetain
				}
				manager, err := New(cfg)
				if err != nil {
					t.Fatalf("BLOCKED_NOT_RED: manager: %v", err)
				}
				t.Cleanup(func() {
					_ = child.Teardown(context.Background())
					_ = child.Close()
					_ = manager.Shutdown(context.Background())
				})
				started := manager.Start(context.Background(), StartRequest{Profile: profile(t)})
				if started.SessionID == "" || started.Generation == 0 || manager.ObserveInitialization(started.SessionID, started.Generation, true).State != session.Ready {
					t.Fatal("BLOCKED_NOT_RED: readiness")
				}
				result := manager.RoundTrip(context.Background(), RoundTripRequest{SessionID: started.SessionID, Generation: started.Generation, Method: "textDocument/definition", Params: json.RawMessage(`{"textDocument":{"uri":"file:///a.go"},"position":{"line":0,"character":0}}`), Deadline: time.Now().Add(3 * time.Second), MaxMessages: tc.maxMessages, MaxBytes: 4096})
				if result.RequestMessages != 1 || result.Key.ID == 0 {
					t.Fatalf("BLOCKED_NOT_RED: no WRITE: %+v", result)
				}
				entry, decoded, retained, overflow := probe.snapshot()
				if retained != 0 {
					t.Fatalf("OBSERVATION_UNEXPECTED_RETENTION: %d", retained)
				}
				return result, entry, decoded, overflow
			}
			nilResult, nilEntry, nilDecode, nilOverflow := run(false)
			observed, entry, decoded, overflow := run(true)
			if nilOverflow || nilEntry != 0 || nilDecode != 0 || overflow {
				t.Fatalf("OBSERVATION_PROBE_BOUND: nil=%d/%d/%v observed_overflow=%v", nilEntry, nilDecode, nilOverflow, overflow)
			}
			if nilResult.Failure != tc.wantFailure || observed.Failure != nilResult.Failure || string(observed.Result) != string(nilResult.Result) || observed.Messages != nilResult.Messages || observed.Bytes != nilResult.Bytes || observed.RequestBytes != nilResult.RequestBytes || (observed.ServerError == nil) != (nilResult.ServerError == nil) {
				t.Fatalf("NIL_OBSERVER_EQUIVALENCE: nil=%+v observed=%+v", nilResult, observed)
			}
			if entry != tc.wantEntries || decoded != tc.wantEntries {
				t.Fatalf("OBSERVATION_MANAGER_READ: entry=%d decoded=%d want=%d", entry, decoded, tc.wantEntries)
			}
		})
	}
}
