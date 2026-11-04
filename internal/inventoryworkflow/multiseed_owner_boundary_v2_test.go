package inventoryworkflow

import (
	"context"
	"errors"
	"lsp-trace/internal/censuscontinuation"
	"lsp-trace/internal/inventorysuccessor"
	"lsp-trace/internal/programcrepresentative"
	"lsp-trace/internal/programctransitionhost"
	"path/filepath"
	"reflect"
	"testing"
)

type ownerBoundarySourceV2 struct {
	censuscontinuation.ProgramCMultiSeedQualificationEvidenceV2
	objects    *inventorysuccessor.MultiSeedObjectsV2
	harness    controlledCheckpointHarnessV2
	falseEmpty bool
}

func (s ownerBoundarySourceV2) ResolveQualificationArtifactV2(ctx context.Context, b programcrepresentative.ArtifactBindingDTO) ([]byte, error) {
	return s.objects.Resolve(ctx, b)
}
func (s ownerBoundarySourceV2) RecordedReviewActorV2(ctx context.Context, k, id string) (programcrepresentative.RecordedReviewActorV2, error) {
	return s.harness.recordedReviewActorV2(ctx, k, id)
}
func (s ownerBoundarySourceV2) ReconstructQualifiedMechanicsV2(ctx context.Context, d programcrepresentative.QualificationSubjectV2) ([]programcrepresentative.MechanicsV2, []programcrepresentative.CustodyIdentityDTO, error) {
	if s.falseEmpty {
		return nil, nil, nil
	}
	return s.ProgramCMultiSeedQualificationEvidenceV2.ReconstructQualifiedMechanicsV2(ctx, d)
}
func (s ownerBoundarySourceV2) ReconstructCheckpointConstituentsV2(ctx context.Context, d programcrepresentative.QualificationSubjectV2) ([]programcrepresentative.ConstituentV2, map[programcrepresentative.ArtifactBindingDTO][]byte, error) {
	cs, extra, err := s.ProgramCMultiSeedQualificationEvidenceV2.ReconstructCheckpointConstituentsV2(ctx, d)
	if err != nil {
		return nil, nil, err
	}
	if s.falseEmpty {
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
		extra = map[programcrepresentative.ArtifactBindingDTO][]byte{}
	}
	return cs, extra, nil
}
func ownerBoundaryFixtureV2(t *testing.T) (Request, string, ownerBoundarySourceV2, programcrepresentative.QualificationSubjectExpectedContextV2, programcrepresentative.ProgramCRepresentativeQualificationSubjectV2, []byte) {
	t.Helper()
	ctx := context.Background()
	request, stateDir := multiSeedRetainedCheckpointFixtureV2(t)
	reader := controlledCheckpointImplementationReaderV2{root: filepath.Clean(filepath.Join("..", ".."))}
	subject, err := BuildProgramCMultiSeedQualificationSubjectV2(ctx, request, reader, programcrepresentative.MaxArtifactCanonicalBytesV2)
	if errors.Is(err, programcrepresentative.ErrMultiSeedV2ReviewRequired) {
		t.Skip("controlled owner guard requires disposable declaration-data overlay; production remains unissued")
	}
	if err != nil {
		t.Fatal(err)
	}
	binding, harness, _ := retainControlledCheckpointReviewV2(t, request, subject)
	installControlledActorDeclarationV2(t, harness)
	root, err := request.ProgramC.OpenState()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Error(err)
		}
	})
	verification, err := programctransitionhost.NewCompletionVerificationRequest(ctx, CompletionRequest{StateRoot: root, Receipt: request.ProgramC.Receipt, Ledger: request.ProgramC.Ledger, WorkflowKey: request.ProgramC.WorkflowKey, HeadKey: request.ProgramC.HeadKey, Bindings: request.ProgramC.Bindings})
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := censuscontinuation.BuildProgramCMultiSeedQualificationEvidenceV2(ctx, verification)
	if err != nil {
		t.Fatal(err)
	}
	expected, err := evidence.QualificationSubjectExpectedContextV2()
	if err != nil {
		t.Fatal(err)
	}
	dto, err := subject.DTO()
	if err != nil {
		t.Fatal(err)
	}
	expected.WorkflowIdentity = dto.WorkflowIdentity
	expected.SourcePinSHA256 = dto.SourcePinSHA256
	expected.ImplementationSource = reader
	expected.MaxImplementationSourceBytes = programcrepresentative.MaxArtifactCanonicalBytesV2
	expected.Stage6Implementation = dto.Stage6Implementation
	objects, err := inventorysuccessor.NewMultiSeedObjectsV2(root)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := objects.Resolve(ctx, binding)
	if err != nil {
		t.Fatal(err)
	}
	return request, stateDir, ownerBoundarySourceV2{ProgramCMultiSeedQualificationEvidenceV2: evidence, objects: objects, harness: harness}, expected, subject, raw
}

// OwnerBoundaryCaseForTestsV2 exposes controlled fixture values only to the
// external test archive; no production constructor accepts these values.
type OwnerBoundaryCaseForTestsV2 struct {
	Evidence censuscontinuation.ProgramCMultiSeedQualificationEvidenceV2
	Expected programcrepresentative.QualificationSubjectExpectedContextV2
	Envelope []byte
	Source   programcrepresentative.QualificationEvidenceSourceV2
	Identity inventorysuccessor.WorkflowIdentity
	Snapshot func() map[string]string
}

func OwnerBoundaryFixtureForTestsV2(t *testing.T, forgedReview bool) OwnerBoundaryCaseForTestsV2 {
	t.Helper()
	request, dir, source, expected, subject, raw := ownerBoundaryFixtureV2(t)
	if forgedReview {
		b, h, _ := retainControlledCheckpointReviewActorsV2(t, request, subject, "external-forged-writer", "external-forged-reviewer")
		var err error
		raw, err = source.objects.Resolve(context.Background(), b)
		if err != nil {
			t.Fatal(err)
		}
		source.harness = h
	}
	return OwnerBoundaryCaseForTestsV2{Evidence: source.ProgramCMultiSeedQualificationEvidenceV2, Expected: expected, Envelope: raw, Source: source, Identity: request.Identity, Snapshot: func() map[string]string { return workflowStateSnapshot(t, dir) }}
}
func TestMultiSeedOwnerBoundaryV2(t *testing.T) {
	ctx := context.Background()
	request, stateDir, source, expected, subject, raw := ownerBoundaryFixtureV2(t)
	t.Run("A_UNREGISTERED_ACTOR_AUTHORITY_REFUSAL", func(t *testing.T) {
		// Registered genuine actors and forged actors are deliberately independent.
		binding, fakeHarness, _ := retainControlledCheckpointReviewActorsV2(t, request, subject, "forged-writer", "forged-reviewer")
		forgedRaw, err := source.objects.Resolve(ctx, binding)
		if err != nil {
			t.Fatal(err)
		}
		attacker := source
		attacker.harness = fakeHarness
		if _, err := programcrepresentative.CheckQualificationCalculationV2(ctx, expected, forgedRaw, attacker); err != nil {
			t.Fatalf("A_CANONICAL_FAKE_CLOSURE_SETUP: %v", err)
		}
		before := workflowStateSnapshot(t, stateDir)
		q, err := source.ProgramCMultiSeedQualificationEvidenceV2.VerifyQualificationEnvelopeV2(ctx, expected, forgedRaw, attacker)
		if !errors.Is(err, censuscontinuation.ErrQualificationProvenanceV2) {
			t.Fatalf("A_UNREGISTERED_ACTOR_AUTHORITY_REFUSAL: dedicated provenance refusal required, got %v", err)
		}
		if _, err := q.SubjectBindingV2(); err == nil {
			t.Fatal("A_UNREGISTERED_ACTOR_AUTHORITY_REFUSAL: authority escaped")
		}
		if !reflect.DeepEqual(before, workflowStateSnapshot(t, stateDir)) {
			t.Fatal("A_UNREGISTERED_ACTOR_AUTHORITY_REFUSAL: storage changed")
		}
		t.Log("A_UNREGISTERED_ACTOR_AUTHORITY_REFUSAL PASS: canonical fake closure refused by independent registered provenance; zero authority and writes/CAS")
	})
	t.Run("B_FALSE_EMPTY_AUTHORITY_REFUSAL", func(t *testing.T) {
		attacker := source
		attacker.falseEmpty = true
		calculation, err := programcrepresentative.CheckQualificationCalculationV2(ctx, expected, raw, attacker)
		if err != nil {
			t.Fatal(err)
		}
		d, err := programcrepresentative.BuildDenominatorArtifactV2(calculation)
		if err != nil {
			t.Fatal(err)
		}
		a, err := d.AccountingV2()
		if err != nil {
			t.Fatal(err)
		}
		if a.D != 0 || a.G != 0 || a.E != 4 {
			t.Fatal("B_FALSE_EMPTY_SETUP_NOT_GENUINE_RESEALED_EMPTY")
		}
		if _, ok := any(calculation).(censuscontinuation.VerifiedQualificationV2); ok {
			t.Fatal("B_FALSE_EMPTY_AUTHORITY_REFUSAL: calculation converted to capability")
		}
		before := workflowStateSnapshot(t, stateDir)
		// Extra attacker callbacks are ignored: the concrete owner reconstructs.
		q, err := source.ProgramCMultiSeedQualificationEvidenceV2.VerifyQualificationEnvelopeV2(ctx, expected, raw, attacker)
		if err != nil {
			t.Fatal(err)
		}
		genuine, err := q.BuildDenominatorArtifactV2()
		if err != nil {
			t.Fatal(err)
		}
		a, err = genuine.AccountingV2()
		if err != nil {
			t.Fatal(err)
		}
		if a.D != 24 || a.G != 48 || a.C != 2 || a.E != 1 || a.R != 1 {
			t.Fatalf("B_FALSE_EMPTY_AUTHORITY_REFUSAL: genuine inventory changed %+v", a)
		}
		operation, err := inventorysuccessor.NewMultiSeedCheckpointOperationV2(nil, inventorysuccessor.MultiSeedDenominatorOperationV2, request.SuccessorWorkflowKey, request.SuccessorHeadKey, request.Identity, censuscontinuation.VerifiedQualificationV2{})
		if !errors.Is(err, censuscontinuation.ErrQualificationUnavailableV2) || operation != nil {
			t.Fatal("B_ZERO_CAPABILITY_BEFORE_STORAGE_REFUSAL")
		}
		foreign := expected
		foreign.CorpusIdentity.FrozenCorpusSHA256 = "sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
		if q, err := source.ProgramCMultiSeedQualificationEvidenceV2.VerifyQualificationEnvelopeV2(ctx, foreign, raw, source); err == nil {
			t.Fatal("B_CROSS_SUBJECT_OWNER_REFUSAL")
		} else if _, err = q.SubjectBindingV2(); err == nil {
			t.Fatal("B_CROSS_SUBJECT_AUTHORITY_ESCAPE")
		}
		if !reflect.DeepEqual(before, workflowStateSnapshot(t, stateDir)) {
			t.Fatal("B_FALSE_EMPTY_AUTHORITY_REFUSAL: storage changed")
		}
		t.Log("B_FALSE_EMPTY_AUTHORITY_REFUSAL PASS: false EMPTY calculation cannot authorize; owner retains COMPLETE D24/G48; zero-capability ingress refuses before storage")
	})
}
