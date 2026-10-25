package programctransitionhost

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"lsp-trace/internal/censuscontinuation"
	"lsp-trace/internal/censussourcecontinuation"
	"lsp-trace/internal/publication"
)

func repeatedFixture(statuses ...censuscontinuation.ProgramCTransitionStatus) (RepeatedRequest, repeatedDriverDeps, *[]string) {
	trace := []string{}
	step := 0
	request := RepeatedRequest{
		WorkflowKey: "workflow",
		HeadKey:     "head",
		OpenState: func() (*publication.Root, error) {
			trace = append(trace, "open")
			return &publication.Root{}, nil
		},
	}
	deps := repeatedDriverDeps{
		open: request.OpenState,
		revision: func(context.Context, *publication.Root, string) (censussourcecontinuation.HeadRevision, error) {
			return censussourcecontinuation.HeadRevision{}, nil
		},
		step: func(_ context.Context, got Request) (censuscontinuation.ProgramCTransitionResult, error) {
			trace = append(trace, "step")
			if got.Expected.Generation != uint64(step) {
				return censuscontinuation.ProgramCTransitionResult{}, errors.New("unexpected durable predecessor")
			}
			status := statuses[step]
			result := censuscontinuation.ProgramCTransitionResult{
				Status:          status,
				Ordinal:         step,
				CommittedPrefix: step,
				Head: censussourcecontinuation.HeadRevision{
					Generation: uint64(step + 1),
					Selector:   "sha256:" + strings.Repeat(string(rune('a'+step)), 64),
				},
			}
			if status == censuscontinuation.ProgramCTransitionCommitted {
				result.CommittedPrefix = step + 1
			}
			step++
			return result, nil
		},
		close: func(*publication.Root) error {
			trace = append(trace, "close")
			return nil
		},
	}
	return request, deps, &trace
}

func TestRunRepeatedThreeCommittedTransitionsReopenEveryStep(t *testing.T) {
	request, deps, trace := repeatedFixture(
		censuscontinuation.ProgramCTransitionCommitted,
		censuscontinuation.ProgramCTransitionCommitted,
		censuscontinuation.ProgramCTransitionCommitted,
	)
	request.MaxTransitions = 3
	result, err := runRepeated(context.Background(), request, deps)
	if err != nil {
		t.Fatal(err)
	}
	wantTrace := []string{"open", "step", "close", "open", "step", "close", "open", "step", "close"}
	if !reflect.DeepEqual(*trace, wantTrace) || len(result.Transitions) != 3 || result.Final.Ordinal != 2 || result.Final.CommittedPrefix != 3 {
		t.Fatalf("ASSERT_REPEATED_PROGRAM_C_THREE_DURABLE_REOPENS trace=%v result=%+v", *trace, result)
	}
}

func TestRunRepeatedStopsOnTerminalStatusWithoutRetry(t *testing.T) {
	for _, status := range []censuscontinuation.ProgramCTransitionStatus{
		censuscontinuation.ProgramCTransitionComplete,
		censuscontinuation.ProgramCTransitionRetained,
		censuscontinuation.ProgramCTransitionConflict,
		censuscontinuation.ProgramCTransitionIndeterminate,
	} {
		t.Run(string(status), func(t *testing.T) {
			request, deps, trace := repeatedFixture(status, censuscontinuation.ProgramCTransitionCommitted)
			request.MaxTransitions = 2
			result, err := runRepeated(context.Background(), request, deps)
			if err != nil || !reflect.DeepEqual(*trace, []string{"open", "step", "close"}) || len(result.Transitions) != 1 || result.Final.Status != status {
				t.Fatalf("ASSERT_REPEATED_PROGRAM_C_TERMINAL_NO_RETRY status=%s trace=%v result=%+v err=%v", status, *trace, result, err)
			}
		})
	}
}

func TestRunRepeatedCloseFailureStopsBeforeReopen(t *testing.T) {
	request, deps, trace := repeatedFixture(censuscontinuation.ProgramCTransitionCommitted, censuscontinuation.ProgramCTransitionCommitted)
	request.MaxTransitions = 2
	closeErr := errors.New("synthetic close failure")
	deps.close = func(*publication.Root) error {
		*trace = append(*trace, "close-failed")
		return closeErr
	}
	result, err := runRepeated(context.Background(), request, deps)
	if !errors.Is(err, closeErr) || !reflect.DeepEqual(*trace, []string{"open", "step", "close-failed"}) || len(result.Transitions) != 1 {
		t.Fatalf("ASSERT_REPEATED_PROGRAM_C_CLOSE_FAILURE_STOPS trace=%v result=%+v err=%v", *trace, result, err)
	}
}

func TestRunRepeatedPreservesRevisionAndCloseFailures(t *testing.T) {
	request, deps, trace := repeatedFixture(censuscontinuation.ProgramCTransitionComplete)
	request.MaxTransitions = 1
	revisionErr := errors.New("synthetic revision failure")
	closeErr := errors.New("synthetic close failure")
	deps.revision = func(context.Context, *publication.Root, string) (censussourcecontinuation.HeadRevision, error) {
		return censussourcecontinuation.HeadRevision{}, revisionErr
	}
	deps.close = func(*publication.Root) error {
		*trace = append(*trace, "close-failed")
		return closeErr
	}
	_, err := runRepeated(context.Background(), request, deps)
	if !errors.Is(err, revisionErr) || !errors.Is(err, closeErr) || !reflect.DeepEqual(*trace, []string{"open", "close-failed"}) {
		t.Fatalf("ASSERT_REPEATED_PROGRAM_C_REVISION_CLOSE_FAILURE_CUSTODY trace=%v err=%v", *trace, err)
	}
}

func TestRunRepeatedStepErrorAndCancellationStopWithoutReopen(t *testing.T) {
	t.Run("step-error", func(t *testing.T) {
		request, deps, trace := repeatedFixture(censuscontinuation.ProgramCTransitionCommitted)
		request.MaxTransitions = 2
		stepErr := errors.New("synthetic step failure")
		deps.step = func(context.Context, Request) (censuscontinuation.ProgramCTransitionResult, error) {
			*trace = append(*trace, "step-failed")
			return censuscontinuation.ProgramCTransitionResult{}, stepErr
		}
		if _, err := runRepeated(context.Background(), request, deps); !errors.Is(err, stepErr) || !reflect.DeepEqual(*trace, []string{"open", "step-failed", "close"}) {
			t.Fatalf("ASSERT_REPEATED_PROGRAM_C_STEP_ERROR_NO_REOPEN trace=%v err=%v", *trace, err)
		}
	})
	t.Run("cancelled", func(t *testing.T) {
		request, deps, trace := repeatedFixture(censuscontinuation.ProgramCTransitionCommitted)
		request.MaxTransitions = 2
		ctx, cancel := context.WithCancel(context.Background())
		originalStep := deps.step
		deps.step = func(ctx context.Context, request Request) (censuscontinuation.ProgramCTransitionResult, error) {
			result, err := originalStep(ctx, request)
			cancel()
			return result, err
		}
		if _, err := runRepeated(ctx, request, deps); !errors.Is(err, context.Canceled) || !reflect.DeepEqual(*trace, []string{"open", "step", "close"}) {
			t.Fatalf("ASSERT_REPEATED_PROGRAM_C_CANCEL_NO_REOPEN trace=%v err=%v", *trace, err)
		}
	})
}

func TestRunRepeatedRejectsInvalidLimitBeforeOpen(t *testing.T) {
	for _, limit := range []int{0, -1, MaxRepeatedTransitions + 1} {
		request, deps, trace := repeatedFixture(censuscontinuation.ProgramCTransitionComplete)
		request.MaxTransitions = limit
		if _, err := runRepeated(context.Background(), request, deps); !errors.Is(err, censuscontinuation.ErrInvalidProgramCTransition) || len(*trace) != 0 {
			t.Fatalf("ASSERT_REPEATED_PROGRAM_C_LIMIT_PREOPEN limit=%d trace=%v err=%v", limit, *trace, err)
		}
	}
}

func TestRepeatedRequestHasNoCallerSequencingControls(t *testing.T) {
	typ := reflect.TypeOf(RepeatedRequest{})
	for _, field := range []string{"Ordinal", "Plan", "RawPlan", "Processed", "Expected", "CASHead", "RetryPolicy", "SkipPolicy", "ProgramCOutcome"} {
		if _, ok := typ.FieldByName(field); ok {
			t.Fatalf("ASSERT_REPEATED_PROGRAM_C_NO_CALLER_CONTROL field=%s", field)
		}
	}
}

func TestRunRepeatedExactFixtureThreeRecordDurableReopen(t *testing.T) {
	receiptPath := os.Getenv("LSP_TRACE_PROGRAM_C_RECEIPT")
	ledgerPath := os.Getenv("LSP_TRACE_PROGRAM_C_LEDGER")
	if receiptPath == "" || ledgerPath == "" {
		t.Skip("set LSP_TRACE_PROGRAM_C_RECEIPT and LSP_TRACE_PROGRAM_C_LEDGER")
	}
	stateDir := t.TempDir()
	if err := os.Chmod(stateDir, 0700); err != nil {
		t.Fatal(err)
	}
	receipt := exactFixtureInput(t, receiptPath)
	ledger := exactFixtureInput(t, ledgerPath)
	opens := 0
	request := RepeatedRequest{
		Receipt: receipt, Ledger: ledger,
		WorkflowKey: "exact-repeated-driver", HeadKey: "exact-repeated-driver",
		Bindings: censuscontinuation.ProgramCTransitionBindings{
			RepositoryCommit: strings.Repeat("a", 40), RepositoryTree: strings.Repeat("b", 40),
			FrozenCorpusSHA256: "sha256:" + strings.Repeat("c", 64), CensusReceiptSHA256: receipt.SHA256, LedgerSHA256: ledger.SHA256,
			AlgorithmVersion: "leiden/v1", Seeds: [2]uint64{1, 2}, ReplaysPerSeed: 3,
			ParametersSHA256: "sha256:" + strings.Repeat("d", 64), ResourcePolicySHA256: "sha256:" + strings.Repeat("e", 64),
		},
		MaxTransitions: 3,
		OpenState: func() (*publication.Root, error) {
			opens++
			return publication.OpenRoot(stateDir)
		},
	}
	result, err := RunRepeated(context.Background(), request)
	if err != nil || opens != 3 || len(result.Transitions) != 3 {
		t.Fatalf("ASSERT_REPEATED_PROGRAM_C_REAL_THREE_REOPENS opens=%d result=%+v err=%v", opens, result, err)
	}
	for i, transition := range result.Transitions {
		if transition.Status != censuscontinuation.ProgramCTransitionCommitted || transition.Ordinal != i || transition.CommittedPrefix != i+1 || transition.ReceiptSelector == "" {
			t.Fatalf("ASSERT_REPEATED_PROGRAM_C_REAL_ORDINAL_PREFIX index=%d transition=%+v", i, transition)
		}
	}
	if result.Final.Ordinal != 2 || result.Final.CommittedPrefix != 3 {
		t.Fatalf("ASSERT_REPEATED_PROGRAM_C_REAL_PREFIX_THREE result=%+v", result)
	}
}
