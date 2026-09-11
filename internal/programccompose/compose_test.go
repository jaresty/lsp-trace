package programccompose

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/manageddiagnostic"
)

func capture(t *testing.T, suffix string, nodes []graph.Node, edges []graph.Edge, complete, truncated bool) Input {
	t.Helper()
	seed := graph.InvocationSeed{Label: "seed" + suffix, At: "a.go:1:1", ResolvedURI: "file:///w/a.go", ContentSHA256: "sha256:" + strings.Repeat("a", 64), LanguageID: "go"}
	var frontier []graph.Boundary
	if len(nodes) > 0 {
		frontier = []graph.Boundary{{NodeID: nodes[0].ID, Reason: graph.MaxDepth}}
	}
	r := graph.Result{SchemaVersion: graph.SchemaVersionV5, Invocation: graph.Invocation{WorkspaceURI: "file:///w", Server: graph.ServerInvocation{Command: "gopls"}, LanguageID: "go", Seeds: []graph.InvocationSeed{seed}, Provenance: graph.InvocationProvenance{InvocationID: "invocation-" + suffix, SourceRevision: "commit", ServerVersion: "gopls@1"}}, Nodes: nodes, Edges: edges, Seeds: []graph.SeedResult{{Label: seed.Label}}, Frontier: frontier, Diagnostics: []graph.Diagnostic{{Phase: "capture-" + suffix, Message: "diagnostic-" + suffix}}, Summary: graph.Summary{Complete: complete, Truncated: truncated}, Capabilities: graph.Capabilities{CallHierarchyProvider: true}}
	native, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := graphprovenance.CaptureV5(native, "session", 7, manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable})
	if err != nil {
		t.Fatal(err)
	}
	s := sha256.Sum256(raw)
	return Input{Bytes: raw, Identity: "capture-" + suffix, SHA256: fmt.Sprintf("sha256:%x", s), ByteLength: len(raw), ExactMetadata: ExactMetadata{WorkspaceIdentity: "workspace-1", RevisionCustody: "CALLER_ASSERTED", PositionEncoding: "utf-16", AcquisitionSemantics: "managed-lsp-v1", PrivacyPolicy: "bundle-custodian-v1"}}
}
func node(name string) graph.Node {
	return graph.NewNode(graph.Item{Name: name, Kind: 12, URI: "file:///w/a.go", Range: graph.Range{End: graph.Position{Character: uint32(len(name))}}, SelectionRange: graph.Range{End: graph.Position{Character: uint32(len(name))}}})
}
func edge(a, b graph.Node, sites int) graph.Edge {
	rs := make([]graph.Range, sites)
	for i := range rs {
		rs[i] = graph.Range{Start: graph.Position{Line: uint32(i)}, End: graph.Position{Line: uint32(i), Character: 1}}
	}
	return graph.Edge{CallerNodeID: a.ID, CalleeNodeID: b.ID, CallSites: rs}
}

func TestComposePermutationUnionDedupeReplayAndConservativeCompleteness(t *testing.T) {
	a, b, c := node("a"), node("b"), node("c")
	x := capture(t, "x", []graph.Node{a, b}, []graph.Edge{edge(a, b, 2)}, true, false)
	y := capture(t, "y", []graph.Node{b, c}, []graph.Edge{edge(b, c, 1), edge(c, c, 1)}, false, true)
	x2 := x
	x2.Identity = "capture-x-second-receipt"
	one, err := Compose([]Input{x, y, x2})
	if err != nil {
		t.Fatal(err)
	}
	two, err := Compose([]Input{x2, y, x})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(one.Bytes, two.Bytes) || one.Artifact.OutputSHA256 != two.Artifact.OutputSHA256 {
		t.Fatal("ASSERT_PERMUTATION_DETERMINISM")
	}
	if len(one.Artifact.Nodes) != 3 || len(one.Artifact.Edges) != 3 || len(one.Artifact.Constituents) != 3 {
		t.Fatalf("ASSERT_DISJOINT_OVERLAP_UNION_AND_RECEIPT_MULTIPLICITY nodes=%d edges=%d constituents=%d", len(one.Artifact.Nodes), len(one.Artifact.Edges), len(one.Artifact.Constituents))
	}
	compatibilityJSON, _ := json.Marshal(one.Artifact.Compatibility)
	invocations := map[string]bool{}
	for _, c := range one.Artifact.Constituents {
		invocations[c.InvocationID] = true
	}
	if len(invocations) < 2 || bytes.Contains(compatibilityJSON, []byte("InvocationID")) {
		t.Fatal("ASSERT_DISTINCT_INVOCATIONS_ALLOWED_WITHOUT_HOMOGENEOUS_CLAIM")
	}
	for _, c := range one.Artifact.Constituents {
		if c.InvocationID == "" || len(c.Invocation) == 0 || len(c.Seeds) == 0 || len(c.Frontier) == 0 || len(c.Diagnostics) == 0 || len(c.Summary) == 0 {
			t.Fatal("ASSERT_CONSTITUENT_TRAVERSAL_DIAGNOSTICS_AND_INVOCATION_PRESERVED")
		}
	}
	occ := 0
	self := false
	for _, e := range one.Artifact.Edges {
		occ += len(e.CallSites)
		self = self || e.CallerNodeID == e.CalleeNodeID
	}
	if occ != 4 || !self {
		t.Fatal("ASSERT_PARALLEL_OCCURRENCES_AND_SELF_LOOP")
	}
	if one.Artifact.Completeness.AllTraversalComplete || !one.Artifact.Completeness.AnyTruncated || one.Artifact.Completeness.WholeWorkspace {
		t.Fatal("ASSERT_CONSERVATIVE_COMPLETENESS")
	}
	if _, err = Validate(one.Bytes); err != nil {
		t.Fatal("ASSERT_COMPOSITE_REPLAY", err)
	}
	tampered := append([]byte(nil), one.Bytes...)
	tampered[len(tampered)/2] ^= 1
	if _, err = Validate(tampered); err == nil {
		t.Fatal("ASSERT_TAMPER_REJECTED")
	}
}

func TestComposeRejectsConflictsMismatchesMalformedAndUnknown(t *testing.T) {
	a, b := node("a"), node("b")
	x := capture(t, "x", []graph.Node{a, b}, []graph.Edge{edge(a, b, 1)}, true, false)
	y := capture(t, "y", []graph.Node{a, b}, []graph.Edge{edge(a, b, 1)}, true, false)
	for name, mutate := range map[string]func(*Input){
		"workspace": func(i *Input) { i.ExactMetadata.WorkspaceIdentity = "other" }, "encoding": func(i *Input) { i.ExactMetadata.PositionEncoding = "utf-8" }, "provider-policy": func(i *Input) { i.ExactMetadata.PrivacyPolicy = "other" }, "revision-custody": func(i *Input) { i.ExactMetadata.RevisionCustody = "VERIFIED" }, "acquisition": func(i *Input) { i.ExactMetadata.AcquisitionSemantics = "other" },
	} {
		t.Run(name, func(t *testing.T) {
			z := y
			mutate(&z)
			if _, err := Compose([]Input{x, z}); err == nil {
				t.Fatal("ASSERT_COMPATIBILITY_MISMATCH_REJECTED")
			}
		})
	}
	z := y
	z.Identity = x.Identity
	z.SHA256 = strings.Replace(z.SHA256, "a", "b", 1)
	if _, err := Compose([]Input{x, z}); err == nil {
		t.Fatal("ASSERT_IDENTITY_DIGEST_CONFLICT")
	}
	for name, field := range map[string]any{"session_id": "other", "generation": float64(8)} {
		t.Run(name, func(t *testing.T) {
			var doc map[string]any
			if err := json.Unmarshal(y.Bytes, &doc); err != nil {
				t.Fatal(err)
			}
			doc[name] = field
			raw, err := json.Marshal(doc)
			if err != nil {
				t.Fatal(err)
			}
			raw = append(raw, '\n')
			z := y
			z.Bytes = raw
			z.ByteLength = len(raw)
			sum := sha256.Sum256(raw)
			z.SHA256 = fmt.Sprintf("sha256:%x", sum)
			if _, err := Compose([]Input{x, z}); err == nil {
				t.Fatal("ASSERT_SESSION_GENERATION_MISMATCH_REJECTED")
			}
		})
	}
	bad := x
	bad.Bytes = append(bytes.TrimSpace(bad.Bytes), []byte(`,"unknown":1}`)...)
	bad.ByteLength = len(bad.Bytes)
	s := sha256.Sum256(bad.Bytes)
	bad.SHA256 = fmt.Sprintf("sha256:%x", s)
	if _, err := Compose([]Input{bad, y}); err == nil {
		t.Fatal("ASSERT_UNKNOWN_OR_MALFORMED_REJECTED")
	}
}

func TestComposeInputAndAggregateBoundaries(t *testing.T) {
	a := node("a")
	x := capture(t, "x", []graph.Node{a}, nil, true, false)
	if _, err := Compose([]Input{x}); err == nil {
		t.Fatal("ASSERT_MIN_INPUT_MINUS_ONE")
	}
	many := make([]Input, MaxInputs)
	for i := range many {
		many[i] = x
		many[i].Identity = fmt.Sprintf("i-%02d", i)
	}
	if _, err := Compose(many); err != nil {
		t.Fatal("ASSERT_MAX_INPUT_EQUALITY", err)
	}
	many = append(many, x)
	many[len(many)-1].Identity = "overflow"
	if _, err := Compose(many); err == nil {
		t.Fatal("ASSERT_MAX_INPUT_PLUS_ONE")
	}
}

func TestPolicyIdentity(t *testing.T) {
	if len(PolicyBytes) == 0 || !strings.HasPrefix(PolicyDigest(), "sha256:") {
		t.Fatal("ASSERT_POLICY_BYTES_AND_DIGEST")
	}
	t.Logf("POLICY_DIGEST=%s POLICY_BYTES=%q", PolicyDigest(), PolicyBytes)
}

func TestResourceAndSemanticWorkCapEqualityAndPlusOne(t *testing.T) {
	atLimit := WorkAccounting{Inputs: MaxInputs, NodeRecords: 10_000, EdgeRecords: 100_000, Occurrences: 100_000, NativeReceiptRecords: MaxWorkUnits - MaxInputs - 10_000 - 100_000 - 100_000}
	if err := enforceResourceCaps(MaxTotalInputBytes, atLimit); err != nil {
		t.Fatal("ASSERT_BYTE_AND_SEMANTIC_WORK_CAP_EQUALITY", err)
	}
	if err := enforceResourceCaps(MaxTotalInputBytes+1, atLimit); err == nil {
		t.Fatal("ASSERT_BYTE_CAP_PLUS_ONE")
	}
	tooMuchWork := atLimit
	tooMuchWork.NativeReceiptRecords++
	if err := enforceResourceCaps(MaxTotalInputBytes, tooMuchWork); err == nil {
		t.Fatal("ASSERT_SEMANTIC_WORK_CAP_PLUS_ONE")
	}
	if err := enforceResourceCaps(0, WorkAccounting{NodeRecords: 10_001, MergedNodes: 10_001}); err == nil {
		t.Fatal("ASSERT_NODE_CAP_PLUS_ONE")
	}
	if err := enforceResourceCaps(0, WorkAccounting{Occurrences: 100_001}); err == nil {
		t.Fatal("ASSERT_OCCURRENCE_CAP_PLUS_ONE")
	}
}

func TestInvocationIdentityMutationReplacementAndReorderingBindings(t *testing.T) {
	a := node("a")
	x := capture(t, "x", []graph.Node{a}, nil, true, false)
	y := capture(t, "y", []graph.Node{a}, nil, true, false)
	original, err := Compose([]Input{x, y})
	if err != nil {
		t.Fatal(err)
	}
	replaced := capture(t, "replacement", []graph.Node{a}, nil, true, false)
	changed, err := Compose([]Input{x, replaced})
	if err != nil {
		t.Fatal("ASSERT_DISTINCT_INVOCATION_REPLACEMENT_NOT_FALSE_CONFLICT", err)
	}
	if original.Artifact.CompositeID == changed.Artifact.CompositeID {
		t.Fatal("ASSERT_INVOCATION_REPLACEMENT_CHANGES_COMPOSITE_ID")
	}
	mutated := original.Artifact
	mutated.Constituents[0].InvocationID = "tampered"
	mutated.CompositeID, mutated.OutputSHA256 = "", ""
	pre, _ := json.Marshal(mutated)
	mutated.CompositeID = digest("lsp-trace:program-c-compose:identity:v1", pre)
	pre, _ = json.Marshal(mutated)
	mutated.OutputSHA256 = digest("lsp-trace:program-c-compose:output:v1", pre)
	raw, _ := json.Marshal(mutated)
	if _, err := Validate(append(raw, '\n')); err == nil {
		t.Fatal("ASSERT_RESEALED_INVOCATION_TAMPER_REJECTED")
	}
	reordered := original.Artifact
	reordered.Constituents[0], reordered.Constituents[1] = reordered.Constituents[1], reordered.Constituents[0]
	reordered.CompositeID, reordered.OutputSHA256 = "", ""
	pre, _ = json.Marshal(reordered)
	reordered.CompositeID = digest("lsp-trace:program-c-compose:identity:v1", pre)
	pre, _ = json.Marshal(reordered)
	reordered.OutputSHA256 = digest("lsp-trace:program-c-compose:output:v1", pre)
	raw, _ = json.Marshal(reordered)
	if _, err := Validate(append(raw, '\n')); err == nil {
		t.Fatal("ASSERT_RESEALED_CONSTITUENT_REORDER_REJECTED")
	}
}

func TestFutureSourceSupplyShapeFailsClosed(t *testing.T) {
	a := node("a")
	x := capture(t, "x", []graph.Node{a}, nil, true, false)
	var env map[string]any
	if err := json.Unmarshal(x.Bytes, &env); err != nil {
		t.Fatal(err)
	}
	graphBytes, _ := base64.StdEncoding.DecodeString(env["graph_v5"].(string))
	var native map[string]any
	_ = json.Unmarshal(graphBytes, &native)
	native["source_supply"] = map[string]any{"future": true}
	graphBytes, _ = json.Marshal(native)
	env["graph_v5"] = base64.StdEncoding.EncodeToString(graphBytes)
	env["graph_v5_sha256"] = rawDigest(graphBytes)
	raw, _ := json.Marshal(env)
	x.Bytes, x.ByteLength, x.SHA256 = append(raw, '\n'), len(raw)+1, rawDigest(append(raw, '\n'))
	if _, err := Compose([]Input{x, x}); err == nil {
		t.Fatal("ASSERT_UNADAPTED_V5_SOURCE_SUPPLY_FAILS_CLOSED")
	}
}

func TestComposeRetainedSourceCannotAddNode(t *testing.T) {
	z := capture(t, "z", nil, nil, true, false)
	z2 := z
	z2.Identity = "capture-z-duplicate"
	if got, err := Compose([]Input{z, z2}); err != nil || len(got.Artifact.Nodes) != 0 {
		t.Fatal("ASSERT_RETAINED_BYTES_CANNOT_INTRODUCE_NODE", err)
	}
}

func TestConstituentCanonicalizesAndPreservesEveryV5SourceField(t *testing.T) {
	e := envelope{SchemaVersion: graphprovenance.VersionV5, SessionID: "s", Generation: 1, GraphV5SHA256: "sha256:" + strings.Repeat("a", 64), GraphV5SchemaID: graphprovenance.GraphV5SchemaID, SourcePolicy: graphprovenance.PolicyV2, WorkspaceURI: "file:///w", AnalyzedVersion: "UNVERIFIED", DependencyCompleteness: "UNKNOWN_INCOMPLETE", CaptureBudget: graphprovenance.CaptureBudgetV2{RootStatus: "OPENED", Attempts: 2, ChargedBytes: 3}, Supplies: []graphprovenance.SupplyReceiptV2{{RequestID: "z"}, {RequestID: "a"}}, Captures: []graphprovenance.Receipt{{ID: "z"}, {ID: "a"}}, Bindings: []graphprovenance.BindingV2{{Pointer: "/z"}, {Pointer: "/a"}}}
	n := native{Invocation: json.RawMessage(`{"provenance":{"invocation_id":"i"}}`)}
	n.inv.Provenance.InvocationID = "i"
	c := constituent(Input{Identity: "input", SHA256: "digest"}, e, nil, n)
	if c.SourcePolicy != e.SourcePolicy || c.WorkspaceURI != e.WorkspaceURI || c.AnalyzedVersion != e.AnalyzedVersion || c.DependencyCompleteness != e.DependencyCompleteness || c.CaptureBudget != e.CaptureBudget {
		t.Fatal("ASSERT_V5_SOURCE_SCALARS_PRESERVED")
	}
	if c.Supplies[0].RequestID != "a" || c.Captures[0].ID != "a" || c.Bindings[0].Pointer != "/a" {
		t.Fatal("ASSERT_V5_SOURCE_RECORDS_CANONICAL_ORDER")
	}
}

func TestSourceRecordExactTypedDedupeAndConflict(t *testing.T) {
	receipt := graphprovenance.Receipt{ID: "receipt", URI: "file:///w/a.go", Content: []byte("one")}
	supply := graphprovenance.SupplyReceiptV2{RequestID: "request", Status: "NO_NOTIFICATION_OBSERVATION", Receipt: &receipt}
	a := admitted{env: envelope{Supplies: []graphprovenance.SupplyReceiptV2{supply}, Bindings: []graphprovenance.BindingV2{{Pointer: "/p", URI: receipt.URI, ReceiptIDs: []string{receipt.ID}}}}}
	b := admitted{env: a.env}
	if err := validateSourceRecords([]admitted{a, b}); err != nil {
		t.Fatal("ASSERT_EXACT_TYPED_SOURCE_DEDUPE", err)
	}
	conflict := receipt
	conflict.URI = "file:///w/other.go"
	b.env.Supplies = []graphprovenance.SupplyReceiptV2{{RequestID: supply.RequestID, Status: supply.Status, Receipt: &conflict}}
	if err := validateSourceRecords([]admitted{a, b}); err == nil {
		t.Fatal("ASSERT_SOURCE_ID_CONFLICT_FAILS_CLOSED")
	}
	conflict = receipt
	conflict.ID = "other-receipt"
	conflict.URI = receipt.URI
	conflict.Content = append([]byte(nil), receipt.Content...)
	b.env.Supplies = []graphprovenance.SupplyReceiptV2{{RequestID: "other-request", Status: supply.Status, Receipt: &conflict}}
	if err := validateSourceRecords([]admitted{a, b}); err == nil {
		t.Fatal("ASSERT_CONTENT_DIGEST_METADATA_CONFLICT_FAILS_CLOSED")
	}
}

func TestSemanticWorkCountsEveryTraversedV5SourceRecord(t *testing.T) {
	w := WorkAccounting{Inputs: 2, NodeRecords: 3, EdgeRecords: 4, Occurrences: 5, SupplyRecords: 6, SourceReceiptRecords: 7, CaptureRecords: 8, BindingRecords: 9, NativeReceiptRecords: 10}
	if got, want := semanticWork(w), 54; got != want {
		t.Fatalf("ASSERT_V5_SOURCE_SEMANTIC_WORK got=%d want=%d", got, want)
	}
}
