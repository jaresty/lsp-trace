package inventoryworkflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"lsp-trace/internal/inventorysuccessor"
	"lsp-trace/internal/programcrepresentative"
)

// These adapters are test-only, including the local pin fixture and synthetic
// recorded harness identities. No production declaration or reader is changed.
type controlledCheckpointImplementationReaderV2 struct{ root string }

func (r controlledCheckpointImplementationReaderV2) ReadImplementationFileV2(name string, limit uint64) ([]byte, error) {
	if name == "internal/programcrepresentative/test-only.go" {
		raw := []byte("controlled test-only implementation source\n")
		if uint64(len(raw)) > limit {
			return nil, fmt.Errorf("test source bound")
		}
		return raw, nil
	}
	if filepath.IsAbs(name) || filepath.Clean(name) != name || strings.HasPrefix(name, "..") {
		return nil, fmt.Errorf("test source path refused")
	}
	path := filepath.Join(r.root, filepath.FromSlash(name))
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() < 0 || uint64(info.Size()) > limit {
		return nil, fmt.Errorf("test source bound/type refused")
	}
	return os.ReadFile(path)
}

type controlledCheckpointHarnessV2 struct {
	actors map[string]programcrepresentative.RecordedReviewActorV2
}

func (h controlledCheckpointHarnessV2) recordedReviewActorV2(ctx context.Context, kind, id string) (programcrepresentative.RecordedReviewActorV2, error) {
	if err := ctx.Err(); err != nil {
		return programcrepresentative.RecordedReviewActorV2{}, err
	}
	record, ok := h.actors[kind+":"+id]
	if !ok {
		return programcrepresentative.RecordedReviewActorV2{}, fmt.Errorf("missing test-only harness metadata")
	}
	record.Projection = append([]byte{}, record.Projection...)
	return record, nil
}
func controlledCheckpointReviewScopeV2() []string {
	return []string{"SUBJECT_CANONICAL_BYTES", "SUBJECT_EXPECTED_CONTEXT_RECONSTRUCTION", "IMPLEMENTATION_SEAM_MANIFEST", "STAGE6_POLICY", "STAGE6_LIMITS", "STAGE6_FINALIZATION", "CORPUS_AND_WORKFLOW_BINDINGS", "COMPLETE_RECEIPT_INVENTORY", "TERMINAL_PROGRAM_C_HEAD", "CLAIM_CEILINGS", "ACYCLICITY", "MULTISEED_ORDERED_BIJECTION", "GROUP_LOCAL_TERMINALS", "D_AND_G_CONSERVATION", "BOUNDED_TYPED_SIZE_ADMISSION"}
}
func controlledCheckpointDomainV2(domain string, raw []byte) string {
	return workflowDigest(append(append([]byte(domain), 0), raw...))
}

func retainControlledCheckpointReviewV2(t *testing.T, request Request, subject programcrepresentative.ProgramCRepresentativeQualificationSubjectV2) (programcrepresentative.ArtifactBindingDTO, controlledCheckpointHarnessV2, []byte) {
	return retainControlledCheckpointReviewActorsV2(t, request, subject, "controlled-test-writer", "controlled-test-isolated-reviewer")
}
func retainControlledCheckpointReviewActorsV2(t *testing.T, request Request, subject programcrepresentative.ProgramCRepresentativeQualificationSubjectV2, writerID, reviewerID string) (programcrepresentative.ArtifactBindingDTO, controlledCheckpointHarnessV2, []byte) {
	t.Helper()
	ctx := context.Background()
	root, err := request.ProgramC.OpenState()
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	objects, err := inventorysuccessor.NewMultiSeedObjectsV2(root)
	if err != nil {
		t.Fatal(err)
	}
	heads, err := inventorysuccessor.NewHeads(root)
	if err != nil {
		t.Fatal(err)
	}
	prior, err := heads.Load(ctx, request.SuccessorHeadKey)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := subject.CanonicalBytes()
	if err != nil {
		t.Fatal(err)
	}
	sb := programcrepresentative.ArtifactBindingDTO{SchemaVersion: programcrepresentative.QualificationSubjectSchemaVersionV2, Identity: subject.Identity(), SHA256: workflowDigest(raw), ByteLength: uint64(len(raw))}
	retain := func(b programcrepresentative.ArtifactBindingDTO, body []byte) {
		t.Helper()
		if err := objects.Retain(ctx, b, body); err != nil {
			t.Fatal(err)
		}
	}
	retain(sb, raw)
	_, pb, pr, err := programcrepresentative.BuildStage6PolicyArtifactV2()
	if err != nil {
		t.Fatal(err)
	}
	_, lb, lr, err := programcrepresentative.BuildStage6LimitsArtifactV2()
	if err != nil {
		t.Fatal(err)
	}
	_, fb, fr, err := programcrepresentative.BuildStage6FinalizationArtifactV2()
	if err != nil {
		t.Fatal(err)
	}
	retain(pb, pr)
	retain(lb, lr)
	retain(fb, fr)
	harness := controlledCheckpointHarnessV2{actors: map[string]programcrepresentative.RecordedReviewActorV2{}}
	actors := []programcrepresentative.ReviewActorV2{}
	for i, kind := range []string{"PI_SESSION", "PI_ISOLATED_TASK"} {
		id, parent, verdict := writerID, "", "SUBJECT_PRODUCED"
		if i == 1 {
			id = reviewerID
			parent = writerID
			verdict = "ACCEPT"
		}
		assignment := programcrepresentative.ReviewAssignmentV2{SchemaVersion: "lsp-trace.program-c-representative-review-assignment.private.v2", ActorKind: kind, HarnessActorID: id, ParentActorID: parent, Subject: sb, Scope: controlledCheckpointReviewScopeV2()}
		terminal := programcrepresentative.ReviewTerminalV2{SchemaVersion: "lsp-trace.program-c-representative-review-terminal.private.v2", ActorKind: kind, HarnessActorID: id, ParentActorID: parent, Subject: sb, Scope: controlledCheckpointReviewScopeV2(), Verdict: verdict}
		a, _ := json.Marshal(assignment)
		z, _ := json.Marshal(terminal)
		projection := append(append(append(append([]byte{}, a...), '\n'), z...), '\n')
		rb := programcrepresentative.ArtifactBindingDTO{SchemaVersion: "lsp-trace.program-c-representative-review-transcript-jsonl.private.v2", SHA256: workflowDigest(projection), ByteLength: uint64(len(projection))}
		harness.actors[kind+":"+id] = programcrepresentative.RecordedReviewActorV2{Kind: kind, ID: id, ParentID: parent, IsolationKind: kind, Projection: append([]byte{}, projection...)}
		retain(rb, projection)
		snapshot := programcrepresentative.ReviewTranscriptSnapshotV2{SchemaVersion: "lsp-trace.program-c-representative-review-transcript-snapshot.private.v2", ActorKind: kind, HarnessActorID: id, ParentActorID: parent, RawTranscriptSelector: rb.SHA256, RawTranscriptSHA256: rb.SHA256, RawTranscriptByteLength: rb.ByteLength, AssignmentSubject: sb, AssignmentScope: controlledCheckpointReviewScopeV2(), TerminalSubject: sb, TerminalScope: controlledCheckpointReviewScopeV2(), TerminalVerdict: verdict}
		_, binding, body, err := programcrepresentative.EncodeReviewTranscriptSnapshotV2(snapshot)
		if err != nil {
			t.Fatal(err)
		}
		retain(binding, body)
		actor := programcrepresentative.ReviewActorV2{IdentityKind: kind, HarnessActorID: id, AttributionEvidence: binding}
		body, _ = json.Marshal(actor)
		actor.Identity = controlledCheckpointDomainV2("lsp-trace:program-c-representative-review-actor:v2", body)
		actors = append(actors, actor)
	}
	separation := programcrepresentative.ReviewTaskSeparationV2{SchemaVersion: "lsp-trace.program-c-representative-review-task-separation.private.v2", WriterSessionID: actors[0].HarnessActorID, ReviewerTaskID: actors[1].HarnessActorID, ReviewerParentSessionID: actors[0].HarnessActorID, ReviewerIsolated: true, Subject: sb, ReviewScope: controlledCheckpointReviewScopeV2()}
	_, separationBinding, body, err := programcrepresentative.EncodeReviewTaskSeparationV2(separation)
	if err != nil {
		t.Fatal(err)
	}
	retain(separationBinding, body)
	review := programcrepresentative.QualificationReviewV2{SchemaVersion: programcrepresentative.QualificationReviewSchemaVersionV2, Profile: programcrepresentative.ProfileCallsOnlyFrozenCensusV1, Subject: sb, ReviewScope: controlledCheckpointReviewScopeV2(), Verdict: "ACCEPT", Writer: actors[0], Reviewer: actors[1], SeparationMethod: "DISTINCT_HARNESS_ISOLATED_TASK", SeparationEvidence: separationBinding}
	_, reviewBinding, body, err := programcrepresentative.EncodeQualificationReviewV2(review)
	if err != nil {
		t.Fatal(err)
	}
	retain(reviewBinding, body)
	dto, err := subject.DTO()
	if err != nil {
		t.Fatal(err)
	}
	envelope := programcrepresentative.QualificationEnvelopeV2{SchemaVersion: programcrepresentative.QualificationEnvelopeSchemaVersionV2, Profile: programcrepresentative.ProfileCallsOnlyFrozenCensusV1, Subject: sb, IndependentReviewReceipt: reviewBinding, Stage6Policy: pb, Stage6Limits: lb, Stage6Finalization: fb, QualificationEpoch: dto.TerminalProgramCHead.Generation}
	_, binding, body, err := programcrepresentative.EncodeQualificationEnvelopeV2(envelope)
	if err != nil {
		t.Fatal(err)
	}
	retain(binding, body)
	return binding, harness, prior
}

func runControlledMultiSeedCheckpointAssertionsV2(t *testing.T, request Request, stateDir string, subject programcrepresentative.ProgramCRepresentativeQualificationSubjectV2, implementationSource controlledCheckpointImplementationReaderV2, wantD, wantG uint64) {
	t.Helper()
	ctx := context.Background()
	envelope, harness, priorRaw := retainControlledCheckpointReviewV2(t, request, subject)
	installControlledActorDeclarationV2(t, harness)
	base := MultiSeedCheckpointRequestV2{Workflow: request, Envelope: envelope, ImplementationSource: implementationSource, MaxImplementationSourceBytes: programcrepresentative.MaxArtifactCanonicalBytesV2}
	kinds := []inventorysuccessor.MultiSeedOperationKindV2{inventorysuccessor.MultiSeedDenominatorOperationV2, inventorysuccessor.MultiSeedLineageOperationV2, inventorysuccessor.MultiSeedSuccessorOperationV2}
	var final inventorysuccessor.MultiSeedOperationResultV2
	var lineageBinding programcrepresentative.ArtifactBindingDTO
	cas := uint64(0)
	for _, kind := range kinds {
		one := base
		one.Operation = kind
		result, err := RunMultiSeedCheckpointV2(ctx, one)
		if err != nil {
			t.Fatalf("CONNECTED_THREE_REAL_CAS kind=%s: %v", kind, err)
		}
		if result.Observations.CASAttempts != 1 {
			t.Fatalf("CONNECTED_ONE_CAS_PER_OPERATION kind=%s: %+v", kind, result.Observations)
		}
		cas += result.Observations.CASAttempts
		if kind == inventorysuccessor.MultiSeedLineageOperationV2 {
			lineageBinding = result.Artifact
		}
		final = result
	}
	if cas != 3 || final.Head.Generation != 2 {
		t.Fatal("CONNECTED_EXACT_THREE_CAS")
	}
	gap, err := inventorysuccessor.ParseMultiSeedGapLedgerV2(final.GapLedger)
	if err != nil {
		t.Fatal(err)
	}
	if gap.SchemaVersion != "lsp-trace.describe-preparation-gap-ledger.private.v2" || gap.RepresentativeNomination.Status != "COMPUTED" || gap.RepresentativeNomination.Count == nil || *gap.RepresentativeNomination.Count != wantD || gap.CandidatePreparation.Status != "NOT_COMPUTED" || gap.CandidatePreparation.Count != nil || len(gap.Requirements) != 5 || gap.Authority != 0 || gap.Accepted || gap.Outcome != "WAITING_EVIDENCE" || gap.Completeness != "UNKNOWN" || gap.SourceGraphComplete != "UNKNOWN" || gap.FeatureIdentity != "UNRESOLVED" || gap.Effects != "NONE" || gap.ImplementationDecision != "UNISSUED" || gap.ProductionAdmission != "REFUSED_MISSING_EVIDENCE" || gap.AbsenceInference != "PROHIBITED" {
		t.Fatal("CONNECTED_EXACT_GAPS_D_AND_CEILINGS")
	}
	for i, r := range gap.Requirements {
		if i < 2 {
			if r.Status != "RETAINED" || r.EvidenceSelector == nil || r.EvidenceByteLength == nil {
				t.Fatal("CONNECTED_FIRST_TWO_GAPS_RETAINED")
			}
		} else if r.Status != "NOT_RETAINED" || r.EvidenceSelector != nil || r.EvidenceByteLength != nil {
			t.Fatal("CONNECTED_SOURCE_GAPS_NOT_RETAINED")
		}
	}
	root, err := request.ProgramC.OpenState()
	if err != nil {
		t.Fatal(err)
	}
	objects, err := inventorysuccessor.NewMultiSeedObjectsV2(root)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := objects.Resolve(ctx, lineageBinding)
	if err != nil {
		t.Fatal(err)
	}
	lineage, err := programcrepresentative.ParseLineageV2(raw)
	if err != nil {
		t.Fatal(err)
	}
	if lineage.Accounting.D != wantD || lineage.Accounting.TerminalOuterResults != wantD || lineage.Accounting.UnresolvedOuterResults != 0 || lineage.Accounting.G != wantG || lineage.Accounting.UnresolvedGroups != 0 || lineage.Accounting.SelectedGroups+lineage.Accounting.NoCallsGroups+lineage.Accounting.CallsIneligibleGroups != wantG || uint64(len(lineage.Records)) != wantD {
		t.Fatal("CONNECTED_QUALIFIED_D_G_CONSERVATION")
	}
	mixed := map[int]bool{}
	for _, record := range lineage.Records {
		raw, err := objects.Resolve(ctx, record.Result)
		if err != nil {
			t.Fatal(err)
		}
		result, err := programcrepresentative.ParseMultiSeedResultV2(raw)
		if err != nil {
			t.Fatal(err)
		}
		dto, err := result.DTO()
		if err != nil {
			t.Fatal(err)
		}
		if len(dto.GroupResults) != 2 || dto.Terminal != "GROUPS_TERMINAL_COMPLETE" {
			t.Fatal("CONNECTED_ALL_ORDERED_GROUP_RESULTS")
		}
		if dto.GroupResults[0].Record.Outcome != dto.GroupResults[1].Record.Outcome {
			mixed[dto.Composition.Custody.ConstituentOrdinal] = true
		}
	}
	if len(mixed) != 2 {
		t.Fatal("CONNECTED_BOTH_ELIGIBLE_CONSTITUENTS_HAVE_DIFFERENT_GROUP_OUTCOMES")
	}
	heads, err := inventorysuccessor.NewMultiSeedSuccessorHeadsV2(root)
	if err != nil {
		t.Fatal(err)
	}
	headRaw, err := heads.Load(ctx, request.SuccessorHeadKey)
	if err != nil {
		t.Fatal(err)
	}
	var successor programcrepresentative.SuccessorHeadV2
	if err = json.Unmarshal(headRaw, &successor); err != nil {
		t.Fatal(err)
	}
	prior, err := objects.ResolveSuccessorPredecessorV2(ctx, successor.PriorSuccessor, request.SuccessorWorkflowKey, request.SuccessorHeadKey, request.Identity, successor.PredecessorHead)
	if err != nil || !bytes.Equal(prior, priorRaw) {
		t.Fatal("CONNECTED_FRESH_ROOT_EXACT_PRIOR_RECOVERY")
	}
	revision, err := inventorysuccessor.ParseHeadRevision(prior)
	if err != nil || revision.Generation != 1 || revision.Selector != successor.PriorSuccessor.Identity || successor.PriorSuccessor.SHA256 != workflowDigest(priorRaw) || successor.PriorSuccessor.ByteLength != uint64(len(priorRaw)) || bytes.Contains(prior, []byte(`"head_key"`)) {
		t.Fatal("CONNECTED_PRIOR_BYTE_SELECTOR_AND_EXTERNAL_HEAD_KEY")
	}
	if err = root.Close(); err != nil {
		t.Fatal(err)
	}
	before := workflowStateSnapshot(t, stateDir)
	for _, kind := range kinds {
		one := base
		one.Operation = kind
		replay, err := RunMultiSeedCheckpointV2(ctx, one)
		if err != nil {
			t.Fatal(err)
		}
		if replay.Observations.CASAttempts != 0 || replay.Observations.ImmutablePublishAttempts != 0 {
			t.Fatal("CONNECTED_REPLAY_ZERO_WRITES_CAS")
		}
		if kind == inventorysuccessor.MultiSeedSuccessorOperationV2 && !bytes.Equal(replay.GapLedger, final.GapLedger) {
			t.Fatal("CONNECTED_BYTE_IDENTICAL_REPLAY")
		}
	}
	if !reflect.DeepEqual(before, workflowStateSnapshot(t, stateDir)) {
		t.Fatal("CONNECTED_REPLAY_STATE_BYTES_IDENTICAL")
	}
	assertControlledCheckpointAdversariesV2(t, base, stateDir)
	if t.Failed() {
		t.FailNow()
	}
	assertControlledPriorSpliceV2(t, base, stateDir, priorRaw)
	snapshot := filepath.Join(stateDir, "program-c-representative", "successor-predecessors", "objects", "private", "v2", strings.TrimPrefix(successor.PriorSuccessor.SHA256, "sha256:")+".json")
	for _, name := range []string{"corrupt", "missing"} {
		if name == "corrupt" {
			if err = os.WriteFile(snapshot, []byte("corrupt"), 0600); err != nil {
				t.Fatal(err)
			}
		} else if err = os.Remove(snapshot); err != nil {
			t.Fatal(err)
		}
		one := base
		one.Operation = inventorysuccessor.MultiSeedSuccessorOperationV2
		beforeRefusal := workflowStateSnapshot(t, stateDir)
		observations := inventorysuccessor.MultiSeedOperationObservationsV2{}
		output, err := runMultiSeedCheckpointV2(ctx, one, func(actual inventorysuccessor.MultiSeedOperationObservationsV2) { observations = actual })
		if err == nil || len(output.GapLedger) != 0 || observations.CASAttempts != 0 || observations.ImmutablePublishAttempts != 0 || !reflect.DeepEqual(beforeRefusal, workflowStateSnapshot(t, stateDir)) {
			t.Fatalf("CONNECTED_PRIOR_SNAPSHOT_REFUSAL_%s", name)
		}
	}
	t.Logf("CONNECTED_CONTROLLED_CHECKPOINT PASS runs=24 D=%d G=%d CAS=3 replay_writes=0 replay_CAS=0; synthetic review/pins are test-only, not production authority", wantD, wantG)
}
