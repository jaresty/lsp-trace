package programccompose

import (
	"bytes"
	"crypto/sha256"
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
	r := graph.Result{SchemaVersion: graph.SchemaVersionV5, Invocation: graph.Invocation{WorkspaceURI: "file:///w", Server: graph.ServerInvocation{Command: "gopls"}, LanguageID: "go", Seeds: []graph.InvocationSeed{seed}, Provenance: graph.InvocationProvenance{InvocationID: "session", SourceRevision: "commit", ServerVersion: "gopls@1"}}, Nodes: nodes, Edges: edges, Seeds: []graph.SeedResult{{Label: seed.Label}}, Summary: graph.Summary{Complete: complete, Truncated: truncated}, Capabilities: graph.Capabilities{CallHierarchyProvider: true}}
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

func TestAggregateCapEqualityAndPlusOne(t *testing.T) {
	if err := enforceAggregateCaps(0, 10_000, 100_000); err != nil {
		t.Fatal("ASSERT_AGGREGATE_CAP_EQUALITY", err)
	}
	if err := enforceAggregateCaps(0, 10_001, 100_000); err == nil {
		t.Fatal("ASSERT_NODE_CAP_PLUS_ONE")
	}
	if err := enforceAggregateCaps(0, 10_000, 100_001); err == nil {
		t.Fatal("ASSERT_OCCURRENCE_CAP_PLUS_ONE")
	}
	if err := enforceAggregateCaps(MaxWorkUnits, 0, 0); err != nil {
		t.Fatal("ASSERT_WORK_CAP_EQUALITY", err)
	}
	if err := enforceAggregateCaps(MaxWorkUnits+1, 0, 0); err == nil {
		t.Fatal("ASSERT_WORK_CAP_PLUS_ONE")
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
