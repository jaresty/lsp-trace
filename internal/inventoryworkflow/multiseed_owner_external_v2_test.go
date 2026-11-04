package inventoryworkflow_test

import (
	"context"
	"errors"
	"lsp-trace/internal/censuscontinuation"
	"lsp-trace/internal/inventorysuccessor"
	"lsp-trace/internal/inventoryworkflow"
	"lsp-trace/internal/programcrepresentative"
	"reflect"
	"testing"
)

// This external caller can implement every old evidence callback. The new
// issuer's resolver parameter cannot transfer any of these reconstruction hooks.
type falseEmptyExternalAdapterV2 struct {
	programcrepresentative.QualificationEvidenceSourceV2
}

func (s falseEmptyExternalAdapterV2) ReconstructQualifiedMechanicsV2(context.Context, programcrepresentative.QualificationSubjectV2) ([]programcrepresentative.MechanicsV2, []programcrepresentative.CustodyIdentityDTO, error) {
	return nil, nil, nil
}
func (s falseEmptyExternalAdapterV2) ReconstructCheckpointConstituentsV2(ctx context.Context, d programcrepresentative.QualificationSubjectV2) ([]programcrepresentative.ConstituentV2, map[programcrepresentative.ArtifactBindingDTO][]byte, error) {
	cs, _, err := s.QualificationEvidenceSourceV2.ReconstructCheckpointConstituentsV2(ctx, d)
	if err != nil {
		return nil, nil, err
	}
	for i := range cs {
		cs[i].Classification = "EMPTY"
		cs[i].ProgramCOutcome = "EMPTY"
		cs[i].Disposition = ""
		for j := range cs[i].Runs {
			cs[i].Runs[j].Outcome = "EMPTY"
			cs[i].Runs[j].CommunityArtifact = programcrepresentative.ArtifactBindingDTO{}
			cs[i].Runs[j].CommunityCount = 0
		}
	}
	return cs, map[programcrepresentative.ArtifactBindingDTO][]byte{}, nil
}
func TestMultiSeedExternalOwnerBoundaryV2(t *testing.T) {
	ctx := context.Background()
	t.Run("A_EXTERNAL_UNREGISTERED_PROVENANCE", func(t *testing.T) {
		f := inventoryworkflow.OwnerBoundaryFixtureForTestsV2(t, true)
		if _, err := programcrepresentative.CheckQualificationCalculationV2(ctx, f.Expected, f.Envelope, f.Source); err != nil {
			t.Fatal(err)
		}
		before := f.Snapshot()
		q, err := f.Evidence.VerifyQualificationEnvelopeV2(ctx, f.Expected, f.Envelope, f.Source)
		if !errors.Is(err, censuscontinuation.ErrQualificationProvenanceV2) {
			t.Fatalf("A_EXTERNAL_UNREGISTERED_PROVENANCE: expected dedicated independent registration refusal, got %v", err)
		}
		if _, err = q.SubjectBindingV2(); err == nil {
			t.Fatal("A_EXTERNAL_UNREGISTERED_PROVENANCE: authority escaped")
		}
		if !reflect.DeepEqual(before, f.Snapshot()) {
			t.Fatal("A_EXTERNAL_ZERO_STORAGE_EFFECTS")
		}
		t.Log("A_EXTERNAL_UNREGISTERED_PROVENANCE PASS")
	})
	f := inventoryworkflow.OwnerBoundaryFixtureForTestsV2(t, false)
	t.Run("EXTERNAL_ZERO_CAPABILITY_BEFORE_STORAGE", func(t *testing.T) {
		before := f.Snapshot()
		op, err := inventorysuccessor.NewMultiSeedCheckpointOperationV2(nil, inventorysuccessor.MultiSeedDenominatorOperationV2, "workflow", "head", f.Identity, censuscontinuation.VerifiedQualificationV2{})
		if !errors.Is(err, censuscontinuation.ErrQualificationUnavailableV2) || op != nil {
			t.Fatal("B_EXTERNAL_ZERO_CAPABILITY_BEFORE_OPEN")
		}
		if !reflect.DeepEqual(before, f.Snapshot()) {
			t.Fatal("B_EXTERNAL_ZERO_STORAGE_EFFECTS")
		}
	})
	name := "ExternalSyntheticReviewProvenanceRefusal"
	if f.Legacy {
		name = "B_EXTERNAL_GENUINE_REVIEW_FALSE_EMPTY"
	}
	t.Run(name, func(t *testing.T) {
		bad := falseEmptyExternalAdapterV2{f.Source}
		calc, err := programcrepresentative.CheckQualificationCalculationV2(ctx, f.Expected, f.Envelope, bad)
		if err != nil {
			t.Fatal(err)
		}
		d, err := programcrepresentative.BuildDenominatorArtifactV2(calc)
		if err != nil {
			t.Fatal(err)
		}
		a, err := d.AccountingV2()
		if err != nil || a.E != 4 || a.D != 0 || a.G != 0 {
			t.Fatalf("B_EXTERNAL_FALSE_EMPTY_SETUP: %+v %v", a, err)
		}
		if _, ok := any(calc).(censuscontinuation.VerifiedQualificationV2); ok {
			t.Fatal("B_EXTERNAL_CALCULATION_NOT_AUTHORITY")
		}
		before := f.Snapshot()
		q, err := f.Evidence.VerifyQualificationEnvelopeV2(ctx, f.Expected, f.Envelope, bad)
		if !f.Legacy {
			if !errors.Is(err, censuscontinuation.ErrQualificationProvenanceV2) {
				t.Fatalf("EXTERNAL_SYNTHETIC_EXACT_PROVENANCE_REFUSAL: %v", err)
			}
			if _, capabilityErr := q.SubjectBindingV2(); !errors.Is(capabilityErr, censuscontinuation.ErrQualificationUnavailableV2) {
				t.Fatalf("EXTERNAL_SYNTHETIC_INVALID_CAPABILITY: %v", capabilityErr)
			}
			if !reflect.DeepEqual(before, f.Snapshot()) {
				t.Fatal("B_EXTERNAL_ZERO_STORAGE_EFFECTS")
			}
			t.Log("EXTERNAL_SYNTHETIC_EXACT_PROVENANCE_REFUSAL PASS: B reconstruction not reached")
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		d, err = q.BuildDenominatorArtifactV2()
		if err != nil {
			t.Fatal(err)
		}
		a, err = d.AccountingV2()
		if err != nil || a.C != 2 || a.E != 1 || a.R != 1 || a.D != 24 || a.G != 48 {
			t.Fatalf("B_EXTERNAL_GENUINE_REVIEW_FALSE_EMPTY: owner did not reconstruct COMPLETE %+v %v", a, err)
		}
		if !reflect.DeepEqual(before, f.Snapshot()) {
			t.Fatal("B_EXTERNAL_ZERO_STORAGE_EFFECTS")
		}
		t.Log("B_EXTERNAL_GENUINE_REVIEW_FALSE_EMPTY PASS")
	})
}
