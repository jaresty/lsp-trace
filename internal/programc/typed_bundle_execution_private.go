package programc

// Controlled offline execution only: not a provider grant, qualification,
// funding envelope, grouping-policy acceptance or source-completeness claim.
import (
	"bytes"
	"context"
	"errors"
	"time"

	rg "lsp-trace/internal/referencegrouping"
)

// GroupingBundleExpectationPrivate binds an independently constructed bundle.
// Never infer this expectation from the candidate being checked.
type GroupingBundleExpectationPrivate struct{ commitment rg.CompositionCommitment }

func NewGroupingBundleExpectationPrivate(b GroupingCaseBundlePrivate) (GroupingBundleExpectationPrivate, error) {
	if len(b.canonical) == 0 {
		return GroupingBundleExpectationPrivate{}, errors.New("missing constructed bundle")
	}
	return GroupingBundleExpectationPrivate{b.Commitment()}, nil
}

// ExecutionObservation records completed synchronous stages, not RSS samples,
// process work, termination, or physical resource/wall-time guarantees.
type ExecutionObservation struct {
	ElapsedNanos int64 `json:"elapsed_nanos"`
	Imported     bool  `json:"imported"`
	Executed     bool  `json:"executed"`
	Checked      bool  `json:"checked"`
}
type ExecutionCode string

const (
	CodeTimeout             ExecutionCode = "TIMEOUT"
	CodeCancelled           ExecutionCode = "CANCELLED"
	CodeInternalAdmission   ExecutionCode = "INTERNAL_ADMISSION"
	CodeInternalComputation ExecutionCode = "INTERNAL_COMPUTATION"
)

type ExecutionFailure struct {
	Code    ExecutionCode `json:"code"`
	Message string        `json:"message"`
}

func (f *ExecutionFailure) Error() string { return string(f.Code) + ": " + f.Message }
func executionFailurePrivate(code ExecutionCode, err error) *ExecutionFailure {
	message := ""
	if err != nil {
		message = err.Error()
	}
	if len(message) > 1024 {
		message = message[:1024]
	}
	return &ExecutionFailure{Code: code, Message: message}
}
func executionContextFailurePrivate(ctx context.Context) *ExecutionFailure {
	if err := ctx.Err(); err != nil {
		code := CodeCancelled
		if errors.Is(err, context.DeadlineExceeded) {
			code = CodeTimeout
		}
		return executionFailurePrivate(code, err)
	}
	return nil
}

// ExecuteGroupingCaseBundlePrivate imports detached input, executes the original
// batch and performs FULL CheckReport replay before returning detached bytes.
// Cancellation is cooperative at stage boundaries. Leiden and replay cannot be
// interrupted: cancellation suppresses output only after synchronous work returns.
// There is no goroutine timeout facade and no hard wall or memory guarantee.
func ExecuteGroupingCaseBundlePrivate(ctx context.Context, expected GroupingBundleExpectationPrivate, raw []byte) ([]byte, ExecutionObservation, *ExecutionFailure) {
	// The key is package-private; only package test fixtures install a value-only
	// notification. No exported constructor accepts an execution callback.
	var observer func(bundleExecutionStagePrivate)
	if ctx != nil {
		observer, _ = ctx.Value(bundleExecutionObserverKeyPrivate{}).(func(bundleExecutionStagePrivate))
	}
	return executeGroupingCaseBundlePrivate(ctx, expected, raw, observer)
}

type bundleExecutionObserverKeyPrivate struct{}
type bundleExecutionStagePrivate uint8

const (
	bundleInputDetachedPrivate bundleExecutionStagePrivate = iota + 1
	bundleExecutionStartedPrivate
	bundleExecutionFinishedPrivate
	bundleReplayStartedPrivate
	bundleReplayFinishedPrivate
)

// Notification-only, invocation-local test seam. Ordinary execution supplies nil;
// no candidate, report, authorization decision or replaceable work is exposed.
func notifyBundleExecutionPrivate(observer func(bundleExecutionStagePrivate), stage bundleExecutionStagePrivate) {
	if observer == nil {
		return
	}
	defer func() { _ = recover() }()
	observer(stage)
}
func executeGroupingCaseBundlePrivate(ctx context.Context, expected GroupingBundleExpectationPrivate, raw []byte, observer func(bundleExecutionStagePrivate)) (report []byte, observation ExecutionObservation, failed *ExecutionFailure) {
	start := time.Now()
	defer func() { observation.ElapsedNanos = time.Since(start).Nanoseconds() }()
	if ctx == nil {
		ctx = context.Background()
	}
	if f := executionContextFailurePrivate(ctx); f != nil {
		return nil, observation, f
	}
	if len(raw) == 0 || len(raw) > GroupingCaseBundleMaxBytesPrivate || expected.commitment.ID == "" {
		return nil, observation, executionFailurePrivate(CodeInternalAdmission, errors.New("bundle request bound or missing trusted expectation"))
	}
	detached := bytes.Clone(raw)
	notifyBundleExecutionPrivate(observer, bundleInputDetachedPrivate)
	if f := executionContextFailurePrivate(ctx); f != nil {
		return nil, observation, f
	}
	bundle, err := ImportGroupingCaseBundlePrivate(expected.commitment, detached)
	if err != nil {
		return nil, observation, executionFailurePrivate(CodeInternalAdmission, err)
	}
	observation.Imported = true
	if f := executionContextFailurePrivate(ctx); f != nil {
		return nil, observation, f
	}
	notifyBundleExecutionPrivate(observer, bundleExecutionStartedPrivate)
	result, err := bundle.ExecuteReport()
	if err != nil {
		return nil, observation, executionFailurePrivate(CodeInternalComputation, err)
	}
	observation.Executed = true
	notifyBundleExecutionPrivate(observer, bundleExecutionFinishedPrivate)
	if f := executionContextFailurePrivate(ctx); f != nil {
		return nil, observation, f
	}
	notifyBundleExecutionPrivate(observer, bundleReplayStartedPrivate)
	report, failed = checkBundleExecutionReportPrivate(bundle, result.CanonicalBytes())
	if failed != nil {
		return nil, observation, failed
	}
	observation.Checked = true
	notifyBundleExecutionPrivate(observer, bundleReplayFinishedPrivate)
	detachedReport := bytes.Clone(report)
	if f := executionContextFailurePrivate(ctx); f != nil {
		return nil, observation, f
	}
	return detachedReport, observation, nil
}

// Fixed full-replay barrier shared with corruption probes in package tests. This
// is not an exported unchecked-report entry or a caller-replaceable validator.
func checkBundleExecutionReportPrivate(bundle GroupingCaseBundlePrivate, report []byte) ([]byte, *ExecutionFailure) {
	if err := bundle.CheckReport(report); err != nil {
		return nil, executionFailurePrivate(CodeInternalComputation, err)
	}
	return report, nil
}
