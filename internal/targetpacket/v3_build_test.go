package targetpacket

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"lsp-trace/internal/censusprogramc"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/programcadmission"
	"lsp-trace/internal/retainedprojection"
	"lsp-trace/internal/sourceobject"
	"lsp-trace/internal/v5sourcesnapshot"
	"lsp-trace/internal/v5sourcesnapshotv2"
	"lsp-trace/internal/v5sourcesnapshotv3"
)

type v3Lookup map[sourceobject.Identity][]byte

func (l v3Lookup) Get(id sourceobject.Identity) (sourceobject.Object, error) {
	return sourceobject.Object{Identity: id, Bytes: append([]byte(nil), l[id]...)}, nil
}

type v3PacketFixture struct {
	request      Request
	target       graph.Node
	callers      []graph.Node
	predecessors []censusprogramc.RepresentativePredecessor
}

func buildV3PacketFixture(t *testing.T, callerCount int) v3PacketFixture {
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
	}
	callers = callers[:callerCount]
	rangeAt := func(start uint32) graph.Range {
		return graph.Range{Start: graph.Position{Line: 1, Character: start}, End: graph.Position{Line: 1, Character: start + 3}}
	}
	native := graph.Result{SchemaVersion: graph.SchemaVersionV5, Invocation: graph.Invocation{Server: graph.ServerInvocation{Command: "fixture"}, Seeds: []graph.InvocationSeed{{Label: "seed", At: "callee.go:1:1", ResolvedURI: target.URI, ContentSHA256: v3Digest(files["callee.go"]), LanguageID: "go"}}, Provenance: graph.InvocationProvenance{InvocationID: "session", SourceRevision: "fixture", ServerVersion: "fixture@1"}}, Nodes: append([]graph.Node{target}, callers...), Seeds: []graph.SeedResult{{Label: "seed", ReachedNodeIDs: []string{target.ID}}}, Summary: graph.Summary{NodeCount: 1 + len(callers), Complete: true}, Capabilities: graph.Capabilities{CallHierarchyProvider: true}}
	for i, caller := range callers {
		native.Edges = append(native.Edges, graph.Edge{RelationID: "relation-" + string(rune('a'+i)), CallerNodeID: caller.ID, CalleeNodeID: target.ID, CallSites: []graph.Range{rangeAt(9)}})
	}
	declaration := target
	origin := graph.NewNode(graph.Item{Name: "Origin", Kind: 12, URI: target.URI, Range: graph.Range{End: graph.Position{Character: 1}}, SelectionRange: graph.Range{End: graph.Position{Character: 1}}})
	native.SiblingCandidates = []graph.SiblingCandidate{{SeedURI: target.URI, SeedLabel: "seed", SeedLabels: []string{"seed"}, SeedIdentity: "session:seed:callee.go:1:1", Origin: origin, Declaration: &declaration, Candidate: target, Direction: "SIBLING", Kind: "TOPMOST_SIBLING", ProviderEvidence: []string{"command=fixture;server_version=fixture@1;invocation=session"}, LSPEvidence: []string{"textDocument/documentSymbol", "textDocument/prepareCallHierarchy"}, SourceDigests: []string{"candidate=" + v3Digest(files["callee.go"]), "origin=" + v3Digest(files["callee.go"])}, Custody: graph.SourceCustodyEvidence{Class: graph.SourceCustodyCallerAssertedLocal, SourceContentSHA256: v3Digest(files["callee.go"]), ClaimCeiling: "NO_AUTHENTICATED_ANALYZED_SOURCE_IDENTITY"}}}
	for i, caller := range callers {
		callerDeclaration := caller
		callerOrigin := graph.NewNode(graph.Item{Name: "CallerOrigin", Kind: 12, URI: caller.URI, Range: graph.Range{End: graph.Position{Character: 1}}, SelectionRange: graph.Range{End: graph.Position{Character: 1}}})
		name := "caller" + string(rune('1'+i)) + ".go"
		label := "caller-" + string(rune('a'+i))
		native.Invocation.Seeds = append(native.Invocation.Seeds, graph.InvocationSeed{Label: label, At: name + ":1:1", ResolvedURI: caller.URI, ContentSHA256: v3Digest(files[name]), LanguageID: "go"})
		native.Seeds = append(native.Seeds, graph.SeedResult{Label: label, ReachedNodeIDs: []string{caller.ID}})
		native.SiblingCandidates = append(native.SiblingCandidates, graph.SiblingCandidate{SeedURI: caller.URI, SeedLabel: label, SeedLabels: []string{label}, SeedIdentity: "session:" + label + ":" + name + ":1:1", Origin: callerOrigin, Declaration: &callerDeclaration, Candidate: caller, Direction: "SIBLING", Kind: "TOPMOST_SIBLING", ProviderEvidence: []string{"command=fixture;server_version=fixture@1;invocation=session"}, LSPEvidence: []string{"textDocument/documentSymbol", "textDocument/prepareCallHierarchy"}, SourceDigests: []string{"candidate=" + v3Digest(files[name]), "origin=" + v3Digest(files[name])}, Custody: graph.SourceCustodyEvidence{Class: graph.SourceCustodyCallerAssertedLocal, SourceContentSHA256: v3Digest(files[name]), ClaimCeiling: "NO_AUTHENTICATED_ANALYZED_SOURCE_IDENTITY"}})
	}
	nativeRaw := v3JSON(t, native)
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
	v2 := v5sourcesnapshotv2.Artifact{SchemaVersion: v5sourcesnapshotv2.Version, Policy: v5sourcesnapshotv2.Policy, ParentSchemaVersion: v5sourcesnapshot.Version, ParentSnapshotDigest: v3Digest(v1raw), ParentSnapshot: v1raw}
	for _, node := range append([]graph.Node{target}, callers...) {
		var binding v5sourcesnapshot.Binding
		for _, candidate := range v1.Bindings {
			if candidate.NodeID == node.ID && candidate.RangeRole == "DECLARATION_RANGE" {
				binding = candidate
				break
			}
		}
		if binding.NodeID == "" {
			t.Fatalf("V1 declaration binding missing for %s", node.ID)
		}
		v2.DisplayBindings = append(v2.DisplayBindings, v5sourcesnapshotv2.DisplayBinding{GraphSubjectID: binding.NodeID, LogicalSourceID: binding.URI, DisplayRange: binding.Range, DisplayRangePolicy: v5sourcesnapshotv2.DisplayRangePolicy, Provenance: v5sourcesnapshotv2.Provenance{Kind: v5sourcesnapshotv2.ProvenanceKind, Method: v5sourcesnapshotv2.ProvenanceMethod}, ReceiptID: binding.ReceiptIDs[0], SourceDigest: binding.SourceDigest, PositionEncoding: v1.PositionEncoding, Status: v5sourcesnapshotv2.Status, Custody: v5sourcesnapshotv2.Custody})
	}
	sort.Slice(v2.DisplayBindings, func(i, j int) bool {
		left := v2.DisplayBindings[i].GraphSubjectID + "\x00" + v2.DisplayBindings[i].LogicalSourceID
		right := v2.DisplayBindings[j].GraphSubjectID + "\x00" + v2.DisplayBindings[j].LogicalSourceID
		return left < right
	})
	v2raw := v3JSON(t, v2)
	v3raw, err := v5sourcesnapshotv3.Build(v2raw, root, v5sourcesnapshotv3.Limits{MaxArtifactBytes: 1 << 26, MaxParentBytes: 1 << 25, MaxReceipts: 100, MaxSourceBytes: 1 << 20, MaxTotalSourceBytes: 1 << 22, MaxBindings: 100, MaxWork: 1000})
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
	nomination := selected()
	nomination.Status, nomination.Authority, nomination.SourceGraphComplete = censusprogramc.CandidateStatus, 0, "UNKNOWN"
	nomination.SelectedNode, nomination.ClaimCeiling = target.ID, "STRUCTURAL"
	nomination.BatchID, nomination.CommunityIdentity, nomination.ExecutionBundleID = "batch", "community", "bundle"
	nomination.SeedLabel, nomination.SeedAt = "seed", "at"
	nomination.Members, nomination.SCCMembers = []string{"member"}, []string{"member"}
	nomination.IncomingPredecessors = predecessors
	census := censusprogramc.Result{CensusID: "census", Admission: programcadmission.Result{Artifact: programcadmission.Artifact{Constituents: []programcadmission.ConstituentReference{{Identity: "constituent", GraphSHA256: custody.GraphDigest, GraphByteLength: int(custody.GraphByteLength)}}}}, Representatives: censusprogramc.RepresentativeSelection{State: "SELECTED", Nominations: []censusprogramc.Representative{nomination}}}
	lookup := v3Lookup{}
	for _, receipt := range v1.Receipts {
		lookup[sourceobject.Identity{Digest: receipt.ContentDigest, ByteLength: uint64(len(receipt.Content))}] = receipt.Content
	}
	for _, receipt := range v3.Receipts {
		lookup[sourceobject.Identity{Digest: receipt.ContentDigest, ByteLength: uint64(len(receipt.Content))}] = receipt.Content
	}
	policy := structPolicy()
	policy.MaxBytes, policy.MaxRanges, policy.MaxObjects, policy.MaxWork = 1<<20, 100, 100, 1000
	request := Request{Census: census, Snapshots: []Snapshot{{ConstituentIdentity: "constituent", ConstituentOrdinal: 0, Raw: v3raw}}, Lookup: lookup, Policy: policy, ResolveLimits: retainedprojection.ResolveLimits{MaxDistinctObjects: uint64(len(lookup)), MaxUniqueSourceBytes: 1 << 22, MaxLogicalSelections: uint64(1 + len(callers) + len(predecessors))}, MaxResponseBytes: 1 << 20}
	return v3PacketFixture{request: request, target: target, callers: callers, predecessors: predecessors}
}

func TestBuildV3OneCallerEndToEnd(t *testing.T) {
	fixture := buildV3PacketFixture(t, 1)
	result, err := Build(fixture.request)
	if err != nil {
		var failure *Failure
		if errors.As(err, &failure) {
			t.Fatalf("ASSERT_CONSUMER_V3_BUILD: stage=%s code=%s cause=%v", failure.Stage, failure.Code, failure.Err)
		}
		t.Fatalf("ASSERT_CONSUMER_V3_BUILD: %v", err)
	}
	if result.State != StatePrepared || len(result.Packets) != 1 {
		t.Fatalf("ASSERT_CONSUMER_ONE_PACKET_PER_NOMINATION: %+v", result)
	}
	packet := result.Packets[0]
	if packet.ConsumerResolution != ConsumerResolved || len(packet.ConsumerAlternatives) != 1 {
		t.Fatalf("ASSERT_CONSUMER_ONE_ALTERNATIVE: %+v", packet.ConsumerAlternatives)
	}
	if len(packet.Projection.Units) != 3 {
		t.Fatalf("ASSERT_CONSUMER_TARGET_CALLER_RELATION_ONE_PROJECTION: units=%d", len(packet.Projection.Units))
	}
	raw, err := EncodeCanonical(packet)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Validate(raw); err != nil {
		t.Fatalf("ASSERT_CONSUMER_V3_CANONICAL_VALID: %v", err)
	}
	if !bytes.Equal(raw, mustEncodePacket(t, packet)) {
		t.Fatal("ASSERT_CONSUMER_V3_CANONICAL_STABLE")
	}
}

func TestBuildV3MultipleCallersDeterministicAndOwned(t *testing.T) {
	fixture := buildV3PacketFixture(t, 2)
	first, err := Build(fixture.request)
	if err != nil || len(first.Packets) != 1 {
		var failure *Failure
		if errors.As(err, &failure) {
			t.Fatalf("ASSERT_CONSUMER_V3_MULTI_BUILD: packets=%d stage=%s code=%s cause=%v", len(first.Packets), failure.Stage, failure.Code, failure.Err)
		}
		t.Fatalf("ASSERT_CONSUMER_V3_MULTI_BUILD: packets=%d err=%v", len(first.Packets), err)
	}
	packet := first.Packets[0]
	if len(packet.ConsumerAlternatives) != 2 || len(packet.Projection.Units) != 5 {
		t.Fatalf("ASSERT_CONSUMER_ALL_CALLERS_RELATIONS_ONE_PROJECTION: alternatives=%d units=%d", len(packet.ConsumerAlternatives), len(packet.Projection.Units))
	}
	for _, alternative := range packet.ConsumerAlternatives {
		if alternative.TargetID != fixture.target.ID || alternative.CallerID == "" || alternative.CallerDisplay.GraphSubjectID != alternative.CallerID || alternative.RelationUnit.GraphSubjectID != alternative.RelationID || alternative.RelationUnit.OccurrenceID != alternative.OccurrenceID || alternative.RelationUnit.EvidenceRange != projectionRange(alternative.CallSite) || alternative.RelationCitation.Role != "RELATION" || !spanCarriesUnit(alternative.RelationSpan, alternative.RelationUnit) || alternative.Custody != packet.Custody {
			t.Fatalf("ASSERT_CONSUMER_EXACT_RETAINED_PROOF: %+v", alternative)
		}
	}
	fixture.request.Census.Representatives.Nominations[0].IncomingPredecessors[0], fixture.request.Census.Representatives.Nominations[0].IncomingPredecessors[1] = fixture.request.Census.Representatives.Nominations[0].IncomingPredecessors[1], fixture.request.Census.Representatives.Nominations[0].IncomingPredecessors[0]
	second, err := Build(fixture.request)
	if err != nil {
		t.Fatal(err)
	}
	left, _ := EncodeCanonical(packet)
	right, _ := EncodeCanonical(second.Packets[0])
	if !bytes.Equal(left, right) || packet.PacketID != second.Packets[0].PacketID {
		t.Fatal("ASSERT_CONSUMER_SHUFFLED_PREDECESSORS_CANONICAL_BYTES_AND_ID")
	}

	fixture = buildV3PacketFixture(t, 1)
	sibling := fixture.request.Census.Representatives.Nominations[0]
	sibling.CommunityIdentity = "community-2"
	sibling.SeedLabel = "seed-2"
	fixture.request.Census.Representatives.Nominations = append(fixture.request.Census.Representatives.Nominations, sibling)
	owned, err := Build(fixture.request)
	if err != nil || len(owned.Packets) != 2 {
		t.Fatalf("ASSERT_CONSUMER_SIBLING_BUILD: %v", err)
	}
	before, _ := EncodeCanonical(owned.Packets[1])
	owned.Packets[0].ConsumerAlternatives[0].RelationSpan.UnitIDs[0] = "mutated"
	owned.Packets[0].ConsumerAlternatives[0].CallerDisplay.ItemRange.Start.Line++
	owned.Packets[0].Projection.EmittedSpans[0].UnitIDs[0] = "projection-mutated"
	after, _ := EncodeCanonical(owned.Packets[1])
	if !bytes.Equal(before, after) {
		t.Fatal("ASSERT_CONSUMER_SIBLING_PACKETS_DO_NOT_ALIAS")
	}
}

func TestValidateV3ConsumerMutationMatrix(t *testing.T) {
	fixture := buildV3PacketFixture(t, 2)
	result, err := Build(fixture.request)
	if err != nil {
		t.Fatal(err)
	}
	base := result.Packets[0]
	reject := func(t *testing.T, mutate func(*Packet)) {
		t.Helper()
		var packet Packet
		raw, _ := EncodeCanonical(base)
		if err := json.Unmarshal(raw, &packet); err != nil {
			t.Fatal(err)
		}
		mutate(&packet)
		for i := range packet.ConsumerAlternatives {
			packet.ConsumerAlternatives[i].ReconciliationID = consumerReconciliationID(packet.ConsumerAlternatives[i])
		}
		raw, _ = EncodeCanonical(packet)
		if _, err := Validate(raw); err == nil {
			t.Fatal("mutation accepted")
		}
	}
	cases := map[string]func(*Packet){
		"resolution": func(p *Packet) { p.ConsumerResolution = ConsumerUnresolved },
		"reorder": func(p *Packet) {
			p.ConsumerAlternatives[0], p.ConsumerAlternatives[1] = p.ConsumerAlternatives[1], p.ConsumerAlternatives[0]
		},
		"duplicate alternative": func(p *Packet) { p.ConsumerAlternatives[1] = p.ConsumerAlternatives[0] },
		"caller":                func(p *Packet) { p.ConsumerAlternatives[0].CallerID = "substituted" },
		"target":                func(p *Packet) { p.ConsumerAlternatives[0].TargetID = "substituted" },
		"occurrence":            func(p *Packet) { p.ConsumerAlternatives[0].OccurrenceID = "substituted" },
		"relation":              func(p *Packet) { p.ConsumerAlternatives[0].RelationID = "substituted" },
		"range":                 func(p *Packet) { p.ConsumerAlternatives[0].CallSite.End.Character++ },
		"citation role":         func(p *Packet) { p.ConsumerAlternatives[0].RelationCitation.Role = "ENDPOINT" },
		"citation evidence":     func(p *Packet) { p.ConsumerAlternatives[0].RelationCitation.EvidenceRange.End.Character++ },
		"unit provenance":       func(p *Packet) { p.ConsumerAlternatives[0].RelationUnit.RelationProvenance = "TEXT" },
		"unit encoding":         func(p *Packet) { p.ConsumerAlternatives[0].RelationUnit.PositionEncoding = "utf-8" },
		"span digest":           func(p *Packet) { p.ConsumerAlternatives[0].RelationSpan.SourceDigest = v3Digest([]byte("foreign")) },
		"span length":           func(p *Packet) { p.ConsumerAlternatives[0].RelationSpan.ByteLength++ },
		"span body":             func(p *Packet) { p.ConsumerAlternatives[0].RelationSpan.Body = "foreign" },
		"duplicated proof": func(p *Packet) {
			p.Projection.Citations = append(p.Projection.Citations, p.ConsumerAlternatives[0].RelationCitation)
		},
		"coordinated caller": func(p *Packet) {
			a := &p.ConsumerAlternatives[0]
			a.CallerDisplay.GraphSubjectID = "substituted"
			a.CallerID = "substituted"
			for i := range p.Projection.Units {
				if p.Projection.Units[i].UnitID == a.CallerDisplay.UnitID {
					p.Projection.Units[i] = a.CallerDisplay
				}
			}
		},
		"coordinated source": func(p *Packet) {
			a := &p.ConsumerAlternatives[0]
			a.CallerLogicalSourceID = "file:///foreign.go"
			a.CallerDisplay.LogicalSourceID = a.CallerLogicalSourceID
			a.RelationUnit.LogicalSourceID = a.CallerLogicalSourceID
			a.RelationSpan.LogicalSourceID = a.CallerLogicalSourceID
			for i := range p.Projection.Units {
				if p.Projection.Units[i].UnitID == a.CallerDisplay.UnitID {
					p.Projection.Units[i] = a.CallerDisplay
				}
				if p.Projection.Units[i].UnitID == a.RelationUnit.UnitID {
					p.Projection.Units[i] = a.RelationUnit
				}
			}
			for i := range p.Projection.EmittedSpans {
				if p.Projection.EmittedSpans[i].SourceDigest == a.RelationSpan.SourceDigest {
					p.Projection.EmittedSpans[i] = a.RelationSpan
				}
			}
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) { reject(t, mutate) })
	}
	for _, mutate := range []func(*Packet){func(p *Packet) { p.Authority = 1 }, func(p *Packet) { p.Accepted = true }, func(p *Packet) { p.Completeness = "COMPLETE" }} {
		reject(t, mutate)
	}
	raw, _ := EncodeCanonical(base)
	unknown := bytes.Replace(raw, []byte(`{"schema_version":`), []byte(`{"unknown":true,"schema_version":`), 1)
	if _, err := Validate(unknown); err == nil {
		t.Fatal("ASSERT_CONSUMER_UNKNOWN_FIELD_REJECTED")
	}
	duplicate := bytes.Replace(raw, []byte(`{"schema_version":`), []byte(`{"schema_version":"bad","schema_version":`), 1)
	if _, err := Validate(duplicate); err == nil {
		t.Fatal("ASSERT_CONSUMER_DUPLICATE_KEY_REJECTED")
	}
	if _, err := Validate(append(raw, []byte(`{}`)...)); err == nil {
		t.Fatal("ASSERT_CONSUMER_TRAILING_JSON_REJECTED")
	}
}

func mustEncodePacket(t *testing.T, packet Packet) []byte {
	t.Helper()
	raw, err := EncodeCanonical(packet)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func v3JSON(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func v3Digest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}
