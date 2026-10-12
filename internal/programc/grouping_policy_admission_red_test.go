package programc

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"lsp-trace/internal/programcadmission"
	"lsp-trace/internal/referencegrouping"
)

// stage4TestAdmission is test mechanics only. It is neither a production
// qualification receipt nor authority for production grouping.
type stage4TestAdmission struct {
	family, evidenceClass, trustClass            string
	policyID, policyDigest, inputID, inputDigest string
	policyBytes, inputBytes                      []byte
	policyByteLength, inputByteLength            int
	validFrom, validThrough                      uint64
}

func stage4QualifiedAdmission(t *testing.T, in programcadmission.TypedInput, epoch uint64) stage4TestAdmission {
	t.Helper()
	projection := mustPrepareTypedProjection(t, in)
	inputBytes := []byte(fmt.Sprintf("TEST_ONLY_REFERENCE_GROUPING_INPUT_V1\x00%s\x00%s\x00%s\x00%d\x00%q\x00%v", in.Version, in.CompositeID, in.CompositeOutputSHA256, in.Authority, projection.SelectedKinds(), projection.Occurrences()))
	inputSum := sha256.Sum256(inputBytes)
	policyBytes := []byte("TEST_ONLY_GROUPING_POLICY_V1\n")
	policySum := sha256.Sum256(policyBytes)
	return stage4TestAdmission{
		family: "LSP_TRACE_TEST_GROUPING_MECHANICS_RECEIPT", evidenceClass: "TEST_ONLY_STRUCTURAL_EVIDENCE", trustClass: "TEST_ONLY_TRUST_ANCHOR",
		policyBytes: append([]byte(nil), policyBytes...), policyID: fmt.Sprintf("sha256:%x", policySum), policyByteLength: len(policyBytes), policyDigest: fmt.Sprintf("sha256:%x", policySum),
		inputBytes: append([]byte(nil), inputBytes...), inputID: fmt.Sprintf("sha256:%x", inputSum), inputByteLength: len(inputBytes), inputDigest: fmt.Sprintf("sha256:%x", inputSum),
		validFrom: epoch, validThrough: epoch + 2,
	}
}

func stage4AdmissionVerifier(expected stage4TestAdmission, currentEpoch uint64) func(stage4TestAdmission) bool {
	return func(got stage4TestAdmission) bool {
		return got.family == "LSP_TRACE_TEST_GROUPING_MECHANICS_RECEIPT" && got.evidenceClass == "TEST_ONLY_STRUCTURAL_EVIDENCE" && got.trustClass == "TEST_ONLY_TRUST_ANCHOR" && reflect.DeepEqual(got, expected) && currentEpoch >= got.validFrom && currentEpoch <= got.validThrough
	}
}

func assertStage4Unqualified(t *testing.T, got TypedComputation, failure *Failure, projectionCalls, kernelCalls int) {
	t.Helper()
	if projectionCalls != 0 || kernelCalls != 0 || failure == nil || failure.Code != FailureCode("GROUPING_POLICY_UNQUALIFIED") || failure.Message != "grouping policy unqualified" || !reflect.DeepEqual(got, TypedComputation{}) {
		t.Fatalf("ASSERT_STAGE4_UNQUALIFIED_PRECEDES_PROJECTION_AND_KERNEL projection=%d kernel=%d got=%+v failure=%+v", projectionCalls, kernelCalls, got, failure)
	}
}

func TestStage4CallsComputeAndRecomputeRemainExactHistoricalBytesAndDelegateOnce(t *testing.T) {
	fixture := typedComputeFixture(t, false)
	computed, failure := ComputeTyped(fixture.callsOnly, 53)
	if failure != nil {
		t.Fatal(failure)
	}
	want := marshalExactOutcomeJSON(t, computed.Outcome())

	computeCalls := 0
	got, failure := computeTypedWithGroupingGate(fixture.callsOnly, 53, stage4TestAdmission{}, nil, func(in typedProjectionInput, selection typedSelection, seed uint64) (Outcome, *Failure) {
		computeCalls++
		return computeTypedProjection(in, selection, seed)
	})
	if failure != nil || computeCalls != 1 || !bytes.Equal(marshalExactOutcomeJSON(t, got.Outcome()), want) {
		t.Fatalf("ASSERT_STAGE4_CALLS_COMPUTE_BYTES_AND_DELEGATION calls=%d failure=%+v", computeCalls, failure)
	}

	recomputeCalls := 0
	recomputed, failure := recomputeTypedWithGroupingGate(computed, stage4TestAdmission{}, nil, func(in typedProjectionInput, selection typedSelection, seed uint64) (Outcome, *Failure) {
		recomputeCalls++
		return computeTypedProjection(in, selection, seed)
	})
	if failure != nil || recomputeCalls != 1 || !bytes.Equal(marshalExactOutcomeJSON(t, recomputed.Outcome()), want) {
		t.Fatalf("ASSERT_STAGE4_CALLS_RECOMPUTE_BYTES_AND_DELEGATION calls=%d failure=%+v", recomputeCalls, failure)
	}
}

func TestStage4ComputeAndRecomputeQualifiedDelegateExactlyOnceWithExactSuccess(t *testing.T) {
	fixture := typedComputeFixture(t, false)
	for _, tc := range []struct {
		name  string
		input programcadmission.TypedInput
	}{
		{"references", fixture.referencesOnly}, {"definitions", fixture.definitionsOnly}, {"mixed", fixture.mixed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			projection := mustPrepareTypedProjection(t, tc.input)
			selection, ok := inferTypedSelection(projection.SelectedKinds())
			if !ok {
				t.Fatal("BLOCKED_NOT_RED typed selection")
			}
			wantOutcome, failure := computeTypedProjection(projection, selection, 59)
			if failure != nil {
				t.Fatal(failure)
			}
			wantBytes := marshalExactOutcomeJSON(t, wantOutcome)
			admission := stage4QualifiedAdmission(t, tc.input, 7)

			computeCalls := 0
			computed, failure := computeTypedWithGroupingGate(tc.input, 59, admission, stage4AdmissionVerifier(admission, 7), func(in typedProjectionInput, selection typedSelection, seed uint64) (Outcome, *Failure) {
				computeCalls++
				return computeTypedProjection(in, selection, seed)
			})
			if failure != nil || computeCalls != 1 || !bytes.Equal(marshalExactOutcomeJSON(t, computed.Outcome()), wantBytes) {
				t.Fatalf("ASSERT_STAGE4_COMPUTE_QUALIFIED_EXACT_SUCCESS calls=%d got=%+v failure=%+v", computeCalls, computed, failure)
			}

			recomputeCalls := 0
			recomputed, failure := recomputeTypedWithGroupingGate(internallyFormedTypedCarrier(t, tc.input, 59), admission, stage4AdmissionVerifier(admission, 7), func(in typedProjectionInput, selection typedSelection, seed uint64) (Outcome, *Failure) {
				recomputeCalls++
				return computeTypedProjection(in, selection, seed)
			})
			if failure != nil || recomputeCalls != 1 || !bytes.Equal(marshalExactOutcomeJSON(t, recomputed.Outcome()), wantBytes) {
				t.Fatalf("ASSERT_STAGE4_RECOMPUTE_QUALIFIED_EXACT_SUCCESS calls=%d got=%+v failure=%+v", recomputeCalls, recomputed, failure)
			}
		})
	}
}

func arbitraryPreStage5Policy(t *testing.T, id string) referencegrouping.GroupingPolicy {
	t.Helper()
	policy, err := referencegrouping.BuildGroupingPolicy(referencegrouping.GroupingPolicyRequest{
		PolicyID:      id,
		Epoch:         7,
		EligibleKinds: []string{"CALLS", "REFERENCES_SYMBOL", "RESOLVES_TO_DEFINITION"},
		Rules: []referencegrouping.GroupingPolicyRule{
			{RelationKind: "CALLS", Action: "INCLUDE", RationaleCode: "TEST_ONLY_PRE_STAGE5_CALLS", Weight: referencegrouping.Rational{Numerator: 1, Denominator: 1}},
			{RelationKind: "REFERENCES_SYMBOL", Action: "DOWNWEIGHT", RationaleCode: "TEST_ONLY_PRE_STAGE5_REFERENCES", Weight: referencegrouping.Rational{Numerator: 1, Denominator: 2}},
			{RelationKind: "RESOLVES_TO_DEFINITION", Action: "CAP", RationaleCode: "TEST_ONLY_PRE_STAGE5_DEFINITIONS", Weight: referencegrouping.Rational{Numerator: 1, Denominator: 1}, Cap: 2},
		},
		Limits: referencegrouping.GroupingPolicyLimits{MaxRules: 8, MaxCanonicalBytes: 4096, MaxStringBytes: 256, MaxAbsoluteNumerator: 8, MaxDenominator: 8, MaxCap: 32},
	})
	if err != nil {
		t.Fatal(err)
	}
	return policy
}

func testStage5ReferenceGroupingInput(t *testing.T) (programcadmission.TypedInput, referencegrouping.AdmittedInput) {
	t.Helper()
	fixture := typedComputeFixture(t, false)
	query := []byte("{\"query\":true}\n")
	queryDigest := sha256.Sum256(query)
	methods := fixture.methods.Occurrences()
	for i := range methods {
		methods[i].QuerySelector = "methods/query.json"
		methods[i].QueryDigest = fmt.Sprintf("sha256:%x", queryDigest)
		methods[i].QueryLength = len(query)
		methods[i].EvidenceSelector = "methods/evidence.json"
	}
	admittedMethods, err := programcadmission.AdmitUntrustedMethodEvidenceEnvelope(typedMethodEnvelope(methods))
	if err != nil {
		t.Fatal(err)
	}
	typed, err := programcadmission.PrepareTypedInput(programcadmission.TypedInputRequest{Methods: admittedMethods, EnableMethods: true, Kinds: []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol}})
	if err != nil {
		t.Fatal(err)
	}
	projection, err := programcadmission.PreparePrivateLeidenProjection(typed)
	if err != nil {
		t.Fatal(err)
	}
	type ledgerRow struct {
		Identity   string `json:"identity"`
		Kind       string `json:"kind"`
		From       string `json:"from"`
		To         string `json:"to"`
		Ordinal    int    `json:"ordinal"`
		RelationID string `json:"relation_id"`
	}
	rows := make([]ledgerRow, len(projection.Occurrences()))
	outcomes := make([]referencegrouping.CandidateOutcome, len(rows))
	custodyByEndpoint := map[string]bool{}
	for i, occurrence := range projection.Occurrences() {
		rows[i] = ledgerRow{occurrence.Identity, string(occurrence.Kind), occurrence.From, occurrence.To, occurrence.Ordinal, occurrence.RelationID}
		outcomes[i] = referencegrouping.CandidateOutcome{OccurrenceIdentity: occurrence.Identity, Disposition: "ADMITTED"}
		custodyByEndpoint[occurrence.From], custodyByEndpoint[occurrence.To] = true, true
	}
	ledger, err := json.Marshal(struct {
		Domain      string      `json:"domain"`
		Version     string      `json:"version"`
		Claim       string      `json:"claim"`
		Occurrences []ledgerRow `json:"occurrences"`
	}{"PROGRAM_C_TYPED_OCCURRENCES", "v1", "ADMITTED_TYPED_OCCURRENCES_EXACT", rows})
	if err != nil {
		t.Fatal(err)
	}
	ledger = append(ledger, '\n')
	type mapping struct {
		Identity         string `json:"identity"`
		Kind             string `json:"kind"`
		From             string `json:"from"`
		To               string `json:"to"`
		Ordinal          int    `json:"ordinal"`
		QuerySelector    string `json:"query_selector"`
		EvidenceSelector string `json:"evidence_selector"`
	}
	mappings := make([]mapping, len(projection.MethodEvidence()))
	for i, occurrence := range projection.MethodEvidence() {
		mappings[i] = mapping{occurrence.Identity, string(occurrence.Kind), occurrence.From, occurrence.To, occurrence.Ordinal, occurrence.QuerySelector, occurrence.EvidenceSelector}
	}
	mappingBytes, err := json.Marshal(struct {
		Family   string    `json:"family"`
		Version  string    `json:"version"`
		Mappings []mapping `json:"mappings"`
	}{"METHOD_MAPPING_RECEIPT", "v1", mappings})
	if err != nil {
		t.Fatal(err)
	}
	mappingBytes = append(mappingBytes, '\n')
	predecessors := []referencegrouping.PredecessorRecord{
		{Role: "OCCURRENCE_LEDGER", Selector: "ledgers/occurrences.json", Schema: "lsp-trace.private.program-c-occurrence-ledger.v1", Bytes: ledger},
		{Role: "METHOD_QUERY", Selector: "methods/query.json", Schema: "lsp-trace.private.program-c-method-query.v0", Bytes: query},
		{Role: "METHOD_EVIDENCE", Selector: "methods/evidence.json", Schema: programcadmission.MethodEvidenceFamily, Bytes: projection.MethodCanonicalBytes()},
		{Role: "METHOD_MAPPING_RECEIPT", Selector: "methods/mapping-receipt.json", Schema: "lsp-trace.private.program-c-method-mapping-receipt.v1", Bytes: mappingBytes},
	}
	custody := make([]referencegrouping.EndpointCustody, 0, len(custodyByEndpoint))
	for endpoint := range custodyByEndpoint {
		custody = append(custody, referencegrouping.EndpointCustody{Endpoint: endpoint, PredecessorSelector: "methods/evidence.json", Custody: "CALLER_ASSERTED"})
	}
	req := referencegrouping.InputRequest{TypedInput: typed, Predecessors: predecessors, OccurrenceLedgerBytes: ledger, EndpointCustody: custody, OccurrenceAdmissionPolicyID: "test-only-occurrence-admission-v1", CompositionPolicyID: "test-only-composition-v1", PrivacyPolicy: "test-only-private-v1", EligibleKinds: []programcadmission.RelationKind{programcadmission.TypedCalls, programcadmission.TypedReferencesSymbol, programcadmission.TypedResolvesToDefinition}, Limits: referencegrouping.ResourceLimits{MaxArtifacts: 8, MaxNodes: 16, MaxOccurrences: 16, MaxContributions: 16, MaxCanonicalBytes: 1 << 20, MaxStringBytes: 4096}, CandidateOutcomes: outcomes, Accounting: referencegrouping.Accounting{Candidates: len(rows), Included: len(rows), Terminal: true}}
	artifact, err := referencegrouping.BuildInput(req)
	if err != nil {
		t.Fatal(err)
	}
	input, err := referencegrouping.ParseAndAdmit(artifact.CanonicalBytes(), predecessors, referencegrouping.ParseLimits{MaxRawBytes: 1 << 20, MaxArtifacts: 8, MaxNodes: 16, MaxOccurrences: 16, MaxContributions: 16, MaxCanonicalBytes: 1 << 20, MaxStringBytes: 4096})
	if err != nil || input.ByteLength() == 0 || !input.MatchesTypedInput(typed) {
		t.Fatalf("BLOCKED_NOT_RED genuine admitted reference grouping input err=%v input=%+v", err, input)
	}
	return typed, input
}

func TestRealComputeAndRecomputeRefuseActionBearingPolicyBeforeProjectionAndKernel(t *testing.T) {
	referencesOnly, input := testStage5ReferenceGroupingInput(t)
	policy := arbitraryPreStage5Policy(t, "test-only-action-bearing-policy")
	var qualification referencegrouping.VerifiedQualification

	if got, ok := newQualifiedGroupingAdmission(referencesOnly, policy, input, qualification, 7); ok || !reflect.DeepEqual(got, qualifiedGroupingAdmission{}) {
		t.Fatalf("ASSERT_REAL_ADMISSION_ZERO_UNVERIFIED_REFUSED got=%+v ok=%v", got, ok)
	}
	projectionCalls, kernelCalls := 0, 0
	got, failure := computeTypedQualifiedWithProjection(referencesOnly, 61, policy, input, qualification, 7,
		func(programcadmission.TypedInput) (typedProjectionInput, typedSelection, error) {
			projectionCalls++
			panic("projection invoked before qualification")
		},
		func(typedProjectionInput, typedSelection, uint64) (Outcome, *Failure) {
			kernelCalls++
			panic("kernel invoked before qualification")
		})
	assertStage4Unqualified(t, got, failure, projectionCalls, kernelCalls)

	carrier := internallyFormedTypedCarrier(t, referencesOnly, 61)
	carrier.groupingAdmission = qualifiedGroupingAdmission{policy: policy, input: input, qualification: qualification, epoch: 7}
	carrier.seal = typedComputationSeal(carrier)
	projectionCalls, kernelCalls = 0, 0
	got, failure = recomputeTypedQualifiedWithProjection(carrier,
		func(programcadmission.TypedInput) (typedProjectionInput, typedSelection, error) {
			projectionCalls++
			panic("projection invoked before qualification")
		},
		func(typedProjectionInput, typedSelection, uint64) (Outcome, *Failure) {
			kernelCalls++
			panic("kernel invoked before qualification")
		})
	assertStage4Unqualified(t, got, failure, projectionCalls, kernelCalls)

	genericType := reflect.TypeOf(stage4TestAdmission{})
	for _, forbidden := range []string{"policy", "rules", "actions", "weight", "cap", "normalization", "layer"} {
		if _, ok := genericType.FieldByName(forbidden); ok {
			t.Fatalf("ASSERT_GENERIC_TEST_GATE_CANNOT_RECEIVE_OR_INTERPRET_POLICY_ACTIONS field=%q", forbidden)
		}
	}
	productionAdmissionType := reflect.TypeOf(qualifiedGroupingAdmission{})
	for _, marker := range []string{"appliedPolicy", "appliedRules", "policyApplication"} {
		if _, ok := productionAdmissionType.FieldByName(marker); ok {
			t.Fatalf("ASSERT_PRODUCTION_POLICY_APPLICATION_ENTRY_POINT_ABSENT_OR_DEFERRED marker=%q", marker)
		}
	}
}

func TestRealRecomputeIntegrityPrecedesQualificationRefusal(t *testing.T) {
	got, failure := recomputeTypedQualified(TypedComputation{})
	if failure == nil || failure.Code != FailureCode("TYPED_INPUT_INTEGRITY") || failure.Message != "TYPED_INPUT_INTEGRITY" || !reflect.DeepEqual(got, TypedComputation{}) {
		t.Fatalf("ASSERT_REAL_RECOMPUTE_INTEGRITY_PRECEDENCE got=%+v failure=%+v", got, failure)
	}
	// No production QUALIFIED carrier is fabricated here. Stale qualification
	// refusal becomes constructible only through the independent verifier.
}

func TestStage4ComputeAndRecomputeEveryGroupingBindingFieldFailsBeforeProjectionAndKernel(t *testing.T) {
	fixture := typedComputeFixture(t, false)
	base := stage4QualifiedAdmission(t, fixture.referencesOnly, 7)
	mutatedBytes := func(in []byte) []byte {
		out := append([]byte(nil), in...)
		out[0] ^= 0xff
		return out
	}
	for _, tc := range []struct {
		name   string
		mutate func(*stage4TestAdmission)
		epoch  uint64
	}{
		{"stale-validity-epoch", func(*stage4TestAdmission) {}, 10},
		{"policy-bytes", func(x *stage4TestAdmission) { x.policyBytes = mutatedBytes(x.policyBytes) }, 7},
		{"policy-id", func(x *stage4TestAdmission) { x.policyID = "sha256:" + fmt.Sprintf("%064x", 1) }, 7},
		{"policy-byte-length", func(x *stage4TestAdmission) { x.policyByteLength++ }, 7},
		{"policy-digest", func(x *stage4TestAdmission) { x.policyDigest = "sha256:" + fmt.Sprintf("%064x", 2) }, 7},
		{"reference-grouping-input-bytes", func(x *stage4TestAdmission) { x.inputBytes = mutatedBytes(x.inputBytes) }, 7},
		{"reference-grouping-input-id", func(x *stage4TestAdmission) { x.inputID = "sha256:" + fmt.Sprintf("%064x", 3) }, 7},
		{"reference-grouping-input-byte-length", func(x *stage4TestAdmission) { x.inputByteLength++ }, 7},
		{"reference-grouping-input-digest", func(x *stage4TestAdmission) { x.inputDigest = "sha256:" + fmt.Sprintf("%064x", 4) }, 7},
	} {
		t.Run(tc.name, func(t *testing.T) {
			admission := base
			tc.mutate(&admission)
			t.Run("compute", func(t *testing.T) {
				projectionCalls, kernelCalls := 0, 0
				got, failure := computeTypedWithGroupingAdmissionAndProjection(fixture.referencesOnly, 61, admission, stage4AdmissionVerifier(base, tc.epoch),
					func(programcadmission.TypedInput) (typedProjectionInput, typedSelection, error) {
						projectionCalls++
						panic("projection invoked before grouping qualification")
					},
					func(typedProjectionInput, typedSelection, uint64) (Outcome, *Failure) {
						kernelCalls++
						panic("kernel invoked before grouping qualification")
					})
				assertStage4Unqualified(t, got, failure, projectionCalls, kernelCalls)
			})
			t.Run("recompute", func(t *testing.T) {
				projectionCalls, kernelCalls := 0, 0
				got, failure := recomputeTypedWithGroupingAdmissionAndProjection(internallyFormedTypedCarrier(t, fixture.referencesOnly, 61), admission, stage4AdmissionVerifier(base, tc.epoch),
					func(programcadmission.TypedInput) (typedProjectionInput, typedSelection, error) {
						projectionCalls++
						panic("projection invoked before grouping qualification")
					},
					func(typedProjectionInput, typedSelection, uint64) (Outcome, *Failure) {
						kernelCalls++
						panic("kernel invoked before grouping qualification")
					})
				assertStage4Unqualified(t, got, failure, projectionCalls, kernelCalls)
			})
		})
	}
}
