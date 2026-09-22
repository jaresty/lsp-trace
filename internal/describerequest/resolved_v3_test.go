package describerequest

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"lsp-trace/internal/censusprogramc"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/programcadmission"
	"lsp-trace/internal/retainedprojection"
	"lsp-trace/internal/sourceobject"
	"lsp-trace/internal/sourceprojection"
	"lsp-trace/internal/sourceprojectionv2"
	"lsp-trace/internal/targetpacket"
	"lsp-trace/internal/v5sourcesnapshot"
	"lsp-trace/internal/v5sourcesnapshotv2"
	"lsp-trace/internal/v5sourcesnapshotv3"
)

type resolvedLookup map[sourceobject.Identity][]byte

func (l resolvedLookup) Get(id sourceobject.Identity) (sourceobject.Object, error) {
	return sourceobject.Object{Identity: id, Bytes: append([]byte(nil), l[id]...)}, nil
}

type resolvedFixture struct {
	request targetpacket.Request
	result  targetpacket.Result
	target  graph.Node
}

func resolvedPacketResult(t *testing.T, callerCount int) resolvedFixture {
	t.Helper()
	root := t.TempDir()
	files := map[string][]byte{
		"callee.go":  []byte("package a\nfunc C(){}\n"),
		"caller1.go": []byte("package a\nfunc A(){C();C()}\n"),
		"caller2.go": []byte("package a\nfunc B(){C()}\n"),
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(root, name), body, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	uri := func(name string) string { return (&url.URL{Scheme: "file", Path: filepath.Join(root, name)}).String() }
	target := graph.NewNode(graph.Item{Name: "C", Kind: 12, URI: uri("callee.go"), Range: graph.Range{End: graph.Position{Line: 1, Character: 10}}, SelectionRange: graph.Range{End: graph.Position{Line: 1, Character: 6}}})
	callers := []graph.Node{
		graph.NewNode(graph.Item{Name: "A", Kind: 12, URI: uri("caller1.go"), Range: graph.Range{End: graph.Position{Line: 1, Character: 17}}, SelectionRange: graph.Range{End: graph.Position{Line: 1, Character: 6}}}),
		graph.NewNode(graph.Item{Name: "B", Kind: 12, URI: uri("caller2.go"), Range: graph.Range{End: graph.Position{Line: 1, Character: 13}}, SelectionRange: graph.Range{End: graph.Position{Line: 1, Character: 6}}}),
	}[:callerCount]
	native := graph.Result{SchemaVersion: graph.SchemaVersionV5, Invocation: graph.Invocation{Server: graph.ServerInvocation{Command: "fixture"}, Seeds: []graph.InvocationSeed{{Label: "seed", At: "callee.go:1:1", ResolvedURI: target.URI, ContentSHA256: resolvedDigest(files["callee.go"]), LanguageID: "go"}}, Provenance: graph.InvocationProvenance{InvocationID: "session", SourceRevision: "fixture", ServerVersion: "fixture@1"}}, Nodes: append([]graph.Node{target}, callers...), Seeds: []graph.SeedResult{{Label: "seed", ReachedNodeIDs: []string{target.ID}}}, Summary: graph.Summary{NodeCount: 1 + len(callers), Complete: true}, Capabilities: graph.Capabilities{CallHierarchyProvider: true}}
	for i, caller := range callers {
		native.Edges = append(native.Edges, graph.Edge{RelationID: "relation-" + string(rune('a'+i)), CallerNodeID: caller.ID, CalleeNodeID: target.ID, CallSites: []graph.Range{{Start: graph.Position{Line: 1, Character: 9}, End: graph.Position{Line: 1, Character: 12}}}})
	}
	declaration := target
	origin := graph.NewNode(graph.Item{Name: "Origin", Kind: 12, URI: target.URI, Range: graph.Range{End: graph.Position{Character: 1}}, SelectionRange: graph.Range{End: graph.Position{Character: 1}}})
	native.SiblingCandidates = []graph.SiblingCandidate{{SeedURI: target.URI, SeedLabel: "seed", SeedLabels: []string{"seed"}, SeedIdentity: "session:seed:callee.go:1:1", Origin: origin, Declaration: &declaration, Candidate: target, Direction: "SIBLING", Kind: "TOPMOST_SIBLING", ProviderEvidence: []string{"command=fixture;server_version=fixture@1;invocation=session"}, LSPEvidence: []string{"textDocument/documentSymbol", "textDocument/prepareCallHierarchy"}, SourceDigests: []string{"candidate=" + resolvedDigest(files["callee.go"]), "origin=" + resolvedDigest(files["callee.go"])}, Custody: graph.SourceCustodyEvidence{Class: graph.SourceCustodyCallerAssertedLocal, SourceContentSHA256: resolvedDigest(files["callee.go"]), ClaimCeiling: "NO_AUTHENTICATED_ANALYZED_SOURCE_IDENTITY"}}}
	for i, caller := range callers {
		name := "caller" + string(rune('1'+i)) + ".go"
		label := "caller-" + string(rune('a'+i))
		callerDeclaration := caller
		callerOrigin := graph.NewNode(graph.Item{Name: "CallerOrigin", Kind: 12, URI: caller.URI, Range: graph.Range{End: graph.Position{Character: 1}}, SelectionRange: graph.Range{End: graph.Position{Character: 1}}})
		native.Invocation.Seeds = append(native.Invocation.Seeds, graph.InvocationSeed{Label: label, At: name + ":1:1", ResolvedURI: caller.URI, ContentSHA256: resolvedDigest(files[name]), LanguageID: "go"})
		native.Seeds = append(native.Seeds, graph.SeedResult{Label: label, ReachedNodeIDs: []string{caller.ID}})
		native.SiblingCandidates = append(native.SiblingCandidates, graph.SiblingCandidate{SeedURI: caller.URI, SeedLabel: label, SeedLabels: []string{label}, SeedIdentity: "session:" + label + ":" + name + ":1:1", Origin: callerOrigin, Declaration: &callerDeclaration, Candidate: caller, Direction: "SIBLING", Kind: "TOPMOST_SIBLING", ProviderEvidence: []string{"command=fixture;server_version=fixture@1;invocation=session"}, LSPEvidence: []string{"textDocument/documentSymbol", "textDocument/prepareCallHierarchy"}, SourceDigests: []string{"candidate=" + resolvedDigest(files[name]), "origin=" + resolvedDigest(files[name])}, Custody: graph.SourceCustodyEvidence{Class: graph.SourceCustodyCallerAssertedLocal, SourceContentSHA256: resolvedDigest(files[name]), ClaimCeiling: "NO_AUTHENTICATED_ANALYZED_SOURCE_IDENTITY"}})
	}
	nativeRaw := resolvedJSON(t, native)
	v5raw, err := graphprovenance.CaptureV5(nativeRaw, "session", 1, manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable})
	if err != nil {
		t.Fatal(err)
	}
	v1raw, err := v5sourcesnapshot.Build(v5raw, root, "utf-16")
	if err != nil {
		t.Fatal(err)
	}
	var v1 v5sourcesnapshot.Artifact
	if err := json.Unmarshal(v1raw, &v1); err != nil {
		t.Fatal(err)
	}
	v2 := v5sourcesnapshotv2.Artifact{SchemaVersion: v5sourcesnapshotv2.Version, Policy: v5sourcesnapshotv2.Policy, ParentSchemaVersion: v5sourcesnapshot.Version, ParentSnapshotDigest: resolvedDigest(v1raw), ParentSnapshot: v1raw}
	for _, node := range append([]graph.Node{target}, callers...) {
		var binding v5sourcesnapshot.Binding
		for _, candidate := range v1.Bindings {
			if candidate.NodeID == node.ID && candidate.RangeRole == "DECLARATION_RANGE" {
				binding = candidate
				break
			}
		}
		if binding.NodeID == "" {
			t.Fatalf("declaration binding missing for %s", node.ID)
		}
		v2.DisplayBindings = append(v2.DisplayBindings, v5sourcesnapshotv2.DisplayBinding{GraphSubjectID: binding.NodeID, LogicalSourceID: binding.URI, DisplayRange: binding.Range, DisplayRangePolicy: v5sourcesnapshotv2.DisplayRangePolicy, Provenance: v5sourcesnapshotv2.Provenance{Kind: v5sourcesnapshotv2.ProvenanceKind, Method: v5sourcesnapshotv2.ProvenanceMethod}, ReceiptID: binding.ReceiptIDs[0], SourceDigest: binding.SourceDigest, PositionEncoding: v1.PositionEncoding, Status: v5sourcesnapshotv2.Status, Custody: v5sourcesnapshotv2.Custody})
	}
	sort.Slice(v2.DisplayBindings, func(i, j int) bool {
		return v2.DisplayBindings[i].GraphSubjectID+"\x00"+v2.DisplayBindings[i].LogicalSourceID < v2.DisplayBindings[j].GraphSubjectID+"\x00"+v2.DisplayBindings[j].LogicalSourceID
	})
	v3raw, err := v5sourcesnapshotv3.Build(resolvedJSON(t, v2), root, v5sourcesnapshotv3.Limits{MaxArtifactBytes: 1 << 26, MaxParentBytes: 1 << 25, MaxReceipts: 100, MaxSourceBytes: 1 << 20, MaxTotalSourceBytes: 1 << 22, MaxBindings: 100, MaxWork: 1000})
	if err != nil {
		t.Fatal(err)
	}
	var v3 v5sourcesnapshotv3.Artifact
	if err := json.Unmarshal(v3raw, &v3); err != nil {
		t.Fatal(err)
	}
	admitted, err := retainedprojection.Admit(v3raw)
	if err != nil {
		t.Fatal(err)
	}
	var targetKey retainedprojection.Key
	for _, key := range admitted.DisplayKeys() {
		if key.GraphSubjectID == target.ID {
			targetKey = key
		}
	}
	plan, err := retainedprojection.Select(admitted, retainedprojection.Request{Target: targetKey, Selections: []retainedprojection.Key{targetKey}})
	if err != nil {
		t.Fatal(err)
	}
	custody, err := admitted.CustodyBinding(plan)
	if err != nil {
		t.Fatal(err)
	}
	predecessors := make([]censusprogramc.RepresentativePredecessor, 0, len(v3.Bindings))
	for _, relation := range v3.Bindings {
		predecessors = append(predecessors, censusprogramc.RepresentativePredecessor{RelationID: relation.RelationID, OccurrenceID: relation.OccurrenceID, CallerID: relation.CallerNodeID, TargetID: relation.CalleeNodeID, CallSite: relation.Range})
	}
	nomination := censusprogramc.Representative{Status: censusprogramc.CandidateStatus, Authority: 0, SourceGraphComplete: "UNKNOWN", CensusID: "census", ConstituentIdentity: "constituent", ConstituentOrdinal: 0, SelectionState: "SELECTED", SelectedNode: target.ID, ClaimCeiling: "STRUCTURAL", BatchID: "batch", CommunityIdentity: "community", ExecutionBundleID: "bundle", SeedLabel: "seed", SeedAt: "at", Members: []string{"member"}, SCCMembers: []string{"member"}, IncomingPredecessors: predecessors}
	census := censusprogramc.Result{CensusID: "census", Admission: programcadmission.Result{Artifact: programcadmission.Artifact{Constituents: []programcadmission.ConstituentReference{{Identity: "constituent", GraphSHA256: custody.GraphDigest, GraphByteLength: int(custody.GraphByteLength)}}}}, Representatives: censusprogramc.RepresentativeSelection{State: "SELECTED", Nominations: []censusprogramc.Representative{nomination}}}
	lookup := resolvedLookup{}
	for _, receipt := range v1.Receipts {
		lookup[sourceobject.Identity{Digest: receipt.ContentDigest, ByteLength: uint64(len(receipt.Content))}] = receipt.Content
	}
	for _, receipt := range v3.Receipts {
		lookup[sourceobject.Identity{Digest: receipt.ContentDigest, ByteLength: uint64(len(receipt.Content))}] = receipt.Content
	}
	request := targetpacket.Request{Census: census, Snapshots: []targetpacket.Snapshot{{ConstituentIdentity: "constituent", Raw: v3raw}}, Lookup: lookup, Policy: sourceprojection.Policy{PolicyID: "p", BodyRequested: true, MaxBytes: 1 << 20, MaxRanges: 100, MaxObjects: 100, MaxWork: 1000, EnforceLimits: true}, ResolveLimits: retainedprojection.ResolveLimits{MaxDistinctObjects: uint64(len(lookup)), MaxUniqueSourceBytes: 1 << 22, MaxLogicalSelections: uint64(1 + len(callers) + len(predecessors))}, MaxResponseBytes: 1 << 20}
	result, err := targetpacket.Build(request)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := targetpacket.EncodeCanonical(result.Packets[0])
	if err != nil {
		t.Fatal(err)
	}
	validated, err := targetpacket.Validate(raw)
	if err != nil {
		t.Fatal(err)
	}
	result.Packets[0] = validated
	return resolvedFixture{request: request, result: result, target: target}
}

func clonePacket(t *testing.T, p targetpacket.Packet) targetpacket.Packet {
	t.Helper()
	raw, err := targetpacket.EncodeCanonical(p)
	if err != nil {
		t.Fatal(err)
	}
	var out targetpacket.Packet
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestBuildResolvedV3OneAlternativeExact(t *testing.T) {
	fx := resolvedPacketResult(t, 1)
	p := fx.result.Packets[0]
	if _, err := targetpacket.Validate(mustPacketBytes(t, p)); err != nil {
		t.Fatalf("ASSERT_RESOLVED_FIXTURE_VALID: %v", err)
	}
	a := p.ConsumerAlternatives[0]
	projectionCaller := endpointUnit(t, p, a.CallerID)
	if !reflect.DeepEqual(projectionCaller, a.CallerDisplay) {
		t.Fatalf("ASSERT_RESOLVED_CALLER_DISPLAY_MATCH: projection=%+v retained=%+v", projectionCaller, a.CallerDisplay)
	}
	records, err := Build(fx.result, 90000)
	if err != nil || len(records) != 1 {
		t.Fatalf("ASSERT_RESOLVED_ONE_REQUEST: records=%d err=%v", len(records), err)
	}
	r := records[0]
	for _, exact := range []string{a.CallerDisplay.Body, endpointUnit(t, p, fx.target.ID).Body, a.CallerID, a.CallerLogicalSourceID, fx.target.ID, p.Lineage.SelectedLogicalSourceID, a.RelationID, a.OccurrenceID, a.ReconciliationID, graphRangeText(a.CallSite)} {
		if !strings.Contains(r.Envelope.Prompt, exact) {
			t.Fatalf("ASSERT_RESOLVED_EXACT_METADATA: missing %q", exact)
		}
	}
	if r.Lineage.AlternativeID != a.ReconciliationID || r.Lineage.AlternativeOrdinal != 0 || r.Lineage.ConsumerResolution != string(targetpacket.ConsumerResolved) {
		t.Fatalf("ASSERT_RESOLVED_EXACT_LINEAGE: %+v", r.Lineage)
	}
}

func TestBuildResolvedV3ManyCanonicalPermutation(t *testing.T) {
	fx := resolvedPacketResult(t, 2)
	baseline, err := Build(fx.result, 90000)
	if err != nil || len(baseline) != 2 {
		t.Fatalf("ASSERT_RESOLVED_MANY_REQUESTS: records=%d err=%v", len(baseline), err)
	}
	seen := map[string]int{}
	for i, r := range baseline {
		seen[r.Lineage.AlternativeID]++
		if r.Lineage.AlternativeOrdinal != i {
			t.Fatalf("ASSERT_RESOLVED_CANONICAL_ORDER: %+v", r.Lineage)
		}
	}
	for _, a := range fx.result.Packets[0].ConsumerAlternatives {
		if seen[a.ReconciliationID] != 1 {
			t.Fatalf("ASSERT_RESOLVED_EVERY_ALTERNATIVE_ONCE: %s=%d", a.ReconciliationID, seen[a.ReconciliationID])
		}
	}
	permutedRequest := fx.request
	permutedRequest.Census.Representatives.Nominations = append([]censusprogramc.Representative(nil), fx.request.Census.Representatives.Nominations...)
	permutedRequest.Census.Representatives.Nominations[0].IncomingPredecessors = append([]censusprogramc.RepresentativePredecessor(nil), fx.request.Census.Representatives.Nominations[0].IncomingPredecessors...)
	predecessors := permutedRequest.Census.Representatives.Nominations[0].IncomingPredecessors
	predecessors[0], predecessors[1] = predecessors[1], predecessors[0]
	permutedPackets, err := targetpacket.Build(permutedRequest)
	if err != nil {
		t.Fatal(err)
	}
	again, err := Build(permutedPackets, 90000)
	if err != nil {
		t.Fatal(err)
	}
	leftRecords, _ := Bytes(baseline)
	rightRecords, _ := Bytes(again)
	leftNDJSON, _ := NDJSON(baseline)
	rightNDJSON, _ := NDJSON(again)
	if !bytes.Equal(leftRecords, rightRecords) || !bytes.Equal(leftNDJSON, rightNDJSON) {
		t.Fatal("ASSERT_RESOLVED_PERMUTATION_CANONICAL_BYTES")
	}
}

func TestEndpointSourceResolvedV3ExtractionMatrix(t *testing.T) {
	p := resolvedPacketResult(t, 1).result.Packets[0]
	a := p.ConsumerAlternatives[0]
	if _, _, err := endpointSource(p, a.CallerID, a.CallerLogicalSourceID); err != nil {
		t.Fatalf("ASSERT_ENDPOINT_COALESCED_SPAN_SUPPORTED: %v", err)
	}
	cases := map[string]func(*targetpacket.Packet){
		"missing-body": func(p *targetpacket.Packet) { endpointUnitPtr(p, a.CallerID).Body = "" },
		"digest-mismatch": func(p *targetpacket.Packet) {
			endpointSpanPtr(p, endpointUnitPtr(p, a.CallerID).UnitID).SourceDigest = "sha256:" + strings.Repeat("0", 64)
		},
		"length-mismatch":  func(p *targetpacket.Packet) { endpointSpanPtr(p, endpointUnitPtr(p, a.CallerID).UnitID).ByteLength++ },
		"missing-endpoint": func(p *targetpacket.Packet) { endpointUnitPtr(p, a.CallerID).Role = "RELATION" },
		"duplicate-endpoint": func(p *targetpacket.Packet) {
			p.Projection.Units = append(p.Projection.Units, *endpointUnitPtr(p, a.CallerID))
		},
		"non-containing-span": func(p *targetpacket.Packet) {
			endpointSpanPtr(p, endpointUnitPtr(p, a.CallerID).UnitID).Range.End = sourceprojection.Position{}
		},
		"ambiguous-span": func(p *targetpacket.Packet) {
			p.Projection.EmittedSpans = append(p.Projection.EmittedSpans, *endpointSpanPtr(p, endpointUnitPtr(p, a.CallerID).UnitID))
		},
		"caller-target-substitution": func(p *targetpacket.Packet) {
			u := endpointUnitPtr(p, a.CallerID)
			u.GraphSubjectID = p.Lineage.SelectedNode
			u.LogicalSourceID = p.Lineage.SelectedLogicalSourceID
		},
		"target-caller-substitution": func(p *targetpacket.Packet) {
			u := endpointUnitPtr(p, p.Lineage.SelectedNode)
			u.GraphSubjectID = a.CallerID
			u.LogicalSourceID = a.CallerLogicalSourceID
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			q := clonePacket(t, p)
			mutate(&q)
			if _, _, err := endpointSource(q, a.CallerID, a.CallerLogicalSourceID); err == nil {
				t.Fatal("ASSERT_ENDPOINT_INVALID_REJECTED")
			}
		})
	}
}

func TestBuildResolvedV3RejectsPacketAndProofMutationsBeforeRendering(t *testing.T) {
	base := resolvedPacketResult(t, 1).result
	for name, mutate := range map[string]func(*targetpacket.Packet){
		"packet": func(p *targetpacket.Packet) { p.Authority = 1 },
		"proof":  func(p *targetpacket.Packet) { p.ConsumerAlternatives[0].RelationSpan.Body = "substituted" },
	} {
		t.Run(name, func(t *testing.T) {
			result := targetpacket.Result{State: base.State, Packets: []targetpacket.Packet{clonePacket(t, base.Packets[0])}}
			mutate(&result.Packets[0])
			records, err := Build(result, 90000)
			if err == nil || len(records) != 0 {
				t.Fatalf("ASSERT_RESOLVED_MUTATION_REJECTED_BEFORE_RENDER: records=%d err=%v", len(records), err)
			}
		})
	}
}

func TestBuildResolvedV3DefensiveOwnership(t *testing.T) {
	fx := resolvedPacketResult(t, 2)
	first, err := Build(fx.result, 90000)
	if err != nil {
		t.Fatal(err)
	}
	baseline, _ := Bytes(first)
	ndjson, _ := NDJSON(first)
	envelopes, err := ParseNDJSON(ndjson, first)
	if err != nil {
		t.Fatal(err)
	}
	first[0].Envelope.Prompt = "mutated"
	first[0].RecordID = "mutated"
	baseline[0] ^= 0xff
	ndjson[0] ^= 0xff
	envelopes[0].Prompt = "mutated"
	second, err := Build(fx.result, 90000)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := Bytes(second)
	freshNDJSON, _ := NDJSON(second)
	parsed, err := ParseNDJSON(freshNDJSON, second)
	if err != nil || len(parsed) != 2 || bytes.Equal(got, baseline) || bytes.Equal(freshNDJSON, ndjson) {
		t.Fatal("ASSERT_RESOLVED_OUTPUTS_DEFENSIVELY_OWNED")
	}
}

func TestBuildResolvedV3SemanticIDsAndUnresolvedC2(t *testing.T) {
	resolved := resolvedPacketResult(t, 2).result
	records, err := Build(resolved, 90000)
	if err != nil {
		t.Fatal(err)
	}
	if records[0].RecordID == records[1].RecordID || records[0].Envelope.MessageID == records[1].Envelope.MessageID || records[0].Lineage.LineageIdentity == records[1].Lineage.LineageIdentity {
		t.Fatal("ASSERT_RESOLVED_SEMANTIC_CHANGE_CHANGES_IDS")
	}
	unresolved, err := Build(packetResult(t), 90000)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(unresolved[0].Envelope.Prompt, "C2 source:") || strings.Contains(unresolved[0].Envelope.Prompt, "C2 OUTWARD_CONSUMER graph_subject_id") {
		t.Fatal("ASSERT_UNRESOLVED_NEVER_INVENTS_C2")
	}
}

func endpointUnit(t *testing.T, p targetpacket.Packet, subject string) sourceprojectionv2.Unit {
	t.Helper()
	unit := endpointUnitPtr(&p, subject)
	if unit == nil {
		t.Fatalf("endpoint unit missing for %s", subject)
	}
	return *unit
}

func endpointUnitPtr(p *targetpacket.Packet, subject string) *sourceprojectionv2.Unit {
	for i := range p.Projection.Units {
		if p.Projection.Units[i].Role == "ENDPOINT" && p.Projection.Units[i].GraphSubjectID == subject {
			return &p.Projection.Units[i]
		}
	}
	return nil
}
func endpointSpanPtr(p *targetpacket.Packet, unitID string) *sourceprojection.Span {
	for i := range p.Projection.EmittedSpans {
		for _, id := range p.Projection.EmittedSpans[i].UnitIDs {
			if id == unitID {
				return &p.Projection.EmittedSpans[i]
			}
		}
	}
	return nil
}
func mustPacketBytes(t *testing.T, p targetpacket.Packet) []byte {
	t.Helper()
	raw, err := targetpacket.EncodeCanonical(p)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func resolvedJSON(t *testing.T, v any) []byte {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func resolvedDigest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}
