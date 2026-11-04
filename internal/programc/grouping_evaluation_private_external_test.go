package programc_test

// Controlled fixtures use existing untrusted structural admission only. They do
// not qualify a policy or claim actual provider execution or application quality.
import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/manageddiagnostic"
	pc "lsp-trace/internal/programc"
	"lsp-trace/internal/programcadmission"
	"lsp-trace/internal/programccompose"
	rg "lsp-trace/internal/referencegrouping"
)

func evaluationLimits() rg.CompositionLimits {
	return rg.CompositionLimits{MaxRawBytes: 1 << 20, MaxOccurrences: 128, MaxPairContributions: 128, MaxPairs: 128, MaxCanonicalBytes: 1 << 20, MaxStringBytes: 4096, MaxDenominator: 1 << 20, MaxAccumulator: 1 << 20}
}
func evaluationPolicy(t *testing.T, probe string) rg.GroupingPolicy {
	t.Helper()
	rules := []rg.GroupingPolicyRule{}
	for _, k := range []string{"CALLS", "REFERENCES_SYMBOL", "RESOLVES_TO_DEFINITION"} {
		r := rg.GroupingPolicyRule{RelationKind: k, Action: "INCLUDE", RationaleCode: "CONTROLLED_CANDIDATE_" + probe, Weight: rg.Rational{Numerator: 1, Denominator: 1}}
		if k != "CALLS" {
			switch probe {
			case "H":
				r.Action = "EXCLUDE"
				r.Weight.Numerator = 0
			case "L":
				r.Action = "DOWNWEIGHT"
				r.Weight.Denominator = 4
			case "B":
				r.Action = "CAP"
				r.Cap = 4
			case "N":
				r.Action = "NORMALIZE"
				r.Normalization = "BY_KIND_MULTIPLICITY"
			}
		}
		rules = append(rules, r)
	}
	p, e := rg.BuildGroupingPolicy(rg.GroupingPolicyRequest{PolicyID: "CONTROLLED_CANDIDATE_" + probe, Epoch: 8, EligibleKinds: []string{"CALLS", "REFERENCES_SYMBOL", "RESOLVES_TO_DEFINITION"}, Rules: rules, Limits: rg.GroupingPolicyLimits{MaxRules: 3, MaxCanonicalBytes: 1 << 16, MaxStringBytes: 256, MaxAbsoluteNumerator: 1 << 20, MaxDenominator: 1 << 20, MaxCap: 128}})
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func evaluationRequest(t *testing.T, kinds []programcadmission.RelationKind, probe string) pc.GroupingEvaluationCaseRequestPrivate {
	t.Helper()
	i := evaluationInput(t, kinds)
	return pc.GroupingEvaluationCaseRequestPrivate{CaseID: "controlled-case", SelectorID: "declared-selector", SelectorBytes: []byte("controlled selector declaration; not proof of query execution\n"), RunID: "run-1", Seed: 73, Input: i, Policy: evaluationPolicy(t, probe), ProjectionLimits: evaluationLimits()}
}
func TestGroupingEvaluationPrivateNoCallsFractionalLeiden(t *testing.T) {
	r := evaluationRequest(t, []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol, programcadmission.TypedResolvesToDefinition}, "L")
	if _, e := rg.ComposeQualified(r.Input, r.Policy, rg.VerifiedQualification{}, 8, r.ProjectionLimits); e == nil {
		t.Fatal("ASSERT_PRODUCTION_REFUSES_UNQUALIFIED")
	}
	t.Log("CONTROLLED_ADMITTED_NO_CALLS_FRACTIONAL_FIXTURE_READY")
	c, e := pc.NewGroupingEvaluationCasePrivate(r)
	if e != nil {
		t.Fatal(e)
	}
	result, e := pc.EvaluateGroupingCasePrivate(c)
	if e != nil {
		t.Fatalf("ASSERT_NO_CALLS_FRACTIONAL_LEIDEN_AVAILABLE: %v", e)
	}
	p := result.Partition()
	if p.Outcome != "COMPLETE" || p.Algorithm != "gonum.org/v1/gonum/graph/community.Leiden" || len(p.Communities) == 0 {
		t.Fatalf("ASSERT_NO_CALLS_FRACTIONAL_LEIDEN_AVAILABLE: %+v", p)
	}
	t.Log("ASSERT_NO_CALLS_FRACTIONAL_LEIDEN_AVAILABLE: PASS")
}

func TestGroupingEvaluationPrivateAllModesAndProbes(t *testing.T) {
	kinds := []programcadmission.RelationKind{programcadmission.TypedCalls, programcadmission.TypedReferencesSymbol, programcadmission.TypedResolvesToDefinition}
	probes, e := rg.CalibrationProbesPrivate(8)
	if e != nil {
		t.Fatal(e)
	}
	for mask := 1; mask < 8; mask++ {
		selected := []programcadmission.RelationKind{}
		for j, k := range kinds {
			if mask&(1<<j) != 0 {
				selected = append(selected, k)
			}
		}
		for probeIndex, name := range []string{"H", "U", "L", "B", "N"} {
			t.Run(fmt.Sprintf("%d/%s", mask, name), func(t *testing.T) {
				r := evaluationRequest(t, selected, "L")
				r.Input = evaluationInput(t, selected, true)
				r.Policy = probes[probeIndex]
				c, e := pc.NewGroupingEvaluationCasePrivate(r)
				if e != nil {
					t.Fatal(e)
				}
				result, e := pc.EvaluateGroupingCasePrivate(c)
				if e != nil {
					t.Fatal(e)
				}
				p := c.Projection()
				nodes := p.NodeIdentities()
				partition := result.Partition()
				members := []string{}
				for _, community := range partition.Communities {
					members = append(members, community.Members...)
				}
				sort.Strings(members)
				if !reflect.DeepEqual(nodes, members) {
					t.Fatal("ASSERT_EVALUATION_COMPLETE_ENDPOINT_PARTITION")
				}
				pairs := p.PairContributions()
				for _, pair := range pairs {
					if pair.SourceCustodyIdentity == "" || pair.SourcePredecessorIdentity == "" {
						t.Fatal("ASSERT_EVALUATION_CUSTODY_PRESERVED")
					}
				}
				active := len(pairs) > 0
				if !active {
					if partition.Outcome != "EMPTY" || len(partition.Communities) != len(nodes) || !strings.Contains(string(result.CanonicalBytes()), "EMPTY_ACTIVE_PROJECTION_NOT_SOURCE_ABSENCE") {
						t.Fatal("ASSERT_EVALUATION_ZERO_ACTIVE_SINGLETONS")
					}
				} else if partition.Outcome != "COMPLETE" {
					t.Fatal("ASSERT_EVALUATION_NONEMPTY_LEIDEN")
				}
				if name == "B" {
					counts := map[string]int{}
					seen := map[string]int{}
					for _, o := range p.Occurrences() {
						if o.Kind != "CALLS" {
							seen[o.Kind]++
							want := "CAPPED"
							if seen[o.Kind] > 4 {
								want = "EXCLUDED"
							}
							if o.TerminalDisposition != want {
								t.Fatal("ASSERT_EVALUATION_CAP4_CANONICAL_FIRST_FOUR")
							}
						}
						if o.Kind != "CALLS" && o.TerminalDisposition == "CAPPED" {
							counts[o.Kind]++
						}
					}
					for _, kind := range selected {
						if kind != programcadmission.TypedCalls && counts[string(kind)] != 4 {
							t.Fatal("ASSERT_EVALUATION_CAP4_WHOLE_KIND_FIRST_FOUR")
						}
					}
				}
				if name == "L" {
					for _, pair := range pairs {
						if pair.Kind != "CALLS" && pair.Weight != (rg.Rational{Numerator: 1, Denominator: 4}) {
							t.Fatal("ASSERT_EVALUATION_FRACTIONAL_PAIRS")
						}
					}
				}
				if e := pc.CheckGroupingEvaluationPrivate(c, result.CanonicalBytes()); e != nil {
					t.Fatal("ASSERT_EVALUATION_REPLAY", e)
				}
				again, e := pc.EvaluateGroupingCasePrivate(c)
				if e != nil || again.ID() != result.ID() || !bytes.Equal(again.CanonicalBytes(), result.CanonicalBytes()) {
					t.Fatal("ASSERT_EVALUATION_STABLE_SEEDED_REPLAY")
				}
				if _, e := rg.ParseGroupingPolicyQualificationReceipt(result.CanonicalBytes(), rg.QualificationParseLimits{MaxRawBytes: 1 << 20, MaxCases: 128, MaxMetrics: 128, MaxCriteria: 128, MaxReviewers: 128, MaxStringBytes: 4096}); e == nil {
					t.Fatal("ASSERT_EVALUATION_CANNOT_PARSE_AS_QUALIFICATION")
				}
				if _, e := rg.ParseCompositionReceipt(result.CanonicalBytes(), evaluationLimits()); e == nil {
					t.Fatal("ASSERT_EVALUATION_CANNOT_PARSE_AS_COMPOSITION")
				}
				var raw map[string]any
				if e := json.Unmarshal(result.CanonicalBytes(), &raw); e != nil {
					t.Fatal(e)
				}
				if raw["Authority"] != float64(0) || raw["Accepted"] != false || raw["SourceNodeUniverse"] != "UNKNOWN" || raw["UniverseScope"] != rg.EvaluationUniversePrivate || raw["PolicyStatus"] != "UNQUALIFIED" || raw["Completeness"] != "UNKNOWN" {
					t.Fatal("ASSERT_EVALUATION_FIXED_CEILING")
				}
				t.Log("ASSERT_EVALUATION_ALL_MODES_AND_PROBES: PASS")
			})
		}
	}
}
func TestGroupingEvaluationPrivateFrozenSubstitutionsAndMissingBindings(t *testing.T) {
	kinds := []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol, programcadmission.TypedResolvesToDefinition}
	request := evaluationRequest(t, kinds, "L")
	c, e := pc.NewGroupingEvaluationCasePrivate(request)
	if e != nil {
		t.Fatal(e)
	}
	result, e := pc.EvaluateGroupingCasePrivate(c)
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		name   string
		mutate func(*pc.GroupingEvaluationCaseRequestPrivate)
	}{
		{"input", func(r *pc.GroupingEvaluationCaseRequestPrivate) {
			r.Input = evaluationInput(t, []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol})
		}},
		{"selector-bytes", func(r *pc.GroupingEvaluationCaseRequestPrivate) {
			r.SelectorBytes = []byte("foreign declared selection")
		}},
		{"selector-id", func(r *pc.GroupingEvaluationCaseRequestPrivate) { r.SelectorID = "foreign-selector" }},
		{"policy", func(r *pc.GroupingEvaluationCaseRequestPrivate) { r.Policy = evaluationPolicy(t, "U") }},
		{"seed", func(r *pc.GroupingEvaluationCaseRequestPrivate) { r.Seed++ }},
		{"run", func(r *pc.GroupingEvaluationCaseRequestPrivate) { r.RunID = "foreign-run" }},
		{"case-label", func(r *pc.GroupingEvaluationCaseRequestPrivate) { r.CaseID = "foreign-label" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := request
			tc.mutate(&r)
			foreign, e := pc.NewGroupingEvaluationCasePrivate(r)
			if e != nil {
				t.Fatal(e)
			}
			foreignResult, e := pc.EvaluateGroupingCasePrivate(foreign)
			if e != nil {
				t.Fatal(e)
			}
			if pc.CheckGroupingCasePrivate(c, foreign.CanonicalBytes()) == nil || pc.CheckGroupingEvaluationPrivate(c, foreignResult.CanonicalBytes()) == nil || pc.CheckGroupingEvaluationPrivate(foreign, result.CanonicalBytes()) == nil {
				t.Fatal("ASSERT_EVALUATION_RESEALED_FOREIGN_LINK_REFUSED")
			}
			t.Log("ASSERT_EVALUATION_RESEALED_FOREIGN_LINK_REFUSED: PASS")
		})
	}
	// Corrupt all disconnected result surfaces; freshly reencoding these is not a
	// substitute for checking against the opaque frozen declaration and replay.
	for _, field := range []string{"Case", "Declaration", "Partition", "Pairs", "ActiveOccurrenceCount", "UniverseScope", "Authority", "Accepted"} {
		t.Run("result-"+field, func(t *testing.T) {
			var raw map[string]json.RawMessage
			if e := json.Unmarshal(result.CanonicalBytes(), &raw); e != nil {
				t.Fatal(e)
			}
			raw[field] = json.RawMessage("null")
			resealed, e := json.Marshal(raw)
			if e != nil {
				t.Fatal(e)
			}
			resealed = append(resealed, '\n')
			if pc.CheckGroupingEvaluationPrivate(c, resealed) == nil {
				t.Fatal("ASSERT_EVALUATION_DISCONNECTED_RESULT_REFUSED")
			}
		})
	}
	// A locally recomputed digest cannot reconnect altered declaration fields.
	for _, field := range []string{"NumericContract", "Projection", "Bindings", "Selector", "Seed", "RunID", "CaseID", "UniverseScope", "SourceNodeUniverse"} {
		t.Run("declaration-"+field, func(t *testing.T) {
			var raw map[string]any
			if e := json.Unmarshal(result.CanonicalBytes(), &raw); e != nil {
				t.Fatal(e)
			}
			declaration := raw["Declaration"].(map[string]any)
			declaration[field] = nil
			resealed, e := json.Marshal(raw)
			if e != nil {
				t.Fatal(e)
			}
			resealed = append(resealed, '\n')
			digest := sha256.Sum256(resealed)
			if pc.CheckGroupingEvaluationPrivate(c, resealed) == nil {
				t.Fatalf("ASSERT_EVALUATION_RESEALED_DECLARATION_REFUSED %s digest=%x", field, digest)
			}
		})
	}
	for _, mutate := range []func(*pc.GroupingEvaluationCaseRequestPrivate){func(r *pc.GroupingEvaluationCaseRequestPrivate) { r.CaseID = "" }, func(r *pc.GroupingEvaluationCaseRequestPrivate) { r.RunID = "" }, func(r *pc.GroupingEvaluationCaseRequestPrivate) { r.SelectorID = "" },
		func(r *pc.GroupingEvaluationCaseRequestPrivate) { r.SelectorID = string([]byte{0xff}) }, func(r *pc.GroupingEvaluationCaseRequestPrivate) { r.SelectorBytes = nil }, func(r *pc.GroupingEvaluationCaseRequestPrivate) { r.SelectorBytes = make([]byte, (1<<16)+1) }, func(r *pc.GroupingEvaluationCaseRequestPrivate) { r.CaseID = strings.Repeat("x", 4097) }, func(r *pc.GroupingEvaluationCaseRequestPrivate) { r.Input = rg.AdmittedInput{} }, func(r *pc.GroupingEvaluationCaseRequestPrivate) { r.Policy = rg.GroupingPolicy{} }} {
		r := request
		mutate(&r)
		if _, e := pc.NewGroupingEvaluationCasePrivate(r); e == nil {
			t.Fatal("ASSERT_EVALUATION_MISSING_OR_OVERSIZED_BINDING_REFUSED")
		}
	}
	if _, e := pc.EvaluateGroupingCasePrivate(pc.GroupingEvaluationCasePrivate{}); e == nil {
		t.Fatal("ASSERT_EVALUATION_MISSING_CASE_REFUSED")
	}
	if pc.CheckGroupingEvaluationPrivate(c, nil) == nil || pc.CheckGroupingEvaluationPrivate(c, make([]byte, rg.EvaluationMaxBytesPrivate+1)) == nil {
		t.Fatal("ASSERT_EVALUATION_RAW_BOUND")
	}
	// Caller mutations cannot revise the declaration, partition, or projection.
	before := c.CanonicalBytes()
	request.SelectorBytes[0] ^= 1
	raw := c.CanonicalBytes()
	raw[0] ^= 1
	p := result.Partition()
	p.Communities[0].Members[0] = "foreign"
	nodes := c.Projection().NodeIdentities()
	nodes[0] = "foreign"
	if !bytes.Equal(before, c.CanonicalBytes()) || pc.CheckGroupingEvaluationPrivate(c, result.CanonicalBytes()) != nil {
		t.Fatal("ASSERT_EVALUATION_FROZEN_COPIES")
	}
	t.Log("ASSERT_EVALUATION_FROZEN_BINDINGS: PASS")
}

func TestGroupingEvaluationPrivateInactiveTypedEndpoints(t *testing.T) {
	for _, action := range []string{"EXCLUDE", "CAP", "SEPARATE_LAYER"} {
		t.Run(action, func(t *testing.T) {
			r := evaluationRequest(t, []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol, programcadmission.TypedResolvesToDefinition}, "U")
			r.Input = evaluationInput(t, []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol, programcadmission.TypedResolvesToDefinition}, true)
			rules := r.Policy.Rules()
			for j := range rules {
				if rules[j].RelationKind == "RESOLVES_TO_DEFINITION" {
					rules[j].Action = action
					switch action {
					case "EXCLUDE":
						rules[j].Weight.Numerator = 0
					case "CAP":
						rules[j].Cap = 1
					case "SEPARATE_LAYER":
						rules[j].Layer = "CONTROLLED_DEFINITIONS"
					}
				}
			}
			var e error
			r.Policy, e = rg.BuildGroupingPolicy(rg.GroupingPolicyRequest{PolicyID: "CONTROLLED_INACTIVE_" + action, Epoch: 8, EligibleKinds: r.Policy.EligibleKinds(), Rules: rules, Limits: rg.GroupingPolicyLimits{MaxRules: 3, MaxCanonicalBytes: 1 << 16, MaxStringBytes: 256, MaxAbsoluteNumerator: 1, MaxDenominator: 1, MaxCap: 4}})
			if e != nil {
				t.Fatal(e)
			}
			c, e := pc.NewGroupingEvaluationCasePrivate(r)
			if e != nil {
				t.Fatal(e)
			}
			out, e := pc.EvaluateGroupingCasePrivate(c)
			if e != nil {
				t.Fatal(e)
			}
			projection := c.Projection()
			sourceSet := map[string]bool{}
			for _, contribution := range r.Input.Contributions() {
				sourceSet[contribution.From] = true
				sourceSet[contribution.To] = true
			}
			expectedNodes := make([]string, 0, len(sourceSet))
			for id := range sourceSet {
				expectedNodes = append(expectedNodes, id)
			}
			sort.Strings(expectedNodes)
			if len(expectedNodes) != 3 {
				t.Fatal("BLOCKED_NOT_RED controlled admitted endpoint fixture")
			}
			if !reflect.DeepEqual(expectedNodes, projection.NodeIdentities()) {
				t.Fatal("ASSERT_EVALUATION_INACTIVE_TYPED_ENDPOINT_UNIVERSE")
			}
			t.Log("ASSERT_EVALUATION_INACTIVE_TYPED_ENDPOINT_UNIVERSE: PASS")
			active := map[string]bool{}
			for _, pair := range projection.PairContributions() {
				active[pair.From] = true
				active[pair.To] = true
			}
			inactive := []string{}
			for _, id := range expectedNodes {
				if !active[id] {
					inactive = append(inactive, id)
				}
			}
			if len(inactive) != 1 {
				t.Fatal("ASSERT_EVALUATION_REAL_INACTIVE_ENDPOINT_FIXTURE")
			}
			found := false
			for _, community := range out.Partition().Communities {
				if reflect.DeepEqual(community.Members, inactive) {
					found = true
				}
			}
			if !found || len(projection.Occurrences()) != len(r.Input.Contributions()) {
				t.Fatal("ASSERT_EVALUATION_INACTIVE_TYPED_ENDPOINT_SINGLETON")
			}
			t.Log("ASSERT_EVALUATION_INACTIVE_TYPED_ENDPOINT_SINGLETON: PASS")
		})
	}
}

func evaluationInput(t *testing.T, kinds []programcadmission.RelationKind, rich ...bool) rg.AdmittedInput {
	t.Helper()
	node := func(name string, line uint32) graph.Node {
		r := graph.Range{Start: graph.Position{Line: line}, End: graph.Position{Line: line, Character: 1}}
		return graph.NewNode(graph.Item{Name: name, Kind: 12, URI: "file:///controlled/fixture", Range: r, SelectionRange: r})
	}
	a, b := node("a", 0), node("b", 1)
	inactive := node("inactive-definition-endpoint", 2)
	callsNeeded, methodsNeeded := false, false
	for _, k := range kinds {
		callsNeeded = callsNeeded || k == programcadmission.TypedCalls
		methodsNeeded = methodsNeeded || k != programcadmission.TypedCalls
	}
	var calls programcadmission.CompositeProjectionAdmission
	if callsNeeded {
		capture := func(nodes []graph.Node, edges []graph.Edge, id string) programccompose.Input {
			seed := graph.InvocationSeed{Label: "seed", At: "fixture:1:1", ResolvedURI: "file:///controlled/fixture", ContentSHA256: "sha256:" + strings.Repeat("a", 64), LanguageID: "go"}
			r := graph.Result{SchemaVersion: graph.SchemaVersionV5, Invocation: graph.Invocation{WorkspaceURI: "file:///controlled", Server: graph.ServerInvocation{Command: "controlled"}, LanguageID: "go", Seeds: []graph.InvocationSeed{seed}, Provenance: graph.InvocationProvenance{InvocationID: "controlled-invocation", SourceRevision: "controlled-commit", ServerVersion: "controlled@1"}}, Nodes: nodes, Edges: edges, Seeds: []graph.SeedResult{{Label: seed.Label}}, Summary: graph.Summary{Complete: false}, Capabilities: graph.Capabilities{CallHierarchyProvider: true}}
			native, e := json.Marshal(r)
			if e != nil {
				t.Fatal(e)
			}
			raw, e := graphprovenance.CaptureV5(native, "controlled-session", 1, manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable})
			if e != nil {
				t.Fatal(e)
			}
			h := sha256.Sum256(raw)
			return programccompose.Input{Bytes: raw, Identity: id, SHA256: fmt.Sprintf("sha256:%x", h), ByteLength: len(raw), ExactMetadata: programccompose.ExactMetadata{WorkspaceIdentity: "controlled-workspace", RevisionCustody: "CALLER_ASSERTED", PositionEncoding: "utf-16", AcquisitionSemantics: "managed-lsp-v1", PrivacyPolicy: "controlled-private"}}
		}
		edges := []graph.Edge{{CallerNodeID: a.ID, CalleeNodeID: b.ID, CallSites: []graph.Range{{End: graph.Position{Character: 1}}}}, {CallerNodeID: b.ID, CalleeNodeID: a.ID, CallSites: []graph.Range{{Start: graph.Position{Line: 1}, End: graph.Position{Line: 1, Character: 1}}}}}
		composite, e := programccompose.Compose([]programccompose.Input{capture([]graph.Node{a, b}, edges, "calls"), capture(nil, nil, "empty")})
		if e != nil {
			t.Fatal(e)
		}
		admitted, e := programcadmission.Admit(composite.Bytes)
		if e != nil {
			t.Fatal(e)
		}
		calls = admitted.Admission
	}
	var methods programcadmission.MethodProjectionAdmission
	if methodsNeeded {
		makeOccurrence := func(id string, k programcadmission.RelationKind, ordinal int, from, to string) programcadmission.MethodOccurrenceEvidence {
			method := map[programcadmission.RelationKind]string{programcadmission.TypedReferencesSymbol: "textDocument/references", programcadmission.TypedResolvesToDefinition: "textDocument/definition"}[k]
			sr := map[programcadmission.RelationKind]string{programcadmission.TypedReferencesSymbol: "REFERENCING_OCCURRENCE", programcadmission.TypedResolvesToDefinition: "QUERY_SYMBOL"}[k]
			tr := map[programcadmission.RelationKind]string{programcadmission.TypedReferencesSymbol: "REFERENCED_SYMBOL", programcadmission.TypedResolvesToDefinition: "DEFINITION_TARGET"}[k]
			q, e := []byte("{\"query\":true}\n"), []byte("{\"evidence\":true}\n")
			qh, eh := sha256.Sum256(q), sha256.Sum256(e)
			return programcadmission.MethodOccurrenceEvidence{Identity: id, Kind: k, Ordinal: ordinal, From: from, To: to, SourceRole: sr, TargetRole: tr, Method: method, QuerySelector: "methods/query.json", QueryDigest: fmt.Sprintf("sha256:%x", qh), QueryLength: len(q), EvidenceSelector: "methods/evidence.json", EvidenceDigest: fmt.Sprintf("sha256:%x", eh), EvidenceLength: len(e), Provider: "controlled-provider", ProviderVersion: "v1", SessionID: "controlled-session", Generation: 1, Revision: "controlled-commit", Custody: "CALLER_ASSERTED", PositionEncoding: "utf-16"}
		}
		occurrences := []programcadmission.MethodOccurrenceEvidence{makeOccurrence("ref-0", programcadmission.TypedReferencesSymbol, 0, a.ID, b.ID), makeOccurrence("ref-1", programcadmission.TypedReferencesSymbol, 1, a.ID, a.ID), makeOccurrence("def-0", programcadmission.TypedResolvesToDefinition, 0, a.ID, b.ID)}
		// More than four per kind distinguishes whole-kind CAP4 from a per-pair
		// cap, and repeated/reverse endpoints exercise typed multiplicity.
		if len(rich) > 0 && rich[0] {
			for j := 2; j < 6; j++ {
				from, to := a.ID, b.ID
				if j == 5 {
					from, to = b.ID, a.ID
				}
				occurrences = append(occurrences, makeOccurrence(fmt.Sprintf("ref-%d", j), programcadmission.TypedReferencesSymbol, j, from, to))
			}
			for j := 1; j < 5; j++ {
				occurrences = append(occurrences, makeOccurrence(fmt.Sprintf("def-%d", j), programcadmission.TypedResolvesToDefinition, j, b.ID, inactive.ID))
			}
		}
		var e error
		methods, e = programcadmission.AdmitUntrustedMethodEvidenceEnvelope(programcadmission.UntrustedMethodEvidenceEnvelope{Family: programcadmission.MethodEvidenceFamily, Version: programcadmission.MethodEvidenceFamily, ClaimCeiling: programcadmission.MethodEvidenceClaimCeiling, Authority: 0, Accepted: false, Completeness: "UNKNOWN", Provider: "controlled-provider", ProviderVersion: "v1", SessionID: "controlled-session", Generation: 1, Revision: "controlled-commit", Custody: "CALLER_ASSERTED", PositionEncoding: "utf-16", Occurrences: occurrences})
		if e != nil {
			t.Fatal(e)
		}
	}
	typed, e := programcadmission.PrepareTypedInput(programcadmission.TypedInputRequest{Calls: calls, Methods: methods, EnableMethods: methodsNeeded, Kinds: kinds})
	if e != nil {
		t.Fatal(e)
	}
	projection, e := programcadmission.PreparePrivateLeidenProjection(typed)
	if e != nil {
		t.Fatal(e)
	}
	marshal := func(v any) []byte {
		b, e := json.Marshal(v)
		if e != nil {
			t.Fatal(e)
		}
		return append(b, '\n')
	}
	type ledgerOccurrence struct {
		Identity   string `json:"identity"`
		Kind       string `json:"kind"`
		From       string `json:"from"`
		To         string `json:"to"`
		Ordinal    int    `json:"ordinal"`
		RelationID string `json:"relation_id"`
	}
	rows := make([]ledgerOccurrence, len(typed.Occurrences))
	outcomes := make([]rg.CandidateOutcome, len(rows))
	for i, o := range typed.Occurrences {
		rows[i] = ledgerOccurrence{o.Identity, string(o.Kind), o.From, o.To, o.Ordinal, o.RelationID}
		outcomes[i] = rg.CandidateOutcome{OccurrenceIdentity: o.Identity, Disposition: "ADMITTED"}
	}
	ledger := marshal(struct {
		Domain      string             `json:"domain"`
		Version     string             `json:"version"`
		Claim       string             `json:"claim"`
		Occurrences []ledgerOccurrence `json:"occurrences"`
	}{"PROGRAM_C_TYPED_OCCURRENCES", "v1", "ADMITTED_TYPED_OCCURRENCES_EXACT", rows})
	predecessors := []rg.PredecessorRecord{{Role: "OCCURRENCE_LEDGER", Selector: "ledgers/occurrences.json", Schema: "lsp-trace.private.program-c-occurrence-ledger.v1", Bytes: ledger}}
	custody := []rg.EndpointCustody{}
	if callsNeeded {
		predecessors = append(predecessors, rg.PredecessorRecord{Role: "ACQUISITION_RECEIPT", Selector: "calls/acquisition.json", Schema: "lsp-trace.private.program-c-acquisition-receipt.v1", Bytes: marshal(projection.CompositeSourceBinding())}, rg.PredecessorRecord{Role: "RETAINED_COMPOSITE", Selector: "calls/composite.json", Schema: "lsp-trace.private.program-c-composite.v1", Bytes: projection.RetainedCallsCanonicalBytes()})
		for _, n := range []graph.Node{a, b} {
			custody = append(custody, rg.EndpointCustody{Endpoint: n.ID, PredecessorSelector: "calls/acquisition.json", Custody: "CALLER_ASSERTED"})
		}
	}
	if methodsNeeded {
		type mapping struct {
			Identity         string `json:"identity"`
			Kind             string `json:"kind"`
			From             string `json:"from"`
			To               string `json:"to"`
			Ordinal          int    `json:"ordinal"`
			QuerySelector    string `json:"query_selector"`
			EvidenceSelector string `json:"evidence_selector"`
		}
		mappings := []mapping{}
		for _, o := range projection.MethodEvidence() {
			mappings = append(mappings, mapping{o.Identity, string(o.Kind), o.From, o.To, o.Ordinal, o.QuerySelector, o.EvidenceSelector})
		}
		sort.Slice(mappings, func(i, j int) bool { return mappings[i].Identity < mappings[j].Identity })
		receipt := marshal(struct {
			Family   string    `json:"family"`
			Version  string    `json:"version"`
			Mappings []mapping `json:"mappings"`
		}{"METHOD_MAPPING_RECEIPT", "v1", mappings})
		predecessors = append(predecessors, rg.PredecessorRecord{Role: "METHOD_QUERY", Selector: "methods/query.json", Schema: "lsp-trace.private.program-c-method-query.v0", Bytes: []byte("{\"query\":true}\n")}, rg.PredecessorRecord{Role: "METHOD_EVIDENCE", Selector: "methods/evidence.json", Schema: programcadmission.MethodEvidenceFamily, Bytes: projection.MethodCanonicalBytes()}, rg.PredecessorRecord{Role: "METHOD_MAPPING_RECEIPT", Selector: "methods/mapping-receipt.json", Schema: "lsp-trace.private.program-c-method-mapping-receipt.v1", Bytes: receipt})
		methodNodes := []graph.Node{a, b}
		if len(rich) > 0 && rich[0] {
			methodNodes = append(methodNodes, inactive)
		}
		for _, n := range methodNodes {
			custody = append(custody, rg.EndpointCustody{Endpoint: n.ID, PredecessorSelector: "methods/evidence.json", Custody: "CALLER_ASSERTED"})
		}
	}
	artifact, e := rg.BuildInput(rg.InputRequest{TypedInput: typed, Predecessors: predecessors, OccurrenceLedgerBytes: ledger, EndpointCustody: custody, OccurrenceAdmissionPolicyID: "controlled-occurrence-v1", CompositionPolicyID: "controlled-identity-v1", PrivacyPolicy: "controlled-private", EligibleKinds: []programcadmission.RelationKind{programcadmission.TypedCalls, programcadmission.TypedReferencesSymbol, programcadmission.TypedResolvesToDefinition}, Limits: rg.ResourceLimits{MaxArtifacts: 16, MaxNodes: 32, MaxOccurrences: 32, MaxContributions: 32, MaxCanonicalBytes: 1 << 20, MaxStringBytes: 4096}, CandidateOutcomes: outcomes, Accounting: rg.Accounting{Candidates: len(rows), Included: len(rows), Terminal: true}})
	if e != nil {
		t.Fatal(e)
	}
	admitted, e := rg.AdmitInput(artifact, predecessors)
	if e != nil {
		t.Fatal(e)
	}
	return admitted
}
