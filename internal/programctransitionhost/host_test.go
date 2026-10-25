package programctransitionhost

import (
	"context"
	"os"
	"reflect"
	"strings"
	"testing"

	"lsp-trace/internal/censuscontinuation"
	"lsp-trace/internal/publication"
)

type exactFixtureFile struct{ path string }

func (r exactFixtureFile) ReadBounded(_ context.Context, limit int64) ([]byte, error) {
	info, err := os.Lstat(r.path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() > limit {
		return nil, censuscontinuation.ErrInvalidProgramCTransition
	}
	return os.ReadFile(r.path)
}

func exactFixtureInput(t *testing.T, path string) censuscontinuation.ExactProgramCTransitionInput {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return censuscontinuation.ExactProgramCTransitionInput{Reader: exactFixtureFile{path: path}, ByteLength: int64(len(raw)), SHA256: digest(raw)}
}

func TestVerifiedPlanSourceReturnsDefensiveCopies(t *testing.T) {
	source := verifiedPlanSource{raw: []byte("canonical-plan")}
	first, err := source.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	first[0] = 'X'
	second, err := source.Load(context.Background())
	if err != nil || string(second) != "canonical-plan" {
		t.Fatalf("ASSERT_VERIFIED_PLAN_DEFENSIVE_COPY got=%q err=%v", second, err)
	}
}

func TestRequestHasNoCallerPlanSelectorOrPublicationRoot(t *testing.T) {
	typ := reflect.TypeOf(Request{})
	for _, field := range []string{"PublicationRoot", "Plan", "RawPlan", "PublicationSelector", "ConstituentSelector"} {
		if _, ok := typ.FieldByName(field); ok {
			t.Fatalf("ASSERT_PROGRAM_C_HOST_NO_CALLER_CUSTODY field=%s", field)
		}
	}
}

func TestExactFixturePilotOneRecordDurableReopen(t *testing.T) {
	receiptPath := os.Getenv("LSP_TRACE_PROGRAM_C_RECEIPT")
	ledgerPath := os.Getenv("LSP_TRACE_PROGRAM_C_LEDGER")
	if receiptPath == "" || ledgerPath == "" {
		t.Skip("set LSP_TRACE_PROGRAM_C_RECEIPT and LSP_TRACE_PROGRAM_C_LEDGER")
	}
	stateDir := t.TempDir()
	if err := os.Chmod(stateDir, 0700); err != nil {
		t.Fatal(err)
	}
	open := func() *publication.Root {
		root, err := publication.OpenRoot(stateDir)
		if err != nil {
			t.Fatal(err)
		}
		return root
	}
	receipt := exactFixtureInput(t, receiptPath)
	ledger := exactFixtureInput(t, ledgerPath)
	bindings := censuscontinuation.ProgramCTransitionBindings{
		RepositoryCommit: strings.Repeat("a", 40), RepositoryTree: strings.Repeat("b", 40),
		FrozenCorpusSHA256: "sha256:" + strings.Repeat("c", 64), CensusReceiptSHA256: receipt.SHA256, LedgerSHA256: ledger.SHA256,
		AlgorithmVersion: "leiden/v1", Seeds: [2]uint64{1, 2}, ReplaysPerSeed: 3,
		ParametersSHA256: "sha256:" + strings.Repeat("d", 64), ResourcePolicySHA256: "sha256:" + strings.Repeat("e", 64),
	}
	root := open()
	request := Request{StateRoot: root, Receipt: receipt, Ledger: ledger, WorkflowKey: "exact-fixture-pilot", HeadKey: "exact-fixture-pilot", Bindings: bindings}
	first, err := InvokeOne(context.Background(), request)
	if err != nil || first.Status != censuscontinuation.ProgramCTransitionCommitted || first.CommittedPrefix != 1 || first.Ordinal != 0 || (first.ProgramCOutcome != "COMPLETE" && first.ProgramCOutcome != "EMPTY") || first.ReceiptSelector == "" || first.FailureStage != "" || first.Failure != "" {
		t.Fatalf("ASSERT_PROGRAM_C_EXACT_PILOT_FIRST_RECORD_COMMITTED result=%+v err=%v", first, err)
	}
	if err = root.Close(); err != nil {
		t.Fatal(err)
	}
	root = open()
	defer root.Close()
	request.StateRoot = root
	request.Expected = first.Head
	second, err := InvokeOne(context.Background(), request)
	if err != nil || second.Status != censuscontinuation.ProgramCTransitionCommitted || second.CommittedPrefix != 2 || second.Ordinal != 1 || (second.ProgramCOutcome != "COMPLETE" && second.ProgramCOutcome != "EMPTY") || second.ReceiptSelector == "" || second.ReceiptSelector == first.ReceiptSelector || second.FailureStage != "" || second.Failure != "" {
		t.Fatalf("ASSERT_PROGRAM_C_EXACT_PILOT_OPTIONAL_PREFIX_1_2 result=%+v first=%+v err=%v", second, first, err)
	}
	t.Logf("ASSERT_PROGRAM_C_EXACT_PILOT first_status=%s first_ordinal=%d first_prefix=%d first_outcome=%s first_receipt=%s second_status=%s second_ordinal=%d second_prefix=%d second_outcome=%s second_receipt=%s", first.Status, first.Ordinal, first.CommittedPrefix, first.ProgramCOutcome, first.ReceiptSelector, second.Status, second.Ordinal, second.CommittedPrefix, second.ProgramCOutcome, second.ReceiptSelector)
}
