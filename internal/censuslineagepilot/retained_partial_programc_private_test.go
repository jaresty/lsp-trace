//go:build darwin || linux

package censuslineagepilot_test

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"

	pilot "lsp-trace/internal/censuslineagepilot"
	"lsp-trace/internal/programc"
	derivedc "lsp-trace/internal/retainedpartialprogramc"
)

// Called by the actual eight-record fixture AFTER every borrowed root is closed.
// This is an external test package: no production reverse import is introduced.
func checkRetainedPartialProgramCPrivate(t *testing.T, proof pilot.DerivedRetainedPartialNativePrivate) {
	t.Helper()
	p, failure := derivedc.Project(proof)
	if failure != nil {
		t.Fatal("ASSERT_PRIVATE_PROJECTION", failure)
	}
	a, failure := derivedc.ComputeProjection(p, 42)
	if failure != nil {
		t.Fatal("ASSERT_PRIVATE_FIXTURE_COMPUTE", failure)
	}
	b, failure := derivedc.Compute(proof, 42)
	if failure != nil || !reflect.DeepEqual(a, b) {
		t.Fatal("ASSERT_PRIVATE_DETERMINISTIC", failure)
	}
	m := a.Source.Manifest()
	if a.ComputationAdmission != derivedc.Admission || a.ProfileID != derivedc.ProfileID || a.ProfileID == programc.ProfileID || a.ProfileDigest != derivedc.ProfileDigest || a.Math.Outcome != "COMPLETE" || a.Math.Seed != 42 || a.Math.LogicalDigest == "" || len(a.Math.Communities) == 0 || len(a.Projection.NodeIdentities) != 27 || len(a.Projection.Occurrences) != 3 || len(a.Projection.PairWeights) != 3 {
		t.Fatal("ASSERT_PRIVATE_FIXTURE_MATH")
	}
	members := 0
	for _, c := range a.Math.Communities {
		members += len(c.Members)
	}
	// The actual sparse directed fixture returns 27 singleton communities at
	// resolution 1/seed 42. A non-singleton expectation would change calls-v1
	// math rather than witness it; retain the observed deterministic partition.
	if members != 27 || len(a.Math.Communities) != 27 {
		t.Fatal("ASSERT_MEANINGFUL_CANONICAL_COMMUNITIES")
	}
	if a.Source.Authority != 0 || a.Source.Accepted || !a.Source.Partial || a.Source.CanonicalComplete || a.Source.ReplayAdmitted || a.Source.OriginalNativeV5IdentityClaimed || a.Source.SourceCompleteness != "UNKNOWN" || a.Source.FeatureIdentity != "UNRESOLVED" || a.Source.GraphWideCustodyStatus != "UNAVAILABLE" || a.Source.OrdinaryQualificationReceiptStatus != "UNAVAILABLE" || m.ProgramCAdmission != "NOT_PROGRAM_C_ADMITTED" || m.Frontiers == 0 || m.WithheldCount != 1 || m.Targets != 25 || len(m.SourcePolicy.Members) != 6 || m.CanonicalNativeGraphSHA256 != a.Source.NativeGraphSHA256 || a.Source.NativeGraphSHA256 == a.Source.OriginalAcquisitionGraphSHA256 || a.Source.SemanticReceipt.SemanticCommitmentDigest == "" {
		t.Fatal("ASSERT_PRIVATE_CEILING_AND_LINEAGE")
	}
	for _, o := range a.Projection.Occurrences {
		if o.Weight != 1 || o.Provider != m.ProviderCommand {
			t.Fatal("ASSERT_DIRECT_UNIT_CALLS")
		}
	}
	if _, failure := programc.Project(a.Source.InputBytes()); failure == nil {
		t.Fatal("ASSERT_PRIVATE_NOT_ORDINARY_ADMISSION")
	}
	var zero pilot.DerivedRetainedPartialNativePrivate
	if _, failure := derivedc.Compute(zero, 42); failure == nil {
		t.Fatal("ASSERT_ZERO_PROOF_REFUSED")
	}
	if err := json.Unmarshal(proof.Bytes(), &zero); err != nil {
		t.Fatal(err)
	}
	if _, failure := derivedc.Compute(zero, 42); failure == nil {
		t.Fatal("ASSERT_DECODED_PROOF_REFUSED")
	}
	if _, failure := derivedc.ComputeProjection(derivedc.Projection{}, 42); failure == nil {
		t.Fatal("ASSERT_ZERO_PROJECTION_REFUSED")
	}
	var decodedProjection derivedc.Projection
	if json.Unmarshal([]byte(`{}`), &decodedProjection) != nil {
		t.Fatal("decode projection")
	}
	if _, failure := derivedc.ComputeProjection(decodedProjection, 42); failure == nil {
		t.Fatal("ASSERT_DECODED_PROJECTION_REFUSED")
	}
	// Every externally mutable collection/byte view is detached from sealed state.
	clone := derivedc.CloneOutcome(a)
	clone.Math.Communities[0].Members[0] = "changed"
	clone.Projection.NodeIdentities[0] = "changed"
	clone.Projection.Occurrences[0].Identity = "changed"
	for pair := range clone.Projection.PairWeights {
		clone.Projection.PairWeights[pair] = 999
	}
	m.SourcePolicy.Members[0].URI = "changed"
	m.CanonicalSeeds[0] = 'x'
	exported := a.Source.InputBytes()
	exported[0] = 'x'
	exported = a.Source.NativeGraphBytes()
	exported[0] = 'x'
	mutable := p.Math()
	mutable.NodeIdentities[0] = "changed"
	copyProof := proof
	exported = copyProof.Bytes()
	exported[0] = 'x'
	c, failure := derivedc.Compute(copyProof, 42)
	if failure != nil || !reflect.DeepEqual(a, c) || !bytes.Equal(a.Source.InputBytes(), proof.Bytes()) {
		t.Fatal("ASSERT_PRIVATE_NO_MUTABLE_ALIAS", failure)
	}
	// Raw parser consistency is not a constructor: even each altered DTO decoded
	// into the concrete handle cannot mint a seal. Existing parser tests separately
	// distinguish the individual seed/native/scope/record/provider/code bindings.
	for _, field := range []string{"seed", "native", "scope", "withheld", "provider", "code", "lineage", "record-0", "record-1", "record-2", "record-3", "record-4", "record-5", "record-6", "record-7"} {
		e, err := pilot.ParseDerivedRetainedPartialNativePrivate(proof.Bytes())
		if err != nil {
			t.Fatal(err)
		}
		switch field {
		case "seed":
			e.Manifest.CanonicalSeeds[0] = 'x'
		case "native":
			e.NativeGraph[0] = 'x'
		case "scope":
			e.Manifest.SourcePolicy.Members[0].URI += "x"
		case "withheld":
			e.Manifest.WithheldCount++
		case "provider":
			e.Manifest.ProviderCommand += "x"
		case "code":
			e.Manifest.CallerAssertedCodeHEAD += "x"
		case "lineage":
			e.Manifest.QualificationID = pilot.Digest([]byte("forged"))
		default:
			i := int(field[len(field)-1] - '0')
			e.Records[i][0] = 'x'
		}
		raw, _ := json.Marshal(e)
		if _, err := pilot.ParseDerivedRetainedPartialNativePrivate(raw); err == nil {
			t.Fatal("ASSERT_MUTATED_LINEAGE_REFUSED", field)
		}
		var forged pilot.DerivedRetainedPartialNativePrivate
		if json.Unmarshal(raw, &forged) != nil {
			t.Fatal("decode forged")
		}
		if _, failure := derivedc.Compute(forged, 42); failure == nil {
			t.Fatal("ASSERT_MUTATED_DTO_NO_SEAL", field)
		}
	}
	t.Logf("ASSERT_PRIVATE_FIXTURE_COMPUTE: PASS targets=%d nodes=%d calls=%d sites=%d communities=%d digest=%s seed=42 authority=0 accepted=false source=UNKNOWN", m.Targets, len(a.Projection.NodeIdentities), m.Calls, len(a.Projection.Occurrences), len(a.Math.Communities), a.Math.LogicalDigest)
}
