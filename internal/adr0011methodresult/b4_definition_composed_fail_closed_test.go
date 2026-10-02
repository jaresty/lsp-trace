package adr0011methodresult

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"testing"
	"time"

	v5 "lsp-trace/internal/adr0011genericv5proposal"
	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/internal/session"
	"lsp-trace/sessionruntime"
)

// Each negative uses a fresh manager-held lease. Caller-supplied replay data
// may not substitute for the selected WRITE or READ, even with the same wire ID.
func TestADR0011PrivateComposedDefinitionFailClosed(t *testing.T) {
	root := filepath.Join("testdata", "adr0011-composed-b4-manager-id1-held-v1")
	assets := bridgeManifestAssets(t, filepath.Join(root, "manifest.json"), composedManifestSHA)
	cases := []struct {
		name, fixture string
		response      []byte
		mutate        func(*composedNegativeInput)
	}{
		{name: "cross-transaction", fixture: "A", mutate: func(v *composedNegativeInput) { v.replay.Write.Transaction = "another-transaction" }},
		{name: "substituted-write", fixture: "A", mutate: func(v *composedNegativeInput) {
			v.replay.Write.RequestFrame = append([]byte(nil), v.replay.Write.RequestFrame...)
			v.replay.Write.RequestFrame[0] = 'X'
		}},
		{name: "mismatched-typed-ID", fixture: "A", mutate: func(v *composedNegativeInput) { v.replay.Write.RequestID = []byte(`"1"`) }},
		{name: "mismatched-selection", fixture: "A", mutate: func(v *composedNegativeInput) { v.selection.CompletedOwnerKey = "foreign-owner" }},
		{name: "missing-target-source", fixture: "B", mutate: func(v *composedNegativeInput) { delete(v.sources, "file:///w/target-b.go") }},
		{name: "malformed-selected-result", fixture: "A", response: []byte(`{"jsonrpc":"2.0","id":1,"result":[{}]}`)},
		{name: "duplicate-selected-result-member", fixture: "A", response: []byte(`{"jsonrpc":"2.0","id":1,"result":{"uri":"file:///w/target-a.go","uri":"file:///w/target-a.go","range":{"start":{"line":1,"character":5},"end":{"line":1,"character":12}}}}`)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			replay, occurrence, sources, _, buildChild, req, owner, profile := composedFixture(t, assets, root, tc.fixture)
			validated, err := runtimeprofile.Validate(profile)
			if err != nil {
				bridgeFixtureFatal(t, "profile: %v", err)
			}
			var child *composedChild
			if tc.response == nil {
				child = buildChild()
			} else {
				input, stdin := io.Pipe()
				stdout, output := io.Pipe()
				child = &composedChild{input: input, stdin: stdin, output: output, stdout: stdout, observed: make(chan error, 1)}
				expected := append([]byte(nil), replay.Write.RequestFrame...)
				framed := append([]byte(fmt.Sprintf("Content-Length: %d\r\n\r\n", len(tc.response))), tc.response...)
				go func() {
					reader := lspwire.NewReader(input, lspwire.DefaultLimits())
					msg, _, frame, retained, e := reader.ReadWithFrameIfWithin(4096)
					if e != nil || !retained || !bytes.Equal(frame, expected) || msg.Method != "textDocument/definition" {
						child.observed <- fmt.Errorf("request control: %v", e)
						return
					}
					_, e = output.Write(framed)
					child.observed <- e
				}()
				req.CaptureDefinitionResponseFrameMaxBytes = int64(len(framed))
			}
			manager, err := sessionruntime.New(sessionruntime.Config{Limits: sessionruntime.Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 2, MaxCancels: 2, MaxTombstones: 4, MaxObservations: 64, MaxOperations: 2}, Starter: composedStarter{child}})
			if err != nil {
				bridgeFixtureFatal(t, "manager: %v", err)
			}
			t.Cleanup(func() {
				_ = child.Teardown(context.Background())
				_ = child.Close()
				_ = manager.Shutdown(context.Background())
			})
			started := manager.Start(context.Background(), sessionruntime.StartRequest{Profile: runtimeprofile.Resolve(validated)})
			if started.SessionID != req.SessionID || started.Generation != req.Generation || manager.ObserveInitialization(started.SessionID, started.Generation, true).State != session.Ready {
				bridgeFixtureFatal(t, "manager readiness")
			}
			result, lease := manager.RoundTripPrivateB4(context.Background(), req, owner)
			if result.Failure != "" || result.ServerError != nil || lease == (sessionruntime.B4DefinitionLease{}) || result.Key != (lspwire.RequestKey{Generation: 1, ID: 1}) {
				bridgeFixtureFatal(t, "private selection failure=%s", result.Failure)
			}
			select {
			case e := <-child.observed:
				if e != nil {
					bridgeFixtureFatal(t, "request/response control: %v", e)
				}
			case <-time.After(3 * time.Second):
				bridgeFixtureFatal(t, "request/response control timeout")
			}
			state := composedNegativeInput{replay: replay, selection: sessionruntime.B4DefinitionSelectionKey{SessionID: started.SessionID, Key: result.Key, Transaction: owner.Transaction, CompletedOwnerKey: owner.CompletedOwnerKey}, sources: sources}
			if tc.mutate != nil {
				tc.mutate(&state)
			}
			got := CheckPrivateComposedB4Definition(manager, lease, state.selection, state.replay, occurrence, state.sources)
			if len(got.Candidates) != 0 || got.Status == DefinitionBridgeCandidateItems || got.Status == DefinitionBridgeCandidateEmpty || got.Accepted || got.Authority != 0 || got.Completeness != "UNKNOWN" || got.ClaimCeiling != "NO_PRODUCER_AUTHENTICATION" {
				t.Fatalf("nonclosed %s: %+v", tc.name, got)
			}
		})
	}
}

type composedNegativeInput struct {
	replay    v5.B4bFullCandidateInput
	selection sessionruntime.B4DefinitionSelectionKey
	sources   map[string][]byte
}
