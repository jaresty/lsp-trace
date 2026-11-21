package sessionruntime

import (
	"context"
	"fmt"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/runtimeprofile"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPrivateDiscoveryOwnedCallsSameRequests(t *testing.T) {
	for _, tc := range []struct {
		mode       string
		directions PrivateCallDirections
	}{
		{"outgoing", PrivateCallsOutgoing}, {"incoming", PrivateCallsIncoming}, {"both", PrivateCallsIncoming | PrivateCallsOutgoing},
		{"withhold-calls", PrivateCallsOutgoing}, {"bad-cleanup", PrivateCallsOutgoing}, {"retention-underfunded", PrivateCallsOutgoing}, {"unreserved-capture", PrivateCallsOutgoing}, {"ignored-failure", PrivateCallsOutgoing},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			root, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			body := []byte("package x\nfunc X() {}\n")
			path := filepath.Join(root, "x.go")
			if err := os.WriteFile(path, body, 0600); err != nil {
				t.Fatal(err)
			}
			uri := (&url.URL{Scheme: "file", Path: path}).String()
			profile, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "test", Workspace: root, Profile: "go", EnvironmentReference: "local"})
			if err != nil {
				t.Fatal(err)
			}
			pipeMode := tc.mode
			if tc.mode == "ignored-failure" {
				pipeMode = "withhold-calls"
			}
			pipe := newOwnedDiscoveryPipe(uri, pipeMode)
			starts := 0
			c := PrivateDiscoveryConstruction{CaptureOwnedMethods: true, CaptureCallDirections: tc.directions, NewStarter: func() (Starter, error) { starts++; return oneChildStarter{pipe}, nil }, Start: StartRequest{Profile: runtimeprofile.Resolve(profile), LanguageID: "go"}, Deadline: time.Now().Add(10 * time.Second), Limits: discoveryLimitsTest(), Members: []PrivateDiscoveryMember{{Path: "x.go", URI: uri, LanguageID: "go"}}}
			if tc.mode == "retention-underfunded" {
				c.Limits.RetainedBytes = 32 << 20
			}
			observed, err := RunPrivateDiscovery(context.Background(), c, func(ctx context.Context, s *PrivateDiscoverySession) error {
				if err := s.Supply(ctx, uri, "go"); err != nil {
					return err
				}
				if _, err := s.DocumentSymbols(ctx, lsp.DocumentSymbolParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}}); err != nil {
					return err
				}
				if tc.mode == "unreserved-capture" {
					lease := SessionLeaseRequest{SessionID: s.SessionID(), Generation: s.Generation(), Workspace: root}
					line, character := uint32(1), uint32(5)
					return s.client.m.WithReadySessionLease(ctx, lease, func(held context.Context) error {
						_, err := s.client.m.CaptureDirectionalEvidencePrivateV2(held, lease, DirectionalCallHierarchyRequestPrivate{Directions: tc.directions, SessionID: lease.SessionID, Generation: lease.Generation, URI: uri, LanguageID: "go", Line: &line, Character: &character, Deadline: c.Deadline, MaxMessages: 32, MaxBytes: PrivateDirectionalEvidenceMaxBytesV2})
						return err
					})
				}
				p := lsp.PrepareCallHierarchyParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}, Position: lsp.Position{Line: 1, Character: 5}}
				for repeat := 0; repeat < 2; repeat++ {
					items, err := s.PrepareCallHierarchy(ctx, p)
					if err != nil {
						t.Logf("immediate directional failure: %v", err)
						if tc.mode == "ignored-failure" {
							return nil
						}
						return err
					}
					if len(items) != 1 {
						return fmt.Errorf("prepared identity missing")
					}
					if tc.directions&PrivateCallsOutgoing != 0 {
						calls, err := s.OutgoingCalls(ctx, items[0])
						if err != nil {
							return err
						}
						if len(calls) != 1 {
							return fmt.Errorf("outgoing lost")
						}
						calls[0].To.Name = "caller mutation"
					}
					if tc.directions&PrivateCallsIncoming != 0 {
						calls, err := s.IncomingCalls(ctx, items[0])
						if err != nil {
							return err
						}
						if len(calls) != 1 {
							return fmt.Errorf("incoming lost")
						}
					}
				}
				return nil
			})
			pipe.mu.Lock()
			methods := append([]string(nil), pipe.methods...)
			pipe.mu.Unlock()
			if starts != 1 {
				t.Fatal("ASSERT_OWNED_CALL_ONCE: duplicate construction", starts)
			}
			if tc.mode == "withhold-calls" || tc.mode == "bad-cleanup" || tc.mode == "retention-underfunded" || tc.mode == "unreserved-capture" || tc.mode == "ignored-failure" {
				if err == nil || observed.OwnedMethods.NativeCallsAvailable() {
					t.Fatal("ASSERT_OWNED_CALL_WITHHOLDS: escaped originals", err)
				}
				if (tc.mode == "retention-underfunded" || tc.mode == "unreserved-capture") && len(methods) != 1 {
					t.Fatal("ASSERT_OWNED_CALL_PREFUND: effect before debit", methods)
				}
				t.Log("ASSERT_OWNED_CALL_WITHHOLDS: PASS", tc.mode, methods)
				return
			}
			if err != nil || observed.Failed || !observed.CleanupJoined {
				t.Fatal("ASSERT_OWNED_CALL_EXECUTION:", err, methods)
			}
			if !observed.OwnedMethods.NativeCallsAvailable() || len(observed.OwnedMethods.state.calls) != 1 {
				t.Fatal("ASSERT_OWNED_CALL_RETENTION: missing original directional owner")
			}
			v := observed.OwnedMethods.state.calls[0].view
			if string(v.Supply.Content) != string(body) || v.Supply.URI != uri || v.OperationID == "" || v.Request.Directions != tc.directions {
				t.Fatal("ASSERT_OWNED_CALL_RETENTION: source/direction binding lost")
			}
			counts := map[string]int{}
			for _, m := range methods {
				counts[m]++
			}
			for _, r := range v.Requests {
				if r.RequestID == 0 || r.Generation != v.Session.Generation || r.Wire.RequestIDPrivate != r.RequestID || len(r.RequestFrame) == 0 || len(r.ResponseFrames) == 0 || len(r.Observations) == 0 {
					t.Fatal("ASSERT_OWNED_CALL_RETENTION: original frame/key lost", r.Method)
				}
				if counts[r.Method] != 1 {
					t.Fatal("ASSERT_OWNED_CALL_ONCE: repeated original request", methods)
				}
			}
			want := 2
			if tc.directions&PrivateCallsIncoming != 0 {
				want++
			} else if counts["callHierarchy/incomingCalls"] != 0 {
				t.Fatal("ASSERT_OWNED_CALL_DIRECTION: unrequested incoming")
			}
			if tc.directions&PrivateCallsOutgoing != 0 {
				want++
			} else if counts["callHierarchy/outgoingCalls"] != 0 {
				t.Fatal("ASSERT_OWNED_CALL_DIRECTION: unrequested outgoing")
			}
			if len(methods) != want || len(v.Requests) != want-1 {
				t.Fatal("ASSERT_OWNED_CALL_ONCE: wrong request denominator", methods, len(v.Requests))
			}
			taken, takeErr := observed.OwnedMethods.TakeNativeCalls(context.Background(), 0)
			wantNative := tc.directions == PrivateCallsIncoming|PrivateCallsOutgoing
			if (takeErr == nil) != wantNative {
				t.Fatal("ASSERT_NATIVE_TRANSFER_DIRECTIONS:", takeErr)
			}
			if wantNative && (len(taken.Requests) != 3 || taken.Requests[0].RequestID != v.Requests[0].RequestID) {
				t.Fatal("ASSERT_NATIVE_TRANSFER_ORIGINALS")
			}
			if _, again := observed.OwnedMethods.TakeNativeCalls(context.Background(), 0); again == nil {
				t.Fatal("ASSERT_NATIVE_TRANSFER_REPLAY: accepted spent slot")
			}
			pipe.mu.Lock()
			after := len(pipe.methods)
			pipe.mu.Unlock()
			if after != len(methods) {
				t.Fatal("ASSERT_NATIVE_TRANSFER_RECAPTURE")
			}
			t.Log("ASSERT_OWNED_CALL_RETENTION: PASS actual detached source/frames/keys", methods)
			t.Log("ASSERT_NATIVE_TRANSFER: PASS direction completeness, one shot, no recapture")
		})
	}
}
