package adr0011methodresult

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/adr0011querytarget"
	"lsp-trace/internal/programcadmission"
	"lsp-trace/internal/referencegrouping"
	"lsp-trace/sessionruntime"
)

func groupingCapture(t *testing.T, ctx context.Context, m *sessionruntime.Manager, q PrivateManagedGroupingRequest) sessionruntime.PrivateGroupingOwnedHandoff {
	t.Helper()
	refs, defs := false, false
	for _, k := range q.Kinds {
		refs = refs || k == programcadmission.TypedReferencesSymbol
		defs = defs || k == programcadmission.TypedResolvesToDefinition
	}
	h, err := m.CapturePrivateGroupingOwned(ctx, sessionruntime.PrivateGroupingOwnedRequest{Session: q.Session, URI: q.URI, OccurrenceID: q.OccurrenceID, Revision: q.Revision, Privacy: q.Privacy, Line: q.Line, Character: q.Character, References: refs, Definitions: defs, Deadline: q.Deadline})
	if err != nil {
		t.Fatal(err)
	}
	return h
}
func TestPrivateManagedGroupingCUECapabilityModel(t *testing.T) {
	m, q, _ := groupingFixture(t, false, true, "cue")
	got, err := AcquirePrivateManagedGroupingInput(context.Background(), m, q)
	if err != nil || len(got.Ledger) != 2 || got.Dispositions["textDocument/references"] != "UNAVAILABLE" || got.Dispositions["CALLS"] != "UNAVAILABLE" {
		t.Fatalf("ASSERT_PRIVATE_CUE_CAPABILITY_MODEL: %v", err)
	}
	var query struct {
		Metadata sessionruntime.SessionMetadata
		Supply   privateGroupingSupply
	}
	if err := json.Unmarshal(got.Predecessors[0].Bytes, &query); err != nil {
		t.Fatal(err)
	}
	if query.Metadata.CallHierarchySupport || query.Supply.Classification != "LSP_SUPPLIED" || query.Supply.Method != "textDocument/didOpen" || !bytes.Equal(query.Supply.Content, []byte("package fixture\nlet  Query = string\n")) || len(query.Supply.Params) == 0 {
		t.Fatal("ASSERT_PRIVATE_CUE_NO_CALLS_OR_SUPPLY_LOSS")
	}
	t.Log("ASSERT_PRIVATE_CUE_CAPABILITY_MODEL: PASS (controlled, not provider qualification)")
}

func TestPrivateManagedGroupingModesAndNativeOriginals(t *testing.T) {
	for _, mode := range []string{"references-only", "definitions-only", "both", "unavailable-references", "all-unavailable", "observed-empty", "observed-null"} {
		t.Run(mode, func(t *testing.T) {
			refs, defs := true, true
			if mode == "unavailable-references" || mode == "all-unavailable" {
				refs = false
			}
			if mode == "all-unavailable" {
				defs = false
			}
			m, q, p := groupingFixture(t, refs, defs)
			if mode == "references-only" {
				q.Kinds = []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol}
			}
			if mode == "definitions-only" {
				q.Kinds = []programcadmission.RelationKind{programcadmission.TypedResolvesToDefinition}
			}
			p.mu.Lock()
			if mode == "observed-empty" {
				p.results["textDocument/references"] = []byte("[ ]")
				p.results["textDocument/definition"] = []byte("[]")
			}
			if mode == "observed-null" {
				p.results["textDocument/references"] = []byte("null")
				p.results["textDocument/definition"] = []byte("null")
			}
			originals := map[string][]byte{}
			for method, raw := range p.results {
				originals[method] = append([]byte(nil), raw...)
			}
			p.mu.Unlock()
			got, err := AcquirePrivateManagedGroupingInput(context.Background(), m, q)
			if err != nil {
				t.Fatal(err)
			}
			want := 4
			if mode == "references-only" || mode == "definitions-only" || mode == "unavailable-references" {
				want = 2
			}
			if mode == "all-unavailable" || mode == "observed-empty" || mode == "observed-null" {
				want = 0
			}
			if len(got.Input.Contributions()) != want || len(got.Ledger) != want {
				t.Fatalf("ASSERT_PRIVATE_NATIVE_MULTIPLICITY: mode=%s got=%+v", mode, got.Ledger)
			}
			for i := 0; i < len(got.Ledger); i += 2 {
				a, b := got.Ledger[i], got.Ledger[i+1]
				if a.Ordinal != 0 || b.Ordinal != 1 || a.Identity == b.Identity || a.From != b.From || a.To != b.To {
					t.Fatalf("ASSERT_PRIVATE_ORIGINAL_ORDINALS: %+v", got.Ledger)
				}
			}
			if want == 0 {
				if got.Input.ByteLength() != 0 || len(got.Predecessors) != 0 {
					t.Fatal("ASSERT_PRIVATE_EMPTY_NO_GROUPING_CLAIM")
				}
			} else {
				if err := got.Replay(q.Root); err != nil {
					t.Fatalf("ASSERT_PRIVATE_ORIGINAL_REPLAY: %v", err)
				}
				if !got.Input.MatchesTypedInput(got.TypedInput) {
					t.Fatal("ASSERT_PRIVATE_TYPED_MATCH")
				}
				var query struct {
					Source                     []byte
					Symbol                     privateGroupingNative
					Methods                    []privateGroupingNative
					Revision, Custody, Privacy string
				}
				if err := json.Unmarshal(got.Predecessors[0].Bytes, &query); err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(query.Symbol.Result, originals["textDocument/documentSymbol"]) || query.Revision != q.Revision || query.Custody != "CALLER_ASSERTED" || query.Privacy != q.Privacy || len(query.Source) == 0 {
					t.Fatal("ASSERT_PRIVATE_QUERY_ORIGINALS")
				}
				for _, native := range query.Methods {
					if !bytes.Equal(native.Result, originals[native.Method]) || native.RequestID == 0 || native.Write.Key.ID != native.RequestID || native.Read.Key.ID != native.RequestID || native.Source == nil {
						t.Fatalf("ASSERT_PRIVATE_METHOD_NATIVE_BYTES: %+v", native)
					}
				}
			}
			if mode == "unavailable-references" || mode == "all-unavailable" {
				if got.Dispositions["textDocument/references"] != "UNAVAILABLE" {
					t.Fatal("ASSERT_PRIVATE_UNAVAILABLE_NOT_EMPTY")
				}
				p.mu.Lock()
				for _, method := range p.methods {
					if method == "textDocument/references" {
						t.Error("ASSERT_PRIVATE_UNAVAILABLE_NOT_SENT")
					}
				}
				p.mu.Unlock()
			}
			if mode == "observed-empty" || mode == "observed-null" {
				if got.Dispositions["textDocument/references"] != "OBSERVED_EMPTY" || got.Dispositions["textDocument/definition"] != "OBSERVED_EMPTY" {
					t.Fatal("ASSERT_PRIVATE_OBSERVED_EMPTY_NOT_UNAVAILABLE")
				}
			}
			t.Log("ASSERT_PRIVATE_ORIGINAL_ORDINALS: PASS", mode)
		})
	}
}
func TestPrivateManagedGroupingOpaqueCustodyRefusals(t *testing.T) {
	for _, mode := range []string{"zero", "foreign-manager", "foreign-workspace", "session", "generation", "stale-source", "query-target", "revision", "privacy", "source-uri", "occurrence", "method-selection", "retired-lease", "cancelled", "replay-handle"} {
		t.Run(mode, func(t *testing.T) {
			m, q, _ := groupingFixture(t, true, true)
			var retired sessionruntime.PrivateGroupingOwnedHandoff
			var retiredCtx context.Context
			err := m.WithReadySessionLease(context.Background(), q.Session, func(ctx context.Context) error {
				h := groupingCapture(t, ctx, m, q)
				retired = h
				retiredCtx = ctx
				if mode == "retired-lease" {
					return nil
				}
				candidate := q
				switch mode {
				case "zero":
					h = sessionruntime.PrivateGroupingOwnedHandoff{}
				case "foreign-manager":
					other, _, _ := groupingFixture(t, true, true)
					m = other
				case "foreign-workspace":
					candidate.Session.Workspace += "-foreign"
				case "session":
					candidate.Session.SessionID += "-foreign"
				case "generation":
					candidate.Session.Generation++
				case "stale-source":
					u, _ := url.Parse(q.URI)
					if err := os.WriteFile(u.Path, []byte("package fixture\nfunc Other() {}\n"), 0644); err != nil {
						t.Fatal(err)
					}
					d := m.PrepareDocument(ctx, sessionruntime.DocumentRequest{SessionID: q.Session.SessionID, Generation: q.Session.Generation, URI: q.URI, CaptureSupply: true, PreparedNoFollow: true})
					if d.Failure != "" {
						t.Fatal(d.Failure)
					}
				case "query-target":
					candidate.Character++
				case "revision":
					candidate.Revision = "foreign-revision"
				case "privacy":
					candidate.Privacy = "foreign-privacy"
				case "source-uri":
					candidate.URI += ".foreign"
				case "occurrence":
					candidate.OccurrenceID += "-foreign"
				case "method-selection":
					candidate.Kinds = []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol}
				case "cancelled":
					c, cancel := context.WithCancel(ctx)
					cancel()
					ctx = c
				case "replay-handle":
					if first, err := IssuePrivateManagedGroupingInput(ctx, m, h, q); err != nil || first.Input.ByteLength() == 0 {
						t.Fatalf("baseline issuance: %v", err)
					}
				}
				got, err := IssuePrivateManagedGroupingInput(ctx, m, h, candidate)
				if err == nil || got.Input.ByteLength() != 0 || len(got.Predecessors) != 0 {
					t.Fatalf("ASSERT_PRIVATE_OWNED_BINDING_REFUSAL: mode=%s err=%v", mode, err)
				}
				if mode == "query-target" {
					t.Log("ASSERT_PRIVATE_QUERY_TARGET_BINDING: PASS")
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if mode == "retired-lease" {
				if got, err := IssuePrivateManagedGroupingInput(retiredCtx, m, retired, q); err == nil || got.Input.ByteLength() != 0 {
					t.Fatal("ASSERT_PRIVATE_RETIRED_LEASE_REFUSED")
				}
			}
		})
	}
}
func TestPrivateManagedGroupingReplayTamperAndMalformed(t *testing.T) {
	for _, mode := range []string{"query-bytes", "method-bytes", "ledger-bytes", "ledger-snapshot", "disposition-snapshot", "foreign-root", "publication-refusal", "malformed-result", "result-swap", "expired"} {
		t.Run(mode, func(t *testing.T) {
			m, q, p := groupingFixture(t, true, true)
			if mode == "publication-refusal" {
				q.Root.Close()
			}
			if mode == "malformed-result" || mode == "result-swap" {
				p.mu.Lock()
				if mode == "result-swap" {
					p.results["textDocument/references"] = append([]byte(nil), p.results["textDocument/documentSymbol"]...)
				} else {
					p.results["textDocument/definition"] = []byte(`[{"uri":"file:///bad"}]`)
				}
				p.mu.Unlock()
			}
			if mode == "expired" {
				q.Deadline = time.Now().Add(-time.Second)
			}
			got, err := AcquirePrivateManagedGroupingInput(context.Background(), m, q)
			if mode == "publication-refusal" || mode == "malformed-result" || mode == "result-swap" || mode == "expired" {
				if err == nil || got.Input.ByteLength() != 0 {
					t.Fatal("ASSERT_PRIVATE_FAILURE_NO_SUCCESS_ARTIFACT")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "query-bytes":
				got.Predecessors[0].Bytes[0] ^= 1
			case "method-bytes":
				got.Predecessors[1].Bytes[0] ^= 1
			case "ledger-bytes":
				got.Predecessors[3].Bytes[0] ^= 1
			case "ledger-snapshot":
				got.Ledger[1].Ordinal = 0
			case "disposition-snapshot":
				got.Dispositions["textDocument/references"] = "UNAVAILABLE"
			case "foreign-root":
				_, other, _ := groupingFixture(t, true, true)
				q.Root = other.Root
			}
			if got.Replay(q.Root) == nil {
				t.Fatal("ASSERT_PRIVATE_REPLAY_ORIGINALS_REQUIRED")
			}
			if mode == "query-bytes" || mode == "method-bytes" || mode == "ledger-bytes" {
				if _, err := referencegrouping.ParseAndAdmit(got.Input.CanonicalBytes(), got.Predecessors, privateGroupingParseLimits()); err == nil {
					t.Fatal("ASSERT_PRIVATE_PREDECESSOR_TAMPER_REFUSED")
				}
			}
		})
	}
}

func TestPrivateManagedGroupingNativeCancellation(t *testing.T) {
	for _, mode := range []string{"cancel", "deadline"} {
		t.Run(mode, func(t *testing.T) {
			m, q, p := groupingFixture(t, true, true)
			entered := make(chan struct{}, 1)
			p.mu.Lock()
			p.dropMethod = "textDocument/references"
			p.entered = entered
			p.mu.Unlock()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if mode == "deadline" {
				q.Deadline = time.Now().Add(100 * time.Millisecond)
			}
			done := make(chan error, 1)
			go func() {
				got, err := AcquirePrivateManagedGroupingInput(ctx, m, q)
				if err == nil || got.Input.ByteLength() != 0 || len(got.Predecessors) != 0 {
					done <- nil
					return
				}
				done <- err
			}()
			select {
			case <-entered:
			case <-time.After(2 * time.Second):
				t.Fatal("ASSERT_PRIVATE_CANCEL_REAL_METHOD_NOT_REACHED")
			}
			if mode == "cancel" {
				cancel()
			}
			select {
			case err := <-done:
				if err == nil {
					t.Fatal("ASSERT_PRIVATE_NATIVE_CANCEL_NO_SUCCESS")
				}
			case <-time.After(4 * time.Second):
				t.Fatal("ASSERT_PRIVATE_NATIVE_CANCEL_NOT_PROPAGATED")
			}
			t.Log("ASSERT_PRIVATE_NATIVE_CANCEL_NO_SUCCESS: PASS", mode)
		})
	}
}

func TestPrivateManagedGroupingCanonicalTargetRefusesBeforeDR(t *testing.T) {
	m, q, p := groupingFixture(t, true, true)
	p.mu.Lock()
	p.results["textDocument/documentSymbol"] = []byte("[]")
	p.mu.Unlock()
	got, err := AcquirePrivateManagedGroupingInput(context.Background(), m, q)
	if err == nil || got.Input.ByteLength() != 0 {
		t.Fatal("ASSERT_PRIVATE_CANONICAL_TARGET_REQUIRED")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, method := range p.methods {
		if method == "textDocument/references" || method == "textDocument/definition" {
			t.Fatal("ASSERT_PRIVATE_TARGET_REFUSAL_BEFORE_DR")
		}
	}
	t.Log("ASSERT_PRIVATE_CANONICAL_TARGET_REQUIRED: PASS")
}

func groupingMEDEffects(t *testing.T, p *groupingPeer) (int, int) {
	t.Helper()
	p.mu.Lock()
	methods := 0
	for _, method := range p.methods {
		// The peer may finish recording initialized after readiness returns.
		// Startup traffic is not a document/native effect of the tested entry.
		if method != "initialize" && method != "initialized" {
			methods++
		}
	}
	p.mu.Unlock()
	entries, err := os.ReadDir(p.publicationPath)
	if err != nil {
		t.Fatal(err)
	}
	return methods, len(entries)
}

func TestPrivateManagedGroupingMEDEffectCounterStartupBoundary(t *testing.T) {
	p := &groupingPeer{publicationPath: t.TempDir(), methods: []string{"initialize", "initialized"}}
	if methods, publications := groupingMEDEffects(t, p); methods != 0 || publications != 0 {
		t.Fatal("ASSERT_MED2_STARTUP_NOT_ENTRY_EFFECT")
	}
	p.methods = append(p.methods, "textDocument/didOpen", "textDocument/documentSymbol")
	if methods, publications := groupingMEDEffects(t, p); methods != 2 || publications != 0 {
		t.Fatal("ASSERT_MED2_DOCUMENT_NATIVE_EFFECTS_STILL_COUNTED")
	}
}

func TestPrivateManagedGroupingMEDDeadline(t *testing.T) {
	for _, mode := range []string{"past", "substituted-future", "unchanged", "equal-instant"} {
		t.Run(mode, func(t *testing.T) {
			m, q, p := groupingFixture(t, true, true)
			if err := m.WithReadySessionLease(context.Background(), q.Session, func(ctx context.Context) error {
				h := groupingCapture(t, ctx, m, q)
				candidate := q
				if mode == "past" {
					candidate.Deadline = time.Now().Add(-time.Second)
				}
				if mode == "substituted-future" {
					candidate.Deadline = q.Deadline.Add(time.Second)
				}
				if mode == "equal-instant" {
					candidate.Deadline = q.Deadline.In(time.FixedZone("controlled-offset", 3600))
				}
				beforeM, beforeP := groupingMEDEffects(t, p)
				got, err := IssuePrivateManagedGroupingInput(ctx, m, h, candidate)
				afterM, afterP := groupingMEDEffects(t, p)
				if mode == "unchanged" || mode == "equal-instant" {
					if err != nil || got.Input.ByteLength() == 0 {
						t.Fatalf("ASSERT_MED1_CAPTURED_DEADLINE_CONTROL: %v", err)
					}
					return nil
				}
				if err == nil || got.Input.ByteLength() != 0 || afterM != beforeM || afterP != beforeP {
					t.Fatalf("ASSERT_MED1_DEADLINE_BOUND_BEFORE_PUBLICATION: %s err=%v methods=%d/%d publications=%d/%d", mode, err, beforeM, afterM, beforeP, afterP)
				}
				t.Log("ASSERT_MED1_DEADLINE_BOUND_BEFORE_PUBLICATION: PASS", mode)
				return nil
			}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPrivateManagedGroupingMEDFieldPreflight(t *testing.T) {
	for _, entry := range []string{"Acquire", "Issue", "Capture"} {
		for _, field := range []string{"uri", "session", "workspace", "occurrence", "revision", "privacy", "policy", "occurrence-policy", "utf8", "utf8-byte-length", "kinds"} {
			if entry == "Capture" && (field == "policy" || field == "occurrence-policy" || field == "kinds") {
				continue
			}
			t.Run(entry+"/"+field, func(t *testing.T) {
				m, q, p := groupingFixture(t, true, true)
				bad := q
				large := strings.Repeat("x", 4097)
				switch field {
				case "uri":
					bad.URI = large
				case "session":
					bad.Session.SessionID = large
				case "workspace":
					bad.Session.Workspace = large
				case "occurrence-policy":
					bad.OccurrenceAdmissionPolicyID = large
				case "utf8-byte-length":
					bad.Revision = strings.Repeat("é", 2049)
				case "occurrence":
					bad.OccurrenceID = large
				case "revision":
					bad.Revision = large
				case "privacy":
					bad.Privacy = large
				case "policy":
					bad.CompositionPolicyID = large
				case "utf8":
					bad.Revision = string([]byte{255})
				case "kinds":
					bad.Kinds = append(append([]programcadmission.RelationKind(nil), q.Kinds...), programcadmission.TypedCalls, programcadmission.TypedCalls)
				}
				exercise := func(ctx context.Context, h sessionruntime.PrivateGroupingOwnedHandoff) {
					beforeM, beforeP := groupingMEDEffects(t, p)
					var err error
					switch entry {
					case "Acquire":
						_, err = AcquirePrivateManagedGroupingInput(ctx, m, bad)
					case "Issue":
						_, err = IssuePrivateManagedGroupingInput(ctx, m, h, bad)
					case "Capture":
						_, err = m.CapturePrivateGroupingOwned(ctx, sessionruntime.PrivateGroupingOwnedRequest{Session: bad.Session, URI: bad.URI, OccurrenceID: bad.OccurrenceID, Revision: bad.Revision, Privacy: bad.Privacy, Line: bad.Line, Character: bad.Character, References: true, Definitions: true, Deadline: bad.Deadline})
					}
					afterM, afterP := groupingMEDEffects(t, p)
					if err == nil || afterM != beforeM || afterP != beforeP {
						t.Fatalf("ASSERT_MED2_FIELD_PREFLIGHT_NO_EFFECTS: %s/%s err=%v methods=%d/%d publications=%d/%d", entry, field, err, beforeM, afterM, beforeP, afterP)
					}
					if entry == "Issue" {
						if got, err := IssuePrivateManagedGroupingInput(ctx, m, h, q); err != nil || got.Input.ByteLength() == 0 {
							t.Fatalf("ASSERT_MED2_ISSUE_PREFLIGHT_BEFORE_CONSUMPTION: %s %v", field, err)
						}
					}
				}
				if entry == "Acquire" {
					exercise(context.Background(), sessionruntime.PrivateGroupingOwnedHandoff{})
					return
				}
				if err := m.WithReadySessionLease(context.Background(), q.Session, func(ctx context.Context) error {
					var h sessionruntime.PrivateGroupingOwnedHandoff
					if entry == "Issue" {
						h = groupingCapture(t, ctx, m, q)
					}
					exercise(ctx, h)
					return nil
				}); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestPrivateManagedGroupingMEDAggregatePreflight(t *testing.T) {
	source := make([]byte, 1<<20)
	v := sessionruntime.PrivateGroupingOwnedView{Source: source, Supply: sessionruntime.DocumentSupply{Content: source}}
	var err error
	allocations := testing.AllocsPerRun(1, func() { _, err = privateGroupingQueryBytes(v) })
	if err == nil || allocations != 0 {
		t.Fatalf("ASSERT_MED2_AGGREGATE_BEFORE_DERIVED_COPY_OR_MARSHAL: err=%v allocations=%g", err, allocations)
	}
	t.Log("ASSERT_MED2_AGGREGATE_BEFORE_DERIVED_COPY_OR_MARSHAL: PASS")
}

func TestPrivateManagedGroupingMEDExactFieldLimit(t *testing.T) {
	m, q, _ := groupingFixture(t, true, true)
	q.Revision = strings.Repeat("é", 2048)
	q.OccurrenceID = strings.Repeat("x", 4096)
	q.Privacy = strings.Repeat("p", 4096)
	q.OccurrenceAdmissionPolicyID = strings.Repeat("a", 4096)
	q.CompositionPolicyID = strings.Repeat("c", 4096)
	if got, err := AcquirePrivateManagedGroupingInput(context.Background(), m, q); err != nil || got.Input.ByteLength() == 0 {
		t.Fatalf("ASSERT_MED2_EXACT_UTF8_BYTE_LIMIT_CONTROL: %v", err)
	}
}

func TestPrivateManagedGroupingMEDArithmeticAndAggregateBoundary(t *testing.T) {
	cap := sessionruntime.PrivateGroupingQueryMaxBytes
	if cap != uint64(chainMaxBytes) {
		t.Fatal("ASSERT_MED2_QUERY_CAP_MATCH")
	}
	for _, tc := range []struct {
		total, n, mult, extra uint64
		want                  bool
	}{{cap - 1, 1, 1, 0, true}, {cap, 1, 1, 0, false}, {0, ^uint64(0), 6, 2, false}, {^uint64(0), 0, 0, 0, false}, {0, 0, 0, ^uint64(0), false}} {
		result, ok := sessionruntime.PrivateGroupingCheckedEncodedSize(tc.total, tc.n, tc.mult, tc.extra)
		if ok != tc.want || (ok && result > cap) {
			t.Fatalf("ASSERT_MED2_CHECKED_ENCODING_ARITHMETIC: %+v result=%d ok=%v", tc, result, ok)
		}
	}
	buffer := make([]byte, int(cap))
	low, high := 0, len(buffer)
	for low < high {
		mid := low + (high-low+1)/2
		_, ok := sessionruntime.PrivateGroupingQueryEncodedBound(sessionruntime.PrivateGroupingOwnedView{Source: buffer[:mid]})
		if ok {
			low = mid
		} else {
			high = mid - 1
		}
	}
	v := sessionruntime.PrivateGroupingOwnedView{Source: buffer[:low]}
	bound, ok := sessionruntime.PrivateGroupingQueryEncodedBound(v)
	if !ok {
		t.Fatal("ASSERT_MED2_AGGREGATE_BOUNDARY_ADMITTED")
	}
	raw, err := privateGroupingQueryBytes(v)
	if err != nil || uint64(len(raw)) > bound {
		t.Fatalf("ASSERT_MED2_AGGREGATE_BOUNDARY_CONTROL: len=%d bound=%d err=%v", len(raw), bound, err)
	}
	v.Source = buffer[:low+1]
	if _, err := privateGroupingQueryBytes(v); err == nil {
		t.Fatal("ASSERT_MED2_AGGREGATE_BOUNDARY_PLUS_ONE_REFUSED")
	}
	for _, bad := range []sessionruntime.PrivateGroupingOwnedView{{Methods: make([]sessionruntime.OwnedMethodPair, 3)}, {Unavailable: []string{"a", "b", "c"}}, {Metadata: sessionruntime.SessionMetadata{ProviderName: string([]byte{255})}}, {Metadata: sessionruntime.SessionMetadata{ServerCommand: strings.Repeat("x", 4097)}}} {
		if _, ok := sessionruntime.PrivateGroupingQueryEncodedBound(bad); ok {
			t.Fatal("ASSERT_MED2_QUERY_METADATA_COUNT_UTF8_REFUSED")
		}
	}
	escaped := strings.Repeat("<\x00", 2048)
	worst := sessionruntime.PrivateGroupingOwnedView{Revision: escaped, Metadata: sessionruntime.SessionMetadata{ServerCommand: escaped}, Target: adr0011querytarget.Candidate{SymbolName: escaped}}
	worstBound, ok := sessionruntime.PrivateGroupingQueryEncodedBound(worst)
	worstRaw, worstErr := privateGroupingQueryBytes(worst)
	if !ok || worstErr != nil || uint64(len(worstRaw)) > worstBound {
		t.Fatalf("ASSERT_MED2_JSON_ESCAPING_UPPER_BOUND: bytes=%d bound=%d err=%v", len(worstRaw), worstBound, worstErr)
	}
	t.Log("ASSERT_MED2_CHECKED_ENCODING_ARITHMETIC: PASS")
}

// Test-only fixture export permits an external-package test of the real private
// entry. It neither creates a query receipt nor grants policy qualification.
func PrivateManagedGroupingFixtureForExternalTest(t *testing.T) (*sessionruntime.Manager, PrivateManagedGroupingRequest) {
	m, q, _ := groupingFixture(t, true, true)
	return m, q
}
