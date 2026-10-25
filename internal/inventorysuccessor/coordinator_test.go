package inventorysuccessor

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"lsp-trace/internal/censuscontinuation"
	"lsp-trace/internal/censussourcecontinuation"
	"lsp-trace/internal/publication"
)

type memoryHeads struct {
	raw       []byte
	cas       int
	outcome   censussourcecontinuation.CASOutcome
	loadErr   error
	commitErr error
}

func (h *memoryHeads) Load(context.Context, string) ([]byte, error) {
	return bytes.Clone(h.raw), h.loadErr
}
func (h *memoryHeads) CompareAndSwap(_ context.Context, _ string, expected censussourcecontinuation.HeadRevision, raw []byte) censussourcecontinuation.CASResult {
	h.cas++
	current, err := ParseHeadRevision(h.raw)
	if err != nil {
		return censussourcecontinuation.CASResult{Err: err}
	}
	if current != expected {
		return censussourcecontinuation.CASResult{Outcome: censussourcecontinuation.CASNotCommitted}
	}
	outcome := h.outcome
	if outcome == "" {
		outcome = censussourcecontinuation.CASCommitted
	}
	if outcome == censussourcecontinuation.CASCommitted || outcome == censussourcecontinuation.CASCommitUncertain {
		h.raw = bytes.Clone(raw)
	}
	return censussourcecontinuation.CASResult{Outcome: outcome, Err: h.commitErr}
}

func validRequest(heads censussourcecontinuation.HeadStore) Request {
	bindings := censuscontinuation.ProgramCTransitionBindings{
		CensusReceiptSHA256:  "sha256:ecc4a9280e1b3896de6c9799485bc642111e4140ccfc0f458e03e6e25441a751",
		LedgerSHA256:         "sha256:2fe7d6d4713e29b85dfeb73b5d9bdf5b7edec39d2cc317f9588d414a0c372bee",
		PlanSHA256:           "sha256:b9c8832810279b207777b45babcd62442f022d01aa853b6f0beefd4c3293de46",
		ResourcePolicySHA256: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}
	identity := WorkflowIdentity{
		Profile: "CALLS_ONLY_FROZEN_CENSUS_V1", Contract: "adr0012-private-successor-v1",
		CensusReceiptSHA256: bindings.CensusReceiptSHA256, AvailabilityLedgerSHA256: bindings.LedgerSHA256,
		SourcePinSHA256:    "sha256:7b0fe344ac7d34fee7101d998b1f958f22b71d08f9060a104066732c6017d4a6",
		ProgramCPlanSHA256: bindings.PlanSHA256, PrivacyPolicySHA256: "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		RetentionPolicySHA256: "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
		ResourcePolicySHA256:  bindings.ResourcePolicySHA256, ReplayPolicySHA256: "sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
		QualificationPolicySHA256: "sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
	}
	identity.ProgramCBindingsSHA256 = bindingsDigest(bindings)
	return Request{WorkflowKey: "inventory-workflow", HeadKey: "inventory-head", Identity: identity,
		ProgramC: censuscontinuation.ProgramCTransitionVerificationRequest{WorkflowKey: "program-c-workflow", HeadKey: "program-c-head", Bindings: bindings}, Heads: heads}
}

func verifiedProgramC(context.Context, censuscontinuation.ProgramCTransitionVerificationRequest) (censuscontinuation.ProgramCTransitionResult, error) {
	return censuscontinuation.ProgramCTransitionResult{Status: censuscontinuation.ProgramCTransitionComplete, Head: censussourcecontinuation.HeadRevision{Generation: 791, Selector: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}, CommittedPrefix: 395, ConstituentCount: 395}, nil
}

func coordinateTest(ctx context.Context, request Request) (Result, error) {
	return coordinateNext(ctx, request, verifiedProgramC)
}

func TestCoordinateNextCommitsWaitingEvidenceBeforeDescribe(t *testing.T) {
	heads := &memoryHeads{}
	got, err := coordinateTest(context.Background(), validRequest(heads))
	if err != nil || got.Outcome != OutcomeWaitingEvidence || got.CurrentStage != StageDescribePreparation || got.LastCompletedStage != StageProgramC || got.Head.Generation != 1 || heads.cas != 1 {
		t.Fatalf("ASSERT_SUCCESSOR_WAITING_EVIDENCE got=%+v cas=%d err=%v", got, heads.cas, err)
	}
	if got.Authority != 0 || got.Accepted || got.FeatureIdentity != "UNRESOLVED" || got.Completeness != "UNKNOWN" || got.ProductionAdmission != "REFUSED_MISSING_EVIDENCE" || got.SourceGraphComplete != "UNKNOWN" || got.AbsenceInference != "PROHIBITED" {
		t.Fatalf("ASSERT_SUCCESSOR_AUTHORITY_CEILINGS got=%+v", got)
	}
	want := []EvidenceGap{GapRepresentativeNominationDenominator, GapCommunityRepresentativeLineage, GapRetainedSourceClosure, GapFullDefinitionBindings, GapCallerOccurrenceV3Custody}
	if !equalGaps(got.MissingEvidence, want) {
		t.Fatalf("ASSERT_SUCCESSOR_EXACT_GAPS got=%v want=%v", got.MissingEvidence, want)
	}
}

func TestCoordinateNextCompletedStopIsZeroWriteNoOp(t *testing.T) {
	heads := &memoryHeads{}
	first, err := coordinateTest(context.Background(), validRequest(heads))
	if err != nil {
		t.Fatal(err)
	}
	before := bytes.Clone(heads.raw)
	second, err := coordinateTest(context.Background(), validRequest(heads))
	if err != nil || !reflect.DeepEqual(second, first) || heads.cas != 1 || !bytes.Equal(before, heads.raw) {
		t.Fatalf("ASSERT_SUCCESSOR_STOP_NOOP first=%+v second=%+v cas=%d err=%v", first, second, heads.cas, err)
	}
}

func TestCoordinateNextIncompleteProgramCDoesNotWrite(t *testing.T) {
	heads := &memoryHeads{}
	incomplete := func(context.Context, censuscontinuation.ProgramCTransitionVerificationRequest) (censuscontinuation.ProgramCTransitionResult, error) {
		return censuscontinuation.ProgramCTransitionResult{Status: censuscontinuation.ProgramCTransitionCommitted, Head: censussourcecontinuation.HeadRevision{Generation: 790, Selector: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}, CommittedPrefix: 394, ConstituentCount: 395}, nil
	}
	if _, err := coordinateNext(context.Background(), validRequest(heads), incomplete); err == nil || heads.cas != 0 {
		t.Fatalf("ASSERT_SUCCESSOR_INCOMPLETE_PREDECESSOR err=%v cas=%d", err, heads.cas)
	}
}

func TestCoordinateNextConflictAndUncertainAreTyped(t *testing.T) {
	for _, tc := range []struct {
		name      string
		cas       censussourcecontinuation.CASOutcome
		commitErr error
		want      Outcome
	}{
		{name: "conflict", cas: censussourcecontinuation.CASNotCommitted, want: OutcomeConflict},
		{name: "uncertain", cas: censussourcecontinuation.CASCommitUncertain, commitErr: errors.New("synthetic post-commit uncertainty"), want: OutcomeIndeterminateExecution},
	} {
		t.Run(tc.name, func(t *testing.T) {
			heads := &memoryHeads{outcome: tc.cas, commitErr: tc.commitErr}
			got, err := coordinateTest(context.Background(), validRequest(heads))
			if err != nil || got.Outcome != tc.want || heads.cas != 1 {
				t.Fatalf("ASSERT_SUCCESSOR_TYPED_CAS got=%+v cas=%d err=%v", got, heads.cas, err)
			}
		})
	}
}

type cancelledCASHeads struct{ memoryHeads }

func (h *cancelledCASHeads) CompareAndSwap(context.Context, string, censussourcecontinuation.HeadRevision, []byte) censussourcecontinuation.CASResult {
	h.cas++
	return censussourcecontinuation.CASResult{Err: context.Canceled}
}

func TestCoordinateNextCancellationDuringCASIsTyped(t *testing.T) {
	heads := &cancelledCASHeads{}
	got, err := coordinateTest(context.Background(), validRequest(heads))
	if err != nil || got.Outcome != OutcomeIndeterminateExecution || heads.cas != 1 {
		t.Fatalf("ASSERT_SUCCESSOR_UNKNOWN_CAS_INDETERMINATE got=%+v cas=%d err=%v", got, heads.cas, err)
	}
	for _, tc := range []struct {
		name  string
		heads *memoryHeads
	}{
		{name: "load", heads: &memoryHeads{loadErr: context.Canceled}},
		{name: "not-committed", heads: &memoryHeads{outcome: censussourcecontinuation.CASNotCommitted, commitErr: context.Canceled}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, testErr := coordinateTest(context.Background(), validRequest(tc.heads))
			if testErr != nil || result.Outcome != OutcomeCancelled {
				t.Fatalf("ASSERT_SUCCESSOR_CANCELLATION_PATH got=%+v err=%v", result, testErr)
			}
		})
	}
}

func TestCoordinateNextCancellationDoesNotWrite(t *testing.T) {
	heads := &memoryHeads{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got, err := coordinateTest(ctx, validRequest(heads))
	if err != nil || got.Outcome != OutcomeCancelled || heads.cas != 0 {
		t.Fatalf("ASSERT_SUCCESSOR_CANCELLED got=%+v cas=%d err=%v", got, heads.cas, err)
	}
}

func TestCoordinateNextPublicationBackedCloseReopenNoRepublish(t *testing.T) {
	path, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Chmod(path, 0o700); err != nil {
		t.Fatal(err)
	}
	open := func() (*publication.Root, censussourcecontinuation.HeadStore) {
		root, openErr := publication.OpenRoot(path)
		if openErr != nil {
			t.Fatal(openErr)
		}
		heads, headsErr := NewHeads(root)
		if headsErr != nil {
			_ = root.Close()
			t.Fatal(headsErr)
		}
		return root, heads
	}
	root, heads := open()
	first, err := coordinateTest(context.Background(), validRequest(heads))
	if err != nil {
		t.Fatal(err)
	}
	if err = root.Close(); err != nil {
		t.Fatal(err)
	}
	root, heads = open()
	second, err := coordinateTest(context.Background(), validRequest(heads))
	if closeErr := root.Close(); err == nil && closeErr != nil {
		err = closeErr
	}
	if err != nil || !reflect.DeepEqual(first, second) || second.Outcome != OutcomeWaitingEvidence {
		t.Fatalf("ASSERT_SUCCESSOR_PUBLICATION_REOPEN first=%+v second=%+v err=%v", first, second, err)
	}
}

func TestParseHeadRevisionRejectsSelfRehashedFutureGeneration(t *testing.T) {
	heads := &memoryHeads{}
	if _, err := coordinateTest(context.Background(), validRequest(heads)); err != nil {
		t.Fatal(err)
	}
	var value head
	if err := strict(heads.raw, &value); err != nil {
		t.Fatal(err)
	}
	value.Generation = 2
	raw, err := encodeHead(&value)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ParseHeadRevision(raw); err == nil {
		t.Fatal("ASSERT_SUCCESSOR_FUTURE_GENERATION_REJECTED")
	}
}

func TestParseHeadRevisionRejectsMutation(t *testing.T) {
	heads := &memoryHeads{}
	if _, err := coordinateTest(context.Background(), validRequest(heads)); err != nil {
		t.Fatal(err)
	}
	heads.raw[len(heads.raw)-2] ^= 1
	if _, err := ParseHeadRevision(heads.raw); err == nil {
		t.Fatal("ASSERT_SUCCESSOR_HEAD_MUTATION_REJECTED")
	}
}

func equalGaps(a, b []EvidenceGap) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
