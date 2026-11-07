package sessionruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/runtimeprofile"
)

// All traffic terminates in existing io.Pipe Child machinery. No external
// executable, installed provider, worker, network or application source is used.
func privateSourceFixture(t *testing.T, run func(*Manager, context.Context, PrivateGroupingOwnedHandoff) error) error {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "query.go")
	if err = os.WriteFile(file, []byte("package fixture\nfunc Query() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	profile, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "test", Workspace: root, Profile: "go", EnvironmentReference: "local"})
	if err != nil {
		t.Fatal(err)
	}
	in, stdin := io.Pipe()
	stdout, out := io.Pipe()
	child := &roundTripChild{input: in, stdin: stdin, output: out, stdout: stdout}
	go func() {
		reader := lspwire.NewReader(in, lspwire.DefaultLimits())
		for {
			msg, e := reader.Read()
			if e != nil {
				return
			}
			if len(msg.ID) == 0 {
				continue
			}
			var result any
			switch msg.Method {
			case "initialize":
				result = map[string]any{"serverInfo": map[string]string{"name": "controlled-pipe", "version": "source-v1"}, "capabilities": map[string]any{"positionEncoding": "utf-16", "documentSymbolProvider": true, "referencesProvider": true, "definitionProvider": true}}
			case "textDocument/documentSymbol":
				result = json.RawMessage(`[{"name":"Query","kind":12,"range":{"start":{"line":1,"character":0},"end":{"line":1,"character":15}},"selectionRange":{"start":{"line":1,"character":5},"end":{"line":1,"character":10}}}]`)
			case "textDocument/references", "textDocument/definition":
				result = []any{}
			}
			raw, _ := json.Marshal(result)
			body := fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"result":%s}`, msg.ID, raw)
			if _, e = fmt.Fprintf(out, "Content-Length: %d\r\n\r\n%s", len(body), body); e != nil {
				return
			}
		}
	}()
	m, err := New(Config{Limits: Limits{MaxSessions: 1, MaxRequests: 8, MaxChildren: 8, MaxCancels: 4, MaxTombstones: 16, MaxObservations: 128, MaxOperations: 4}, Starter: oneChildStarter{child}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { child.Close(); child.Teardown(context.Background()); m.Shutdown(context.Background()) })
	deadline := time.Now().Add(5 * time.Second)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	s := m.Start(ctx, StartRequest{Profile: runtimeprofile.Resolve(profile), Deadline: deadline, LanguageID: "go"})
	if s.Failure != "" {
		t.Fatal(s.Failure)
	}
	pending := m.BeginReadiness(ctx, s.SessionID, s.Generation, deadline)
	ready, ok := m.WaitReadiness(ctx, pending.ID)
	if !ok || ready.State != ReadinessReady {
		t.Fatalf("controlled readiness: %+v", ready)
	}
	lease := SessionLeaseRequest{SessionID: s.SessionID, Generation: s.Generation, Workspace: root}
	return m.WithReadySessionLease(ctx, lease, func(leased context.Context) error {
		h, e := m.CapturePrivateGroupingOwned(leased, PrivateGroupingOwnedRequest{Session: lease, URI: (&url.URL{Scheme: "file", Path: file}).String(), OccurrenceID: "controlled-query", Revision: "controlled-revision", Privacy: "private-controlled", Line: 1, Character: 5, References: true, Definitions: true, Deadline: deadline})
		if e != nil {
			return e
		}
		return run(m, leased, h)
	})
}

func TestPrivateGroupingSourceEvidenceOriginalAndOneTake(t *testing.T) {
	var saved PrivateGroupingSourceEvidence
	err := privateSourceFixture(t, func(m *Manager, ctx context.Context, h PrivateGroupingOwnedHandoff) error {
		if _, e := m.TakePrivateGroupingSourceEvidence(ctx, h); e == nil {
			t.Fatal("ASSERT_SOURCE_BEFORE_CONSUME_REFUSED")
		}
		var diagnostic PrivateGroupingOwnedView
		if e := m.ConsumePrivateGroupingOwned(ctx, h, func(v PrivateGroupingOwnedView) error {
			diagnostic = v
			v.Source[0] = '!'
			v.Supply.Content[0] = '!'
			v.Symbol.Result[0] = '!'
			v.Methods[0].Result[0] = '!'
			return nil
		}); e != nil {
			return e
		}
		var e error
		saved, e = m.TakePrivateGroupingSourceEvidence(ctx, h)
		if e != nil {
			return e
		}
		if _, e = m.TakePrivateGroupingSourceEvidence(ctx, h); e == nil {
			t.Fatal("ASSERT_SOURCE_ONE_TAKE")
		}
		v, e := saved.Snapshot(PrivateGroupingQueryMaxBytes)
		if e != nil || v.Source[0] != 'p' || v.Supply.Content[0] != 'p' || v.Symbol.Result[0] != '[' || v.Methods[0].Result[0] != '[' || diagnostic.Source[0] != '!' {
			t.Fatal("ASSERT_SOURCE_RETAINED_ORIGINAL", e)
		}
		v.Source[0] = '?'
		v.Symbol.Result[0] = '?'
		again, e := saved.Snapshot(PrivateGroupingQueryMaxBytes)
		if e != nil || again.Source[0] != 'p' || again.Symbol.Result[0] != '[' {
			t.Fatal("ASSERT_SOURCE_SNAPSHOT_NO_ALIAS", e)
		}
		bound, ok := saved.EncodedBound()
		if !ok {
			t.Fatal("ASSERT_SOURCE_BOUND")
		}
		if _, e := saved.Snapshot(bound - 1); e == nil {
			t.Fatal("ASSERT_SOURCE_BOUND_BEFORE_COPY")
		}
		if _, e := saved.Snapshot(bound); e != nil {
			t.Fatal("ASSERT_SOURCE_EXACT_BOUND", e)
		}
		if _, e := saved.Snapshot(PrivateGroupingQueryMaxBytes + 1); e == nil {
			t.Fatal("ASSERT_SOURCE_NO_CAP_INCREASE")
		}
		t.Log("ASSERT_SOURCE_RETAINED_ORIGINAL: PASS; ASSERT_SOURCE_ONE_TAKE: PASS; ASSERT_SOURCE_SNAPSHOT_NO_ALIAS: PASS; ASSERT_SOURCE_BOUND_BEFORE_COPY: PASS")
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	v, err := saved.Snapshot(PrivateGroupingQueryMaxBytes)
	if err != nil || !bytes.Contains(v.Source, []byte("func Query")) {
		t.Fatal("ASSERT_SOURCE_HISTORICAL_AFTER_LEASE", err)
	}
}

func TestPrivateGroupingSourceEvidenceRefusals(t *testing.T) {
	for _, mode := range []string{"failed-callback", "cancelled-callback", "cancel-after-consume", "forged", "foreign-manager", "after-lease", "stale-document"} {
		t.Run(mode, func(t *testing.T) {
			var manager *Manager
			var expired context.Context
			var handoff PrivateGroupingOwnedHandoff
			err := privateSourceFixture(t, func(m *Manager, ctx context.Context, h PrivateGroupingOwnedHandoff) error {
				manager, expired, handoff = m, ctx, h
				callctx, cancel := context.WithCancel(ctx)
				defer cancel()
				e := m.ConsumePrivateGroupingOwned(callctx, h, func(PrivateGroupingOwnedView) error {
					if mode == "failed-callback" {
						return errors.New("controlled callback failure")
					}
					if mode == "cancelled-callback" {
						cancel()
					}
					return nil
				})
				if mode == "failed-callback" || mode == "cancelled-callback" {
					if e == nil {
						t.Fatal("ASSERT_SOURCE_FAILED_CONSUME")
					}
				} else if e != nil {
					return e
				}
				switch mode {
				case "after-lease":
					return nil
				case "cancel-after-consume":
					cancel()
				case "forged":
					h = PrivateGroupingOwnedHandoff{}
				case "foreign-manager":
					m = &Manager{}
				case "stale-document":
					// Mutate the actual managed document under its owner lock, not the
					// captured view. This is an explicit stale-state intervention.
					m.mu.Lock()
					delete(m.sessions[h.capture.request.Session.SessionID].documents, h.capture.request.URI)
					m.mu.Unlock()
				}
				if _, e = m.TakePrivateGroupingSourceEvidence(callctx, h); e == nil {
					t.Fatal("ASSERT_SOURCE_REFUSAL", mode)
				}
				t.Log("ASSERT_SOURCE_REFUSAL: PASS", mode)
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if mode == "after-lease" {
				if _, e := manager.TakePrivateGroupingSourceEvidence(context.WithoutCancel(expired), handoff); e == nil {
					t.Fatal("ASSERT_SOURCE_REFUSAL", mode)
				}
				t.Log("ASSERT_SOURCE_REFUSAL: PASS", mode)
			}
		})
	}
	if _, e := (PrivateGroupingSourceEvidence{}).Snapshot(PrivateGroupingQueryMaxBytes); e == nil {
		t.Fatal("ASSERT_SOURCE_ZERO_PROOF")
	}
}
