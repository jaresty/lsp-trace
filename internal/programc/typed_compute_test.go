package programc

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/manageddiagnostic"
	"lsp-trace/internal/programcadmission"
	"lsp-trace/internal/programccompose"
)

func TestComputeTypedCallsDeepEqualHistoricalCompositeForTwoSeedsWithoutMutation(t *testing.T) {
	for _, seed := range []uint64{19, 23} {
		t.Run(fmt.Sprintf("seed-%d", seed), func(t *testing.T) {
			fixture := typedComputeFixture(t, false)
			beforeInput := snapshotTypedInput(fixture.callsOnly)
			beforeAdmission := snapshotAdmission(fixture.calls.Admission)
			historical, failure := ComputeComposite(fixture.calls.Admission, seed)
			if failure != nil {
				t.Fatal(failure)
			}
			bridge, err := programcadmission.PreparePrivateLeidenProjection(fixture.callsOnly)
			if err != nil {
				t.Fatal(err)
			}
			got, failure := computeTypedProjection(bridge, callsOnly, seed)
			if failure != nil {
				t.Fatal(failure)
			}
			if !reflect.DeepEqual(got, historical) {
				t.Fatalf("ASSERT_TYPED_CALLS_FULL_HISTORICAL_OUTCOME_EQUAL seed=%d got=%#v historical=%#v", seed, got, historical)
			}
			if !reflect.DeepEqual(snapshotTypedInput(fixture.callsOnly), beforeInput) || !reflect.DeepEqual(snapshotAdmission(fixture.calls.Admission), beforeAdmission) {
				t.Fatal("ASSERT_TYPED_CALLS_CONSTRUCTION_AND_COMPUTE_DO_NOT_MUTATE_INPUT")
			}
		})
	}
}

func TestComputeTypedDefinitionsAndReferencesFieldExactCurrentModel(t *testing.T) {
	fixture := typedComputeFixture(t, false)
	for _, tc := range []struct {
		name      string
		input     programcadmission.TypedInput
		wantNodes int
	}{
		{"references", fixture.referencesOnly, 3},
		{"definitions", fixture.definitionsOnly, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bridge, err := programcadmission.PreparePrivateLeidenProjection(tc.input)
			if err != nil {
				t.Fatal(err)
			}
			got, failure := computeTypedProjection(bridge, methodsOnly, 7)
			if failure != nil {
				t.Fatal(failure)
			}
			if len(got.Projection.NodeIdentities) != tc.wantNodes || len(got.Projection.Occurrences) != len(tc.input.Occurrences) {
				t.Fatalf("ASSERT_TYPED_D_R_EXACT_SELECTED_COUNTS got=%+v", got.Projection)
			}
			want := expectedMethodOccurrences(tc.input, got.Projection.NodeIDs)
			if !reflect.DeepEqual(got.Projection.Occurrences, want) {
				t.Fatalf("ASSERT_TYPED_D_R_OCCURRENCES_FIELD_EXACT got=%#v want=%#v", got.Projection.Occurrences, want)
			}
			if !reflect.DeepEqual(got.CompositeSource, programcadmission.CompositeSourceBinding{}) || !reflect.DeepEqual(got.Projection.CompositeSource, programcadmission.CompositeSourceBinding{}) || !reflect.DeepEqual(got.Source, SourceBinding{}) || !reflect.DeepEqual(got.Projection.Source, SourceBinding{}) {
				t.Fatalf("ASSERT_TYPED_D_R_ZERO_COMPOSITE_SOURCE got=%+v", got)
			}
			assertTypedProjectionMatchesInput(t, got.Projection, tc.input)
		})
	}
}

func TestComputeTypedMixedExactClaimAndFullCompositeSource(t *testing.T) {
	fixture := typedComputeFixture(t, false)
	bridge, err := programcadmission.PreparePrivateLeidenProjection(fixture.mixed)
	if err != nil {
		t.Fatal(err)
	}
	got, failure := computeTypedProjection(bridge, mixed, 19)
	if failure != nil {
		t.Fatal(failure)
	}
	historical, failure := ComputeComposite(fixture.calls.Admission, 19)
	if failure != nil {
		t.Fatal(failure)
	}
	const wantClaim = "PRIVATE_PROGRAM_C_TYPED_RELATION_QUALIFICATION;STRUCTURAL_PARTITION_ONLY;AUTHORITY_0;ACCEPTED_FALSE;COMPLETENESS_UNKNOWN;FEATURE_IDENTITY_UNRESOLVED;NO_PRODUCER_AUTHENTICATION;NO_EXECUTION_CLAIM"
	if got.ClaimCeiling != wantClaim || !reflect.DeepEqual(got.CompositeSource, historical.CompositeSource) || !reflect.DeepEqual(got.Projection.CompositeSource, historical.Projection.CompositeSource) {
		t.Fatalf("ASSERT_TYPED_MIXED_EXACT_CLAIM_AND_FULL_SOURCE got=%+v historical=%+v", got, historical)
	}
	for _, forbidden := range []string{"AUTHORITY_1", "ACCEPTED_TRUE", "COMPLETENESS_COMPLETE", "FEATURE_IDENTITY_RESOLVED", "PRODUCER_AUTHENTICATED", "EXECUTION_CONFIRMED"} {
		if strings.Contains(got.ClaimCeiling, forbidden) {
			t.Fatalf("ASSERT_TYPED_CLAIM_FORBIDS_STRONGER_SUBSTRING %q", forbidden)
		}
	}
}

func TestComputeTypedSelectionFailuresHaveExactProgramCCodesAndAtomicZero(t *testing.T) {
	fixture := typedComputeFixture(t, false)
	methods, err := programcadmission.PreparePrivateLeidenProjection(fixture.referencesOnly)
	if err != nil {
		t.Fatal(err)
	}
	calls, err := programcadmission.PreparePrivateLeidenProjection(fixture.callsOnly)
	if err != nil {
		t.Fatal(err)
	}
	assert := func(name string, got Outcome, failure *Failure, code FailureCode, message string) {
		t.Helper()
		if failure == nil || failure.Code != code || failure.Message != message || !reflect.DeepEqual(got, Outcome{}) {
			t.Fatalf("%s outcome=%+v failure=%+v", name, got, failure)
		}
	}
	got, failure := computeTypedProjection(methods, callsOnly, 1)
	assert("ASSERT_TYPED_SELECTION_MISMATCH_EXACT_FAILURE", got, failure, codeTypedSelectionMismatch, "typed selection does not match admitted relation kinds")
	got, failure = computeTypedProjection(calls, methodsOnly, 1)
	assert("ASSERT_TYPED_SELECTION_MISMATCH_EXACT_FAILURE", got, failure, codeTypedSelectionMismatch, "typed selection does not match admitted relation kinds")
	got, failure = computeTypedProjection(calls, typedSelection(255), 1)
	assert("ASSERT_TYPED_SELECTION_UNKNOWN_EXACT_FAILURE", got, failure, codeTypedSelectionUnknown, "unknown typed selection")
}

func TestComputeTypedMixedRejectsAggregateOccurrencesAboveCapAtomically(t *testing.T) {
	in := oversizedMixedTypedProjectionInput{occurrences: make([]programcadmission.TypedOccurrence, MaxOccurrences+1)}
	for i := range in.occurrences {
		kind := programcadmission.TypedReferencesSymbol
		relationID := ""
		if i%2 == 0 {
			kind = programcadmission.TypedCalls
			relationID = fmt.Sprintf("relation-%d", i)
		}
		in.occurrences[i] = programcadmission.TypedOccurrence{
			Identity:   fmt.Sprintf("occurrence-%06d", i),
			RelationID: relationID,
			From:       "a",
			To:         "b",
			Kind:       kind,
			Ordinal:    i,
		}
	}

	got, failure := computeTypedProjection(in, mixed, 1)
	wantMessage := fmt.Sprintf("occurrence cap exceeded: %d > %d", MaxOccurrences+1, MaxOccurrences)
	if failure == nil || failure.Code != CodeResourceLimit || failure.Message != wantMessage {
		t.Fatalf("ASSERT_TYPED_MIXED_AGGREGATE_OCCURRENCE_CAP_EXACT_RESOURCE_LIMIT failure=%+v", failure)
	}
	if !reflect.DeepEqual(got, Outcome{}) {
		t.Fatalf("ASSERT_TYPED_MIXED_AGGREGATE_OCCURRENCE_CAP_ATOMIC_ZERO outcome=%+v", got)
	}
}

type oversizedMixedTypedProjectionInput struct {
	occurrences []programcadmission.TypedOccurrence
}

func (in oversizedMixedTypedProjectionInput) Nodes() []string { return []string{"a", "b"} }
func (in oversizedMixedTypedProjectionInput) Occurrences() []programcadmission.TypedOccurrence {
	return in.occurrences
}
func (in oversizedMixedTypedProjectionInput) PairWeights() map[programcadmission.TypedPair]int {
	return map[programcadmission.TypedPair]int{{From: "a", To: "b"}: len(in.occurrences)}
}
func (in oversizedMixedTypedProjectionInput) SelectedKinds() []programcadmission.RelationKind {
	return []programcadmission.RelationKind{programcadmission.TypedCalls, programcadmission.TypedReferencesSymbol}
}
func (in oversizedMixedTypedProjectionInput) CompositeSourceBinding() programcadmission.CompositeSourceBinding {
	return programcadmission.CompositeSourceBinding{}
}
func (in oversizedMixedTypedProjectionInput) CombinedClaimCeiling() string { return "test-only" }
func (in oversizedMixedTypedProjectionInput) RetainedCallsAdmission() programcadmission.CompositeProjectionAdmission {
	return programcadmission.CompositeProjectionAdmission{}
}

func expectedMethodOccurrences(in programcadmission.TypedInput, ids map[string]int64) []Occurrence {
	out := make([]Occurrence, len(in.Occurrences))
	for i, occurrence := range in.Occurrences {
		out[i] = Occurrence{Identity: occurrence.Identity, From: ids[occurrence.From], To: ids[occurrence.To], Weight: 1}
	}
	return out
}

func assertTypedProjectionMatchesInput(t *testing.T, got Projection, in programcadmission.TypedInput) {
	t.Helper()
	wantNodes := map[string]bool{}
	for _, occurrence := range in.Occurrences {
		wantNodes[occurrence.From], wantNodes[occurrence.To] = true, true
	}
	if len(got.NodeIdentities) != len(wantNodes) || len(got.Occurrences) != len(in.Occurrences) {
		t.Fatalf("ASSERT_TYPED_PROJECTION_EXACT_MEMBERSHIP got=%+v in=%+v", got, in)
	}
	for pair, weight := range in.PairWeights {
		gotPair := Pair{From: got.NodeIDs[pair.From], To: got.NodeIDs[pair.To]}
		if got.PairWeights[gotPair] != float64(weight) {
			t.Fatalf("ASSERT_TYPED_PROJECTION_EXACT_PAIR_MAP got=%v want=%d", got.PairWeights[gotPair], weight)
		}
	}
}

type typedInputSnapshot struct {
	Version, CompositeID, Output, Claim string
	Authority                           int
	Complete                            string
	Occurrences                         []programcadmission.TypedOccurrence
	Weights                             map[programcadmission.TypedPair]int
}

func snapshotTypedInput(in programcadmission.TypedInput) typedInputSnapshot {
	occ := append([]programcadmission.TypedOccurrence(nil), in.Occurrences...)
	weights := make(map[programcadmission.TypedPair]int, len(in.PairWeights))
	for p, w := range in.PairWeights {
		weights[p] = w
	}
	return typedInputSnapshot{in.Version, in.CompositeID, in.CompositeOutputSHA256, in.ClaimCeiling, in.Authority, in.SourceGraphComplete, occ, weights}
}

type admissionSnapshot struct {
	Nodes       []string
	Occurrences []programcadmission.Occurrence
	Claim       string
	Source      programcadmission.CompositeSourceBinding
}

func snapshotAdmission(in programcadmission.CompositeProjectionAdmission) admissionSnapshot {
	return admissionSnapshot{in.NodeIdentities(), in.Occurrences(), in.ClaimCeiling(), in.SourceBinding()}
}

type typedComputeFixtures struct {
	calls                                             programcadmission.Result
	callsOnly, referencesOnly, definitionsOnly, mixed programcadmission.TypedInput
}

func typedComputeFixture(t *testing.T, reverse bool) typedComputeFixtures {
	t.Helper()
	a, b, isolate := node("typed-compute-a", 0), node("typed-compute-b", 1), node("typed-compute-isolate", 2)
	call := graph.Edge{CallerNodeID: a.ID, CalleeNodeID: b.ID, CallSites: []graph.Range{{End: graph.Position{Character: 1}}, {End: graph.Position{Character: 2}}}}
	inputs := []programccompose.Input{typedComputeCapture(t, "one", []graph.Node{a, b, isolate}, []graph.Edge{call}), typedComputeCapture(t, "two", []graph.Node{isolate, b, a}, nil)}
	if reverse {
		inputs[0], inputs[1] = inputs[1], inputs[0]
	}
	composite, err := programccompose.Compose(inputs)
	if err != nil {
		t.Fatal(err)
	}
	calls, err := programcadmission.Admit(composite.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	envelope := typedMethodEnvelope([]programcadmission.MethodOccurrenceEvidence{
		typedMethodOccurrence("typed-reference-1", programcadmission.TypedReferencesSymbol, 1, a.ID, b.ID),
		typedMethodOccurrence("typed-definition-0", programcadmission.TypedResolvesToDefinition, 0, a.ID, b.ID),
		typedMethodOccurrence("typed-reference-self", programcadmission.TypedReferencesSymbol, 2, isolate.ID, isolate.ID),
		typedMethodOccurrence("typed-reference-0", programcadmission.TypedReferencesSymbol, 0, a.ID, b.ID),
		typedMethodOccurrence("typed-definition-1", programcadmission.TypedResolvesToDefinition, 1, a.ID, b.ID),
	})
	if reverse {
		for l, r := 0, len(envelope.Occurrences)-1; l < r; l, r = l+1, r-1 {
			envelope.Occurrences[l], envelope.Occurrences[r] = envelope.Occurrences[r], envelope.Occurrences[l]
		}
	}
	methods, err := programcadmission.AdmitUntrustedMethodEvidenceEnvelope(envelope)
	if err != nil {
		t.Fatal(err)
	}
	prepare := func(kinds ...programcadmission.RelationKind) programcadmission.TypedInput {
		needMethods := false
		for _, kind := range kinds {
			needMethods = needMethods || kind != programcadmission.TypedCalls
		}
		in, err := programcadmission.PrepareTypedInput(programcadmission.TypedInputRequest{Calls: calls.Admission, Methods: methods, EnableMethods: needMethods, Kinds: kinds})
		if err != nil {
			t.Fatal(err)
		}
		return in
	}
	return typedComputeFixtures{calls, prepare(programcadmission.TypedCalls), prepare(programcadmission.TypedReferencesSymbol), prepare(programcadmission.TypedResolvesToDefinition), prepare(programcadmission.TypedCalls, programcadmission.TypedReferencesSymbol, programcadmission.TypedResolvesToDefinition)}
}

func typedComputeCapture(t *testing.T, suffix string, nodes []graph.Node, edges []graph.Edge) programccompose.Input {
	t.Helper()
	seed := graph.InvocationSeed{Label: "seed-" + suffix, At: "a.go:1:1", ResolvedURI: "file:///w/a.go", ContentSHA256: "sha256:" + strings.Repeat("a", 64), LanguageID: "go"}
	r := graph.Result{SchemaVersion: graph.SchemaVersionV5, Invocation: graph.Invocation{WorkspaceURI: "file:///w", Server: graph.ServerInvocation{Command: "gopls"}, LanguageID: "go", Seeds: []graph.InvocationSeed{seed}, Provenance: graph.InvocationProvenance{InvocationID: "invocation-" + suffix, SourceRevision: "commit", ServerVersion: "gopls@1"}}, Nodes: nodes, Edges: edges, Seeds: []graph.SeedResult{{Label: seed.Label}}, Summary: graph.Summary{Complete: false}, Capabilities: graph.Capabilities{CallHierarchyProvider: true}}
	native, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := graphprovenance.CaptureV5(native, "session", 7, manageddiagnostic.QueryResult{Status: manageddiagnostic.QueryUnavailable})
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	return programccompose.Input{Bytes: raw, Identity: "capture-" + suffix, SHA256: fmt.Sprintf("sha256:%x", sum), ByteLength: len(raw), ExactMetadata: programccompose.ExactMetadata{WorkspaceIdentity: "workspace-1", RevisionCustody: "CALLER_ASSERTED", PositionEncoding: "utf-16", AcquisitionSemantics: "managed-lsp-v1", PrivacyPolicy: "bundle-custodian-v1"}}
}
func typedMethodEnvelope(o []programcadmission.MethodOccurrenceEvidence) programcadmission.UntrustedMethodEvidenceEnvelope {
	return programcadmission.UntrustedMethodEvidenceEnvelope{Family: programcadmission.MethodEvidenceFamily, Version: programcadmission.MethodEvidenceFamily, ClaimCeiling: programcadmission.MethodEvidenceClaimCeiling, Authority: 0, Accepted: false, Completeness: "UNKNOWN", Provider: "gopls", ProviderVersion: "v0.17.1", SessionID: "session-1", Generation: 7, Revision: "commit-1", Custody: "CALLER_ASSERTED", PositionEncoding: "utf-16", Occurrences: o}
}
func typedMethodOccurrence(identity string, kind programcadmission.RelationKind, ordinal int, from, to string) programcadmission.MethodOccurrenceEvidence {
	methods := map[programcadmission.RelationKind]string{programcadmission.TypedReferencesSymbol: "textDocument/references", programcadmission.TypedResolvesToDefinition: "textDocument/definition"}
	source := map[programcadmission.RelationKind]string{programcadmission.TypedReferencesSymbol: "REFERENCING_OCCURRENCE", programcadmission.TypedResolvesToDefinition: "QUERY_SYMBOL"}
	target := map[programcadmission.RelationKind]string{programcadmission.TypedReferencesSymbol: "REFERENCED_SYMBOL", programcadmission.TypedResolvesToDefinition: "DEFINITION_TARGET"}
	return programcadmission.MethodOccurrenceEvidence{Identity: identity, Kind: kind, Ordinal: ordinal, From: from, To: to, SourceRole: source[kind], TargetRole: target[kind], Method: methods[kind], QuerySelector: "queries/" + identity + ".json", QueryDigest: "sha256:" + strings.Repeat("1", 64), QueryLength: 17, EvidenceSelector: "evidence/" + identity + ".json", EvidenceDigest: "sha256:" + strings.Repeat("2", 64), EvidenceLength: 29, Provider: "gopls", ProviderVersion: "v0.17.1", SessionID: "session-1", Generation: 7, Revision: "commit-1", Custody: "CALLER_ASSERTED", PositionEncoding: "utf-16"}
}
