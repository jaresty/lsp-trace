package inventorysuccessor_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"lsp-trace/internal/programcadmission"
	"lsp-trace/internal/referencegrouping"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/inventorysuccessor"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/sessionruntime"
)

// This responder is only the current controlled test executable, never a real provider.
func TestPrivateInventoryMixedLeaseReplayV2SELFResponder(t *testing.T) {
	i := -1
	for n, a := range os.Args {
		if a == "mixed-lease-self-v2" {
			i = n
			break
		}
	}
	if i < 0 {
		return
	}
	if len(os.Args) != i+2 && len(os.Args) != i+3 {
		os.Exit(71)
	}
	mode := "positive"
	if len(os.Args) == i+3 {
		mode = os.Args[i+2]
	}
	mark := func(event string) {
		if mode != "funding-counter" {
			return
		}
		f, e := os.OpenFile(filepath.Join(os.Args[i+1], "controlled-effects"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if e != nil {
			os.Exit(73)
		}
		_, e = f.WriteString(event + "\n")
		f.Close()
		if e != nil {
			os.Exit(74)
		}
	}
	mark("start")
	uri := (&url.URL{Scheme: "file", Path: filepath.Join(os.Args[i+1], "root.go")}).String()
	r := lsp.Range{Start: lsp.Position{Line: 1}, End: lsp.Position{Line: 1, Character: 21}}
	sel := lsp.Range{Start: lsp.Position{Line: 1, Character: 5}, End: lsp.Position{Line: 1, Character: 9}}
	site := lsp.Range{Start: lsp.Position{Line: 1, Character: 13}, End: lsp.Position{Line: 1, Character: 17}}
	item := lsp.CallHierarchyItem{Name: "Root", URI: uri, Kind: 12, Range: r, SelectionRange: sel}
	reader := lspwire.NewReader(os.Stdin, lspwire.DefaultLimits())
	for {
		msg, err := reader.Read()
		if err != nil || msg.Method == "exit" {
			os.Exit(0)
		}
		if len(msg.ID) == 0 {
			continue
		}
		if msg.Method == "textDocument/documentSymbol" || msg.Method == "textDocument/references" {
			mark("acquisition")
		}
		var result any
		switch msg.Method {
		case "initialize":
			result = map[string]any{"serverInfo": map[string]string{"name": "controlled-SELF", "version": "mixed-lease-v2"}, "capabilities": map[string]any{"positionEncoding": "utf-16", "callHierarchyProvider": true, "documentSymbolProvider": true, "referencesProvider": mode != "unavailable", "definitionProvider": mode != "unavailable"}}
		case "textDocument/documentSymbol":
			result = []any{map[string]any{"name": "Root", "kind": 12, "range": r, "selectionRange": sel}}
		case "textDocument/references", "textDocument/definition":
			if mode == "expiry" {
				time.Sleep(700 * time.Millisecond)
			}
			result = []any{map[string]any{"uri": uri, "range": site}}
			if mode == "empty" {
				result = []any{}
			}
		case "textDocument/prepareCallHierarchy":
			result = []lsp.CallHierarchyItem{item}
		case "callHierarchy/incomingCalls":
			result = []lsp.CallHierarchyIncomingCall{{From: item, FromRanges: []lsp.Range{site, site}}}
		case "callHierarchy/outgoingCalls":
			result = []lsp.CallHierarchyOutgoingCall{{To: item, FromRanges: []lsp.Range{site, site}}}
		}
		raw, _ := json.Marshal(result)
		body := fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"result":%s}`, msg.ID, raw)
		if mode == "frame-prefix" && (msg.Method == "textDocument/prepareCallHierarchy" || strings.HasPrefix(msg.Method, "callHierarchy/")) {
			notice := `{"jsonrpc":"2.0","method":"window/logMessage","params":{"type":3,"message":"controlled prefix"}}`
			if _, err = fmt.Fprintf(os.Stdout, "Content-Length: %d\r\n\r\n%s", len(notice), notice); err != nil {
				os.Exit(75)
			}
			body = fmt.Sprintf(` { "jsonrpc" : "2.0", "id" : %s, "result" : %s } `, msg.ID, raw)
			if _, err = fmt.Fprintf(os.Stdout, "Content-Type: application/vscode-jsonrpc; charset=utf-8\r\nContent-Length: %d\r\n\r\n%s", len(body), body); err != nil {
				os.Exit(76)
			}
			continue
		}
		if _, err = fmt.Fprintf(os.Stdout, "Content-Length: %d\r\n\r\n%s", len(body), body); err != nil {
			os.Exit(72)
		}
	}
}

func mixedStartFixtureV2(t *testing.T) (*sessionruntime.Manager, sessionruntime.StartRequest, string) {
	t.Helper()
	if runtime.GOOS != "darwin" {
		t.Skip("controlled local Darwin SELF only; Linux compile-only")
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, "root.go"), []byte("package fixture\nfunc Root(){ Root() }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	supervisor, err := managedprocess.NewLocalDarwinSupervisor(managedprocess.Options{StderrLimit: 65536, GracePeriod: 100 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	m, err := sessionruntime.New(sessionruntime.Config{Limits: sessionruntime.Limits{MaxSessions: 1, MaxRequests: 8, MaxChildren: 8, MaxCancels: 4, MaxTombstones: 16, MaxObservations: 128, MaxOperations: 4}, Starter: sessionruntime.ManagedStarter{Manager: supervisor}})
	if err != nil {
		t.Fatal(err)
	}
	selected, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "test", Workspace: root, Profile: "go", EnvironmentReference: "local"})
	if err != nil {
		t.Fatal(err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(15 * time.Second)
	start := sessionruntime.StartRequest{Profile: runtimeprofile.Resolve(selected), LanguageID: "go", Deadline: deadline, Process: managedprocess.Spec{Path: exe, Dir: root, Args: []string{"-test.run=^TestPrivateInventoryMixedLeaseReplayV2SELFResponder$", "--", "mixed-lease-self-v2", root}}}
	t.Cleanup(func() {
		c, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		m.Shutdown(c)
	})
	return m, start, (&url.URL{Scheme: "file", Path: filepath.Join(root, "root.go")}).String()
}

func mixedLeaseFixtureV2(t *testing.T, modes ...string) (*sessionruntime.Manager, inventorysuccessor.PrivateNativeCallsRequestV2, context.Context) {
	m, start, uri := mixedStartFixtureV2(t)
	start.Process.Args = append(start.Process.Args, modes...)
	root, deadline := start.Profile.Workspace().String(), start.Deadline
	envelope, err := m.PrivateProviderWindowEnvelope(start, sessionruntime.PrivateProviderWindowPlan{Calls: true, References: true, Definitions: true})
	if err != nil {
		t.Fatal(err)
	}
	ctx, err := m.PrivateProviderWindowContext(context.Background(), envelope, deadline)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = m.FinishPrivateProviderWindow(ctx)
		c, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		m.Shutdown(c)
	})
	s := m.Start(ctx, start)
	if s.Failure != "" {
		t.Fatal(s.Failure)
	}
	pending := m.BeginReadiness(ctx, s.SessionID, s.Generation, deadline)
	ready, ok := m.WaitReadiness(ctx, pending.ID)
	if !ok || ready.State != sessionruntime.ReadinessReady {
		t.Fatalf("readiness: %+v", ready)
	}
	line, ch := uint32(1), uint32(6)
	q := inventorysuccessor.PrivateNativeCallsRequestV2{Session: sessionruntime.SessionLeaseRequest{SessionID: s.SessionID, Generation: s.Generation, Workspace: root}, Directional: sessionruntime.DirectionalCallHierarchyRequestPrivate{SessionID: s.SessionID, Generation: s.Generation, URI: uri, LanguageID: "go", Line: &line, Character: &ch, Deadline: deadline, MaxMessages: 4, MaxBytes: 65536}, Revision: "controlled-mixed-revision", Privacy: "private-mixed-only"}
	return m, q, ctx
}

// Compare with the manager's existing upstream observer, not replay's computed
// ledger. This fails even when producer and replay agree on the same omission.
func TestPrivateInventoryMixedLeaseReplayV2ActualPrepareEmissions(t *testing.T) {
	for _, mode := range []string{"positive", "frame-prefix"} {
		t.Run(mode, func(t *testing.T) {
			m, q, ctx := mixedLeaseFixtureV2(t, mode)
			err := m.WithReadySessionLease(ctx, q.Session, func(base context.Context) error {
				base, cancel := context.WithDeadline(base, q.Directional.Deadline)
				defer cancel()
				_, err := m.CapturePrivateGroupingOwned(base, sessionruntime.PrivateGroupingOwnedRequest{Session: q.Session, URI: q.Directional.URI, OccurrenceID: "prepare-observer", Revision: q.Revision, Privacy: q.Privacy, Line: *q.Directional.Line, Character: *q.Directional.Character, References: true, Definitions: true, Deadline: q.Directional.Deadline})
				if err != nil {
					return err
				}
				h, err := m.CaptureDirectionalEvidencePrivateV2(base, q.Session, q.Directional)
				if err != nil {
					return err
				}
				actual := m.DirectionalObservationsPrivate()
				v, err := m.TakeDirectionalEvidencePrivateV2(base, h)
				if err != nil {
					return err
				}
				if len(v.Requests) != 3 {
					return fmt.Errorf("prepare fixture request count")
				}
				r := v.Requests[0]
				observed := r.Observations[:0:0]
				for _, o := range actual {
					if o.RequestID == r.RequestID && o.Generation == r.Generation && o.Method == r.Method {
						observed = append(observed, o)
					}
				}
				wantCount := 3
				if mode == "frame-prefix" {
					wantCount = 4
				}
				if len(observed) != wantCount || observed[0].Component != "request-frame" || observed[len(observed)-1].Component != "element" || !observed[len(observed)-1].Selected {
					return fmt.Errorf("upstream prepare observer fixture incomplete: %+v", observed)
				}
				if !reflect.DeepEqual(r.Observations, observed) {
					return fmt.Errorf("ASSERT_ACTUAL_PREPARE_EMISSIONS: detached=%+v actual=%+v", r.Observations, observed)
				}
				t.Logf("ASSERT_ACTUAL_PREPARE_EMISSIONS: PASS; mode=%s actual=%d detached=%d", mode, len(observed), len(r.Observations))
				return m.CheckTakenDirectionalEvidencePrivateV2(base, h)
			})
			if err != nil {
				t.Fatal(err)
			}
			completion, err := m.FinishPrivateProviderWindow(ctx)
			if err != nil || !completion.Completed || m.Census().Workers != 0 {
				t.Fatalf("prepare fixture join: %v %+v", err, completion)
			}
		})
	}
}

func TestPrivateInventoryMixedLeaseReplayV2Ownership(t *testing.T) {
	m, q, ctx := mixedLeaseFixtureV2(t)
	called := false
	err := m.WithReadySessionLease(ctx, q.Session, func(base context.Context) error {
		h, err := m.CapturePrivateGroupingOwned(base, sessionruntime.PrivateGroupingOwnedRequest{Session: q.Session, URI: q.Directional.URI, OccurrenceID: "controlled-mixed", Revision: q.Revision, Privacy: q.Privacy, Line: *q.Directional.Line, Character: *q.Directional.Character, References: true, Definitions: true, Deadline: q.Directional.Deadline})
		if err != nil {
			return fmt.Errorf("genuine RD capture prerequisite: %w", err)
		}
		native, err := inventorysuccessor.ConstructNativeCallsPrivateV2(base, m, q)
		if err != nil || !native.Admission.Admission.Valid() || len(native.RawJoins) != 4 {
			return fmt.Errorf("genuine C capture prerequisite: %v", err)
		}
		short, stop := context.WithDeadline(base, q.Directional.Deadline.Add(-time.Second))
		defer stop()
		for name, bad := range map[string]context.Context{"stripped": context.Background(), "outer": ctx, "subset-deadline": short} {
			entered := false
			if e := m.ConsumePrivateGroupingOwned(bad, h, func(sessionruntime.PrivateGroupingOwnedView) error { entered = true; return nil }); e == nil || entered {
				return fmt.Errorf("ASSERT_MIXED_FOREIGN_OWNERSHIP_%s", name)
			}
			t.Logf("ASSERT_MIXED_FOREIGN_OWNERSHIP_%s: PASS", name)
		}
		foreign := new(sessionruntime.Manager)
		if e := foreign.ConsumePrivateGroupingOwned(base, h, func(sessionruntime.PrivateGroupingOwnedView) error { return nil }); e == nil {
			return fmt.Errorf("ASSERT_MIXED_FOREIGN_MANAGER")
		}
		err = m.ConsumePrivateGroupingOwned(base, h, func(v sessionruntime.PrivateGroupingOwnedView) error {
			called = true
			if len(v.Methods) != 2 || v.Query.Generation != q.Session.Generation {
				return fmt.Errorf("genuine RD source mismatch")
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("ASSERT_SAME_LEASE_DIRECTIONAL_CONTINUATION: %w", err)
		}
		if !called {
			return fmt.Errorf("ASSERT_SAME_LEASE_DIRECTIONAL_CONTINUATION: consumer not entered")
		}
		t.Log("ASSERT_SAME_LEASE_DIRECTIONAL_CONTINUATION: PASS")
		// TAKE exclusion must outlive this consumer, not merely its callback.
		if f := m.CheckActiveSessionLeasePrivateV2(base, q.Session); f != "LIFECYCLE_CONFLICT" {
			return fmt.Errorf("ASSERT_DIRECTIONAL_EXCLUSION_AFTER_CONSUME: got %s", f)
		}
		t.Log("ASSERT_DIRECTIONAL_EXCLUSION_AFTER_CONSUME: PASS")
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	completion, err := m.FinishPrivateProviderWindow(ctx)
	if err != nil || !completion.Completed || m.Census().Workers != 0 {
		t.Fatalf("ASSERT_MIXED_LEASE_JOIN: %v %+v %+v", err, completion, m.Census())
	}
	t.Log("ASSERT_MIXED_LEASE_JOIN: PASS")
}

func TestPrivateInventoryMixedLeaseReplayV2ReleasedToken(t *testing.T) {
	m, q, ctx := mixedLeaseFixtureV2(t)
	var captured sessionruntime.PrivateGroupingOwnedHandoff
	var retired context.Context
	err := m.WithReadySessionLease(ctx, q.Session, func(base context.Context) error {
		retired = base
		var e error
		captured, e = m.CapturePrivateGroupingOwned(base, sessionruntime.PrivateGroupingOwnedRequest{Session: q.Session, URI: q.Directional.URI, OccurrenceID: "retired", Revision: q.Revision, Privacy: q.Privacy, Line: *q.Directional.Line, Character: *q.Directional.Character, References: true, Definitions: true, Deadline: q.Directional.Deadline})
		if e != nil {
			return e
		}
		_, e = inventorysuccessor.ConstructNativeCallsPrivateV2(base, m, q)
		return e
	})
	if err != nil {
		t.Fatal(err)
	}
	consume := func(c context.Context) error {
		return m.ConsumePrivateGroupingOwned(c, captured, func(sessionruntime.PrivateGroupingOwnedView) error { return nil })
	}
	if consume(retired) == nil {
		t.Fatal("ASSERT_MIXED_RELEASED_TOKEN")
	}
	err = m.WithReadySessionLease(ctx, q.Session, func(fresh context.Context) error {
		if consume(fresh) == nil {
			return fmt.Errorf("ASSERT_MIXED_FRESH_TOKEN_NOT_CONTINUATION")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	completion, e := m.FinishPrivateProviderWindow(ctx)
	if e != nil || !completion.Completed || m.Census().Workers != 0 {
		t.Fatalf("released control join: %v %+v", e, completion)
	}
	t.Log("ASSERT_MIXED_RELEASED_TOKEN: PASS; retired and fresh tokens refused, original window joined")
}

func TestPrivateInventoryMixedLeaseReplayV2Modes(t *testing.T) {
	for _, mode := range []string{"C", "R", "D", "CR", "CD", "RD", "CRD"} {
		t.Run(mode, func(t *testing.T) {
			owner, root := inventorysuccessor.ControlledManagedInputOwnerForTest(t, false)
			m, start, uri := mixedStartFixtureV2(t)
			q := inventorysuccessor.PrivateMixedInputRequestV2{Manager: m, Root: root, Start: start, URI: uri, OccurrenceID: "controlled-mixed", Revision: "controlled-mixed-revision", Privacy: "private-mixed-only", OccurrencePolicy: "ordinal", CompositionPolicy: "identity", Line: 1, Character: 6}
			for _, s := range mode {
				switch s {
				case 'C':
					q.Kinds = append(q.Kinds, programcadmission.TypedCalls)
				case 'R':
					q.Kinds = append(q.Kinds, programcadmission.TypedReferencesSymbol)
				case 'D':
					q.Kinds = append(q.Kinds, programcadmission.TypedResolvesToDefinition)
				}
			}
			sum := sha256.Sum256([]byte("mixed-" + mode))
			attempt := fmt.Sprintf("sha256:%x", sum)
			op, err := owner.ReserveMixedInputV2(context.Background(), attempt, q)
			if err != nil {
				t.Fatalf("ASSERT_MIXED_RESERVE_%s: %v", mode, err)
			}
			disposition, charge := inventorysuccessor.ControlledMixedHeadForTest(t, owner)
			if disposition != "PENDING" || m.Census().Workers != 0 {
				t.Fatalf("ASSERT_MIXED_RESERVE_%s: disposition=%s census=%+v", mode, disposition, m.Census())
			}
			for _, n := range charge {
				if n == 0 || n > 1e12 {
					t.Fatalf("ASSERT_MIXED_RESERVE_%s: nonfinite quote %v", mode, charge)
				}
			}
			t.Logf("ASSERT_MIXED_RESERVE_%s: PASS; before Start; charge=%v", mode, charge)
			if _, e := owner.ReserveMixedInputV2(context.Background(), fmt.Sprintf("sha256:%x", sha256.Sum256([]byte("nested-"+mode))), q); e == nil {
				t.Fatal("ASSERT_MIXED_PENDING_NO_NESTED_RESERVE")
			}
			t.Log("ASSERT_MIXED_PENDING_NO_NESTED_RESERVE: PASS")
			got, err := inventorysuccessor.ExecuteMixedInputV2(context.Background(), op, q)
			if err != nil {
				t.Fatalf("ASSERT_MIXED_MODE_%s: %v", mode, err)
			}
			admitted, err := referencegrouping.ParseAndAdmit(got.Input.CanonicalBytes(), got.Predecessors, referencegrouping.ParseLimits{MaxRawBytes: 1 << 20, MaxArtifacts: 8, MaxNodes: 1024, MaxOccurrences: 1024, MaxContributions: 1024, MaxCanonicalBytes: 1 << 20, MaxStringBytes: 4096})
			if err != nil || !admitted.MatchesTypedInput(got.TypedInput) || len(admitted.Contributions()) != len(mode) || m.Census().Workers != 0 {
				t.Fatalf("ASSERT_MIXED_MODE_%s: replay=%v contributions=%d workers=%d", mode, err, len(admitted.Contributions()), m.Census().Workers)
			}
			var size int
			for _, p := range got.Predecessors {
				size += len(p.Bytes)
				if p.Role == "NATIVE_EVIDENCE" {
					var proof referencegrouping.PrivateNativeEvidenceV2
					if json.Unmarshal(p.Bytes, &proof) != nil || len(proof.RawJoins) != 4 || proof.RawJoins[0].EdgeLocalOrdinal != proof.RawJoins[1].EdgeLocalOrdinal {
						t.Fatalf("ASSERT_MIXED_RAW_MULTIPLICITY_%s", mode)
					}
					frameBytes, frameCount := 0, 0
					for _, request := range proof.Requests {
						frameBytes += len(request.RequestFrame)
						frameCount++
						for _, frame := range request.ResponseFrames {
							frameBytes += len(frame)
							frameCount++
						}
					}
					t.Logf("ASSERT_NATIVE_FRAME_FOOTPRINT_%s: PASS; frames=%d raw-bytes=%d complete-proof=%d; profile-raw-bound=393216 proof-cap=1048576", mode, frameCount, frameBytes, len(p.Bytes))
					t.Logf("ASSERT_MIXED_RAW_MULTIPLICITY_%s: PASS; raw=4 canonical=1", mode)
				}
			}
			disposition, after := inventorysuccessor.ControlledMixedHeadForTest(t, owner)
			if disposition != "OBSERVED" || after != charge {
				t.Fatalf("ASSERT_MIXED_OBSERVE_%s: disposition=%s charge=%v", mode, disposition, after)
			}
			if strings.Contains(mode, "C") {
				b := got.ReplayOperationBinding()
				if b.Mode != "MIXED_NATIVE_REPLAY_UNQUOTED" {
					t.Fatal("ASSERT_MIXED_NO_OLD_REPLAY_QUOTE")
				}
				if _, e := owner.Reserve(context.Background(), fmt.Sprintf("sha256:%x", sha256.Sum256([]byte("old-replay-"+mode))), b); e == nil {
					t.Fatal("ASSERT_MIXED_NO_OLD_REPLAY_QUOTE")
				}
				t.Log("ASSERT_MIXED_NO_OLD_REPLAY_QUOTE: PASS; initial replay paid, later old quote refused")
			}
			t.Logf("ASSERT_MIXED_OBSERVE_%s: PASS; confirmed ack, no refund", mode)
			t.Logf("ASSERT_MIXED_MODE_%s: PASS; predecessors=%d bytes=%d input=%d joined-workers=0", mode, len(got.Predecessors), size, len(got.Input.CanonicalBytes()))
			if mode == "CRD" {
				mixedNativeTamperControlsV2(t, got)
			}
		})
	}
}

func TestPrivateInventoryMixedLeaseReplayV2FramePrefix(t *testing.T) {
	owner, root := inventorysuccessor.ControlledManagedInputOwnerForTest(t, false)
	m, start, uri := mixedStartFixtureV2(t)
	start.Process.Args = append(start.Process.Args, "frame-prefix")
	q := inventorysuccessor.PrivateMixedInputRequestV2{Manager: m, Root: root, Start: start, URI: uri, OccurrenceID: "frame-prefix", Revision: "controlled", Privacy: "private", OccurrencePolicy: "ordinal", CompositionPolicy: "identity", Line: 1, Character: 6, Kinds: []programcadmission.RelationKind{programcadmission.TypedCalls}}
	op, err := owner.ReserveMixedInputV2(context.Background(), fmt.Sprintf("sha256:%x", sha256.Sum256([]byte("frame-prefix"))), q)
	if err != nil {
		t.Fatal(err)
	}
	got, err := inventorysuccessor.ExecuteMixedInputV2(context.Background(), op, q)
	if err != nil {
		t.Fatal(err)
	}
	for _, artifact := range got.Predecessors {
		if artifact.Role != "NATIVE_EVIDENCE" {
			continue
		}
		var p referencegrouping.PrivateNativeEvidenceV2
		if json.Unmarshal(artifact.Bytes, &p) != nil {
			t.Fatal("actual proof decode")
		}
		for _, r := range p.Requests {
			if len(r.ResponseFrames) != 2 || !bytes.Contains(r.ResponseFrames[1], []byte("Content-Type:")) {
				t.Fatal("ASSERT_NATIVE_ORIGINAL_PREFIX")
			}
		}
		t.Logf("ASSERT_NATIVE_ORIGINAL_PREFIX: PASS; actual SELF lexical frames, prefix order and noncanonical header retained; proof=%d", len(artifact.Bytes))
	}
	mixedNativeTamperControlsV2(t, got)
}

func mixedNativeTamperControlsV2(t *testing.T, got inventorysuccessor.PrivateManagedGroupingInput) {
	var raw, composite []byte
	for _, p := range got.Predecessors {
		if p.Role == "NATIVE_EVIDENCE" {
			raw = p.Bytes
		}
		if p.Role == "RETAINED_COMPOSITE" {
			composite = p.Bytes
		}
	}
	modes := []string{"missing", "request-ordinal", "view-local", "edge-local", "multiplicity", "source", "binding", "selection", "status", "wire-result", "lexical-observation-omission", "lexical-one-deletion", "lexical-deselection", "lexical-duplication", "lexical-reorder", "lexical-unselected", "wire-request-length", "wire-response-length", "wire-result-length", "wire-request-sha", "wire-response-sha", "wire-id", "wire-generation", "request-method", "request-id", "request-params", "response-id", "response-result", "frame-eof", "overflow"}
	for _, selection := range []string{"selected", "unselected"} {
		for _, field := range []string{"component", "method", "id", "generation", "ordinal", "length", "digest", "failure", "selected"} {
			modes = append(modes, selection+"-field-"+field)
		}
	}
	for _, mode := range modes {
		t.Run("proof-"+mode, func(t *testing.T) {
			var p referencegrouping.PrivateNativeEvidenceV2
			if json.Unmarshal(raw, &p) != nil {
				t.Fatal("native proof fixture")
			}
			if parts := strings.Split(mode, "-field-"); len(parts) == 2 {
				i := 0
				if parts[0] == "unselected" {
					i = 1
				}
				n := 0
				if i == 0 {
					n = len(p.Requests[i].Observations) - 1
				}
				o := &p.Requests[i].Observations[n]
				switch parts[1] {
				case "component":
					o.Component += "-changed"
				case "method":
					o.Method += "-changed"
				case "id":
					o.RequestID++
				case "generation":
					o.Generation++
				case "ordinal":
					o.Ordinal++
				case "length":
					o.Length++
				case "digest":
					o.SHA256[0] ^= 1
				case "failure":
					o.Failure = "changed"
				case "selected":
					o.Selected = !o.Selected
				}
			}
			switch mode {
			case "wire-result-length":
				p.Requests[1].Wire.ResultLengthPrivate++
			case "wire-id":
				p.Requests[1].Wire.RequestIDPrivate++
			case "wire-generation":
				p.Requests[1].Wire.GenerationPrivate++
			case "request-method", "request-id", "request-params", "response-id", "response-result", "frame-eof":
				r := &p.Requests[1]
				request := strings.HasPrefix(mode, "request-")
				frame := r.RequestFrame
				if !request {
					frame = r.ResponseFrames[len(r.ResponseFrames)-1]
				}
				at := bytes.Index(frame, []byte("\r\n\r\n"))
				if at < 0 {
					t.Fatal("controlled frame header")
				}
				var fields map[string]json.RawMessage
				if json.Unmarshal(frame[at+4:], &fields) != nil {
					t.Fatal("controlled frame JSON")
				}
				switch mode {
				case "request-method":
					fields["method"] = json.RawMessage(`"callHierarchy/outgoingCalls"`)
				case "request-id", "response-id":
					fields["id"] = json.RawMessage(`9999`)
				case "request-params":
					fields["params"] = json.RawMessage(`{"item":null}`)
				case "response-result":
					fields["result"] = json.RawMessage(`[]`)
				}
				body, _ := json.Marshal(fields)
				changed := append([]byte(fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))), body...)
				if mode == "frame-eof" {
					changed = append(changed, ' ')
				}
				if request {
					r.RequestFrame = changed
					r.Wire.RequestLengthPrivate = uint64(len(changed))
					r.Wire.RequestSHA256Private = sha256.Sum256(changed)
					r.Observations[0].Length = uint64(len(changed))
					r.Observations[0].SHA256 = sha256.Sum256(changed)
				} else {
					r.ResponseFrames[len(r.ResponseFrames)-1] = changed
					r.Wire.ResponseLengthPrivate = uint64(len(changed))
					r.Wire.ResponseSHA256Private = sha256.Sum256(changed)
					n := len(r.Observations) - 1
					r.Observations[n].Length = uint64(len(changed))
					r.Observations[n].SHA256 = sha256.Sum256(changed)
				}
			case "lexical-observation-omission":
				if len(p.Requests[0].Observations) == 0 {
					t.Fatal("missing actual prepare observation")
				}
				p.Requests[0].Observations = nil
			case "lexical-one-deletion":
				p.Requests[1].Observations = p.Requests[1].Observations[1:]
			case "lexical-deselection":
				p.Requests[0].Observations[len(p.Requests[0].Observations)-1].Selected = false
			case "lexical-duplication":
				p.Requests[0].Observations = append(p.Requests[0].Observations, p.Requests[0].Observations[0])
			case "lexical-reorder":
				p.Requests[1].Observations[0], p.Requests[1].Observations[1] = p.Requests[1].Observations[1], p.Requests[1].Observations[0]
			case "lexical-unselected":
				if p.Requests[1].Observations[0].Selected {
					t.Fatal("actual frame observation must be unselected")
				}
				p.Requests[1].Observations[0].Length++
			case "wire-request-length":
				p.Requests[1].Wire.RequestLengthPrivate++
			case "wire-response-length":
				p.Requests[1].Wire.ResponseLengthPrivate++
			case "wire-request-sha":
				p.Requests[1].Wire.RequestSHA256Private[0] ^= 1
			case "wire-response-sha":
				p.Requests[1].Wire.ResponseSHA256Private[0] ^= 1
			case "request-ordinal":
				p.Requests[1].Ordinal++
			case "view-local":
				p.OriginalOrdinals[1][2] = 1
			case "edge-local":
				p.RawJoins[0].EdgeLocalOrdinal++
			case "multiplicity":
				p.RawJoins = p.RawJoins[:len(p.RawJoins)-1]
			case "source":
				p.Sources[0].GraphDigest = "sha256:" + string(bytes.Repeat([]byte("0"), 64))
			case "binding":
				p.Sources[0].Bindings = nil
			case "selection":
				ch := uint32(7)
				p.Parent.Character = &ch
			case "status":
				p.Requests[1].Status = "UNAVAILABLE"
			case "wire-result":
				p.Requests[1].Wire.ResultSHA256Private[0] ^= 1
			case "overflow":
				p.Requests[0].Params = make([]byte, 1<<20)
			}
			if mode == "overflow" {
				if _, err := referencegrouping.EncodePrivateNativeEvidenceV2(p, composite); err == nil {
					t.Fatal("ASSERT_NATIVE_PROOF_OVERFLOW")
				}
				t.Log("ASSERT_NATIVE_PROOF_OVERFLOW: PASS; pre-encode refusal")
				return
			}
			candidate, err := json.Marshal(p)
			if err != nil {
				t.Fatal(err)
			}
			candidate = append(candidate, '\n')
			if mode == "missing" {
				candidate = nil
			}
			if err := referencegrouping.ReplayPrivateNativeEvidenceV2(candidate, composite); err == nil {
				t.Fatalf("ASSERT_NATIVE_PROOF_TAMPER_%s", mode)
			}
			if err := referencegrouping.ReplayPrivateNativeEvidenceV2(raw, composite); err != nil {
				t.Fatalf("original proof after tamper: %v", err)
			}
			t.Logf("ASSERT_NATIVE_PROOF_TAMPER_%s: PASS; changed proof refused, original replayed", mode)
		})
	}
	var original referencegrouping.PrivateNativeEvidenceV2
	if json.Unmarshal(raw, &original) != nil {
		t.Fatal("prepare proof decode")
	}
	for event := range original.Requests[0].Observations {
		for _, field := range []string{"omit", "duplicate", "reorder", "component", "method", "id", "generation", "ordinal", "length", "digest", "failure", "selected"} {
			t.Run(fmt.Sprintf("prepare-event-%d-%s", event, field), func(t *testing.T) {
				var p referencegrouping.PrivateNativeEvidenceV2
				if json.Unmarshal(raw, &p) != nil {
					t.Fatal("prepare proof decode")
				}
				list := &p.Requests[0].Observations
				o := &(*list)[event]
				switch field {
				case "omit":
					*list = append((*list)[:event], (*list)[event+1:]...)
				case "duplicate":
					*list = append(*list, *o)
				case "reorder":
					other := (event + 1) % len(*list)
					(*list)[event], (*list)[other] = (*list)[other], (*list)[event]
				case "component":
					o.Component += "-changed"
				case "method":
					o.Method += "-changed"
				case "id":
					o.RequestID++
				case "generation":
					o.Generation++
				case "ordinal":
					o.Ordinal++
				case "length":
					o.Length++
				case "digest":
					o.SHA256[0] ^= 1
				case "failure":
					o.Failure = "changed"
				case "selected":
					o.Selected = !o.Selected
				}
				candidate, err := json.Marshal(p)
				if err != nil {
					t.Fatal(err)
				}
				if referencegrouping.ReplayPrivateNativeEvidenceV2(append(candidate, '\n'), composite) == nil {
					t.Fatal("ASSERT_PREPARE_EVENT_TAMPER")
				}
				if err := referencegrouping.ReplayPrivateNativeEvidenceV2(raw, composite); err != nil {
					t.Fatalf("original prepare proof: %v", err)
				}
				t.Logf("ASSERT_PREPARE_EVENT_TAMPER: PASS; event=%d field=%s; original replayed", event, field)
			})
		}
	}
	// Removing the predecessor itself cannot silently select the legacy path.
	without := make([]referencegrouping.PredecessorRecord, 0, len(got.Predecessors))
	for _, p := range got.Predecessors {
		if p.Role != "NATIVE_EVIDENCE" {
			without = append(without, p)
		}
	}
	if _, err := referencegrouping.ParseAndAdmit(got.Input.CanonicalBytes(), without, referencegrouping.ParseLimits{MaxRawBytes: 1 << 20, MaxArtifacts: 8, MaxNodes: 1024, MaxOccurrences: 1024, MaxContributions: 1024, MaxCanonicalBytes: 1 << 20, MaxStringBytes: 4096}); err == nil {
		t.Fatal("ASSERT_NATIVE_REQUIRED_PREDECESSOR")
	}
	t.Log("ASSERT_NATIVE_REQUIRED_PREDECESSOR: PASS")
}

func TestPrivateInventoryMixedLeaseReplayV2ObserveUncertain(t *testing.T) {
	owner, root := inventorysuccessor.ControlledManagedInputOwnerForTest(t, false)
	m, start, uri := mixedStartFixtureV2(t)
	q := inventorysuccessor.PrivateMixedInputRequestV2{Manager: m, Root: root, Start: start, URI: uri, OccurrenceID: "uncertain-observe", Revision: "controlled", Privacy: "private", OccurrencePolicy: "ordinal", CompositionPolicy: "identity", Line: 1, Character: 6, Kinds: []programcadmission.RelationKind{programcadmission.TypedCalls, programcadmission.TypedReferencesSymbol, programcadmission.TypedResolvesToDefinition}}
	attempt := fmt.Sprintf("sha256:%x", sha256.Sum256([]byte("uncertain-observe")))
	op, err := owner.ReserveMixedInputV2(context.Background(), attempt, q)
	if err != nil {
		t.Fatal(err)
	}
	inventorysuccessor.ControlledMixedObserveUncertainForTest(owner)
	got, err := inventorysuccessor.ExecuteMixedInputV2(context.Background(), op, q)
	if err == nil || len(got.Input.CanonicalBytes()) != 0 || m.Census().Workers != 0 {
		t.Fatalf("ASSERT_MIXED_OBSERVE_UNCERTAIN: err=%v workers=%d", err, m.Census().Workers)
	}
	if _, err = owner.ReserveMixedInputV2(context.Background(), attempt, q); err == nil {
		t.Fatal("ASSERT_MIXED_OBSERVE_STICKY")
	}
	t.Log("ASSERT_MIXED_OBSERVE_UNCERTAIN: PASS; joined actual work, no output after committed-plus-error Observe, owner sticky")
}

func TestPrivateInventoryMixedLeaseReplayV2FreshCurrency(t *testing.T) {
	for _, shortage := range []bool{true, false} {
		t.Run(fmt.Sprint(shortage), func(t *testing.T) {
			owner, root := inventorysuccessor.ControlledManagedInputOwnerForTest(t, false)
			if shortage {
				owner, root = inventorysuccessor.ControlledMixedInsufficientForTest(t)
			}
			m, start, uri := mixedStartFixtureV2(t)
			start.Process.Args = append(start.Process.Args, "funding-counter")
			marker := filepath.Join(start.Process.Dir, "controlled-effects")
			if _, e := os.Stat(marker); !os.IsNotExist(e) {
				t.Fatal("unexpected pre-existing effects marker")
			}
			q := inventorysuccessor.PrivateMixedInputRequestV2{Manager: m, Root: root, Start: start, URI: uri, OccurrenceID: "fresh-currency", Revision: "controlled", Privacy: "private", OccurrencePolicy: "ordinal", CompositionPolicy: "identity", Line: 1, Character: 6, Kinds: []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol}}
			before := inventorysuccessor.ControlledMixedFundingPreflightForTest(t, owner, q, shortage)
			op, err := owner.ReserveMixedInputV2(context.Background(), fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(fmt.Sprint(shortage)))), q)
			if shortage {
				if !errors.Is(err, inventorysuccessor.ErrPrivateMixedInputV2) || len(m.Records()) != 0 || len(m.Observations()) != 0 || before != inventorysuccessor.ControlledMixedHeadIdentityForTest(t, owner) {
					t.Fatalf("ASSERT_MIXED_FRESH_CURRENCY_NO_EFFECTS: %v", err)
				}
				if _, e := os.Stat(marker); !os.IsNotExist(e) {
					t.Fatal("ASSERT_MIXED_FRESH_CURRENCY_NO_EFFECTS: SELF effects marker exists")
				}
				t.Log("ASSERT_MIXED_FRESH_CURRENCY_NO_EFFECTS: PASS; Start records=0; SELF start/acquisition counter absent; runtime acquisition observations=0; exact head unchanged")
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			got, err := inventorysuccessor.ExecuteMixedInputV2(context.Background(), op, q)
			if err != nil || len(got.Input.CanonicalBytes()) == 0 {
				t.Fatalf("ASSERT_MIXED_RESTORED_CURRENCY_POSITIVE: %v", err)
			}
			marks, e := os.ReadFile(marker)
			if e != nil || strings.Count(string(marks), "start\n") != 1 || strings.Count(string(marks), "acquisition\n") != 2 {
				t.Fatalf("ASSERT_MIXED_RESTORED_CURRENCY_POSITIVE: counters=%q err=%v", marks, e)
			}
			t.Log("ASSERT_MIXED_RESTORED_CURRENCY_POSITIVE: PASS; same 1e12 fixture ceilings, one coordinate restored; actual Start=1 and retained input")
		})
	}
}

func TestPrivateInventoryMixedLeaseReplayV2TerminalControls(t *testing.T) {
	for _, mode := range []string{"unavailable", "empty", "expiry", "insufficient"} {
		t.Run(mode, func(t *testing.T) {
			owner, root := inventorysuccessor.ControlledManagedInputOwnerForTest(t, false)
			if mode == "insufficient" {
				owner, root = inventorysuccessor.ControlledMixedInsufficientForTest(t)
			}
			m, start, uri := mixedStartFixtureV2(t)
			start.Process.Args = append(start.Process.Args, mode)
			if mode == "expiry" {
				start.Deadline = time.Now().Add(500 * time.Millisecond)
			}
			q := inventorysuccessor.PrivateMixedInputRequestV2{Manager: m, Root: root, Start: start, URI: uri, OccurrenceID: "terminal-" + mode, Revision: "controlled", Privacy: "private", OccurrencePolicy: "ordinal", CompositionPolicy: "identity", Line: 1, Character: 6, Kinds: []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol}}
			attempt := fmt.Sprintf("sha256:%x", sha256.Sum256([]byte("terminal-"+mode)))
			op, err := owner.ReserveMixedInputV2(context.Background(), attempt, q)
			if mode == "insufficient" {
				if err == nil || m.Census().Workers != 0 {
					t.Fatalf("ASSERT_MIXED_UNFUNDED_BEFORE_EFFECTS: %v %+v", err, m.Census())
				}
				t.Log("ASSERT_MIXED_UNFUNDED_BEFORE_EFFECTS: PASS; fresh single-currency fixture refuses; no Start (isolated causal assertions in FreshCurrency)")
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			got, err := inventorysuccessor.ExecuteMixedInputV2(context.Background(), op, q)
			if err == nil || len(got.Input.CanonicalBytes()) != 0 {
				t.Fatalf("ASSERT_MIXED_TERMINAL_%s: %v", mode, err)
			}
			if mode == "unavailable" && !errors.Is(err, inventorysuccessor.ErrPrivateMixedInputUnavailableV2) && !strings.Contains(err.Error(), "state=FAILED failure=TOOL_NOT_IMPLEMENTED") {
				t.Fatalf("ASSERT_MIXED_UNAVAILABLE_NOT_EMPTY: %v", err)
			}
			if mode == "empty" && !errors.Is(err, inventorysuccessor.ErrPrivateMixedInputEmptyV2) {
				t.Fatalf("ASSERT_MIXED_EMPTY_NOT_UNAVAILABLE: %v", err)
			}
			disposition, _ := inventorysuccessor.ControlledMixedHeadForTest(t, owner)
			if disposition == "OBSERVED" {
				t.Fatalf("ASSERT_MIXED_TERMINAL_%s: observed on failure", mode)
			}
			t.Logf("ASSERT_MIXED_TERMINAL_%s: PASS; no output; disposition=%s", mode, disposition)
		})
	}
}
