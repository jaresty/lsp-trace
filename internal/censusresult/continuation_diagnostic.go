package censusresult

import (
	"bytes"
	"encoding/json"
	"errors"

	"lsp-trace/internal/strictjson"
)

const (
	ContinuationDiagnosticSchemaVersion   = "lsp-trace.census-continuation-diagnostic.v1"
	ContinuationDiagnosticSchemaVersionV2 = "lsp-trace.census-continuation-diagnostic.v2"
)

type ContinuationCause string
type ContinuationCode string

const (
	ContinuationHostConstructionFailed ContinuationCause = "HOST_CONSTRUCTION_FAILED"
	ContinuationDescriptorUnavailable  ContinuationCause = "DESCRIPTOR_UNAVAILABLE"
	ContinuationStopCheckpointMismatch ContinuationCause = "STOP_CHECKPOINT_MISMATCH"
	ContinuationPublicationFailed      ContinuationCause = "PUBLICATION_FAILED"
	ContinuationCaptureFailed          ContinuationCause = "CAPTURE_FAILED"

	CodeContinuationHostConstructionFailed ContinuationCode = "CONTINUATION_HOST_CONSTRUCTION_FAILED"
	CodeContinuationDescriptorUnavailable  ContinuationCode = "CONTINUATION_DESCRIPTOR_UNAVAILABLE"
	CodeContinuationStopCheckpointMismatch ContinuationCode = "CONTINUATION_STOP_CHECKPOINT_MISMATCH"
	CodeContinuationPublicationFailed      ContinuationCode = "CONTINUATION_PUBLICATION_FAILED"
	CodeContinuationCaptureFailed          ContinuationCode = "CONTINUATION_CAPTURE_FAILED"
)

type ContinuationDiagnosticContext struct {
	ConstructionStage   string
	ConstructionReason  string
	PublicSelector      string
	ExpectedStage       string
	ExpectedStatus      string
	ObservedStage       string
	ObservedStatus      string
	ResourceCategory    string
	ResourceObserved    int64
	ResourceLimit       int64
	ResourceField       string
	CaptureInvariant    string
	CaptureCallerAction string
	CaptureRecovery     string
}

type ContinuationDiagnosticV2 struct {
	ContinuationDiagnostic
	RecoverySelectorState   string `json:"recovery_selector_state"`
	PreservedCensusSelector string `json:"preserved_census_selector,omitempty"`
	LastSuccessfulStage     string `json:"last_successful_stage,omitempty"`
	TerminalStage           string `json:"terminal_stage"`
	TerminalStatus          string `json:"terminal_status"`
	Authority               int    `json:"authority"`
}

func NewContinuationDiagnosticV2(d ContinuationDiagnostic, preservedCensusSelector string) (ContinuationDiagnosticV2, error) {
	if err := validateContinuationDiagnostic(d); err != nil {
		return ContinuationDiagnosticV2{}, err
	}
	if preservedCensusSelector != "" && !safeSelector(preservedCensusSelector) {
		return ContinuationDiagnosticV2{}, errors.New("invalid preserved census selector")
	}
	v2 := ContinuationDiagnosticV2{
		ContinuationDiagnostic:  d,
		RecoverySelectorState:   "RECOVERY_SELECTOR_UNAVAILABLE",
		PreservedCensusSelector: preservedCensusSelector,
		TerminalStage:           d.Stage,
		TerminalStatus:          d.Status,
	}
	v2.SchemaVersion = ContinuationDiagnosticSchemaVersionV2
	if d.ObservedStage != "" {
		v2.LastSuccessfulStage = d.ObservedStage
		v2.TerminalStage = d.ObservedStage
		v2.TerminalStatus = d.ObservedStatus
	}
	return v2, nil
}

type ContinuationFailure struct {
	Cause   ContinuationCause
	Context ContinuationDiagnosticContext
}

func (e *ContinuationFailure) Error() string { return "continuation failed" }

func NewContinuationFailure(cause ContinuationCause, context ContinuationDiagnosticContext) error {
	if _, err := NewContinuationDiagnostic(cause, context); err != nil {
		return err
	}
	return &ContinuationFailure{Cause: cause, Context: context}
}

func ContinuationDiagnosticFromError(err error) (ContinuationDiagnostic, bool) {
	var failure *ContinuationFailure
	if !errors.As(err, &failure) {
		return ContinuationDiagnostic{}, false
	}
	diagnostic, buildErr := NewContinuationDiagnostic(failure.Cause, failure.Context)
	return diagnostic, buildErr == nil
}

type ContinuationDiagnostic struct {
	SchemaVersion         string           `json:"schema_version"`
	Status                string           `json:"status"`
	Code                  ContinuationCode `json:"code"`
	Phase                 string           `json:"phase"`
	Stage                 string           `json:"stage"`
	FailedField           string           `json:"failed_field"`
	Invariant             string           `json:"invariant"`
	CensusCommitPreserved bool             `json:"census_commit_preserved"`
	Retry                 bool             `json:"retry"`
	RetryAction           string           `json:"retry_action"`
	CallerAction          string           `json:"caller_action"`
	Guidance              string           `json:"guidance"`
	PublicSelector        string           `json:"public_selector,omitempty"`
	ExpectedStage         string           `json:"expected_stage,omitempty"`
	ExpectedStatus        string           `json:"expected_status,omitempty"`
	ObservedStage         string           `json:"observed_stage,omitempty"`
	ObservedStatus        string           `json:"observed_status,omitempty"`
	ConstructionStage     string           `json:"construction_stage,omitempty"`
	ConstructionReason    string           `json:"construction_reason,omitempty"`
	ResourceCategory      string           `json:"resource_category,omitempty"`
	ResourceObserved      int64            `json:"resource_observed,omitempty"`
	ResourceLimit         int64            `json:"resource_limit,omitempty"`
	Recovery              string           `json:"recovery,omitempty"`
}

func NewContinuationDiagnostic(cause ContinuationCause, ctx ContinuationDiagnosticContext) (ContinuationDiagnostic, error) {
	d := ContinuationDiagnostic{
		SchemaVersion:         ContinuationDiagnosticSchemaVersion,
		Status:                "FAILED",
		Phase:                 "CONTINUATION",
		CensusCommitPreserved: true,
		Retry:                 false,
		PublicSelector:        ctx.PublicSelector,
	}
	switch cause {
	case ContinuationHostConstructionFailed:
		if !validConstructionAttribution(ctx.ConstructionStage, ctx.ConstructionReason) {
			return ContinuationDiagnostic{}, errors.New("continuation construction attribution required")
		}
		d.ConstructionStage, d.ConstructionReason = ctx.ConstructionStage, ctx.ConstructionReason
		d.Code = CodeContinuationHostConstructionFailed
		d.Stage = "HOST_CONSTRUCTION"
		d.FailedField = "continuation_host"
		d.Invariant = "configured continuation host must construct before committed census continuation can start"
		d.RetryAction = "do not retry on this connection; census commit is preserved"
		d.CallerAction = "provision the continuation host, reconnect the project session, then resubmit the continuation request"
		d.Guidance = "Provision the secured continuation host configuration, reconnect only this project session, and resubmit; census commit is preserved."
	case ContinuationDescriptorUnavailable:
		d.Code = CodeContinuationDescriptorUnavailable
		d.Stage = "DESCRIPTOR_RESOLUTION"
		d.FailedField = "resume_selector"
		d.Invariant = "caller-supplied public resume selector must resolve to one available continuation descriptor"
		d.RetryAction = "do not retry unchanged; census commit is preserved"
		d.CallerAction = "submit the public selector returned by the paused result or provision the descriptor in the configured continuation store"
		d.Guidance = "Resume with the caller-supplied public selector from the PAUSED result; if unavailable, provision that descriptor in the configured host and retry."
	case ContinuationStopCheckpointMismatch:
		if ctx.ExpectedStage == "" || ctx.ExpectedStatus == "" || ctx.ObservedStage == "" || ctx.ObservedStatus == "" {
			return ContinuationDiagnostic{}, errors.New("continuation checkpoint coordinates required")
		}
		d.Code = CodeContinuationStopCheckpointMismatch
		d.Stage = "STOP_CHECKPOINT"
		d.FailedField = "checkpoint"
		d.Invariant = "stop-after checkpoint stage and status must match the requested safe stop"
		d.RetryAction = "do not resume from the mismatched checkpoint; census commit is preserved"
		d.CallerAction = "resume from the last public selector whose stage and status match the requested stop, or restart continuation from the committed census"
		d.Guidance = "Use a public selector for the expected stop stage and status, or restart continuation from the preserved census commit."
		d.ExpectedStage, d.ExpectedStatus = ctx.ExpectedStage, ctx.ExpectedStatus
		d.ObservedStage, d.ObservedStatus = ctx.ObservedStage, ctx.ObservedStatus
	case ContinuationCaptureFailed:
		if ctx.ObservedStage == "" || ctx.ObservedStatus != "FAILED_CAPTURE" || !validCaptureCoordinates(ctx.ResourceCategory, ctx.ResourceField, ctx.CaptureInvariant, ctx.CaptureCallerAction, ctx.CaptureRecovery, ctx.ResourceObserved, ctx.ResourceLimit) {
			return ContinuationDiagnostic{}, errors.New("continuation capture coordinates required")
		}
		d.Code = CodeContinuationCaptureFailed
		d.Stage = "CAPTURE"
		d.FailedField = ctx.ResourceField
		d.Invariant = ctx.CaptureInvariant
		d.RetryAction = "do not retry unchanged; census commit is preserved"
		d.CallerAction = ctx.CaptureCallerAction
		d.Recovery = ctx.CaptureRecovery
		d.Guidance = "Apply the reported bounded capture action, then restart continuation from the preserved census commit."
		if ctx.CaptureCallerAction == "RECONFIGURE_MANAGED_SOURCE_SUPPLY_THEN_RESUME" {
			d.Guidance = "Reconfigure managed source supply, then resume using an authorized continuation selector; do not retry unchanged."
		}
		d.ObservedStage, d.ObservedStatus = ctx.ObservedStage, ctx.ObservedStatus
		d.ResourceCategory, d.ResourceObserved, d.ResourceLimit = ctx.ResourceCategory, ctx.ResourceObserved, ctx.ResourceLimit
	case ContinuationPublicationFailed:
		d.Code = CodeContinuationPublicationFailed
		d.Stage = "DESCRIPTOR_PUBLICATION"
		d.FailedField = "continuation_descriptor"
		d.Invariant = "a continuation result must publish one public descriptor before it is resumable"
		d.RetryAction = "retry publication only after host storage is healthy; census commit is preserved"
		d.CallerAction = "repair or provision continuation publication custody, reconnect the project session, and resubmit"
		d.Guidance = "Restore secured continuation publication custody, reconnect only this project session, and resubmit from the preserved census commit."
	default:
		return ContinuationDiagnostic{}, errors.New("invalid continuation cause")
	}
	if err := validateContinuationDiagnostic(d); err != nil {
		return ContinuationDiagnostic{}, err
	}
	return d, nil
}

func validConstructionAttribution(stage, reason string) bool {
	stages := map[string]bool{"ROOT_OPEN": true, "STORE_PRIVATE_VALIDATION": true, "CONTRACT_CONSTRUCTION": true, "RUNTIME_ARITY": true, "WORKER_ASSEMBLY": true, "HOST_ASSEMBLY": true}
	reasons := map[string]bool{"ROOT_OPEN_FAILED": true, "STORE_PRIVATE_VALIDATION_FAILED": true, "CONTRACT_CONSTRUCTION_FAILED": true, "RUNTIME_ARITY_INVALID": true, "WORKER_ASSEMBLY_FAILED": true, "HOST_ASSEMBLY_FAILED": true}
	return stages[stage] && reasons[reason]
}

func validCaptureCoordinates(category, field, invariant, action, recovery string, observed, limit int64) bool {
	if recovery != "RESTART_FROM_PRESERVED_CENSUS_COMMIT" {
		return false
	}
	categoryOK := category == "input" || category == "output" || category == "unique_source"
	fieldOK := field == "graph_bytes" || field == "artifact_bytes" || field == "source_bytes" || field == "total_source_bytes" || field == "receipts" || field == "bindings" || field == "outcomes" || field == "work"
	if categoryOK && fieldOK && observed > limit && limit > 0 && invariant == "OBSERVED_MUST_NOT_EXCEED_LIMIT" && action == "INCREASE_BOUNDED_CAPTURE_LIMIT" {
		return true
	}
	if observed != 0 || limit != 0 || invariant != "CAPTURE_MUST_REACH_TERMINAL_CHECKPOINT" {
		return false
	}
	return (category == "availability" && field == "source_preparation" && (action == "RETRY_WITH_MANAGED_SOURCE_SUPPLY" || action == "RECONFIGURE_MANAGED_SOURCE_SUPPLY_THEN_RESUME")) ||
		(category == "internal" && field == "resolver" && action == "RETRY_OR_REPORT_CAPTURE_IMPLEMENTATION") ||
		(category == "internal" && field == "serialization" && action == "REPORT_CAPTURE_IMPLEMENTATION") ||
		(category == "internal" && field == "runtime_injection" && action == "FIX_CAPTURE_RUNTIME_INJECTION") ||
		(category == "internal" && field == "capture" && action == "RETRY_OR_REPORT_CAPTURE_IMPLEMENTATION")
}

func validateContinuationDiagnostic(d ContinuationDiagnostic) error {
	if d.SchemaVersion != ContinuationDiagnosticSchemaVersion || d.Status != "FAILED" || d.Phase != "CONTINUATION" || !d.CensusCommitPreserved || d.Stage == "" || d.FailedField == "" || d.Invariant == "" || d.RetryAction == "" || d.CallerAction == "" || d.Guidance == "" {
		return errors.New("invalid continuation diagnostic")
	}
	isConstruction := d.Code == CodeContinuationHostConstructionFailed
	if isConstruction != validConstructionAttribution(d.ConstructionStage, d.ConstructionReason) {
		return errors.New("invalid continuation construction diagnostic")
	}
	isMismatch := d.Code == CodeContinuationStopCheckpointMismatch
	isCapture := d.Code == CodeContinuationCaptureFailed
	if isMismatch != (d.ExpectedStage != "" && d.ExpectedStatus != "") {
		return errors.New("invalid continuation checkpoint diagnostic")
	}
	if isCapture != (d.ObservedStage != "" && d.ObservedStatus == "FAILED_CAPTURE" && validCaptureCoordinates(d.ResourceCategory, d.FailedField, d.Invariant, d.CallerAction, d.Recovery, d.ResourceObserved, d.ResourceLimit)) {
		return errors.New("invalid continuation capture diagnostic")
	}
	if !isMismatch && !isCapture && (d.ObservedStage != "" || d.ObservedStatus != "") {
		return errors.New("invalid continuation observed coordinates")
	}
	switch d.Code {
	case CodeContinuationHostConstructionFailed, CodeContinuationDescriptorUnavailable, CodeContinuationStopCheckpointMismatch, CodeContinuationPublicationFailed, CodeContinuationCaptureFailed:
		return nil
	default:
		return errors.New("invalid continuation diagnostic code")
	}
}

func MarshalContinuationDiagnostic(d ContinuationDiagnostic) ([]byte, error) {
	if err := validateContinuationDiagnostic(d); err != nil {
		return nil, err
	}
	return json.Marshal(d)
}

func DecodeContinuationDiagnostic(raw []byte) (ContinuationDiagnostic, error) {
	var d ContinuationDiagnostic
	if err := strictjson.RejectDuplicates(raw); err != nil {
		return d, errors.New("invalid continuation diagnostic JSON")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&d); err != nil {
		return d, err
	}
	if dec.Decode(&struct{}{}) == nil {
		return d, errors.New("trailing JSON")
	}
	if err := validateContinuationDiagnostic(d); err != nil {
		return d, err
	}
	return d, nil
}
