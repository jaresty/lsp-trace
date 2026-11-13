package referencegrouping

import (
	"bytes"
	"testing"

	"lsp-trace/internal/programcadmission"
	"lsp-trace/internal/programccompose"
)

// Ordinary empty CALLS carriers must fully readmit without being relabeled native.
// referenceCapture serializes supplied graphs in memory with managed-lsp-v1
// semantics. It neither creates native source receipts nor grants native custody.
// Deliberately invalid native bytes below must not pass final readmission.
func TestEmptyCallsPredecessorsPure(t *testing.T) {
	for _, mixed := range []bool{false, true} {
		name := "C-empty-ordinary"
		if mixed {
			name = "CR-C-empty-ordinary"
		}
		t.Run(name, func(t *testing.T) {
			f := emptyCallsPredecessorsFixture(t, mixed)
			req := f.request()
			projection, err := programcadmission.PreparePrivateLeidenProjection(req.TypedInput)
			if err != nil {
				t.Fatal("ASSERT_EMPTY_PREDECESSORS_SEALED_PROJECTION", err)
			}
			callsSelected := false
			for _, kind := range projection.SelectedKinds() {
				callsSelected = callsSelected || kind == programcadmission.TypedCalls
			}
			for _, occurrence := range projection.Occurrences() {
				if occurrence.Kind == programcadmission.TypedCalls {
					t.Fatal("ASSERT_EMPTY_PREDECESSORS_ZERO_CALLS")
				}
			}
			if !callsSelected || len(projection.RetainedCallsCanonicalBytes()) == 0 || req.TypedInput.Authority != 0 || req.TypedInput.SourceGraphComplete != "UNKNOWN" {
				t.Fatal("ASSERT_EMPTY_PREDECESSORS_EMPTY_NOT_ABSENT")
			}
			readmitted, err := programcadmission.Admit(projection.RetainedCallsCanonicalBytes())
			if err != nil || !readmitted.Admission.Valid() || len(readmitted.Admission.Occurrences()) != 0 {
				t.Fatal("ASSERT_EMPTY_PREDECESSORS_REAL_COMPOSITE_READMISSION", err)
			}
			readmittedTyped, err := programcadmission.PrepareTypedInput(programcadmission.TypedInputRequest{Calls: readmitted.Admission, Kinds: []programcadmission.RelationKind{programcadmission.TypedCalls}})
			if err != nil {
				t.Fatal("ASSERT_EMPTY_PREDECESSORS_READMITTED_TYPED", err)
			}
			readmittedProjection, err := programcadmission.PreparePrivateLeidenProjection(readmittedTyped)
			if err != nil || !bytes.Equal(readmittedProjection.RetainedCallsCanonicalBytes(), projection.RetainedCallsCanonicalBytes()) {
				t.Fatal("ASSERT_EMPTY_PREDECESSORS_READMITTED_CANONICAL", err)
			}
			if projection.CompositeSourceBinding().Compatibility.AcquisitionSemantics != "managed-lsp-v1" {
				t.Fatal("ASSERT_EMPTY_PREDECESSORS_ORDINARY_NOT_NATIVE")
			}
			ledger, err := canonicalOccurrenceLedger(projection.Occurrences())
			if err != nil || !bytes.Equal(ledger, req.OccurrenceLedgerBytes) {
				t.Fatal("ASSERT_EMPTY_PREDECESSORS_CANONICAL_LEDGER", err)
			}
			if err := validateBoundPredecessors(req.Predecessors, projection.MethodEvidence(), projection.MethodCanonicalBytes(), projection.CompositeSourceBinding(), projection.RetainedCallsCanonicalBytes()); err != nil {
				t.Fatal("ASSERT_EMPTY_PREDECESSORS_BOUND_VALIDATORS_PASSED", err)
			}
			if err := validateRequest(req, projection.Occurrences()); err != nil {
				t.Fatal("ASSERT_EMPTY_PREDECESSORS_REQUEST_VALIDATORS_PASSED", err)
			}
			artifact, err := BuildInput(req)
			if err != nil {
				t.Fatal("ASSERT_EMPTY_PREDECESSORS_BUILD_EXPECTED_SUCCESS", err)
			}
			admitted, err := ParseAndAdmit(artifact.CanonicalBytes(), req.Predecessors, generousParseLimits())
			if err != nil || !admitted.MatchesTypedInput(req.TypedInput) || !bytes.Equal(admitted.CanonicalBytes(), artifact.CanonicalBytes()) || admitted.Artifact().Authority() != 0 || admitted.Artifact().Accepted() || admitted.Artifact().Completeness() != "UNKNOWN" {
				t.Fatal("ASSERT_EMPTY_PREDECESSORS_FULL_READMISSION", err)
			}
			t.Log("ORDINARY_EMPTY_BUILD_PARSE_ADMIT_PASSED; AUTHORITY0_ACCEPTED_FALSE_COMPLETENESS_UNKNOWN")

			// Both byte strings are intentionally invalid, never admitted native
			// evidence. In particular UNAVAILABLE is not OBSERVED_EMPTY.
			for _, invalid := range [][]byte{[]byte("{}\n"), []byte("{\"Status\":\"UNAVAILABLE\"}\n")} {
				if ReplayPrivateNativeEvidenceV2(invalid, projection.RetainedCallsCanonicalBytes()) == nil {
					t.Fatal("ASSERT_EMPTY_PREDECESSORS_INVALID_NATIVE_CONTROL")
				}
				negative := f.request()
				negative.Predecessors = append(negative.Predecessors, PredecessorRecord{Role: "NATIVE_EVIDENCE", Selector: "calls/native-evidence.json", Schema: PrivateNativeEvidenceSchemaV2, Bytes: invalid})
				artifact, err := BuildInput(negative)
				if err != nil {
					t.Fatal("ASSERT_EMPTY_PREDECESSORS_ORDINARY_WITH_PROOF_BUILD", err)
				}
				if _, err := ParseAndAdmit(artifact.CanonicalBytes(), negative.Predecessors, generousParseLimits()); err == nil || err.Error() != "native proof path mismatch" {
					t.Fatal("ASSERT_EMPTY_PREDECESSORS_ORDINARY_WITH_PROOF_NOT_ADMITTED", err)
				}
			}
			t.Log("ORDINARY_EMPTY_INVALID_NATIVE_CONTROLS_REFUSED_AT_READMISSION")

			for _, mutation := range []struct{ role, want string }{
				{"RETAINED_COMPOSITE", "retained composite bytes mismatch"},
				{"ACQUISITION_RECEIPT", "acquisition source binding mismatch"},
				{"OCCURRENCE_LEDGER", "invalid occurrence ledger"},
			} {
				bad := f.request()
				for i := range bad.Predecessors {
					if bad.Predecessors[i].Role == mutation.role {
						bad.Predecessors[i].Bytes = append(bad.Predecessors[i].Bytes, ' ')
					}
				}
				emptyCallsPredecessorsWantError(t, bad, mutation.want)
			}
			tampered := f.request()
			tampered.TypedInput.Authority = 1
			if _, err := BuildInput(tampered); err == nil {
				t.Fatal("ASSERT_EMPTY_PREDECESSORS_TAMPERED_SEAL_REFUSED")
			}
		})
	}

	t.Run("C-absent-requested-refused", func(t *testing.T) {
		_, err := programcadmission.PrepareTypedInput(programcadmission.TypedInputRequest{Kinds: []programcadmission.RelationKind{programcadmission.TypedCalls}})
		if err == nil || err.Error() != "validated CALLS composite admission required" {
			t.Fatal("ASSERT_EMPTY_PREDECESSORS_ABSENT_NOT_EMPTY", err)
		}
	})
	t.Run("C-unrequested-methods-roundtrip", func(t *testing.T) {
		f := referenceFixture(t, []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol})
		projection, err := programcadmission.PreparePrivateLeidenProjection(f.typed)
		if err != nil || len(projection.RetainedCallsCanonicalBytes()) != 0 {
			t.Fatal("ASSERT_EMPTY_PREDECESSORS_UNREQUESTED", err)
		}
		for _, kind := range projection.SelectedKinds() {
			if kind == programcadmission.TypedCalls {
				t.Fatal("ASSERT_EMPTY_PREDECESSORS_UNREQUESTED_SELECTED")
			}
		}
		req := f.request()
		artifact, err := BuildInput(req)
		if err != nil {
			t.Fatal("ASSERT_EMPTY_PREDECESSORS_UNREQUESTED_BUILD", err)
		}
		admitted, err := ParseAndAdmit(artifact.CanonicalBytes(), req.Predecessors, generousParseLimits())
		if err != nil || !admitted.MatchesTypedInput(req.TypedInput) || !bytes.Equal(admitted.CanonicalBytes(), artifact.CanonicalBytes()) || admitted.Artifact().Authority() != 0 || admitted.Artifact().Accepted() || admitted.Artifact().Completeness() != "UNKNOWN" {
			t.Fatal("ASSERT_EMPTY_PREDECESSORS_UNREQUESTED_FULL_READMISSION", err)
		}
		t.Log("UNREQUESTED_C; REAL_BUILD_PARSE_ADMIT_ROUNDTRIP; AUTHORITY0_ACCEPTED_FALSE_COMPLETENESS_UNKNOWN")
	})
}

func emptyCallsPredecessorsWantError(t *testing.T, req InputRequest, want string) {
	t.Helper()
	if _, err := BuildInput(req); err == nil || err.Error() != want {
		t.Fatalf("ASSERT_EMPTY_PREDECESSORS_CURRENT_BOUNDARY got=%v want=%q", err, want)
	}
}

func emptyCallsPredecessorsFixture(t *testing.T, mixed bool) referenceTestFixture {
	t.Helper()
	left, right := referenceCapture(t, nil, nil), referenceCapture(t, nil, nil)
	left.Identity, right.Identity = "empty-calls-left", "empty-calls-right"
	composite, err := programccompose.Compose([]programccompose.Input{left, right})
	if err != nil {
		t.Fatal("ASSERT_EMPTY_PREDECESSORS_COMPOSE", err)
	}
	calls, err := programcadmission.Admit(composite.Bytes)
	if err != nil {
		t.Fatal("ASSERT_EMPTY_PREDECESSORS_ADMIT", err)
	}
	f := referenceTestFixture{}
	typedReq := programcadmission.TypedInputRequest{Calls: calls.Admission, Kinds: []programcadmission.RelationKind{programcadmission.TypedCalls}}
	if mixed {
		f = referenceFixture(t, []programcadmission.RelationKind{programcadmission.TypedReferencesSymbol})
		projection, err := programcadmission.PreparePrivateLeidenProjection(f.typed)
		if err != nil {
			t.Fatal(err)
		}
		typedReq.Methods, err = admitCanonicalMethodEvidence(projection.MethodCanonicalBytes())
		if err != nil {
			t.Fatal(err)
		}
		typedReq.EnableMethods = true
		typedReq.Kinds = append(typedReq.Kinds, programcadmission.TypedReferencesSymbol)
	}
	f.typed, err = programcadmission.PrepareTypedInput(typedReq)
	if err != nil {
		t.Fatal("ASSERT_EMPTY_PREDECESSORS_PREPARE_TYPED", err)
	}
	f.ledger = canonicalOccurrenceLedgerLiteral(t, f.typed.Occurrences)
	if !mixed {
		f.predecessors = []PredecessorRecord{{Role: "OCCURRENCE_LEDGER", Selector: "ledgers/occurrences.json", Schema: "lsp-trace.private.program-c-occurrence-ledger.v1", Bytes: f.ledger}}
	} else {
		for i := range f.predecessors {
			if f.predecessors[i].Role == "OCCURRENCE_LEDGER" {
				f.predecessors[i].Bytes = f.ledger
			}
		}
	}
	projection, err := programcadmission.PreparePrivateLeidenProjection(f.typed)
	if err != nil {
		t.Fatal(err)
	}
	f.predecessors = append(f.predecessors,
		PredecessorRecord{Role: "RETAINED_COMPOSITE", Selector: "calls/composite.json", Schema: "lsp-trace.private.program-c-composite.v1", Bytes: projection.RetainedCallsCanonicalBytes()},
		PredecessorRecord{Role: "ACQUISITION_RECEIPT", Selector: "calls/acquisition.json", Schema: "lsp-trace.private.program-c-acquisition-receipt.v1", Bytes: canonicalCompositeSourceBindingLiteral(t, projection.CompositeSourceBinding())},
	)
	return f
}
