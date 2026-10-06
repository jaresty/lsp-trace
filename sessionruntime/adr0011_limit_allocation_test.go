package sessionruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/session"
)

func TestADR0011PrivateLimitEligibilityBranches(t *testing.T) {
	binding := &OwnedDocumentBinding{URI: "file:///a.go", Version: 1, SHA256: "sha256:abc"}
	base := RoundTripRequest{SessionID: "s", Generation: 1, Method: "textDocument/references", Params: json.RawMessage(`{"textDocument":{"uri":"file:///a.go"}}`), CaptureOwnedMethodPair: true, ExpectedOwnedDocument: binding, MaxMessages: 4, MaxBytes: 4194304}
	if eligibleOwnedMethodRequest(base) {
		t.Fatal("unmarked 4 MiB request admitted")
	}
	marked := base
	marked.ADR0011PrivateLimitAllocationV1 = true
	if !eligibleOwnedMethodRequest(marked) {
		t.Fatal("marked bounded request withheld")
	}
	for _, tc := range []struct {
		name   string
		mutate func(*RoundTripRequest)
	}{
		{"fifth-message", func(r *RoundTripRequest) { r.MaxMessages = 5 }},
		{"over-aggregate", func(r *RoundTripRequest) { r.MaxBytes++ }},
		{"no-document", func(r *RoundTripRequest) { r.ExpectedOwnedDocument = nil }},
		{"over-params", func(r *RoundTripRequest) { r.Params = json.RawMessage(`"` + string(make([]byte, 65536)) + `"`) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := marked
			tc.mutate(&r)
			if eligibleOwnedMethodRequest(r) {
				t.Fatal("marked invalid request admitted")
			}
		})
	}
	for _, method := range []string{"textDocument/definition", "textDocument/references", "textDocument/documentSymbol"} {
		r := base
		r.Method = method
		r.MaxBytes = 1 << 20
		r.MaxMessages = 5
		if !eligibleOwnedMethodRequest(r) {
			t.Fatalf("historical %s eligibility lost", method)
		}
		r.ADR0011PrivateLimitAllocationV1 = true
		if eligibleOwnedMethodRequest(r) != (method == "textDocument/definition") {
			t.Fatalf("marked %s five-message boundary", method)
		}
	}
}

// This synthetic Manager request deliberately exceeds the Owner params cap.
// It witnesses the pre-WRITE seam, not private issuance or Owner reachability.
func TestADR0011ManagerFramedWriteBoundaryOwnerUnreachable(t *testing.T) {
	for _, delta := range []int{0, 1} {
		t.Run(map[int]string{0: "exact", 1: "plus-one"}[delta], func(t *testing.T) {
			input, stdin := io.Pipe()
			stdout, output := io.Pipe()
			child := &roundTripChild{input: input, stdin: stdin, stdout: stdout, output: output}
			limits := lspwire.DefaultLimits()
			limits.MaxBodyBytes = 8 << 20
			done := make(chan struct{})
			go func() {
				defer close(done)
				reader := lspwire.NewReader(input, limits)
				writer := lspwire.NewWriter(output, limits)
				for {
					msg, err := reader.Read()
					if err != nil {
						return
					}
					child.mu.Lock()
					child.requests = append(child.requests, msg)
					child.mu.Unlock()
					_ = writer.Write(lspwire.Message{JSONRPC: lspwire.Version, ID: msg.ID, Result: json.RawMessage(`[]`)})
				}
			}()
			m, err := New(Config{Limits: Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 2, MaxCancels: 2, MaxTombstones: 4, MaxObservations: 64, MaxOperations: 2}, Wire: limits, Starter: oneChildStarter{child}})
			if err != nil {
				t.Fatal(err)
			}
			defer m.Shutdown(context.Background())
			s := m.Start(context.Background(), StartRequest{Profile: profile(t)})
			if got := m.ObserveInitialization(s.SessionID, s.Generation, true); got.State != session.Ready {
				t.Fatal(got)
			}
			req := RoundTripRequest{SessionID: s.SessionID, Generation: s.Generation, Method: "textDocument/references", Deadline: time.Now().Add(3 * time.Second), MaxMessages: 1, MaxBytes: 4194304, CaptureOwnedMethodPair: true, ADR0011PrivateLimitAllocationV1: true}
			// The first pending key in this fresh session is one. Marshaling this very
			// message measures the encoder's header-width transition, not an estimate.
			bodyFor := func(n int) int {
				p := json.RawMessage(`{"padding":"` + strings.Repeat("x", n) + `"}`)
				b, e := json.Marshal(lspwire.Message{JSONRPC: lspwire.Version, ID: json.RawMessage(`1`), Method: req.Method, Params: p})
				if e != nil {
					t.Fatal(e)
				}
				return len(b)
			}
			n := 4194304
			for canonicalRequestFrameBytes(bodyFor(n)) > 4194304 {
				n--
			}
			if canonicalRequestFrameBytes(bodyFor(n)) != 4194304 {
				t.Fatal("unreachable exact canonical frame")
			}
			req.Params = json.RawMessage(`{"padding":"` + strings.Repeat("x", n+delta) + `"}`)
			// The boundary-search loop can exceed the fixture's original deadline under -race.
			req.Deadline = time.Now().Add(3 * time.Second)
			got := m.RoundTrip(context.Background(), req)
			requests, _ := child.snapshot()
			count := 0
			for _, item := range requests {
				if item.Method == req.Method {
					count++
				}
			}
			if delta == 0 {
				if got.Failure != "" || count != 1 {
					t.Fatalf("at cap: failure=%s writes=%d", got.Failure, count)
				}
				obs, ok := got.CompletedRequestWrite()
				if !ok || obs.FrameBytes != 4194304 {
					t.Fatalf("at cap observation: %+v %v", obs, ok)
				}
			} else {
				if got.Failure != session.ResourceExhausted || count != 0 {
					t.Fatalf("over cap: failure=%s writes=%d", got.Failure, count)
				}
				if _, ok := got.CompletedRequestWrite(); ok {
					t.Fatal("over cap completed write")
				}
				if got.Key.ID == 0 || m.Census().Requests != 0 {
					t.Fatalf("pending request leaked: key=%+v census=%+v", got.Key, m.Census())
				}
			}
			if _, ok := got.CompletedOwnedMethodPair(); ok {
				t.Fatal("oversized params yielded owned pair")
			}
			// Frame bytes are never returned as failure text.
			if bytes.Contains([]byte(got.Failure), []byte("padding")) {
				t.Fatal("raw params in failure")
			}
		})
	}
}

func TestADR0011ManagerPairMarkedVersusUnmarked(t *testing.T) {
	for _, tc := range []struct {
		name     string
		marked   bool
		maxBytes int64
		messages int
		wantPair bool
	}{
		{"unmarked-four-meg", false, 4194304, 1, false},
		{"marked-four-meg", true, 4194304, 1, true},
		{"unmarked-historical-five", false, 1 << 20, 5, true},
		{"marked-five", true, 1 << 20, 5, false},
		{"marked-over-four-meg", true, 4194305, 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, s, child, source := ownedDocumentFixture(t, true)
			defer m.Shutdown(context.Background())
			req := documentSymbolOwnedRequest(s, source)
			req.MaxBytes = tc.maxBytes
			req.MaxMessages = tc.messages
			req.ADR0011PrivateLimitAllocationV1 = tc.marked
			got := m.RoundTrip(context.Background(), req)
			_, ok := got.CompletedOwnedMethodPair()
			wantWrites := 1
			wantFailure := session.Failure("")
			if !tc.wantPair {
				// An explicitly expected document cannot be matched when the
				// pair policy is ineligible: the Manager rejects before dispatch.
				wantWrites = 0
				wantFailure = DocumentSupplyUnavailable
			}
			if got.Failure != wantFailure || ok != tc.wantPair {
				t.Fatalf("failure=%s pair=%v want=%v", got.Failure, ok, tc.wantPair)
			}
			requests, _ := child.snapshot()
			count := 0
			for _, r := range requests {
				if r.Method == req.Method {
					count++
				}
			}
			if count != wantWrites {
				t.Fatalf("written requests=%d want=%d", count, wantWrites)
			}
		})
	}
}

func TestADR0011ManagerAggregateAndMessageBoundary(t *testing.T) {
	for _, tc := range []struct {
		name          string
		notifications int
		overBytes     bool
		wantFailure   session.Failure
	}{
		{"four-at-aggregate", 3, false, ""},
		{"four-over-aggregate", 3, true, session.ResourceExhausted},
		{"fifth-message", 4, false, session.ResourceExhausted},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input, stdin := io.Pipe()
			stdout, output := io.Pipe()
			child := &roundTripChild{input: input, stdin: stdin, stdout: stdout, output: output}
			limits := lspwire.DefaultLimits()
			limits.MaxBodyBytes = 8 << 20
			go func() {
				reader := lspwire.NewReader(input, limits)
				writer := lspwire.NewWriter(output, limits)
				msg, err := reader.Read()
				if err != nil {
					return
				}
				// Count precisely the same decoded messages the Manager counts.
				notice := lspwire.Message{JSONRPC: lspwire.Version, Method: "window/logMessage", Params: json.RawMessage(`{"padding":""}`)}
				response := lspwire.Message{JSONRPC: lspwire.Version, ID: msg.ID, Result: json.RawMessage(`[]`)}
				base, _ := json.Marshal(notice)
				end, _ := json.Marshal(response)
				pad := 4194304 - len(end) - tc.notifications*len(base)
				if tc.overBytes {
					pad++
				}
				for i := 0; i < tc.notifications; i++ {
					current := notice
					if i == 0 {
						current.Params = json.RawMessage(`{"padding":"` + strings.Repeat("x", pad) + `"}`)
					}
					if writer.Write(current) != nil {
						return
					}
				}
				_ = writer.Write(response)
			}()
			m, err := New(Config{Limits: Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 2, MaxCancels: 2, MaxTombstones: 4, MaxObservations: 64, MaxOperations: 2}, Wire: limits, Starter: oneChildStarter{child}})
			if err != nil {
				t.Fatal(err)
			}
			defer m.Shutdown(context.Background())
			s := m.Start(context.Background(), StartRequest{Profile: profile(t)})
			if m.ObserveInitialization(s.SessionID, s.Generation, true).State != session.Ready {
				t.Fatal("not ready")
			}
			req := RoundTripRequest{SessionID: s.SessionID, Generation: s.Generation, Method: "textDocument/references", Params: json.RawMessage(`{}`), Deadline: time.Now().Add(3 * time.Second), MaxMessages: 4, MaxBytes: 4194304, CaptureOwnedMethodPair: true, ADR0011PrivateLimitAllocationV1: true}
			got := m.RoundTrip(context.Background(), req)
			if got.Failure != tc.wantFailure {
				t.Fatalf("failure=%s want=%s messages=%d bytes=%d", got.Failure, tc.wantFailure, got.Messages, got.Bytes)
			}
			if !tc.overBytes && got.Bytes != 4194304 && tc.notifications == 3 {
				t.Fatalf("aggregate=%d", got.Bytes)
			}
			if tc.notifications == 4 && got.Messages != 4 {
				t.Fatalf("fifth not bounded: %d", got.Messages)
			}
			if _, ok := got.CompletedOwnedMethodPair(); ok {
				t.Fatal("synthetic oversized notification yielded pair")
			}
		})
	}
}

func TestADR0011CanonicalRequestFrameCountEdge(t *testing.T) {
	const capBytes int64 = 4194304
	// The header width changes with body length; search for the exact edge.
	body := int(capBytes)
	for canonicalRequestFrameBytes(body) > capBytes {
		body--
	}
	if canonicalRequestFrameBytes(body) != capBytes || canonicalRequestFrameBytes(body+1) != capBytes+1 {
		t.Fatal("canonical framed count edge")
	}
	if body <= 65536 {
		t.Fatal("unexpected owner reachability")
	}
}
