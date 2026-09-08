// Package normativeanalytics defines dormant post-admission Program B analytics v2 contracts.
package normativeanalytics

import (
	"errors"
	"lsp-trace/internal/qualificationpolicy"
)

const (
	Family        = "normative-analytics"
	Version       = "v2"
	SchemaVersion = "lsp-trace.normative-analytics.v2"
	Scope         = "NORMATIVE_PROGRAM_B_ONLY"
)

type Operation string

const (
	Analysis Operation = "ANALYSIS"
	Metrics  Operation = "METRICS"
	Ranking  Operation = "RANKING"
)

type Status string

const (
	Complete Status = "COMPLETE"
	Limit    Status = "LIMIT"
)

var (
	ErrProgramBNotAdmitted = errors.New("normative analytics v2: PROGRAM_B_ADMITTED is required for exact revision, operation, and scope")
	ErrInvalidRequest      = errors.New("normative analytics v2: invalid request")
)

type Descriptor struct {
	Operation         Operation
	Command, Protocol string
}

var descriptors = [...]Descriptor{{Analysis, "normative-analysis", "lsp_trace_v2_normative_analysis"}, {Metrics, "normative-metrics", "lsp_trace_v2_normative_metrics"}, {Ranking, "normative-ranking", "lsp_trace_v2_normative_ranking"}}

func Descriptors() []Descriptor {
	out := make([]Descriptor, len(descriptors))
	copy(out, descriptors[:])
	return out
}

type Executor interface {
	Execute(Operation) (int64, error)
}
type Request struct {
	Operation                                    Operation
	BuildRevision                                string
	SubstrateID, MatrixDigest, EvidenceSetDigest string
	Admission                                    qualificationpolicy.ProgramBAdmission
	Limit                                        int64
	Executor                                     Executor
}
type Accounting struct{ Units, Limit int64 }
type Result struct {
	Family, Version, Scope string
	Operation              Operation
	Status                 Status
	Accounting             Accounting
	Evidence               string
}

func Evaluate(request Request) (Result, error) {
	return evaluate(request, func() error {
		return request.Admission.VerifyExecution(qualificationpolicy.ProgramBExecutionExpectation{BuildRevision: request.BuildRevision, Operation: string(request.Operation), Scope: Scope, SubstrateID: request.SubstrateID, MatrixDigest: request.MatrixDigest, EvidenceSetDigest: request.EvidenceSetDigest})
	})
}

func evaluate(request Request, verifyAdmission func() error) (Result, error) {
	if verifyAdmission == nil || verifyAdmission() != nil {
		return Result{}, ErrProgramBNotAdmitted
	}
	if !validOperation(request.Operation) || request.BuildRevision == "" || request.Limit < 1 || request.Executor == nil {
		return Result{}, ErrInvalidRequest
	}
	units, err := request.Executor.Execute(request.Operation)
	if err != nil || units < 0 {
		return Result{}, ErrInvalidRequest
	}
	result := Result{Family: Family, Version: Version, Scope: Scope, Operation: request.Operation, Status: Complete, Accounting: Accounting{Units: units, Limit: request.Limit}}
	if units > request.Limit {
		result.Status = Limit
		return result, nil
	}
	result.Evidence = "ADMITTED_NORMATIVE_RESULT"
	return result, nil
}
func validOperation(operation Operation) bool {
	return operation == Analysis || operation == Metrics || operation == Ranking
}
