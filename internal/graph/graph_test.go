package graph

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func testItem() Item {
	return Item{
		Name: "leaf", Kind: 12, Detail: "func()", URI: "file:///workspace/a.go",
		Range:          Range{Start: Position{Line: 1}, End: Position{Line: 3}},
		SelectionRange: Range{Start: Position{Line: 1, Character: 5}, End: Position{Line: 1, Character: 9}},
		Data:           jsonBytes(`{"token":7,"nested":{"ok":true}}`),
	}
}

func jsonBytes(s string) []byte { return []byte(s) }

func TestNodeIdentityDeterministicAndDataIndependent(t *testing.T) {
	a := testItem()
	b := testItem()
	b.Data = jsonBytes(`{"unstable":"different"}`)

	first := NewNode(a)
	second := NewNode(b)
	if first.ID == "" || first.ID != second.ID {
		t.Fatalf("ASSERT_NODE_ID_DETERMINISTIC: ids = %q and %q", first.ID, second.ID)
	}
	if !bytes.Equal(first.Data, a.Data) {
		t.Fatalf("ASSERT_OPAQUE_DATA_PRESERVED: got %s, want %s", first.Data, a.Data)
	}
}

func TestMergeEdgeUnionsAndSortsCallSites(t *testing.T) {
	r1 := Range{Start: Position{Line: 8}, End: Position{Line: 8, Character: 4}}
	r2 := Range{Start: Position{Line: 2}, End: Position{Line: 2, Character: 4}}
	edges := MergeEdge(nil, Edge{CallerNodeID: "caller", CalleeNodeID: "callee", CallSites: []Range{r1}})
	edges = MergeEdge(edges, Edge{CallerNodeID: "caller", CalleeNodeID: "callee", CallSites: []Range{r2, r1}})

	if len(edges) != 1 || len(edges[0].CallSites) != 2 || edges[0].CallSites[0] != r2 || edges[0].CallSites[1] != r1 {
		t.Fatalf("ASSERT_EDGE_RANGE_UNION_SORTED: %#v", edges)
	}
}

func TestCanonicalizeSortsAllCollectionsAndCountsCycles(t *testing.T) {
	r1 := Range{Start: Position{Line: 8}, End: Position{Line: 8, Character: 4}}
	r2 := Range{Start: Position{Line: 2}, End: Position{Line: 2, Character: 4}}
	r := Result{
		Targets: []string{"b", "a"},
		Nodes:   []Node{{ID: "b", Item: Item{URI: "file:///b"}}, {ID: "a", Item: Item{URI: "file:///a"}}},
		Edges: []Edge{
			{CallerNodeID: "b", CalleeNodeID: "a", CallSites: []Range{r1, r2, r1}},
			{CallerNodeID: "a", CalleeNodeID: "b"},
		},
		Terminals:   []Boundary{{NodeID: "a", Reason: ServerError, Message: "z"}, {NodeID: "a", Reason: ServerError, Message: "a"}},
		Frontier:    []Boundary{{NodeID: "b", Reason: MaxNodes, Message: "z"}, {NodeID: "b", Reason: MaxNodes, Message: "a"}},
		Diagnostics: []Diagnostic{{Phase: "traverse", Method: "z", NodeID: "b", Message: "z"}, {Phase: "prepare", Method: "a", NodeID: "a", Message: "a"}},
	}
	r.Canonicalize()

	if r.Summary.CycleCount != 1 {
		t.Fatalf("ASSERT_SCC_CYCLE_COUNT: got %d, want 1", r.Summary.CycleCount)
	}
	if got := r.Edges[1].CallSites; !reflect.DeepEqual(got, []Range{r2, r1}) {
		t.Fatalf("ASSERT_CANONICAL_CALL_SITES: %#v", got)
	}
	if r.Terminals[0].Message != "a" || r.Frontier[0].Message != "a" || r.Diagnostics[0].Phase != "prepare" {
		t.Fatalf("ASSERT_CANONICAL_AUXILIARY_ORDER: terminals=%#v frontier=%#v diagnostics=%#v", r.Terminals, r.Frontier, r.Diagnostics)
	}
}

func TestValidateItemRejectsMissingFieldsAndInvalidRanges(t *testing.T) {
	valid := testItem()
	for _, tc := range []struct {
		name string
		edit func(*Item)
	}{
		{name: "missing name", edit: func(i *Item) { i.Name = "" }},
		{name: "invalid URI", edit: func(i *Item) { i.URI = "relative.go" }},
		{name: "reversed range", edit: func(i *Item) { i.Range = Range{Start: Position{Line: 3}, End: Position{Line: 2}} }},
		{name: "selection outside range", edit: func(i *Item) { i.SelectionRange = Range{Start: Position{Line: 9}, End: Position{Line: 9}} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			item := valid
			tc.edit(&item)
			if err := ValidateItem(item); err == nil {
				t.Fatalf("ASSERT_INVALID_ITEM_REJECTED: %s", tc.name)
			}
		})
	}
	if err := ValidateItem(valid); err != nil {
		t.Fatalf("ASSERT_VALID_ITEM_ACCEPTED: %v", err)
	}
}

func TestSameNodeIdentityDetectsFabricatedIDCollision(t *testing.T) {
	a := NewNode(testItem())
	b := a
	b.URI = "file:///workspace/other.go"
	if SameNodeIdentity(a, b) {
		t.Fatal("ASSERT_NODE_ID_COLLISION_DETECTED: differing identity fields treated as equal")
	}
	b = a
	b.Data = jsonBytes(`{"transport":"different"}`)
	if !SameNodeIdentity(a, b) {
		t.Fatal("ASSERT_OPAQUE_DATA_NOT_COLLISION: opaque data changed identity")
	}
}

func TestResultSchemaSeedProjection(t *testing.T) {
	r := Result{SchemaVersion: SchemaVersionV2, Seeds: []SeedResult{{Label: "interface", Requested: Target{URI: "file:///a.go", Line: 1, Column: 1}}}}
	encoded, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var v2 map[string]any
	if err := json.Unmarshal(encoded, &v2); err != nil {
		t.Fatal(err)
	}
	if _, ok := v2["seeds"]; !ok {
		t.Fatalf("ASSERT_V2_SEED_PROVENANCE: %s", encoded)
	}

	r.SchemaVersion = SchemaVersionV1
	r.Seeds = nil
	encoded, err = json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var v1 map[string]any
	if err := json.Unmarshal(encoded, &v1); err != nil {
		t.Fatal(err)
	}
	if _, ok := v1["seeds"]; ok {
		t.Fatalf("ASSERT_V1_HAS_NO_SEEDS: %s", encoded)
	}
}

func TestMergeResultsCollapsesDuplicateNodesAndPreservesSeedOutcomes(t *testing.T) {
	node := NewNode(testItem())
	r1 := Result{
		SchemaVersion:     SchemaVersionV2,
		Nodes:             []Node{node},
		Targets:           []string{node.ID},
		Terminals:         []Boundary{{NodeID: node.ID, Reason: ServerReportedNoIncoming}},
		Diagnostics:       []Diagnostic{{Phase: "z", Message: "same"}},
		Summary:           Summary{Complete: true},
		CapabilityQuality: CapabilityQuality{PrepareSucceeded: true, IncomingRequestSuccesses: 1, CrossModuleEdges: Unknown},
		Seeds:             []SeedResult{{Label: "interface", Requested: Target{URI: node.URI, Line: 1, Column: 1}, PreparedTargetIDs: []string{node.ID}, ReachedNodeIDs: []string{node.ID}}},
	}
	r2 := r1
	r2.Seeds = []SeedResult{{Label: "implementation", Requested: Target{URI: node.URI, Line: 2, Column: 1}, PreparedTargetIDs: []string{node.ID}, ReachedNodeIDs: []string{node.ID}, Failure: &SeedFailure{Phase: "prepare", Message: "failed"}}}
	r2.Summary.Complete = false

	merged := MergeResults(r2, r1)
	if len(merged.Nodes) != 1 || len(merged.Targets) != 1 || len(merged.Terminals) != 1 || len(merged.Diagnostics) != 1 {
		t.Fatalf("ASSERT_DUPLICATE_SEEDS_COLLAPSE: %#v", merged)
	}
	if merged.Summary.Complete || len(merged.Seeds) != 2 || merged.Seeds[0].Label != "implementation" || merged.Seeds[1].Label != "interface" {
		t.Fatalf("ASSERT_FAILED_SEED_INCOMPLETE_WITH_GRAPH: %#v", merged)
	}
}

func TestResultUnresolvedEvidenceIsCategorizedCountedAndDoesNotFabricateEdges(t *testing.T) {
	var r Result
	if err := json.Unmarshal([]byte(`{"schema_version":"lsp-trace.graph.v2","diagnostics":[{"phase":"traverse","node_id":"z","category":"DYNAMIC_CALL","message":"indirect target"},{"phase":"traverse","node_id":"a","category":"UNRESOLVED_CALL","message":"callee unavailable"}]}`), &r); err != nil {
		t.Fatal(err)
	}
	r.SchemaVersion = SchemaVersionV2
	r.Summary.Complete = true
	r.Canonicalize()
	encoded, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatal(err)
	}
	diagnostics := got["diagnostics"].([]any)
	first := diagnostics[0].(map[string]any)
	second := diagnostics[1].(map[string]any)
	if first["category"] != "UNRESOLVED_CALL" || second["category"] != "DYNAMIC_CALL" {
		t.Fatalf("ASSERT_EVIDENCE_CATEGORIES_CANONICAL: %s", encoded)
	}
	quality := got["capability_quality"].(map[string]any)
	if quality["unresolved_calls"] != float64(1) || quality["dynamic_calls"] != float64(1) {
		t.Fatalf("ASSERT_EVIDENCE_COUNTERS_EXACT: %s", encoded)
	}
	summary := got["summary"].(map[string]any)
	if summary["traversal_complete"] != false {
		t.Fatalf("ASSERT_UNRESOLVED_EVIDENCE_INCOMPLETE: %s", encoded)
	}
	if edges := got["edges"]; edges != nil {
		t.Fatalf("ASSERT_EVIDENCE_NEVER_FABRICATES_EDGES: %s", encoded)
	}

	r.SchemaVersion = SchemaVersionV1
	encoded, err = json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(encoded, []byte(`"category"`)) || bytes.Contains(encoded, []byte(`"unresolved_calls"`)) || bytes.Contains(encoded, []byte(`"dynamic_calls"`)) {
		t.Fatalf("ASSERT_V1_OMITS_EVIDENCE_EXTENSION: %s", encoded)
	}
}

func TestResultDynamicEvidenceAlonePreservesCompleteness(t *testing.T) {
	var r Result
	if err := json.Unmarshal([]byte(`{"schema_version":"lsp-trace.graph.v2","diagnostics":[{"phase":"traverse","category":"DYNAMIC_CALL","message":"runtime dispatch"}]}`), &r); err != nil {
		t.Fatal(err)
	}
	r.SchemaVersion = SchemaVersionV2
	r.Summary.Complete = true
	r.Canonicalize()
	if !r.Summary.Complete {
		t.Fatal("ASSERT_DYNAMIC_EVIDENCE_REMAINS_ADVISORY: dynamic evidence changed completeness")
	}
}

func TestDispatchRelationshipsAreSeparateCanonicalAndV2Only(t *testing.T) {
	r := Result{SchemaVersion: SchemaVersionV2, DispatchRelationships: []DispatchRelationship{
		{SeedLabel: "b", Interface: Node{ID: "i"}, Implementation: Node{ID: "z"}},
		{SeedLabel: "a", Interface: Node{ID: "i"}, Implementation: Node{ID: "x"}},
		{SeedLabel: "a", Interface: Node{ID: "i"}, Implementation: Node{ID: "x"}},
	}}
	r.Canonicalize()
	if len(r.DispatchRelationships) != 2 || len(r.Edges) != 0 {
		t.Fatalf("ASSERT_DISPATCH_RELATIONSHIPS_SEPARATE_CANONICAL: relationships=%#v edges=%#v", r.DispatchRelationships, r.Edges)
	}
	encoded, err := json.Marshal(r)
	if err != nil || !bytes.Contains(encoded, []byte(`"dispatch_relationships"`)) {
		t.Fatalf("ASSERT_V2_EMITS_DISPATCH_RELATIONSHIPS: encoded=%s err=%v", encoded, err)
	}
	merged := MergeResults(r, r)
	if len(merged.DispatchRelationships) != 2 {
		t.Fatalf("ASSERT_DISPATCH_RELATIONSHIPS_MERGE_DEDUP: relationships=%#v", merged.DispatchRelationships)
	}
	r.SchemaVersion = SchemaVersionV1
	encoded, err = json.Marshal(r)
	if err != nil || bytes.Contains(encoded, []byte(`"dispatch_relationships"`)) {
		t.Fatalf("ASSERT_V1_OMITS_DISPATCH_RELATIONSHIPS: encoded=%s err=%v", encoded, err)
	}
}

func TestV2EvidenceReceiptProjectsDiscoverySemanticsWithStableRelationIdentities(t *testing.T) {
	makeResult := func(reverse bool) Result {
		siblings := []SiblingCandidate{
			{SeedURI: "file:///b.go", Candidate: Node{ID: "candidate-b"}},
			{SeedURI: "file:///a.go", Candidate: Node{ID: "candidate-a"}},
		}
		dispatch := []DispatchRelationship{
			{SeedLabel: "b", Interface: Node{ID: "interface-b"}, Implementation: Node{ID: "implementation-b"}},
			{SeedLabel: "a", Interface: Node{ID: "interface-a"}, Implementation: Node{ID: "implementation-a"}},
		}
		if reverse {
			siblings[0], siblings[1] = siblings[1], siblings[0]
			dispatch[0], dispatch[1] = dispatch[1], dispatch[0]
		}
		return Result{
			SchemaVersion:     SchemaVersionV2,
			Invocation:        Invocation{Provenance: InvocationProvenance{SourceRevision: "commit-abc"}},
			SiblingCandidates: siblings, DispatchRelationships: dispatch,
		}
	}
	encode := func(r Result) ([]byte, map[string]any) {
		t.Helper()
		r.Canonicalize()
		encoded, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		var decoded map[string]any
		if err := json.Unmarshal(encoded, &decoded); err != nil {
			t.Fatal(err)
		}
		return encoded, decoded
	}

	firstBytes, first := encode(makeResult(false))
	secondBytes, _ := encode(makeResult(true))
	if !bytes.Equal(firstBytes, secondBytes) {
		t.Fatalf("ASSERT_RECEIPT_CANONICAL_INSERTION_INDEPENDENT: %s != %s", firstBytes, secondBytes)
	}
	traceReceipt, traceReceiptOK := first["trace_receipt"].(map[string]any)
	t.Run("immutable trace receipt", func(t *testing.T) {
		if !traceReceiptOK || traceReceipt["receipt_version"] != "lsp-trace.receipt.v1" {
			t.Fatalf("ASSERT_TRACE_RECEIPT_PRESENT: %#v", first)
		}
		digest, _ := traceReceipt["content_digest"].(string)
		if !strings.HasPrefix(digest, "sha256:") {
			t.Fatalf("ASSERT_TRACE_RECEIPT_DIGEST_DOMAIN: %#v", traceReceipt)
		}
		changed := makeResult(false)
		changed.Invocation.Provenance.SourceRevision = "commit-def"
		changedBytes, changedDocument := encode(changed)
		_ = changedBytes
		changedReceipt, _ := changedDocument["trace_receipt"].(map[string]any)
		if changedReceipt["content_digest"] == digest {
			t.Fatalf("ASSERT_TRACE_RECEIPT_COMMITS_PROVENANCE: first=%#v changed=%#v", traceReceipt, changedReceipt)
		}
		firstEvidence, _ := first["evidence_receipt"].(map[string]any)
		changedEvidence, _ := changedDocument["evidence_receipt"].(map[string]any)
		firstRelations, _ := firstEvidence["relations"].([]any)
		changedRelations, _ := changedEvidence["relations"].([]any)
		if len(firstRelations) == 0 || len(changedRelations) == 0 || firstRelations[0].(map[string]any)["relation_id"] != changedRelations[0].(map[string]any)["relation_id"] {
			t.Fatalf("ASSERT_RELATION_ID_IS_SEMANTIC_NOT_REVISION_SALTED: first=%#v changed=%#v", firstEvidence, changedEvidence)
		}
	})
	semantics, semanticsOK := first["evidence_semantics"].(map[string]any)
	t.Run("claim ceiling", func(t *testing.T) {
		callEdges, _ := semantics["call_edges"].(map[string]any)
		discovery, _ := semantics["discovery_relations"].(map[string]any)
		if !semanticsOK || callEdges["evidence_class"] != "SERVER_REPORTED_CALL_HIERARCHY" {
			t.Fatalf("ASSERT_EVIDENCE_SEMANTICS_PRESENT: %#v", semantics)
		}
		if discovery["support_contribution"] != float64(0) {
			t.Fatalf("ASSERT_DISCOVERY_CLAIM_CEILING_ZERO: %#v", discovery)
		}
		unsupported, _ := callEdges["does_not_support"].([]any)
		if len(unsupported) == 0 {
			t.Fatalf("ASSERT_CALL_EDGE_CLAIM_CEILING: %#v", callEdges)
		}
	})
	receipt, receiptOK := first["evidence_receipt"].(map[string]any)
	if !receiptOK {
		receipt = map[string]any{}
	}
	t.Run("receipt present", func(t *testing.T) {
		if !receiptOK {
			t.Errorf("ASSERT_V2_EVIDENCE_RECEIPT_PRESENT: %s", firstBytes)
		}
	})
	t.Run("support total zero", func(t *testing.T) {
		if receipt["support_total"] != float64(0) {
			t.Errorf("ASSERT_DISCOVERY_SUPPORT_TOTAL_ZERO: %#v", receipt)
		}
	})
	relations, relationsOK := receipt["relations"].([]any)
	t.Run("projects all discovery relations", func(t *testing.T) {
		if !relationsOK || len(relations) != 4 {
			t.Errorf("ASSERT_RECEIPT_PROJECTS_EACH_DISCOVERY_RELATION: %#v", receipt)
		}
	})
	t.Run("relation semantics", func(t *testing.T) {
		seenIDs := map[string]bool{}
		if len(relations) == 0 {
			t.Error("ASSERT_RELATION_IDENTITIES_STABLE_DISTINCT: no projected relations")
			t.Error("ASSERT_DISCOVERY_RELATION_LABELED_ONLY: no projected relations")
			t.Error("ASSERT_DISCOVERY_RELATION_SUPPORT_ZERO: no projected relations")
		}
		for _, raw := range relations {
			relation := raw.(map[string]any)
			id, _ := relation["relation_id"].(string)
			if !strings.HasPrefix(id, "sha256:") || seenIDs[id] {
				t.Errorf("ASSERT_RELATION_IDENTITIES_STABLE_DISTINCT: %#v", relations)
			}
			seenIDs[id] = true
			if relation["evidence_role"] != "DISCOVERY_ONLY" || relation["evidence_class"] != "DISCOVERY_NOMINATION" || relation["direction"] == "" || relation["locator"] == "" || relation["source_revision"] != "commit-abc" {
				t.Errorf("ASSERT_DISCOVERY_RELATION_LABELED_ONLY: %#v", relation)
			}
			if relation["support_contribution"] != float64(0) {
				t.Errorf("ASSERT_DISCOVERY_RELATION_SUPPORT_ZERO: %#v", relation)
			}
		}
	})
}

func TestV1ProjectionBytesRemainUnchangedByEvidenceReceipt(t *testing.T) {
	r := Result{
		SchemaVersion:         SchemaVersionV1,
		SiblingCandidates:     []SiblingCandidate{{SeedURI: "file:///a.go", Candidate: Node{ID: "candidate"}}},
		DispatchRelationships: []DispatchRelationship{{SeedLabel: "seed", Interface: Node{ID: "interface"}, Implementation: Node{ID: "implementation"}}},
	}
	encoded, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	const want = `{"schema_version":"lsp-trace.graph.v1","invocation":{"workspace_uri":"","target":{"uri":"","line":0,"column":0},"server":{"command":"","arguments":null},"limits":{"max_depth":0,"max_nodes":0,"timeout_ms":0}},"capabilities":{"call_hierarchy_provider":false},"targets":null,"nodes":null,"edges":null,"terminals":null,"frontier":null,"diagnostics":null,"summary":{"node_count":0,"edge_count":0,"terminal_count":0,"cycle_count":0,"complete":false,"truncated":false}}`
	if !bytes.Equal(encoded, []byte(want)) {
		t.Fatalf("ASSERT_V1_RECEIPT_EXTENSION_BYTE_STABLE: got %s", encoded)
	}
}

func TestCanonicalizeEquivalentInsertionOrdersProduceIdenticalJSON(t *testing.T) {
	makeResult := func(reverse bool) Result {
		nodes := []Node{{ID: "a", Item: Item{URI: "file:///a"}}, {ID: "b", Item: Item{URI: "file:///b"}}}
		edges := []Edge{{CallerNodeID: "a", CalleeNodeID: "b"}, {CallerNodeID: "b", CalleeNodeID: "a"}}
		if reverse {
			nodes[0], nodes[1] = nodes[1], nodes[0]
			edges[0], edges[1] = edges[1], edges[0]
		}
		return Result{SchemaVersion: SchemaVersion, Nodes: nodes, Edges: edges}
	}
	a, b := makeResult(false), makeResult(true)
	a.Canonicalize()
	b.Canonicalize()
	aj, _ := json.Marshal(a)
	bj, _ := json.Marshal(b)
	if !bytes.Equal(aj, bj) {
		t.Fatalf("ASSERT_CANONICAL_JSON_STABLE: %s != %s", aj, bj)
	}
}
