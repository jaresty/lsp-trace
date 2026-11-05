package adr0011methodresult

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/programcadmission"
	"lsp-trace/internal/referencegrouping"
	"lsp-trace/sessionruntime"
)

type groupingSyntaxTrace struct{ calls, limit int }

func (a *groupingSyntaxTrace) WithObjectAdmission(fn func() error) error {
	a.calls++
	if a.limit >= 0 && a.calls > a.limit {
		return fmt.Errorf("controlled admission refusal")
	}
	return fn()
}
func TestPrivateManagedGroupingParserExtractionGolden(t *testing.T) {
	loc := `{"uri":"file:///a.go","range":{"start":{"line":1,"character":0},"end":{"line":1,"character":2}}}`
	link := `{"targetUri":"file:///a.go","targetRange":{"start":{"line":0,"character":0},"end":{"line":2,"character":0}},"targetSelectionRange":{"start":{"line":1,"character":0},"end":{"line":1,"character":2}}}`
	raw := []string{"null", " null ", "[]", "[ ]", "{}", loc, link, "[" + loc + "]", "[" + loc + "," + loc + "]", "[" + link + "]", "[" + loc + "," + link + "]", "[" + loc + ",{}]", "null null", "[null]", "[1]", "[true]", `[{"uri":"a","uri":"b"}]`, strings.Replace(loc, `"line":1`, `"line":2147483647`, 1), strings.Replace(loc, `"line":1`, `"line":2147483648`, 1), strings.Replace(loc, `"line":1`, `"line":-1`, 1), strings.Replace(loc, `"line":1`, `"line":1.5`, 1), strings.Replace(loc, `"line":1`, `"line":1e0`, 1), strings.Replace(loc, `file:///a.go`, `file:///é.go`, 1), "[" + strings.Replace(link, `"line":1`, `"line":9`, 1) + "]"}
	h := sha256.New()
	cases := 0
	for _, method := range []string{"textDocument/definition", "textDocument/references", "not-a-method"} {
		for _, max := range []int{-1, 0, 1, 2, 4} {
			for _, limit := range []int{-2, -1, 0, 1, 2} {
				for _, body := range raw {
					a := &groupingSyntaxTrace{limit: limit}
					var r Result
					var f *Failure
					if limit == -2 {
						r, f = parseRawUntrusted(method, []byte(body), max)
					} else {
						r, f = parseRawUntrustedWithAdmission(method, []byte(body), max, a)
					}
					record, err := json.Marshal(struct {
						Method, Raw string
						Max, Limit  int
						Result      Result
						Failure     *Failure
						Callbacks   int
					}{method, body, max, limit, r, f, a.calls})
					if err != nil {
						t.Fatal(err)
					}
					h.Write(record)
					h.Write([]byte{'\n'})
					cases++
				}
			}
		}
	}
	digest := fmt.Sprintf("%x", h.Sum(nil))
	if digest != "0c490841b271239616efa30179ef7ffe1664d58ecfedb492a5de0229c4877249" || cases != 1800 {
		t.Fatalf("ASSERT_PARSER_EXACT_ORIGINAL_GRAMMAR_AND_ADMISSION_TRACE: cases=%d digest=%s", cases, digest)
	}
	t.Logf("PARSER_DIFFERENTIAL_GOLDEN cases=%d sha256=%s type_pkg=%s", cases, digest, reflect.TypeOf(Result{}).PkgPath())
}

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
	got, err := groupingAcquire(t, context.Background(), m, q)
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
			got, err := groupingAcquire(t, context.Background(), m, q)
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
				if err := groupingReplay(t, got, q.Root); err != nil {
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
					if first, err := groupingIssue(t, ctx, m, h, q); err != nil || first.Input.ByteLength() == 0 {
						t.Fatalf("baseline issuance: %v", err)
					}
				}
				got, err := groupingIssue(t, ctx, m, h, candidate)
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
				if got, err := groupingIssue(t, retiredCtx, m, retired, q); err == nil || got.Input.ByteLength() != 0 {
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
			got, err := groupingAcquire(t, context.Background(), m, q)
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
			if groupingReplay(t, got, q.Root) == nil {
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
				got, err := groupingAcquire(t, ctx, m, q)
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
	got, err := groupingAcquire(t, context.Background(), m, q)
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
				got, err := groupingIssue(t, ctx, m, h, candidate)
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
						_, err = groupingAcquire(t, ctx, m, bad)
					case "Issue":
						_, err = groupingIssue(t, ctx, m, h, bad)
					case "Capture":
						_, err = m.CapturePrivateGroupingOwned(ctx, sessionruntime.PrivateGroupingOwnedRequest{Session: bad.Session, URI: bad.URI, OccurrenceID: bad.OccurrenceID, Revision: bad.Revision, Privacy: bad.Privacy, Line: bad.Line, Character: bad.Character, References: true, Definitions: true, Deadline: bad.Deadline})
					}
					afterM, afterP := groupingMEDEffects(t, p)
					if err == nil || afterM != beforeM || afterP != beforeP {
						t.Fatalf("ASSERT_MED2_FIELD_PREFLIGHT_NO_EFFECTS: %s/%s err=%v methods=%d/%d publications=%d/%d", entry, field, err, beforeM, afterM, beforeP, afterP)
					}
					if entry == "Issue" {
						if got, err := groupingIssue(t, ctx, m, h, q); err != nil || got.Input.ByteLength() == 0 {
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

func TestPrivateManagedGroupingMEDExactFieldLimit(t *testing.T) {
	m, q, _ := groupingFixture(t, true, true)
	q.Revision = strings.Repeat("é", 2048)
	q.OccurrenceID = strings.Repeat("x", 4096)
	q.Privacy = strings.Repeat("p", 4096)
	q.OccurrenceAdmissionPolicyID = strings.Repeat("a", 4096)
	q.CompositionPolicyID = strings.Repeat("c", 4096)
	if got, err := groupingAcquire(t, context.Background(), m, q); err != nil || got.Input.ByteLength() == 0 {
		t.Fatalf("ASSERT_MED2_EXACT_UTF8_BYTE_LIMIT_CONTROL: %v", err)
	}
}

// Test-only fixture export permits an external-package test of the real private
// entry. It neither creates a query receipt nor grants policy qualification.
func PrivateManagedGroupingFixtureForExternalTest(t *testing.T, replay ...PrivateManagedGroupingInput) (*sessionruntime.Manager, PrivateManagedGroupingRequest) {
	m, q, _ := groupingFixture(t, true, true)
	if len(replay) == 0 {
		return m, groupingPermit(t, m, q, "ACQUIRE", sessionruntime.PrivateGroupingOwnedHandoff{})
	}
	owner, _ := groupingResourceOwner(t)
	var err error
	q.Operation, err = owner.Reserve(context.Background(), privateGroupingDigest([]byte("controlled-external-replay")), replay[0].ReplayOperationBinding())
	if err != nil {
		t.Fatal(err)
	}
	return m, q
}
