package inventoryworkflow

import (
	"bytes"
	"context"
	"encoding/json"
	"lsp-trace/internal/censuscontinuation"
	"lsp-trace/internal/censussourcecontinuation"
	"lsp-trace/internal/inventorysuccessor"
	"lsp-trace/internal/programcrepresentative"
	"lsp-trace/internal/publication"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
)

func controlledCheckpointQualificationV2(t *testing.T) censuscontinuation.VerifiedQualificationV2 {
	t.Helper()
	_, _, source, expected, _, raw := ownerBoundaryFixtureV2(t)
	q, err := source.ProgramCMultiSeedQualificationEvidenceV2.VerifyQualificationEnvelopeV2(context.Background(), expected, raw, source)
	if err != nil {
		t.Fatal(err)
	}
	return q
}

type frozenSuccessorWireForTestsV2 struct {
	SchemaVersion          string                              `json:"schema_version"`
	Selector               string                              `json:"selector"`
	WorkflowKey            string                              `json:"workflow_key"`
	Identity               inventorysuccessor.WorkflowIdentity `json:"identity"`
	Authority              uint8                               `json:"authority"`
	Generation             uint64                              `json:"generation"`
	PredecessorHead        string                              `json:"predecessor_head"`
	CurrentStage           inventorysuccessor.Stage            `json:"current_stage"`
	LastCompletedStage     inventorysuccessor.Stage            `json:"last_completed_stage"`
	Outcome                inventorysuccessor.Outcome          `json:"outcome"`
	MissingEvidence        []inventorysuccessor.EvidenceGap    `json:"missing_evidence"`
	Accepted               bool                                `json:"accepted"`
	FeatureIdentity        string                              `json:"feature_identity"`
	Completeness           string                              `json:"completeness"`
	Effects                string                              `json:"effects"`
	ImplementationDecision string                              `json:"implementation_decision"`
	ProductionAdmission    string                              `json:"production_admission"`
	SourceGraphComplete    string                              `json:"source_graph_complete"`
	AbsenceInference       string                              `json:"absence_inference"`
}

func seedControlledSuccessorV2(t *testing.T, root *publication.Root, q censuscontinuation.VerifiedQualificationV2) (inventorysuccessor.WorkflowIdentity, []byte) {
	t.Helper()
	s, err := q.SubjectDTOv2()
	if err != nil {
		t.Fatal(err)
	}
	var identity inventorysuccessor.WorkflowIdentity
	raw, _ := json.Marshal(s.WorkflowIdentity)
	if err = json.Unmarshal(raw, &identity); err != nil {
		t.Fatal(err)
	}
	v := frozenSuccessorWireForTestsV2{SchemaVersion: "lsp-trace.inventory-successor-head.private.v1", WorkflowKey: "controlled-successor", Identity: identity, Generation: 1, PredecessorHead: s.TerminalProgramCHead.Selector, CurrentStage: inventorysuccessor.StageDescribePreparation, LastCompletedStage: inventorysuccessor.StageProgramC, Outcome: inventorysuccessor.OutcomeWaitingEvidence, MissingEvidence: []inventorysuccessor.EvidenceGap{inventorysuccessor.GapRepresentativeNominationDenominator, inventorysuccessor.GapCommunityRepresentativeLineage, inventorysuccessor.GapRetainedSourceClosure, inventorysuccessor.GapFullDefinitionBindings, inventorysuccessor.GapCallerOccurrenceV3Custody}, FeatureIdentity: "UNRESOLVED", Completeness: "UNKNOWN", Effects: "NONE", ImplementationDecision: "UNISSUED", ProductionAdmission: "REFUSED_MISSING_EVIDENCE", SourceGraphComplete: "UNKNOWN", AbsenceInference: "PROHIBITED"}
	raw, _ = json.Marshal(v)
	v.Selector = workflowDigest(raw)
	raw, _ = json.Marshal(v)
	if _, err = inventorysuccessor.ParseHeadRevision(raw); err != nil {
		t.Fatal(err)
	}
	heads, err := inventorysuccessor.NewHeads(root)
	if err != nil {
		t.Fatal(err)
	}
	if r := heads.CompareAndSwap(context.Background(), "external-storage-key", censussourcecontinuation.HeadRevision{}, raw); r.Outcome != censussourcecontinuation.CASCommitted {
		t.Fatal(r)
	}
	return identity, raw
}
func checkpointFilesV2(t *testing.T, directory string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(directory, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		name, err := filepath.Rel(directory, path)
		if err != nil {
			return err
		}
		out[name] = workflowDigest(raw)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}
func TestMultiSeedV2ControlledThreeCAS(t *testing.T) {
	t.Log("procedure: test-archive-only reviewed context; actual three private operations with close/reopen; exact snapshot; fresh-root byte-identical replay with zero publication/CAS; no production pins asserted")
	ctx := context.Background()
	q := controlledCheckpointQualificationV2(t)
	directory, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Chmod(directory, 0700); err != nil {
		t.Fatal(err)
	}
	root, err := publication.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	identity, priorRaw := seedControlledSuccessorV2(t, root, q)
	if err = root.Close(); err != nil {
		t.Fatal(err)
	}
	var final inventorysuccessor.MultiSeedOperationResultV2
	cas := uint64(0)
	kinds := []inventorysuccessor.MultiSeedOperationKindV2{inventorysuccessor.MultiSeedDenominatorOperationV2, inventorysuccessor.MultiSeedLineageOperationV2, inventorysuccessor.MultiSeedSuccessorOperationV2}
	for _, kind := range kinds {
		root, err = publication.OpenRoot(directory)
		if err != nil {
			t.Fatal(err)
		}
		operation, err := inventorysuccessor.NewMultiSeedCheckpointOperationV2(root, kind, "controlled-successor", "external-storage-key", identity, q)
		if err != nil {
			t.Fatal(err)
		}
		final, err = operation.Execute(ctx)
		if err != nil {
			t.Fatalf("THREE_CAS_OPERATION_SUCCESS kind=%s: %v", kind, err)
		}
		observations := operation.ObservationsV2()
		if observations.CASAttempts != 1 {
			t.Fatalf("ONE_CAS_PER_OPERATION kind=%s: %+v", kind, observations)
		}
		cas += observations.CASAttempts
		if err = root.Close(); err != nil {
			t.Fatal(err)
		}
	}
	if cas != 3 || final.Head.Generation != 2 {
		t.Fatal("EXACT_THREE_CAS_CHECKPOINT")
	}
	var gap programcrepresentative.GapLedgerV2
	if err = json.Unmarshal(final.GapLedger, &gap); err != nil {
		t.Fatal(err)
	}
	if gap.SchemaVersion != "lsp-trace.describe-preparation-gap-ledger.private.v2" || len(gap.Requirements) != 5 || gap.RepresentativeNomination.Status != "COMPUTED" || gap.RepresentativeNomination.Count == nil || gap.CandidatePreparation.Status != "NOT_COMPUTED" || gap.CandidatePreparation.Count != nil || gap.Authority != 0 || gap.Accepted || gap.Outcome != "WAITING_EVIDENCE" {
		t.Fatal("EXACT_CHECKPOINT_CLAIM_CEILING")
	}
	for i, r := range gap.Requirements {
		if i < 2 {
			if r.Status != "RETAINED" || r.EvidenceSelector == nil || r.EvidenceByteLength == nil {
				t.Fatal("FIRST_TWO_GAPS_RETAINED")
			}
		} else if r.Status != "NOT_RETAINED" || r.EvidenceSelector != nil || r.EvidenceByteLength != nil {
			t.Fatal("SOURCE_GAPS_STILL_MISSING")
		}
	}
	before := checkpointFilesV2(t, directory)
	for _, kind := range kinds {
		root, err = publication.OpenRoot(directory)
		if err != nil {
			t.Fatal(err)
		}
		operation, err := inventorysuccessor.NewMultiSeedCheckpointOperationV2(root, kind, "controlled-successor", "external-storage-key", identity, q)
		if err != nil {
			t.Fatal(err)
		}
		replay, err := operation.Execute(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if observations := operation.ObservationsV2(); observations.CASAttempts != 0 || observations.ImmutablePublishAttempts != 0 {
			t.Fatalf("EXACT_REPLAY_ZERO_WRITES_CAS: %+v", observations)
		}
		if kind == inventorysuccessor.MultiSeedSuccessorOperationV2 && !bytes.Equal(replay.GapLedger, final.GapLedger) {
			t.Fatal("BYTE_IDENTICAL_GAP_REPLAY")
		}
		if err = root.Close(); err != nil {
			t.Fatal(err)
		}
	}
	if !reflect.DeepEqual(before, checkpointFilesV2(t, directory)) {
		t.Fatal("BYTE_IDENTICAL_ROOT_REPLAY")
	}
	root, err = publication.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	heads, err := inventorysuccessor.NewMultiSeedSuccessorHeadsV2(root)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := heads.Load(ctx, "external-storage-key")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = inventorysuccessor.ParseHeadRevision(raw); err == nil {
		t.Fatal("FROZEN_V1_PARSER_REMAINS_STRICT")
	}
	var successor programcrepresentative.SuccessorHeadV2
	if err = json.Unmarshal(raw, &successor); err != nil {
		t.Fatal(err)
	}
	objects, err := inventorysuccessor.NewMultiSeedObjectsV2(root)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := objects.ResolveSuccessorPredecessorV2(ctx, successor.PriorSuccessor, "controlled-successor", "external-storage-key", identity, successor.PredecessorHead)
	if err != nil || !bytes.Equal(resolved, priorRaw) {
		t.Fatal("FRESH_ROOT_EXACT_V1_SNAPSHOT_RECOVERY")
	}
	operation, err := inventorysuccessor.NewMultiSeedCheckpointOperationV2(root, inventorysuccessor.MultiSeedSuccessorOperationV2, "controlled-successor", "external-storage-key", identity, q)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	failures := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := operation.Execute(ctx); failures <- err }()
	}
	wg.Wait()
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	if operation.ObservationsV2().CASAttempts != 0 {
		t.Fatal("CONCURRENT_REPLAY_ZERO_CAS")
	}
}
