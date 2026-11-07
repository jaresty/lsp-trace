package inventorysuccessor_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cc "lsp-trace/internal/censuscontinuation"
	"lsp-trace/internal/describerequest"
	"lsp-trace/internal/describeworker"
	inv "lsp-trace/internal/inventorysuccessor"
	sc "lsp-trace/internal/inventorysuccessor/sourcecontinuation"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/lspwire"
	pc "lsp-trace/internal/programc"
	"lsp-trace/internal/programcadmission"
	rg "lsp-trace/internal/referencegrouping"
	"lsp-trace/internal/retainedprojection"
	"lsp-trace/internal/sourceobject"
	"lsp-trace/internal/sourceprojection"
	tp "lsp-trace/internal/targetpacket"
	"lsp-trace/internal/v5sourcesnapshotv3"
	"lsp-trace/sessionruntime"
)

// The only subprocess is this controlled existing Go test binary. References /
// definitions deliberately return a real observed symbol selection coordinate.
// Foreign/snippet modes exercise source gaps, not alternate source authority.
func TestPrivateSourceSELFResponder(t *testing.T) {
	at := -1
	for n, a := range os.Args {
		if a == "private-source-self" {
			at = n
			break
		}
	}
	if at < 0 {
		return
	}
	root := os.Args[at+1]
	mode := "positive"
	if len(os.Args) > at+2 {
		mode = os.Args[at+2]
	}
	uri := (&url.URL{Scheme: "file", Path: filepath.Join(root, "root.go")}).String()
	display := lsp.Range{Start: lsp.Position{Line: 1}, End: lsp.Position{Line: 1, Character: 22}}
	selection := lsp.Range{Start: lsp.Position{Line: 1, Character: 5}, End: lsp.Position{Line: 1, Character: 9}}
	site := lsp.Range{Start: lsp.Position{Line: 1, Character: 13}, End: lsp.Position{Line: 1, Character: 18}}
	item := lsp.CallHierarchyItem{Name: "Root", URI: uri, Kind: 12, Range: display, SelectionRange: selection}
	otherSelection := lsp.Range{Start: lsp.Position{Line: 2, Character: 5}, End: lsp.Position{Line: 2, Character: 10}}
	other := lsp.CallHierarchyItem{Name: "Other", URI: uri, Kind: 12, Range: lsp.Range{Start: lsp.Position{Line: 2}, End: lsp.Position{Line: 2, Character: 22}}, SelectionRange: otherSelection}
	reader := lspwire.NewReader(os.Stdin, lspwire.DefaultLimits())
	for {
		msg, err := reader.Read()
		if err != nil || msg.Method == "exit" {
			os.Exit(0)
		}
		if len(msg.ID) == 0 {
			continue
		}
		var result any
		switch msg.Method {
		case "initialize":
			result = map[string]any{"serverInfo": map[string]string{"name": "controlled-source-self", "version": "v1"}, "capabilities": map[string]any{"positionEncoding": "utf-16", "callHierarchyProvider": true, "documentSymbolProvider": true, "referencesProvider": true, "definitionProvider": true}}
		case "textDocument/documentSymbol":
			result = []any{map[string]any{"name": "Root", "kind": 12, "range": display, "selectionRange": selection}, map[string]any{"name": "Other", "kind": 12, "range": other.Range, "selectionRange": other.SelectionRange}}
		case "textDocument/references", "textDocument/definition":
			targetURI, r := uri, otherSelection
			if mode == "foreign" {
				targetURI = (&url.URL{Scheme: "file", Path: filepath.Join(root, "foreign.go")}).String()
			}
			if mode == "snippet" {
				r = site
			}
			result = []any{map[string]any{"uri": targetURI, "range": r}}
			if strings.HasPrefix(mode, "link") && msg.Method == "textDocument/definition" {
				full := other.Range
				if mode == "link-envelope" {
					full.End.Character = 21
				}
				if mode == "link-selection" {
					r.Start.Character++
					r.End.Character++
				}
				if mode == "link-foreign" {
					targetURI = (&url.URL{Scheme: "file", Path: filepath.Join(root, "foreign.go")}).String()
				}
				result = []any{map[string]any{"targetUri": targetURI, "targetRange": full, "targetSelectionRange": r}}
			}
		case "textDocument/prepareCallHierarchy":
			result = []lsp.CallHierarchyItem{item}
		case "callHierarchy/incomingCalls":
			otherSite := lsp.Range{Start: lsp.Position{Line: 2, Character: 14}, End: lsp.Position{Line: 2, Character: 18}}
			result = []lsp.CallHierarchyIncomingCall{{From: other, FromRanges: []lsp.Range{otherSite, otherSite}}}
		case "callHierarchy/outgoingCalls":
			result = []lsp.CallHierarchyOutgoingCall{{To: other, FromRanges: []lsp.Range{site, site}}}
		}
		raw, _ := json.Marshal(result)
		body := fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"result":%s}`, msg.ID, raw)
		if _, err = fmt.Fprintf(os.Stdout, "Content-Length: %d\r\n\r\n%s", len(body), body); err != nil {
			os.Exit(72)
		}
	}
}

type sourceStageStore struct {
	*cc.MemoryStore
	puts    int
	fail    bool
	corrupt string
}

func (s *sourceStageStore) MaxObjectBytes() int64 { return 8 << 20 }
func (s *sourceStageStore) Put(c context.Context, b []byte) (string, error) {
	s.puts++
	if s.fail {
		return "", errors.New("controlled source Put failure")
	}
	return s.MemoryStore.Put(c, b)
}
func (s *sourceStageStore) Get(c context.Context, id string) ([]byte, error) {
	b, e := s.MemoryStore.Get(c, id)
	if id == s.corrupt && len(b) > 0 {
		b[0] ^= 1
	}
	return b, e
}

type sourceStageWorker struct{ calls int }

func (w *sourceStageWorker) Run(context.Context, describerequest.Record, string) (describeworker.RunResult, error) {
	w.calls++
	return describeworker.RunResult{}, errors.New("source stage must never execute")
}

func sourceStageContract(t *testing.T) cc.ContinuationContract {
	t.Helper()
	d := "sha256:" + strings.Repeat("1", 64)
	c, e := cc.NewContinuationContract(cc.ContractInput{ContinuationID: d, ProfileID: d, WorkerPinID: d, ProgramCSeed: 9, CaptureLimits: v5sourcesnapshotv3.Limits{MaxArtifactBytes: 8 << 20, MaxParentBytes: 8 << 20, MaxReceipts: 100, MaxSourceBytes: 1 << 20, MaxTotalSourceBytes: 8 << 20, MaxBindings: 1000, MaxWork: 10000}, PacketSourcePolicy: sourceprojection.Policy{PolicyID: "controlled-source", BodyRequested: true, MaxBytes: 1 << 20, MaxRanges: 100, MaxObjects: 100, MaxWork: 1000, EnforceLimits: true}, PacketResolveLimits: retainedprojection.ResolveLimits{MaxDistinctObjects: 100, MaxUniqueSourceBytes: 8 << 20, MaxLogicalSelections: 100}, PacketMaxResponseBytes: 1 << 20, DescribeDeadlineMS: 1000, RetryPolicy: cc.RetryPolicy{MaxAttemptsPerRequest: 1}})
	if e != nil {
		t.Fatal(e)
	}
	return c
}
func sourceStageFixture(t *testing.T, mode string, uncertain bool) (inv.PrivateManagedGroupingInput, cc.PrivateAllModeHandoff, cc.ContinuationContract, sc.PrivateAllModeSourcesRequest) {
	t.Helper()
	owner, root := inv.ControlledManagedInputOwnerForTest(t, false)
	m, start, uri := mixedStartFixtureV2(t)
	if e := os.WriteFile(filepath.Join(start.Profile.Workspace().String(), "root.go"), []byte("package fixture\nfunc Root(){ Other() }\nfunc Other(){ Root() }\n"), 0644); e != nil {
		t.Fatal(e)
	}
	start.Process.Args = []string{"-test.run=^TestPrivateSourceSELFResponder$", "--", "private-source-self", start.Profile.Workspace().String(), mode}
	q := inv.PrivateMixedInputRequestV2{Manager: m, Root: root, Start: start, URI: uri, OccurrenceID: "source-controlled", Revision: "source-controlled-revision", Privacy: "private-source-controlled", OccurrencePolicy: "ordinal", CompositionPolicy: "identity", Line: 1, Character: 6, Kinds: []programcadmission.RelationKind{programcadmission.TypedCalls, programcadmission.TypedReferencesSymbol, programcadmission.TypedResolvesToDefinition}}
	op, e := owner.ReserveMixedInputV2(context.Background(), fmt.Sprintf("sha256:%x", sha256.Sum256([]byte("source-"+mode))), q)
	if e != nil {
		t.Fatal(e)
	}
	if uncertain {
		inv.ControlledMixedObserveUncertainForTest(owner)
	}
	input, e := inv.ExecuteMixedInputV2(context.Background(), op, q)
	if uncertain {
		if _, proofErr := input.SourceOriginalsV1(); proofErr == nil {
			t.Fatal("ASSERT_SOURCE_UNCERTAIN_NO_CAP")
		}
		if e == nil {
			t.Fatal("ASSERT_SOURCE_UNCERTAIN_PRODUCER")
		}
		t.Log("ASSERT_SOURCE_UNCERTAIN_NO_CAP: PASS")
		return input, cc.PrivateAllModeHandoff{}, cc.ContinuationContract{}, sc.PrivateAllModeSourcesRequest{}
	}
	if e != nil {
		t.Fatal("ASSERT_SOURCE_ACTUAL_PRODUCER", e)
	}
	if m.Census().Workers != 0 {
		t.Fatal("ASSERT_SOURCE_PRODUCER_JOINED")
	}
	originals, e := input.SourceOriginalsV1()
	if e != nil {
		t.Fatal("ASSERT_SOURCE_CONFIRMED_ORIGINALS", e)
	}
	var slots []cc.PrivateAllModeSlot
	var entries []cc.PrivateAllModeEntry
	for i, mode := range []string{"RD", "C", "CRD", "D", "CD", "R", "CR"} {
		var rules []rg.GroupingPolicyRule
		for n, k := range []string{"CALLS", "REFERENCES_SYMBOL", "RESOLVES_TO_DEFINITION"} {
			r := rg.GroupingPolicyRule{RelationKind: k, Action: "EXCLUDE", RationaleCode: "CONTROLLED_SOURCE_MODE", Weight: rg.Rational{Numerator: 0, Denominator: 1}}
			if strings.ContainsRune(mode, []rune{'C', 'R', 'D'}[n]) {
				r.Action = "INCLUDE"
				r.Weight.Numerator = 1
			}
			rules = append(rules, r)
		}
		policy, e := rg.BuildGroupingPolicy(rg.GroupingPolicyRequest{PolicyID: "source-" + mode, Epoch: 1, EligibleKinds: []string{"CALLS", "REFERENCES_SYMBOL", "RESOLVES_TO_DEFINITION"}, Rules: rules, Limits: rg.GroupingPolicyLimits{MaxRules: 3, MaxCanonicalBytes: 1 << 16, MaxStringBytes: 256, MaxAbsoluteNumerator: 1, MaxDenominator: 1, MaxCap: 1}})
		if e != nil {
			t.Fatal(e)
		}
		slot := cc.PrivateAllModeSlot{Ordinal: i, Mode: mode, RunID: fmt.Sprintf("source-run-%d", i), SeedNamespace: "source-original-schedule", Seed: 7}
		c, e := pc.NewGroupingEvaluationCasePrivate(pc.GroupingEvaluationCaseRequestPrivate{CaseID: fmt.Sprintf("source-case-%d", i), RunID: slot.RunID, SelectorID: "source-selection", SelectorBytes: []byte("source selection\n"), Seed: 7, Input: input.Input, Policy: policy, ProjectionLimits: rg.CompositionLimits{MaxRawBytes: 1 << 20, MaxOccurrences: 128, MaxPairContributions: 128, MaxPairs: 128, MaxCanonicalBytes: 1 << 20, MaxStringBytes: 4096, MaxDenominator: 1 << 20, MaxAccumulator: 1 << 20}})
		if e != nil {
			t.Fatal(e)
		}
		result, e := pc.EvaluateGroupingCasePrivate(c)
		if e != nil {
			t.Fatal(e)
		}
		slots = append(slots, slot)
		entries = append(entries, cc.PrivateAllModeEntry{Slot: slot, Disposition: cc.PrivateAllModeReady, Lineage: fmt.Sprintf("source-lineage-%d", i), Input: input.Input, Policy: policy, Case: c, Result: result})
	}
	h, e := cc.NewPrivateAllModeHandoff(start.Profile.Workspace().String(), q.Revision, slots, entries)
	if e != nil {
		t.Fatal(e)
	}
	raw := []byte("package fixture\nfunc Root(){ Other() }\nfunc Other(){ Root() }\n")
	request := sc.PrivateAllModeSourcesRequest{PolicyID: "controlled-source", Limits: tp.PrivateSourceLimits{MaxDocumentBytes: 1 << 20, MaxTotalBytes: 8 << 20, MaxWork: 10000, MaxObjects: 1000}, Deadline: input.SourceDeadlineV1(), Originals: []inv.PrivateProducerOriginals{originals}, Objects: []sourceobject.Object{{Identity: sourceobject.Identity{Digest: fmt.Sprintf("sha256:%x", sha256.Sum256(raw)), ByteLength: uint64(len(raw))}, Bytes: raw}}}
	return input, h, sourceStageContract(t), request
}

func TestPrivateSourceConnectedAllSevenAndResume(t *testing.T) {
	input, h, c, q := sourceStageFixture(t, "positive", false)
	inv.ControlledSourceProjectionNoReadmitForTest(t, input)
	store := &sourceStageStore{MemoryStore: cc.NewMemoryStore()}
	worker := &sourceStageWorker{}
	base := cc.Run(context.Background(), cc.Request{PrivateAllMode: &h, Workspace: qWorkspace(q), Contract: c, Store: store, StopAfter: cc.StopAfterSourceIntents})
	if base.Err != nil {
		t.Fatal(base.Err)
	}
	before := store.puts
	ctx, cancel := context.WithDeadline(context.Background(), q.Deadline)
	defer cancel()
	req := sc.Request{PrivateAllMode: &h, PrivateSources: &q, Workspace: qWorkspace(q), Contract: c, Store: store, Worker: worker, StopAfter: cc.StopAfterPrivateSourceRequests}
	got := sc.Run(ctx, req)
	if got.Err != nil || got.PrivateSources == nil {
		t.Fatal("ASSERT_CONNECTED_SOURCE_PAUSE", got.Err)
	}
	out := got.PrivateSources
	if got.Status != cc.StatusPaused || out.Counts.Admitted == 0 || out.Counts.Requests == 0 || out.Counts.Admitted+out.Counts.Unresolved != out.Counts.Intents || worker.calls != 0 || store.puts-before != 7 {
		t.Fatal("ASSERT_CONNECTED_SOURCE_BALANCE", out.Counts, worker.calls, store.puts-before, out.Outcomes, base.PrivateAllMode.Intents[0])
	}
	if out.ClaimCeiling != base.PrivateAllMode.ClaimCeiling || !strings.Contains(out.ClaimCeiling, "NO_DESCRIBE") {
		t.Fatal("ASSERT_SOURCE_GROUPING_CEILING_UNCHANGED")
	}
	modes := map[string]bool{}
	roles := map[string]bool{}
	for n, o := range out.Outcomes {
		if o.Intent != n {
			t.Fatal("ASSERT_SOURCE_INTENT_MULTIPLICITY_ORDER")
		}
		i := base.PrivateAllMode.Intents[o.Intent]
		if i.Seed != 7 || i.Slot < 0 || i.Slot >= 7 || i.Mode != []string{"RD", "C", "CRD", "D", "CD", "R", "CR"}[i.Slot] {
			t.Fatal("ASSERT_SOURCE_SEED_SCHEDULE")
		}
		if i.Endpoint.Disposition == "EXCLUDED" && (o.Status != "UNRESOLVED" || o.Gap != "GROUPING_ENDPOINT_EXCLUDED") {
			t.Fatal("ASSERT_SOURCE_EXCLUDED_RETAINED")
		}
		if o.Status == "SOURCE_ADMITTED" {
			i := base.PrivateAllMode.Intents[o.Intent]
			modes[i.Mode] = true
			roles[i.Endpoint.Kind+"/"+i.Endpoint.Role] = true
		}
	}
	if out.Counts.Admitted != 32 || out.Counts.Unresolved != 24 || out.Counts.Requests != 2 || out.Counts.Intents != 56 {
		t.Fatal("ASSERT_SOURCE_EXACT_ORIGINAL_COUNTS", out.Counts)
	}
	if len(modes) != 7 {
		t.Fatalf("ASSERT_SOURCE_ALL_SEVEN_POSITIVE: modes=%v outcomes=%+v", modes, out.Outcomes)
	}
	for _, role := range []string{"CALLS/CALLER_OCCURRENCE", "CALLS/CALLEE_TARGET", "REFERENCES_SYMBOL/REFERENCED_SYMBOL", "REFERENCES_SYMBOL/REFERENCING_OCCURRENCE", "RESOLVES_TO_DEFINITION/QUERY_OCCURRENCE", "RESOLVES_TO_DEFINITION/DEFINITION_TARGET"} {
		if !roles[role] {
			t.Fatal("ASSERT_SOURCE_POSITIVE_FULL_DEFINITION_ROLE", role, out.Outcomes)
		}
	}
	if describerequest.Validate(out.Requests) != nil {
		t.Fatal("ASSERT_SOURCE_EXISTING_RECORD_VALIDATE")
	}
	for _, r := range out.Requests {
		if r.Lineage.ConsumerResolution != describerequest.PrivateSourceOnly || r.Lineage.Authority != 0 || r.Lineage.Accepted || r.Envelope.SourceRevision != strings.TrimPrefix(q.Objects[0].Identity.Digest, "sha256:") || (!strings.Contains(r.Envelope.Prompt, "func Root(){ Other() }") && !strings.Contains(r.Envelope.Prompt, "func Other(){ Root() }")) || strings.Contains(r.Envelope.Prompt, "source-run-") || strings.Contains(r.Envelope.Prompt, "source-lineage-") || strings.Contains(r.Envelope.Prompt, "source-case-") {
			t.Fatal("ASSERT_SOURCE_ONLY_PROMPT_SEPARATION", r)
		}
	}
	for _, mutate := range []func(*describerequest.Record){func(r *describerequest.Record) { r.Envelope.Prompt += "substitution" }, func(r *describerequest.Record) { r.Envelope.SourceRevision = "foreign" }, func(r *describerequest.Record) { r.Envelope.AdmissionID = "foreign" }, func(r *describerequest.Record) { r.Lineage.Authority = 1 }, func(r *describerequest.Record) { r.Lineage.Accepted = true }, func(r *describerequest.Record) { r.Lineage.SourceGraphComplete = "COMPLETE" }, func(r *describerequest.Record) { r.RecordID = "foreign" }} {
		r := out.Requests[0]
		mutate(&r)
		if describerequest.ValidateRecord(r) == nil {
			t.Fatal("ASSERT_SOURCE_RECORD_FIELD_INTEGRITY")
		}
	}
	t.Log("ASSERT_SOURCE_RECORD_FIELD_INTEGRITY: PASS; ASSERT_SOURCE_GROUPING_CEILING_UNCHANGED: PASS; ASSERT_SOURCE_INTENT_MULTIPLICITY_ORDER: PASS; ASSERT_SOURCE_SEED_SCHEDULE: PASS")
	resume := sc.ResumeRequest{PrivateSources: &sc.PrivateAllModeSourcesResume{Version: sc.PrivateAllModeSourcesVersion, Expected: out.Expected, Originals: q.Originals, Definitions: q.Definitions, ExpectedHandoff: h}, Workspace: req.Workspace, Store: store, Worker: worker, StopAfter: req.StopAfter, Selector: out.CheckpointID}
	before = store.puts
	again := sc.Resume(ctx, resume)
	if again.Err != nil || again.CheckpointID != got.CheckpointID || store.puts != before || worker.calls != 0 {
		t.Fatal("ASSERT_SOURCE_EXACT_RESUME_NO_PUT", again.Err)
	}
	secondOriginals, secondErr := input.SourceOriginalsV1()
	if secondErr != nil {
		t.Fatal(secondErr)
	}
	rSub := resume
	sSub := *resume.PrivateSources
	sSub.Originals = []inv.PrivateProducerOriginals{secondOriginals}
	rSub.PrivateSources = &sSub
	if bad := sc.Resume(ctx, rSub); bad.Err == nil {
		t.Fatal("ASSERT_SOURCE_EXACT_OPAQUE_SUBSTITUTION")
	}
	t.Log("ASSERT_SOURCE_EXACT_OPAQUE_SUBSTITUTION: PASS")
	for _, mutate := range []func(*sc.ResumeRequest){func(r *sc.ResumeRequest) { r.Selector = "sha256:" + strings.Repeat("2", 64) }, func(r *sc.ResumeRequest) { r.StopAfter = cc.StopAfterSourceIntents }, func(r *sc.ResumeRequest) {
		s := *r.PrivateSources
		s.Version = cc.PrivateAllModeVersion
		r.PrivateSources = &s
	}, func(r *sc.ResumeRequest) { s := *r.PrivateSources; s.Originals = nil; r.PrivateSources = &s }, func(r *sc.ResumeRequest) {
		s := *r.PrivateSources
		s.Expected = sc.PrivateAllModeSourcesExpected{}
		r.PrivateSources = &s
	}, func(r *sc.ResumeRequest) {
		s := *r.PrivateSources
		s.ExpectedHandoff = cc.PrivateAllModeHandoff{}
		r.PrivateSources = &s
	}} {
		r := resume
		mutate(&r)
		if bad := sc.Resume(ctx, r); bad.Err == nil || store.puts != before || worker.calls != 0 {
			t.Fatal("ASSERT_SOURCE_RESUME_SUBSTITUTION")
		}
	}
	persisted := *out
	persisted.CheckpointID = ""
	sourceBytes, _ := json.Marshal(persisted)
	requestBytes, _ := describerequest.Bytes(out.Requests)
	for _, id := range []string{out.CheckpointID, base.CheckpointID, fmt.Sprintf("sha256:%x", sha256.Sum256(sourceBytes)), fmt.Sprintf("sha256:%x", sha256.Sum256(requestBytes))} {
		store.corrupt = id
		if bad := sc.Resume(ctx, resume); bad.Err == nil || store.puts != before || worker.calls != 0 {
			t.Fatal("ASSERT_SOURCE_RESUME_TAMPER", id)
		}
	}
	store.corrupt = ""
	unowned := cc.Run(ctx, cc.Request{PrivateAllMode: &h, PrivateSources: &cc.PrivateAllModeSourcesRequest{PolicyID: q.PolicyID, Limits: q.Limits, Deadline: q.Deadline}, Workspace: req.Workspace, Contract: c, Store: store, Worker: worker, StopAfter: req.StopAfter})
	if unowned.Err == nil || store.puts != before || worker.calls != 0 {
		t.Fatal("ASSERT_CONNECTED_LOWER_SOURCE_REFUSED")
	}
	cancelled, stop := context.WithCancel(ctx)
	stop()
	before = store.puts
	if bad := sc.Run(cancelled, req); bad.Err == nil || store.puts != before {
		t.Fatal("ASSERT_SOURCE_CANCEL_BEFORE_PUT")
	}
	store.fail = true
	if bad := sc.Run(ctx, req); bad.Err == nil {
		t.Fatal("ASSERT_SOURCE_STORE_FAILURE")
	}
	store.fail = false
	legacy := cc.Run(context.Background(), cc.Request{PrivateAllMode: &h, Workspace: req.Workspace, Contract: c, Store: store, StopAfter: cc.StopAfterSourceIntents})
	if legacy.Err != nil || legacy.CheckpointID != base.CheckpointID || legacy.PrivateAllMode.Counts != base.PrivateAllMode.Counts {
		t.Fatal("ASSERT_SOURCE_V1_CONTINUITY", legacy.Err)
	}
	// Every endpoint join field is independently substituted against genuine
	// original handles. Returned query coordinates are selected internally.
	prepared, prepareErr := inv.PreparePrivateProducerOriginals(q.Originals[0], q.Limits)
	if prepareErr != nil {
		t.Fatal(prepareErr)
	}
	seenEndpoint := map[tp.PrivateTypedEndpoint]bool{}
	for _, i := range base.PrivateAllMode.Intents {
		if seenEndpoint[i.Endpoint] {
			continue
		}
		seenEndpoint[i.Endpoint] = true
		if i.Endpoint.Kind == "" || i.Endpoint.Disposition == "EXCLUDED" {
			continue
		}
		a, gap := inv.AdmitPrivateEndpointSource(i, prepared, nil, q.Objects, q.Limits)
		if !a.Valid() {
			t.Fatal("ASSERT_SOURCE_DIRECT_POSITIVE", gap)
		}
		for _, mutate := range []func(*tp.PrivateSourceIntent){func(x *tp.PrivateSourceIntent) { x.InputID = "foreign" }, func(x *tp.PrivateSourceIntent) { x.Endpoint.Node = "foreign" }, func(x *tp.PrivateSourceIntent) { x.Endpoint.Occurrence = "foreign" }, func(x *tp.PrivateSourceIntent) { x.Endpoint.Role = "foreign" }, func(x *tp.PrivateSourceIntent) { x.Endpoint.Predecessor = "foreign" }, func(x *tp.PrivateSourceIntent) { x.Endpoint.Custody = "foreign" }, func(x *tp.PrivateSourceIntent) { x.Endpoint.Disposition = "EMPTY" }, func(x *tp.PrivateSourceIntent) { x.Endpoint.Ordinal++ }} {
			v := i
			mutate(&v)
			if bad, _ := inv.AdmitPrivateEndpointSource(v, prepared, nil, q.Objects, q.Limits); bad.Valid() {
				t.Fatal("ASSERT_SOURCE_ENDPOINT_SUBSTITUTION", v)
			}
		}
	}
	// Public original-query field substitutions cannot remint producer custody.
	for _, path := range []string{"Query.URI", "Query.Line", "Query.Character", "Query.SessionID", "Query.Generation", "Query.DocumentVersion", "Query.Encoding", "Query.SourceDigest", "Target.SymbolID", "Target.SymbolName", "Target.DocumentVersion", "Target.Encoding", "Target.SourceDigest", "Revision", "Privacy", "Supply.URI", "Supply.DocumentVersion", "Supply.Content", "Symbol.SessionID", "Symbol.Generation", "Symbol.RequestID"} {
		bad := input
		bad.Predecessors = append([]rg.PredecessorRecord(nil), input.Predecessors...)
		for n, p := range bad.Predecessors {
			if p.Role != "METHOD_QUERY" {
				continue
			}
			var fields map[string]any
			if e := json.Unmarshal(p.Bytes, &fields); e != nil {
				t.Fatal(e)
			}
			parts := strings.Split(path, ".")
			if len(parts) == 1 {
				fields[parts[0]] = "substitution"
			} else {
				nested := fields[parts[0]].(map[string]any)
				switch nested[parts[1]].(type) {
				case float64:
					nested[parts[1]] = float64(987)
				default:
					nested[parts[1]] = "substitution"
				}
			}
			raw, _ := json.Marshal(fields)
			bad.Predecessors[n].Bytes = append(raw, '\n')
		}
		if _, e := bad.SourceOriginalsV1(); e == nil {
			t.Fatal("ASSERT_SOURCE_ORIGINAL_FIELD_SUBSTITUTION", path)
		}
	}
	t.Log("ASSERT_SOURCE_ORIGINAL_FIELD_SUBSTITUTION: PASS; Query/Target/session/generation/version/encoding/revision/privacy/source digest/supply/native transaction fields")
	// An already-issued original handle must not retain public backing arrays.
	for _, p := range input.Predecessors {
		if p.Role == "METHOD_QUERY" {
			before := p.Bytes[0]
			p.Bytes[0] ^= 1
			_, aliasErr := inv.PreparePrivateProducerOriginals(q.Originals[0], q.Limits)
			p.Bytes[0] = before
			if aliasErr != nil {
				t.Fatal("ASSERT_SOURCE_ORIGINALS_NOT_PUBLIC_ALIAS", aliasErr)
			}
		}
	}
	t.Log("ASSERT_SOURCE_ORIGINALS_NOT_PUBLIC_ALIAS: PASS")
	// Confirm public diagnostic mutation cannot remint producer originals.
	input.Ledger[0].Ordinal++
	if _, e := input.SourceOriginalsV1(); e == nil {
		t.Fatal("ASSERT_SOURCE_ORIGINAL_GETTER_NOT_DTO")
	}
	t.Logf("ASSERT_SOURCE_ALL_SEVEN_POSITIVE: PASS; modes=%v roles=%v admitted=%d unresolved=%d requests=%d worker_calls=%d", modes, roles, out.Counts.Admitted, out.Counts.Unresolved, out.Counts.Requests, worker.calls)
	t.Log("ASSERT_SOURCE_ONLY_PROMPT_SEPARATION: PASS; ASSERT_SOURCE_EXACT_RESUME_NO_PUT: PASS; ASSERT_SOURCE_RESUME_SUBSTITUTION: PASS; ASSERT_SOURCE_ENDPOINT_SUBSTITUTION: PASS; ASSERT_SOURCE_V1_CONTINUITY: PASS")
}

// Workspace is supplied explicitly from a verified admitted unit, never from a
// path scan. The fixture's root is the fixed producer binding's exact workspace.
func qWorkspace(q sc.PrivateAllModeSourcesRequest) string { return q.Originals[0].Workspace() }

func TestPrivateSourceOriginalsUnavailableAndGaps(t *testing.T) {
	if _, e := (inv.PrivateManagedGroupingInput{}).SourceOriginalsV1(); e == nil {
		t.Fatal("ASSERT_SOURCE_ZERO_ORIGINALS")
	}
	sourceStageFixture(t, "uncertain", true)
	for _, mode := range []string{"foreign", "snippet"} {
		t.Run(mode, func(t *testing.T) {
			_, h, c, q := sourceStageFixture(t, mode, false)
			ctx, cancel := context.WithDeadline(context.Background(), q.Deadline)
			defer cancel()
			store := &sourceStageStore{MemoryStore: cc.NewMemoryStore()}
			got := sc.Run(ctx, sc.Request{PrivateAllMode: &h, PrivateSources: &q, Workspace: qWorkspace(q), Contract: c, Store: store, StopAfter: sc.StopAfterPrivateSourceRequests})
			if got.Err != nil {
				t.Fatal(got.Err)
			}
			o := got.PrivateSources
			base := cc.Run(ctx, cc.Request{PrivateAllMode: &h, Workspace: qWorkspace(q), Contract: c, Store: store, StopAfter: cc.StopAfterSourceIntents})
			if base.Err != nil {
				t.Fatal(base.Err)
			}
			for _, row := range o.Outcomes {
				i := base.PrivateAllMode.Intents[row.Intent]
				if i.Endpoint.Disposition == "EXCLUDED" {
					continue
				}
				returned := i.Endpoint.Kind == "REFERENCES_SYMBOL" && i.Endpoint.Role == "REFERENCING_OCCURRENCE" || i.Endpoint.Kind == "RESOLVES_TO_DEFINITION" && i.Endpoint.Role == "DEFINITION_TARGET"
				if returned && (row.Status != "UNRESOLVED" || row.Gap != "OWN_FULL_DEFINITION_OR_VERIFIED_SOURCE_UNAVAILABLE") {
					t.Fatal("ASSERT_SOURCE_RETURNED_FULL_DEFINITION_REQUIRED", mode, row)
				}
			}
			t.Log("ASSERT_SOURCE_RETURNED_FULL_DEFINITION_REQUIRED: PASS", mode)
			if o.Counts.Admitted == 0 || o.Counts.Unresolved == 0 || o.Counts.Intents != o.Counts.Admitted+o.Counts.Unresolved {
				t.Fatal("ASSERT_SOURCE_FOREIGN_OR_SNIPPET_GAPS", o.Counts)
			}
			for _, r := range o.Outcomes {
				if r.Status == "UNRESOLVED" && r.Gap == "" {
					t.Fatal("ASSERT_SOURCE_TYPED_GAP_REQUIRED")
				}
			}
			t.Log("ASSERT_SOURCE_FOREIGN_OR_SNIPPET_GAPS: PASS", mode, o.Counts)
		})
	}
	_, h, c, q := sourceStageFixture(t, "positive", false)
	ctx, cancel := context.WithDeadline(context.Background(), q.Deadline)
	defer cancel()
	store := &sourceStageStore{MemoryStore: cc.NewMemoryStore()}
	workspace := qWorkspace(q)
	q.Originals = nil
	got := sc.Run(ctx, sc.Request{PrivateAllMode: &h, PrivateSources: &q, Workspace: workspace, Contract: c, Store: store, StopAfter: sc.StopAfterPrivateSourceRequests})
	if got.Err != nil || got.PrivateSources.Counts.Admitted != 0 || got.PrivateSources.Counts.Unresolved != got.PrivateSources.Counts.Intents || got.RequestCount != 0 {
		t.Fatal("ASSERT_SOURCE_NO_PROOF_NOT_EMPTY", got.Err)
	}
}

func TestPrivateSourceLocationLinkSelectionAndEnvelope(t *testing.T) {
	for index, mode := range []string{"link", "link-envelope", "link-selection", "link-foreign"} {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			_, h, c, q := sourceStageFixture(t, mode, false)
			ctx, cancel := context.WithDeadline(context.Background(), q.Deadline)
			defer cancel()
			store := &sourceStageStore{MemoryStore: cc.NewMemoryStore()}
			base := cc.Run(ctx, cc.Request{PrivateAllMode: &h, Workspace: qWorkspace(q), Contract: c, Store: store, StopAfter: cc.StopAfterSourceIntents})
			got := sc.Run(ctx, sc.Request{PrivateAllMode: &h, PrivateSources: &q, Workspace: qWorkspace(q), Contract: c, Store: store, StopAfter: sc.StopAfterPrivateSourceRequests})
			if got.Err != nil || base.Err != nil {
				t.Fatal(got.Err, base.Err)
			}
			seen := 0
			for _, row := range got.PrivateSources.Outcomes {
				i := base.PrivateAllMode.Intents[row.Intent]
				if i.Endpoint.Kind != "RESOLVES_TO_DEFINITION" || i.Endpoint.Role != "DEFINITION_TARGET" || i.Endpoint.Disposition == "EXCLUDED" {
					continue
				}
				seen++
				if (row.Status == "SOURCE_ADMITTED") != (mode == "link") {
					t.Fatal("ASSERT_LOCATIONLINK_DISTINCT_SELECTION_ENVELOPE", mode, row)
				}
			}
			if seen == 0 {
				t.Fatal("ASSERT_LOCATIONLINK_NONVACUOUS")
			}
			t.Log("ASSERT_LOCATIONLINK_DISTINCT_SELECTION_ENVELOPE: PASS", mode)
		})
	}
}

func TestPrivateSourceManagerOnlyAndPendingCompletion(t *testing.T) {
	for _, pending := range []bool{false, true} {
		t.Run(fmt.Sprint(pending), func(t *testing.T) {
			m, native, window := mixedLeaseFixtureV2(t)
			owner, root := inv.ControlledManagedInputOwnerForTest(t, false)
			var unfinished inv.PrivateManagedGroupingInput
			err := m.WithReadySessionLease(window, native.Session, func(ctx context.Context) error {
				h, err := m.CapturePrivateGroupingOwned(ctx, sessionruntime.PrivateGroupingOwnedRequest{Session: native.Session, URI: native.Directional.URI, OccurrenceID: "uncompleted", Revision: native.Revision, Privacy: native.Privacy, Line: *native.Directional.Line, Character: *native.Directional.Character, References: true, Definitions: true, Deadline: native.Directional.Deadline})
				if err != nil {
					return err
				}
				if pending {
					q := inv.PrivateManagedGroupingRequest{Session: native.Session, URI: native.Directional.URI, OccurrenceID: "uncompleted", Revision: native.Revision, Privacy: native.Privacy, Line: *native.Directional.Line, Character: *native.Directional.Character, Deadline: native.Directional.Deadline, OccurrenceAdmissionPolicyID: "ordinal", CompositionPolicyID: "identity", Kinds: []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol, programcadmission.TypedResolvesToDefinition}, Root: root}
					unfinished = inv.ControlledPendingSourceIssueForTest(t, ctx, m, h, q, owner)
					return nil
				}
				if err = m.ConsumePrivateGroupingOwned(ctx, h, func(sessionruntime.PrivateGroupingOwnedView) error { return nil }); err != nil {
					return err
				}
				evidence, err := m.TakePrivateGroupingSourceEvidence(ctx, h)
				if err != nil {
					return err
				}
				v, err := evidence.Snapshot(sessionruntime.PrivateGroupingQueryMaxBytes)
				if err != nil {
					return err
				}
				inv.ControlledNonProducerCandidateForTest(t, evidence, v)
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if pending {
				failed, stop := context.WithCancel(window)
				stop()
				completion, finishErr := m.FinishPrivateProviderWindow(failed)
				if finishErr == nil && completion.Completed {
					t.Fatal("ASSERT_FAILED_FINISH_CONTROL")
				}
				if _, err := unfinished.SourceOriginalsV1(); err == nil {
					t.Fatal("ASSERT_FAILED_FINISH_NO_ORIGINALS")
				}
				t.Log("ASSERT_FAILED_FINISH_NO_ORIGINALS: PASS")
			}
		})
	}
}

// These counterexamples call the actual successful legacy entries, not the
// lower candidate builder. Report only after the active lease and workers drain.
func TestPrivateSourceLegacyIssueRequiresJoinedFinish(t *testing.T) {
	privateSourceLegacyCompletionGuard(t, true)
}

func TestPrivateSourceLegacyAcquireRequiresJoinedFinish(t *testing.T) {
	privateSourceLegacyCompletionGuard(t, false)
}

func privateSourceLegacyCompletionGuard(t *testing.T, issue bool) {
	t.Helper()
	m, start, uri := mixedStartFixtureV2(t)
	// The legacy entries issue R+D, not Calls. Use their exact existing schedule;
	// a Calls window would intentionally remain incomplete at Finish.
	envelope, err := m.PrivateProviderWindowEnvelope(start, sessionruntime.PrivateProviderWindowPlan{References: true, Definitions: true})
	if err != nil {
		t.Fatal(err)
	}
	window, err := m.PrivateProviderWindowContext(context.Background(), envelope, start.Deadline)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = m.FinishPrivateProviderWindow(window) })
	started := m.Start(window, start)
	if started.Failure != "" {
		t.Fatal(started.Failure)
	}
	pending := m.BeginReadiness(window, started.SessionID, started.Generation, start.Deadline)
	ready, ok := m.WaitReadiness(window, pending.ID)
	if !ok || ready.State != sessionruntime.ReadinessReady {
		t.Fatalf("ASSERT_LEGACY_SOURCE_READY: %+v", ready)
	}
	session := sessionruntime.SessionLeaseRequest{SessionID: started.SessionID, Generation: started.Generation, Workspace: start.Profile.Workspace().String()}
	owner, root := inv.ControlledManagedInputOwnerForTest(t, false)
	q := inv.PrivateManagedGroupingRequest{Session: session, URI: uri, OccurrenceID: "legacy-source-completion", Revision: "controlled-mixed-revision", Privacy: "private-mixed-only", Line: 1, Character: 6, Deadline: start.Deadline, OccurrenceAdmissionPolicyID: "ordinal", CompositionPolicyID: "identity", Kinds: []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol, programcadmission.TypedResolvesToDefinition}, Root: root}
	var input inv.PrivateManagedGroupingInput
	var originals inv.PrivateProducerOriginals
	var sourceErr error
	checked := false
	reserve := func(ctx context.Context, mode string, h sessionruntime.PrivateGroupingOwnedHandoff) error {
		binding, err := inv.PrivateManagedGroupingOperationBinding(m, q, mode, h)
		if err != nil {
			return err
		}
		q.Operation, err = owner.Reserve(ctx, fmt.Sprintf("sha256:%x", sha256.Sum256([]byte("legacy-source-"+mode))), binding)
		return err
	}
	if issue {
		err = m.WithReadySessionLease(window, session, func(ctx context.Context) error {
			h, e := m.CapturePrivateGroupingOwned(ctx, sessionruntime.PrivateGroupingOwnedRequest{Session: q.Session, URI: q.URI, OccurrenceID: q.OccurrenceID, Revision: q.Revision, Privacy: q.Privacy, Line: q.Line, Character: q.Character, References: true, Definitions: true, Deadline: q.Deadline})
			if e != nil {
				return e
			}
			if e = reserve(ctx, "ISSUE", h); e != nil {
				return e
			}
			input, e = inv.IssuePrivateManagedGroupingInput(ctx, m, h, q)
			if e != nil {
				return e
			}
			// Observe succeeded, but this callback has NOT returned its lease and
			// Finish has NOT run. Minting here is the premature-eligibility bypass.
			originals, sourceErr = input.SourceOriginalsV1()
			checked = true
			return nil
		})
	} else {
		err = reserve(window, "ACQUIRE", sessionruntime.PrivateGroupingOwnedHandoff{})
		if err == nil {
			input, err = inv.AcquirePrivateManagedGroupingInput(window, m, q)
		}
		if err == nil {
			// Acquire returned its inner lease and confirmed Observe, but did not
			// join this provider window. It is still not a source producer.
			originals, sourceErr = input.SourceOriginalsV1()
			checked = true
		}
	}
	completion, finishErr := m.FinishPrivateProviderWindow(window)
	if finishErr != nil || !completion.Completed || m.Census().Workers != 0 {
		t.Fatalf("ASSERT_LEGACY_SOURCE_CONTROL_DRAINED: completion=%+v err=%v workers=%d entry=%v", completion, finishErr, m.Census().Workers, err)
	}
	if err != nil || !checked || input.Input.ID() == "" || len(input.Ledger) == 0 {
		t.Fatalf("ASSERT_LEGACY_SOURCE_ENTRY_ACTUALLY_SUCCEEDED: issue=%t checked=%t err=%v", issue, checked, err)
	}
	assertion := "ASSERT_LEGACY_ACQUIRE_NO_ORIGINALS_WITHOUT_FINISH"
	if issue {
		assertion = "ASSERT_LEGACY_ISSUE_NO_ORIGINALS_INSIDE_ACTIVE_LEASE"
	}
	if sourceErr == nil || originals.Bound() != 0 {
		t.Fatal(assertion)
	}
	if _, e := input.SourceOriginalsV1(); e == nil {
		t.Fatal("ASSERT_LEGACY_FINISH_DOES_NOT_RETROACTIVELY_MINT")
	}
	t.Log("ASSERT_LEGACY_SOURCE_ENTRY_ACTUALLY_SUCCEEDED: PASS; confirmed Observe, typed input retained; lease and workers drained")
	t.Log(assertion + ": PASS")
}

func TestPrivateSourceVerifiedBytesAndBounds(t *testing.T) {
	_, h, c, q := sourceStageFixture(t, "positive", false)
	workspace := qWorkspace(q)
	ctx, cancel := context.WithDeadline(context.Background(), q.Deadline)
	defer cancel()
	store := &sourceStageStore{MemoryStore: cc.NewMemoryStore()}
	originals := q.Objects[0]
	foreign := append([]byte(nil), originals.Bytes...)
	foreign[0] ^= 1
	for _, object := range []sourceobject.Object{{Identity: originals.Identity, Bytes: foreign}, {Identity: sourceobject.Identity{Digest: fmt.Sprintf("sha256:%x", sha256.Sum256(foreign)), ByteLength: uint64(len(foreign))}, Bytes: foreign}, {Identity: sourceobject.Identity{Digest: originals.Identity.Digest, ByteLength: uint64(len(originals.Bytes) - 1)}, Bytes: originals.Bytes[:len(originals.Bytes)-1]}} {
		local := q
		local.Objects = []sourceobject.Object{object}
		got := sc.Run(ctx, sc.Request{PrivateAllMode: &h, PrivateSources: &local, Workspace: workspace, Contract: c, Store: store, StopAfter: sc.StopAfterPrivateSourceRequests})
		if got.Err != nil {
			t.Fatal(got.Err)
		}
		if got.PrivateSources.Counts.Admitted != 0 || got.PrivateSources.Counts.Requests != 0 {
			t.Fatal("ASSERT_SOURCE_FOREIGN_BYTES_NEVER_ADMITTED")
		}
	}
	for _, mutate := range []func(*sc.PrivateAllModeSourcesRequest){func(x *sc.PrivateAllModeSourcesRequest) { x.Limits.MaxWork = 1 }, func(x *sc.PrivateAllModeSourcesRequest) { x.PolicyID = "foreign" }, func(x *sc.PrivateAllModeSourcesRequest) { x.Originals = []inv.PrivateProducerOriginals{{}} }} {
		local := q
		mutate(&local)
		before := store.puts
		bad := sc.Run(ctx, sc.Request{PrivateAllMode: &h, PrivateSources: &local, Workspace: workspace, Contract: c, Store: store, StopAfter: sc.StopAfterPrivateSourceRequests})
		if bad.Err == nil || store.puts != before {
			t.Fatal("ASSERT_SOURCE_PREFLIGHT_ZERO_PUT", bad.Err)
		}
	}
	local := q
	local.Limits.MaxDocumentBytes = len(originals.Bytes) - 1
	before := store.puts
	got := sc.Run(ctx, sc.Request{PrivateAllMode: &h, PrivateSources: &local, Workspace: workspace, Contract: c, Store: store, StopAfter: sc.StopAfterPrivateSourceRequests})
	if got.Err == nil || store.puts != before {
		t.Fatal("ASSERT_SOURCE_BOUND_BEFORE_PUT")
	}
	if !bytes.Equal(originals.Bytes, q.Objects[0].Bytes) {
		t.Fatal("ASSERT_SOURCE_FIXTURE_BYTES_IMMUTABLE")
	}
	t.Log("ASSERT_SOURCE_FOREIGN_BYTES_NEVER_ADMITTED: PASS; ASSERT_SOURCE_BOUND_BEFORE_PUT: PASS")
}
