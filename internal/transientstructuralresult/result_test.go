package transientstructuralresult

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

func validRequest() Request {
	return Request{Generation: 1, DownDepth: 2, UpDepth: 2, MaxNodes: 100, TimeoutMS: 5000, RequestTimeoutMS: 1000, Analysis: AnalysisRequest{Kind: Neighborhood}}
}

func validAccounting() Accounting {
	return Accounting{
		RequestOmissionReasons: EmptyReasonMap(), NodeOmissionReasons: EmptyReasonMap(), OccurrenceOmissionReasons: EmptyReasonMap(), FrontierOmissionReasons: EmptyReasonMap(),
		RequestAttempted: 1, RequestSucceeded: 1,
		PreparedAttempted: 1, PreparedReturned: 1,
		NodeObserved: 2, NodeAdmitted: 2,
		OccurrenceObserved: 1, OccurrenceAdmitted: 1,
		FrontierObserved: 1, FrontierExpanded: 1,
	}
}

func TestOpaqueIdentityPrivacyDeterminismAndDomains(t *testing.T) {
	const managerID = "ts_0123456789abcdef0123456789abcdef"
	n1, err := NodeID(managerID, 7, "file:///private/alice.go", "Secret")
	if err != nil {
		t.Fatal(err)
	}
	n2, _ := NodeID(managerID, 7, "file:///private/alice.go", "Secret")
	nAlias, _ := NodeID(managerID, 7, "file:///private/alice.go", "Secret")
	nGeneration, _ := NodeID(managerID, 8, "file:///private/alice.go", "Secret")
	e1, _ := EdgeID(managerID, 7, "file:///private/alice.go", "Secret")
	if n1 != n2 || n1 != nAlias {
		t.Fatal("identity is not deterministic or alias-independent")
	}
	if n1 == nGeneration || strings.TrimPrefix(n1, "tn_") == strings.TrimPrefix(e1, "te_") {
		t.Fatal("generation or domain separation failed")
	}
	if !regexp.MustCompile(`^tn_[0-9a-f]{32}$`).MatchString(n1) || !regexp.MustCompile(`^te_[0-9a-f]{32}$`).MatchString(e1) {
		t.Fatalf("bad ids: %q %q", n1, e1)
	}
	if strings.Contains(n1+e1, "alice") || strings.Contains(n1+e1, "Secret") || strings.Contains(n1+e1, "file") {
		t.Fatal("raw identity leaked")
	}
	if _, err := NodeID("session-alias", 7, "x", "y"); err == nil {
		t.Fatal("non-manager identity accepted")
	}
}

func TestNormalizeRequestBoundsAndDefaults(t *testing.T) {
	q := validRequest()
	got, err := NormalizeRequest(q)
	if err != nil {
		t.Fatal(err)
	}
	if got.MaxMessages != DefaultMaxMessages || got.MaxBytes != DefaultMaxBytes {
		t.Fatalf("defaults: %+v", got)
	}
	cases := []Request{
		func() Request { x := q; x.RequestTimeoutMS = 5001; return x }(),
		func() Request { x := q; x.MaxMessages = 4097; return x }(),
		func() Request { x := q; x.MaxBytes = 16777217; return x }(),
		func() Request {
			x := q
			x.Analysis = AnalysisRequest{Kind: Impact, Direction: Incoming, Depth: 0}
			return x
		}(),
		func() Request {
			x := q
			x.Analysis = AnalysisRequest{Kind: Impact, Direction: Incoming, Depth: 3}
			return x
		}(),
	}
	for i, bad := range cases {
		if _, err := NormalizeRequest(bad); err == nil {
			t.Fatalf("case %d accepted", i)
		}
	}
	q.Analysis = AnalysisRequest{Kind: Impact, Direction: Outgoing, Depth: 2}
	if _, err := NormalizeRequest(q); err != nil {
		t.Fatal(err)
	}
}

func TestAccountingExactReasonsEquationsAndCompleteness(t *testing.T) {
	a := validAccounting()
	if err := a.Validate(); err != nil {
		t.Fatal(err)
	}
	bad := a
	bad.NodeObserved++
	if err := bad.Validate(); err == nil {
		t.Fatal("bad node equation accepted")
	}
	bad = a
	bad.Truncated = true
	if err := bad.Validate(); err == nil {
		t.Fatal("truncation accepted")
	}
	bad = a
	bad.NodeOmissionReasons = ReasonMap{DepthBound: 0}
	if err := bad.Validate(); err == nil {
		t.Fatal("incomplete reason map accepted")
	}
	bad = a
	bad.NodeOmissionReasons[OmissionReason("OTHER")] = 0
	if err := bad.Validate(); err == nil {
		t.Fatal("unknown reason accepted")
	}
	bad = a
	bad.NodeOmitted = 1
	bad.NodeOmissionReasons = ReasonMap{Deduplication: 1}
	bad.DeduplicatedNodes = 0
	bad.NodeObserved++
	if err := bad.Validate(); err == nil {
		t.Fatal("non-dedup omission accepted")
	}
	b, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	for _, reason := range OmissionReasons() {
		if !strings.Contains(string(b), `"`+string(reason)+`"`) {
			t.Fatalf("missing reason %s", reason)
		}
	}
}

func TestNeighborhoodAndImpactClosedSemantics(t *testing.T) {
	const managerID = "ts_0123456789abcdef0123456789abcdef"
	root, _ := NodeID(managerID, 1, "root", "")
	caller, _ := NodeID(managerID, 1, "caller", "")
	edge, _ := EdgeID(managerID, 1, caller, root)
	n := NeighborhoodResult{RootNodeID: root, Nodes: []Node{{ID: root}, {ID: caller}}, Edges: []Edge{{ID: edge, CallerNodeID: caller, CalleeNodeID: root}}, IncomingCount: 1, OutgoingCount: 0}
	if err := n.Validate(); err != nil {
		t.Fatal(err)
	}
	n.IncomingCount = 0
	if err := n.Validate(); err == nil {
		t.Fatal("bad neighborhood count accepted")
	}
	i := ImpactResult{RootNodeID: root, Direction: Incoming, Depth: 1, Nodes: []Node{{ID: root}, {ID: caller}}, Edges: []Edge{{ID: edge, CallerNodeID: caller, CalleeNodeID: root}}, ReachableNodeIDs: []string{caller}, WitnessEdgeIDs: []string{edge}}
	if err := i.Validate(2, 2); err != nil {
		t.Fatal(err)
	}
	i.Direction = Outgoing
	if err := i.Validate(2, 2); err == nil {
		t.Fatal("wrong direction membership accepted")
	}
	i.Direction = Incoming
	i.Depth = 3
	if err := i.Validate(2, 2); err == nil {
		t.Fatal("depth beyond traversal accepted")
	}
}

func TestImpactDepthIsIndependentOfEdgeOrder(t *testing.T) {
	const managerID = "ts_0123456789abcdef0123456789abcdef"
	root, _ := NodeID(managerID, 1, "root")
	middle, _ := NodeID(managerID, 1, "middle")
	far, _ := NodeID(managerID, 1, "far")
	nearEdge, _ := EdgeID(managerID, 1, root, middle)
	farEdge, _ := EdgeID(managerID, 1, middle, far)
	i := ImpactResult{
		RootNodeID: root, Direction: Outgoing, Depth: 1,
		Nodes:            []Node{{ID: root}, {ID: middle}, {ID: far}},
		Edges:            []Edge{{ID: nearEdge, CallerNodeID: root, CalleeNodeID: middle}, {ID: farEdge, CallerNodeID: middle, CalleeNodeID: far}},
		ReachableNodeIDs: []string{middle, far}, WitnessEdgeIDs: []string{nearEdge, farEdge},
	}
	if err := i.Validate(1, 1); err == nil {
		t.Fatal("edge order traversed beyond requested depth")
	}
}

func TestGraphDigestBindsTopology(t *testing.T) {
	const managerID = "ts_0123456789abcdef0123456789abcdef"
	root, _ := NodeID(managerID, 1, "root")
	a, _ := NodeID(managerID, 1, "a")
	b, _ := NodeID(managerID, 1, "b")
	edge, _ := EdgeID(managerID, 1, "edge")
	q := validRequest()
	accounting := validAccounting()
	first := NewResult(managerID, 1, root, "utf-16", q, accounting, NeighborhoodResult{RootNodeID: root, Nodes: []Node{{ID: root}, {ID: a}, {ID: b}}, Edges: []Edge{{ID: edge, CallerNodeID: root, CalleeNodeID: a}}, OutgoingCount: 1})
	second := NewResult(managerID, 1, root, "utf-16", q, accounting, NeighborhoodResult{RootNodeID: root, Nodes: []Node{{ID: root}, {ID: a}, {ID: b}}, Edges: []Edge{{ID: edge, CallerNodeID: root, CalleeNodeID: b}}, OutgoingCount: 1})
	if first.GraphDigest == second.GraphDigest {
		t.Fatal("graph digest did not bind edge endpoints")
	}
	first.Analysis = second.Analysis
	if err := first.Validate(); err == nil {
		t.Fatal("topology mutation accepted under stale graph digest")
	}
}

func TestResultFixedSemanticsAndPrivacyShape(t *testing.T) {
	const managerID = "ts_0123456789abcdef0123456789abcdef"
	root, _ := NodeID(managerID, 1, "file:///secret.go", "Hidden")
	r := NewResult(managerID, 1, root, "utf-16", validRequest(), validAccounting(), NeighborhoodResult{RootNodeID: root, Nodes: []Node{{ID: root}}})
	if err := r.Validate(); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(r)
	for _, forbidden := range []string{"file:///secret.go", "Hidden", "frontier_count", "capture_supply", `"relation"`} {
		if strings.Contains(string(b), forbidden) {
			t.Fatalf("privacy/shape leak %q in %s", forbidden, b)
		}
	}
	if r.Relation != Calls || r.CaptureSupply || r.Authority != 0 || r.State != Empty || r.SourceGraphComplete != Unknown || r.Retained || r.Replayable || r.PublicationEligible || r.HydrationEligible {
		t.Fatalf("fixed semantics drift: %+v", r)
	}
	r.State = Complete
	if err := r.Validate(); err == nil {
		t.Fatal("incomplete COMPLETE state accepted")
	}
	r = NewResult(managerID, 1, root, "utf-16", validRequest(), validAccounting(), NeighborhoodResult{RootNodeID: root, Nodes: []Node{{ID: root}}})
	r.ResourcePolicy.PolicyDigest = "sha256:" + strings.Repeat("0", 64)
	if err := r.Validate(); err == nil {
		t.Fatal("mutated frozen policy accepted")
	}
}
