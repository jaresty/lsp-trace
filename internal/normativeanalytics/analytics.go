// Package normativeanalytics defines the post-admission Program B analytics v2
// contract. It intentionally does not register public command or protocol routes;
// callers must first provide independently validated Program B admission.
package normativeanalytics

import "errors"

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
	ErrProgramBNotAdmitted = errors.New("normative analytics v2: PROGRAM_B_ADMITTED is required")
	ErrInvalidRequest      = errors.New("normative analytics v2: invalid request")
)

type Descriptor struct {
	Operation Operation
	Command   string
	Protocol  string
}

var descriptors = [...]Descriptor{
	{Operation: Analysis, Command: "normative-analysis", Protocol: "lsp_trace_v2_normative_analysis"},
	{Operation: Metrics, Command: "normative-metrics", Protocol: "lsp_trace_v2_normative_metrics"},
	{Operation: Ranking, Command: "normative-ranking", Protocol: "lsp_trace_v2_normative_ranking"},
}

// Descriptors returns equivalent, unregistered command and protocol identities for
// each v2 operation. Registration is a separate, admission-governed integration.
func Descriptors() []Descriptor {
	result := make([]Descriptor, len(descriptors))
	copy(result, descriptors[:])
	return result
}

type Admission struct {
	ProgramBAdmitted bool
}

type Request struct {
	Operation Operation
	Admission Admission
	Units     int64
	Limit     int64
}

type Accounting struct {
	Units int64
	Limit int64
}

type Result struct {
	Family     string
	Version    string
	Scope      string
	Operation  Operation
	Status     Status
	Accounting Accounting
	Evidence   string
}

// Evaluate enforces admission before validating or accounting for normative work.
// Units are supplied by the operation-specific executor under its versioned policy;
// this contract records and bounds them without estimating or using wall-clock time.
func Evaluate(request Request) (Result, error) {
	if !request.Admission.ProgramBAdmitted {
		return Result{}, ErrProgramBNotAdmitted
	}
	if !validOperation(request.Operation) || request.Units < 0 || request.Limit < 1 {
		return Result{}, ErrInvalidRequest
	}
	result := Result{
		Family:     Family,
		Version:    Version,
		Scope:      Scope,
		Operation:  request.Operation,
		Status:     Complete,
		Accounting: Accounting{Units: request.Units, Limit: request.Limit},
	}
	if request.Units > request.Limit {
		result.Status = Limit
		return result, nil
	}
	result.Evidence = "ADMITTED_NORMATIVE_RESULT"
	return result, nil
}

func validOperation(operation Operation) bool {
	switch operation {
	case Analysis, Metrics, Ranking:
		return true
	default:
		return false
	}
}
