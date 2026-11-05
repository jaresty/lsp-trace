package programc

// Controlled offline transport only. This is not a provider grant, qualification,
// billing envelope, grouping-policy acceptance, or source-completeness claim.
import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"

	rg "lsp-trace/internal/referencegrouping"
)

const bundleWorkerVersionPrivate = "lsp-trace.private-program-c-bundle-worker.v1"

// GroupingBundleExpectationPrivate can only bind an already constructed bundle.
// Keep it independently of candidate bytes; do not reconstruct it from a candidate.
// It does not authenticate the human supplier of controlled evidence or judgments.
type GroupingBundleExpectationPrivate struct{ commitment rg.CompositionCommitment }

func NewGroupingBundleExpectationPrivate(b GroupingCaseBundlePrivate) (GroupingBundleExpectationPrivate, error) {
	if len(b.canonical) == 0 {
		return GroupingBundleExpectationPrivate{}, errors.New("missing constructed bundle")
	}
	return GroupingBundleExpectationPrivate{b.Commitment()}, nil
}

type checkedBundleResponsePrivate struct {
	Expected rg.CompositionCommitment `json:"expected"`
	Digest   string                   `json:"digest"`
}

type privateSupervisorResult struct {
	legacy Outcome
	report []byte
}

// Value-only lifecycle diagnostics. Nil in production; no setter, request field,
// mutable payload, authorization decision or substituted work is exposed.
type supervisorStagePrivate uint8

const (
	supervisorDeliveryFinishedPrivate supervisorStagePrivate = iota + 1
	supervisorReapedPrivate
	supervisorBeforeVerifyPrivate
	supervisorAfterVerifyPrivate
)

var supervisorObserverPrivate func(supervisorStagePrivate)

func notifySupervisorPrivate(stage supervisorStagePrivate) {
	observer := supervisorObserverPrivate
	if observer == nil {
		return
	}
	defer func() { _ = recover() }()
	observer(stage)
}

// ExecuteGroupingCaseBundleSupervisedPrivate invokes the existing self-reexec
// supervisor. Import, complete execution and the complete CheckReport replay all
// run in the killable child. The parent checks bounded transport commitments, not
// a second grouping algorithm or a weakened report validator. Returned bytes are
// not published to a store; failure always returns nil bytes.
func ExecuteGroupingCaseBundleSupervisedPrivate(ctx context.Context, expected GroupingBundleExpectationPrivate, raw []byte) ([]byte, SupervisionObservation, *SupervisionFailure) {
	if ctx == nil {
		ctx = context.Background()
	}
	if len(raw) == 0 || len(raw) > GroupingCaseBundleMaxBytesPrivate || expected.commitment.ID == "" {
		return nil, SupervisionObservation{Ceiling: ObservationCeiling}, failure(CodeInternalAdmission, errors.New("bundle request bound or missing trusted expectation"))
	}
	c := expected.commitment
	result, observation, failed := supervisePrivateWorker(ctx, workerRequest{Version: bundleWorkerVersionPrivate, Input: raw, BundleExpected: &c})
	if failed != nil {
		return nil, observation, failed
	}
	return result.report, observation, nil
}

func bundleReportDigestPrivate(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func executeBundleWorkerPrivate(request workerRequest) workerResponse {
	bad := func(code SupervisionCode, err error) workerResponse {
		return workerResponse{Version: bundleWorkerVersionPrivate, Failure: failure(code, err)}
	}
	if request.BundleExpected == nil || request.Seed != 0 {
		return bad(CodeInternalAdmission, errors.New("missing separate expected bundle commitment"))
	}
	b, err := ImportGroupingCaseBundlePrivate(*request.BundleExpected, request.Input)
	if err != nil {
		return bad(CodeInternalAdmission, err)
	}
	result, err := b.ExecuteReport()
	if err != nil {
		return bad(CodeInternalComputation, err)
	}
	return finishBundleWorkerReportPrivate(request, b, result.CanonicalBytes())
}

// Fixed child-side finalization, not a caller-replaceable work/authorization
// callback or a public unchecked-report input. Both normal dispatch and controlled
// test-worker corruption probes traverse this exact existing CheckReport barrier.
func finishBundleWorkerReportPrivate(request workerRequest, b GroupingCaseBundlePrivate, report []byte) workerResponse {
	if request.BundleExpected == nil {
		return workerResponse{Version: bundleWorkerVersionPrivate, Failure: failure(CodeInternalAdmission, errors.New("missing bundle expectation"))}
	}
	if err := b.CheckReport(report); err != nil {
		return workerResponse{Version: bundleWorkerVersionPrivate, Failure: failure(CodeInternalComputation, err)}
	}
	// This response is constructed only after actual full expected-batch replay.
	return workerResponse{Version: bundleWorkerVersionPrivate, Report: report, Bundle: &checkedBundleResponsePrivate{Expected: *request.BundleExpected, Digest: bundleReportDigestPrivate(report)}}
}

func acceptBundleResponsePrivate(request workerRequest, response workerResponse) ([]byte, *SupervisionFailure) {
	if request.BundleExpected == nil || response.Bundle == nil || response.Outcome != nil || response.Failure != nil || len(response.Report) == 0 || int64(len(response.Report)) > MaxWorkerOutputBytes || response.Bundle.Expected != *request.BundleExpected || response.Bundle.Digest != bundleReportDigestPrivate(response.Report) {
		return nil, failure(CodeMalformedOutput, errors.New("invalid checked-bundle transport commitment"))
	}
	return response.Report, nil
}
