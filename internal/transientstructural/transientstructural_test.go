package transientstructural

import (
	"encoding/json"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/session"
	"lsp-trace/sessionruntime"
)

func testNode(name string, line uint32) graph.Node {
	return graph.NewNode(graph.Item{
		Name: name, Kind: 12, URI: "file:///private/work/code.go",
		Range:          graph.Range{Start: graph.Position{Line: line}, End: graph.Position{Line: line, Character: 128}},
		SelectionRange: graph.Range{Start: graph.Position{Line: line}, End: graph.Position{Line: line, Character: uint32(len(name))}},
	})
}

func testEdge(caller, callee graph.Node, line uint32) graph.Edge {
	return graph.Edge{RelationID: caller.ID + "->" + callee.ID, CallerNodeID: caller.ID, CalleeNodeID: callee.ID,
		CallSites: []graph.Range{{Start: graph.Position{Line: line}, End: graph.Position{Line: line, Character: 1}}}}
}

func testBounds() BoundsBinding {
	return BoundsBinding{UpDepth: 1, DownDepth: 2, MaxNodes: 10, TimeoutMS: 1000, RequestTimeoutMS: 100, MaxMessages: 8, MaxBytes: 4096, AnalysisKind: AnalysisNeighborhood}
}

func TestProjectionOpaqueDeterministicDirectionalAndSelfCall(t *testing.T) {
	root, a, b, caller := testNode("PrivateRoot", 1), testNode("PrivateA", 2), testNode("PrivateB", 3), testNode("PrivateCaller", 4)
	down := traversalProjection{root: root.ID, nodes: []graph.Node{b, root, a}, edges: []graph.Edge{
		testEdge(a, b, 20), testEdge(root, root, 10), testEdge(root, a, 11),
	}}
	up := traversalProjection{root: root.ID, nodes: []graph.Node{caller, root}, edges: []graph.Edge{testEdge(caller, root, 30)}}
	first, err := project("canonical-session", 7, down, up, testBounds(), Accounting{})
	if err != nil {
		t.Fatal(err)
	}
	second, err := project("canonical-session", 7, down, up, testBounds(), Accounting{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("ASSERT_DETERMINISTIC_PROJECTION")
	}
	if !sort.SliceIsSorted(first.nodes, func(i, j int) bool { return first.nodes[i].ID < first.nodes[j].ID }) ||
		!sort.SliceIsSorted(first.occurrences, func(i, j int) bool { return first.occurrences[i].ID < first.occurrences[j].ID }) {
		t.Fatal("ASSERT_CANONICAL_ORDER")
	}
	opaque := regexp.MustCompile(`^[0-9a-f]{64}$`)
	for _, node := range first.nodes {
		if !opaque.MatchString(node.ID) || node.ID == root.ID || node.ID == a.ID || node.ID == b.ID || node.ID == caller.ID {
			t.Fatalf("ASSERT_OPAQUE_NODE_ID: %q", node.ID)
		}
	}
	for _, occurrence := range first.occurrences {
		if !opaque.MatchString(occurrence.ID) || !opaque.MatchString(occurrence.CallerID) || !opaque.MatchString(occurrence.CalleeID) {
			t.Fatalf("ASSERT_OPAQUE_OCCURRENCE: %+v", occurrence)
		}
	}
	if got := []int{first.accounting.Nodes.Observed, first.accounting.Nodes.Admitted, first.accounting.Nodes.Rejected, first.accounting.Nodes.Omitted}; !reflect.DeepEqual(got, []int{5, 4, 0, 1}) {
		t.Fatalf("ASSERT_NODE_DENOMINATOR: %v", got)
	}
	if got := []int{first.accounting.Occurrences.Observed, first.accounting.Occurrences.Admitted, first.accounting.Occurrences.Rejected, first.accounting.Occurrences.Omitted}; !reflect.DeepEqual(got, []int{4, 4, 0, 0}) {
		t.Fatalf("ASSERT_OCCURRENCE_DENOMINATOR: %v", got)
	}
	assertNodeWitness(t, first, root, DirectionRoot, 0)
	assertNodeWitness(t, first, a, DirectionOutgoing, 1)
	assertNodeWitness(t, first, b, DirectionOutgoing, 2)
	assertNodeWitness(t, first, caller, DirectionIncoming, 1)
	self := 0
	for _, occurrence := range first.occurrences {
		if occurrence.CallerID == first.targetID && occurrence.CalleeID == first.targetID {
			self++
			if !reflect.DeepEqual(occurrence.Witnesses, []Witness{{Direction: DirectionOutgoing, Depth: 1}}) {
				t.Fatalf("ASSERT_SELF_CALL_WITNESS: %+v", occurrence)
			}
		}
	}
	if self != 1 {
		t.Fatalf("ASSERT_SELF_CALL_ONCE: %d", self)
	}
	other, err := project("canonical-session", 8, down, up, testBounds(), Accounting{})
	if err != nil || other.targetID == first.targetID {
		t.Fatal("ASSERT_EXACT_GENERATION_SALTS_IDENTITY")
	}
}

func assertNodeWitness(t *testing.T, projection admittedProjection, raw graph.Node, direction Direction, depth int) {
	t.Helper()
	wantID := opaqueID("lsp-trace/transient-structural/node/v1", identitySalt("canonical-session", 7), raw.ID)
	for _, node := range projection.nodes {
		if node.ID == wantID {
			for _, witness := range node.Witnesses {
				if witness.Direction == direction && witness.Depth == depth {
					return
				}
			}
			t.Fatalf("ASSERT_EXACT_WITNESS: id=%s witnesses=%+v", wantID, node.Witnesses)
		}
	}
	t.Fatalf("ASSERT_REACHABLE_NODE_ADMITTED: %s", wantID)
}

func TestImpactDirectionDepthAndTargetExclusion(t *testing.T) {
	root, a, b, caller := testNode("root", 1), testNode("a", 2), testNode("b", 3), testNode("caller", 4)
	projection, err := project("canonical-session", 7,
		traversalProjection{root: root.ID, nodes: []graph.Node{root, a, b}, edges: []graph.Edge{testEdge(root, root, 9), testEdge(root, a, 10), testEdge(a, b, 11)}},
		traversalProjection{root: root.ID, nodes: []graph.Node{root, caller}, edges: []graph.Edge{testEdge(caller, root, 12)}}, testBounds(), Accounting{})
	if err != nil {
		t.Fatal(err)
	}
	out := analyze(AnalysisRequest{Kind: AnalysisImpact, Direction: DirectionOutgoing, MaxDepth: 1}, projection, testBounds())
	if len(out.Nodes) != 1 || out.Nodes[0].ID == projection.targetID {
		t.Fatalf("ASSERT_IMPACT_OUT_DEPTH_ONE_TARGET_EXCLUDED: %+v", out.Nodes)
	}
	if len(out.Occurrences) != 2 {
		t.Fatalf("ASSERT_IMPACT_SELF_CALL_RETAINED: %+v", out.Occurrences)
	}
	in := analyze(AnalysisRequest{Kind: AnalysisImpact, Direction: DirectionIncoming, MaxDepth: 1}, projection, testBounds())
	if len(in.Nodes) != 1 || len(in.Occurrences) != 1 || in.Occurrences[0].CalleeID != projection.targetID {
		t.Fatalf("ASSERT_IMPACT_INCOMING_ONLY: %+v", in)
	}
	neighborhood := analyze(AnalysisRequest{Kind: AnalysisNeighborhood}, projection, testBounds())
	if len(neighborhood.Nodes) != 4 || len(neighborhood.Occurrences) != 4 {
		t.Fatalf("ASSERT_NEIGHBORHOOD_EXACT_REACHABLE: %+v", neighborhood)
	}
}

func TestProjectionRejectsCollisionAndNodeOverflow(t *testing.T) {
	root := testNode("root", 1)
	collision := testNode("other", 2)
	collision.ID = root.ID
	_, err := project("s", 1,
		traversalProjection{root: root.ID, nodes: []graph.Node{root, collision}},
		traversalProjection{root: root.ID, nodes: []graph.Node{root}}, testBounds(), Accounting{})
	if err == nil {
		t.Fatal("ASSERT_COLLISION_REJECTED")
	}
	bounds := testBounds()
	bounds.MaxNodes = 1
	a := testNode("a", 2)
	got, err := project("s", 1,
		traversalProjection{root: root.ID, nodes: []graph.Node{root, a}, edges: []graph.Edge{testEdge(root, a, 1)}},
		traversalProjection{root: root.ID, nodes: []graph.Node{root}}, bounds, Accounting{})
	if err == nil || !hasOmission(got.accounting, OmissionNodeBound) || !reconciles(got.accounting) {
		t.Fatalf("ASSERT_NODE_OVERFLOW_ACCOUNTED: err=%v accounting=%+v", err, got.accounting)
	}
}

func TestPolicyDigestGraphDigestAndClaimPrivacy(t *testing.T) {
	wantPolicy := PolicyBinding{
		LifecycleID: "lsp-trace.transient-structural-lifecycle", LifecycleVersion: "1", LifecycleSHA256: "sha256:5229736442c03655b9e7e86055b120c3f2988a8de3f46663bfe0a34799bf722c",
		PrivacyID: "lsp-trace.transient-structural-privacy", PrivacyVersion: "1", PrivacySHA256: "sha256:bcb3e20fd7f3bd0220cb05292eccb7a6c5be9ecc489f0495bc4b57f6d6436c7a",
		IdentityID: "lsp-trace.transient-structural-identity", IdentityVersion: "1", IdentitySHA256: "sha256:6ed192ba653818c22e7c0526145aa221fd3ca3613a6afbcb5cd36df421be84f0",
		AnalysisID: "lsp-trace.transient-structural-analysis", AnalysisVersion: "1", AnalysisSHA256: "sha256:24ac85888bd951c8c77f05c69c7f1bd82abfe2b406c613def6c22ee76ee44819",
	}
	policy := frozenPolicyBinding()
	if policy != wantPolicy || policy.LifecycleSHA256 != digestDocument(lifecyclePolicyDocument) || policy.PrivacySHA256 != digestDocument(privacyPolicyDocument) ||
		policy.IdentitySHA256 != digestDocument(identityPolicyDocument) || policy.AnalysisSHA256 != digestDocument(analysisPolicyDocument) {
		t.Fatalf("ASSERT_FROZEN_POLICY_IDS_VERSIONS_DIGESTS: got=%+v", policy)
	}
	mutated := policy
	mutated.LifecycleID = "mutated"
	if frozenPolicyBinding() != wantPolicy {
		t.Fatal("ASSERT_FROZEN_POLICY_PRIVATE_VALUE_COPY")
	}
	if resultSchemaVersion != "lsp-trace.transient-structural-result.v1" || evidenceClassTransientLive != "TRANSIENT_LIVE" || sourceGraphCompleteUnknown != "UNKNOWN" {
		t.Fatal("ASSERT_FROZEN_RESULT_SCHEMA_AND_CONSTANTS")
	}
	root := testNode("secret", 1)
	projection, err := project("canonical-session", 7,
		traversalProjection{root: root.ID, nodes: []graph.Node{root}}, traversalProjection{root: root.ID, nodes: []graph.Node{root}}, testBounds(), Accounting{})
	if err != nil {
		t.Fatal(err)
	}
	first := graphDigest(projection, policy, testBounds())
	second := graphDigest(projection, policy, testBounds())
	changed := testBounds()
	changed.DownDepth++
	if first != second || first == graphDigest(projection, policy, changed) || !strings.HasPrefix(first, "sha256:") {
		t.Fatalf("ASSERT_BOUND_DIGEST: %q %q", first, second)
	}
	result := Result{SchemaVersion: resultSchemaVersion, EvidenceClass: evidenceClassTransientLive, SourceGraphComplete: sourceGraphCompleteUnknown, ClaimCeiling: claimCeiling,
		Phase: PhaseDeliveryCheck, State: StateComplete, Qualification: Qualification{SessionID: "canonical-session", Generation: 7, PositionEncoding: "utf-16"}, TargetID: projection.targetID,
		GraphDigest: first, Policy: policy, Bounds: testBounds(), Accounting: projection.accounting,
		Analysis: analyze(AnalysisRequest{Kind: AnalysisNeighborhood}, projection, testBounds())}
	raw, marshalErr := json.Marshal(result)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	for _, forbidden := range []string{"file:///", "private", "secret", `\"uri\"`, `\"name\"`, `\"detail\"`, `\"range\"`, "call_site", "source_snippet", "source_body"} {
		if strings.Contains(strings.ToLower(string(raw)), strings.ToLower(forbidden)) {
			t.Fatalf("ASSERT_PRIVACY_CEILING: contains %q: %s", forbidden, raw)
		}
	}
	if result.EvidenceClass != evidenceClassTransientLive || result.Authority != 0 || result.SourceGraphComplete != sourceGraphCompleteUnknown || result.Retained || result.Replayable || result.PublicationEligible || result.HydrationEligible {
		t.Fatalf("ASSERT_TRANSIENT_POLICY_CONSTANTS: %+v", result)
	}
}

func TestValidationQualificationAndAccountingAlgebra(t *testing.T) {
	line, character := uint32(1), uint32(2)
	valid := Request{SessionID: "alias", Generation: 2, Target: Target{URI: "file:///x.go", Line: &line, Character: &character}, UpDepth: 1, DownDepth: 2, MaxNodes: 10,
		TimeoutMS: 1000, RequestTimeoutMS: 100, MaxMessages: 8, MaxBytes: 4096, Analysis: AnalysisRequest{Kind: AnalysisImpact, Direction: DirectionOutgoing, MaxDepth: 2}}
	if got := validateRequest(valid); got != "" {
		t.Fatalf("ASSERT_VALID_REQUEST: %s", got)
	}
	mutations := map[string]func(*Request){
		"analysis":     func(r *Request) { r.Analysis.Kind = "PAGERANK" },
		"direction":    func(r *Request) { r.Analysis.Direction = DirectionIncoming; r.Analysis.MaxDepth = 2 },
		"mixed-target": func(r *Request) { r.Target.Symbol = "x" },
		"bounds":       func(r *Request) { r.MaxNodes = 0 },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			copy := valid
			mutate(&copy)
			if got := validateRequest(copy); got != StateInvalidServerResponse {
				t.Fatalf("ASSERT_REQUEST_REJECTED: %s", got)
			}
		})
	}
	records := []sessionruntime.Record{{SessionID: "canonical", Routing: sessionruntime.RoutingMetadata{Alias: "alias"}, Generation: 2, State: session.Ready}}
	if id, state := resolveQualifiedSession(records, "alias", 2); id != "canonical" || state != "" {
		t.Fatalf("ASSERT_ALIAS_CANONICALIZED: %q %q", id, state)
	}
	if _, state := resolveQualifiedSession(records, "canonical", 1); state != StateGenerationChanged {
		t.Fatalf("ASSERT_GENERATION_EXACT: %q", state)
	}
	records = append(records, sessionruntime.Record{SessionID: "other", Routing: sessionruntime.RoutingMetadata{Alias: "alias"}, Generation: 2, State: session.Ready})
	if _, state := resolveQualifiedSession(records, "alias", 2); state != StateInvalidServerResponse {
		t.Fatalf("ASSERT_AMBIGUOUS_ALIAS_CLOSED: %q", state)
	}
	accounting := Accounting{Requests: RequestAccounting{Attempted: 4, Succeeded: 2, Failed: 1, Cancelled: 1}, Preparation: PreparationAccounting{Attempted: 1, Returned: 1},
		Nodes: AdmissionAccounting{Observed: 4, Admitted: 3, Omitted: 1}, Occurrences: AdmissionAccounting{Observed: 3, Admitted: 2, Rejected: 1}, Frontier: FrontierAccounting{Observed: 2, Expanded: 2}}
	if !reconciles(accounting) {
		t.Fatal("ASSERT_ALL_DENOMINATORS_RECONCILE")
	}
	accounting.Requests.Attempted++
	if reconciles(accounting) {
		t.Fatal("ASSERT_REQUEST_DENOMINATOR_DRIFT_REJECTED")
	}
}

func TestFailedAcquisitionAccountsObservedWithoutAdmission(t *testing.T) {
	root, child := testNode("root", 1), testNode("child", 2)
	accounting := Accounting{Requests: RequestAccounting{Attempted: 2, Succeeded: 1, Failed: 1}, Preparation: PreparationAccounting{Attempted: 1, Returned: 1}, Frontier: FrontierAccounting{Observed: 1, Unexpanded: 1}}
	got := accountUnadmitted(accounting, []graph.Node{root, child}, []graph.Edge{testEdge(root, child, 1)}, OmissionTimeout)
	if got.Nodes != (AdmissionAccounting{Observed: 2, Omitted: 2}) || got.Occurrences != (AdmissionAccounting{Observed: 1, Omitted: 1}) || !reconciles(got) || !hasOmission(got, OmissionTimeout) {
		t.Fatalf("ASSERT_FAILED_ACQUISITION_DENOMINATORS: %+v", got)
	}
}

func TestRequestedDepthBoundaryAllowsNonFatalCallSiteWarnings(t *testing.T) {
	result := graph.Result{
		Summary:     graph.Summary{Complete: false, Truncated: true},
		Frontier:    []graph.Boundary{{NodeID: "caller", Reason: graph.MaxDepth}},
		Diagnostics: []graph.Diagnostic{{Phase: "traverse", Method: "callHierarchy/incomingCalls", NodeID: "caller", Message: "SERVER_CALL_SITE_OUTSIDE_CALLER_RANGE"}},
	}
	if !onlyRequestedDepthBoundary(result) {
		t.Fatal("ASSERT_WARNING_ONLY_REQUESTED_DEPTH_BOUNDARY_COMPLETE")
	}
	result.Terminals = []graph.Boundary{{NodeID: "caller", Reason: graph.InvalidServerResponse}}
	if onlyRequestedDepthBoundary(result) {
		t.Fatal("ASSERT_NON_DEPTH_TERMINAL_STILL_REJECTED")
	}
}

func TestOnlyLegalLifecycleTerminalsAndSoleProductionFunction(t *testing.T) {
	legal := map[Phase][]TerminalState{
		PhasePreflight:     {StateUnsupported, StateAmbiguousTarget, StateTargetNotFound, StateResourceLimit, StateTimeout, StateCancelled, StateGenerationChanged, StateInvalidServerResponse},
		PhaseTraversal:     {StatePartial, StateTruncated, StateResourceLimit, StateTimeout, StateCancelled, StateGenerationChanged, StateInvalidServerResponse},
		PhaseAdmission:     {StateResourceLimit, StateCancelled, StateGenerationChanged, StateInvalidServerResponse},
		PhaseAnalysis:      {StateResourceLimit, StateTimeout, StateCancelled, StateGenerationChanged, StateAnalysisFailed},
		PhaseDeliveryCheck: {StateComplete, StateEmpty, StateCancelled, StateGenerationChanged},
	}
	for phase, states := range legal {
		for _, state := range states {
			if !legalTerminalPair(phase, state) {
				t.Fatalf("ASSERT_ADR0006_LEGAL_PAIR_REJECTED: %s/%s", phase, state)
			}
		}
	}
	packageType := reflect.TypeOf((*DomainFailure)(nil))
	if packageType.Elem().NumField() != 4 {
		t.Fatalf("ASSERT_FAILURE_PRIVACY_SURFACE: %+v", packageType.Elem())
	}
	diagnosticType := reflect.TypeOf(TraversalDiagnostic{})
	if diagnosticType.NumField() != 4 {
		t.Fatalf("ASSERT_TRAVERSAL_DIAGNOSTIC_CLOSED_PRIVACY_SURFACE: %+v", diagnosticType)
	}
	if reflect.TypeOf(Execute).NumIn() != 3 || reflect.TypeOf(Execute).NumOut() != 2 {
		t.Fatal("ASSERT_SOLE_ENTRY_SIGNATURE")
	}
}
