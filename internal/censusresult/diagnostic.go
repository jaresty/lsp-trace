package censusresult

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"

	"lsp-trace/internal/strictjson"
)

const DiagnosticSchemaVersion = "lsp-trace.census-diagnostic.v1"

type Stage string
type Code string

const (
	StageSyntax      Stage = "syntax"
	StageConfig      Stage = "config"
	StageDiscovery   Stage = "discovery"
	StageAcquisition Stage = "acquisition"
	StageAssembly    Stage = "assembly"
	StagePublication Stage = "publication"
	StageCommitted   Stage = "committed-degradation"

	CodeInvalidSyntax               Code = "INVALID_SYNTAX"
	CodeInvalidConfig               Code = "INVALID_CONFIG"
	CodeDiscoveryFailed             Code = "DISCOVERY_FAILED"
	CodeAcquisitionFailed           Code = "ACQUISITION_FAILED"
	CodeAssemblyFailed              Code = "ASSEMBLY_FAILED"
	CodePublicationFailed           Code = "PUBLICATION_FAILED"
	CodeCommittedDegraded           Code = "COMMITTED_DEGRADED"
	CodeContinuationHostUnavailable Code = "CONTINUATION_HOST_UNAVAILABLE"
)

type PublicationFailureStage string
type PublicationAccountingCategory string

const MaxPublicationDiagnosticCount uint64 = 1 << 34

const (
	PublicationAccountingUnknown        PublicationAccountingCategory = "UNKNOWN"
	PublicationAccountingCandidateBytes PublicationAccountingCategory = "CANDIDATE_BYTES"
	PublicationAccountingCodecBytes     PublicationAccountingCategory = "CODEC_LIMIT_BYTES"
	PublicationAccountingConstituents   PublicationAccountingCategory = "CONSTITUENT_COUNT"
)

type PublicationAccounting struct {
	Category PublicationAccountingCategory `json:"category"`
	Observed *uint64                       `json:"observed,omitempty"`
	Limit    *uint64                       `json:"limit,omitempty"`
}

const (
	PublicationFailureRootOpen          PublicationFailureStage = "ROOT_OPEN"
	PublicationFailurePrivateValidation PublicationFailureStage = "PRIVATE_VALIDATION"
	PublicationFailureEncode            PublicationFailureStage = "ENCODE"
	PublicationFailureCanonicalize      PublicationFailureStage = "CANONICALIZE"
	PublicationFailureTempWrite         PublicationFailureStage = "TEMP_WRITE"
	PublicationFailureFsync             PublicationFailureStage = "FSYNC"
	PublicationFailureNoReplace         PublicationFailureStage = "NO_REPLACE"
	PublicationFailureVerify            PublicationFailureStage = "VERIFY"
	PublicationFailureReceipt           PublicationFailureStage = "RECEIPT"
	PublicationFailureCleanup           PublicationFailureStage = "CLEANUP"
	PublicationFailureInternal          PublicationFailureStage = "INTERNAL"
)

func validPublicationFailureStage(stage PublicationFailureStage) bool {
	switch stage {
	case PublicationFailureRootOpen, PublicationFailurePrivateValidation, PublicationFailureEncode, PublicationFailureCanonicalize, PublicationFailureTempWrite, PublicationFailureFsync, PublicationFailureNoReplace, PublicationFailureVerify, PublicationFailureReceipt, PublicationFailureCleanup, PublicationFailureInternal:
		return true
	default:
		return false
	}
}

type Diagnostic struct {
	SchemaVersion         string                  `json:"schema_version"`
	Status                string                  `json:"status"`
	Stage                 Stage                   `json:"stage"`
	Code                  Code                    `json:"code"`
	BatchOrdinal          *int                    `json:"batch_ordinal,omitempty"`
	Retry                 bool                    `json:"retry"`
	PublicationFailure    PublicationFailureStage `json:"publication_failure,omitempty"`
	PublicationAccounting *PublicationAccounting  `json:"publication_accounting,omitempty"`
	// Detail is an optional, human-readable, non-authoritative explanation of
	// which specific check produced the failure. It never changes the stage or
	// code contract; it exists so an opaque stage code (for example
	// INVALID_CONFIG) can name its actionable cause. Omitted when empty.
	Detail string `json:"detail,omitempty"`
}

// WithDetail returns a copy of d annotated with an actionable failure reason.
func (d Diagnostic) WithDetail(detail string) Diagnostic {
	d.Detail = detail
	return d
}

func NewContinuationHostUnavailableDiagnostic() Diagnostic {
	return Diagnostic{
		SchemaVersion: DiagnosticSchemaVersion,
		Status:        "SUCCEEDED_DEGRADED",
		Stage:         StageCommitted,
		Code:          CodeContinuationHostUnavailable,
		Retry:         false,
		Detail:        "continuation host capability requires bootstrap continuation configuration; census commit is preserved; provision the host, reconnect, and resubmit; do not retry on this connection",
	}
}

func NewDiagnostic(stage Stage, batchOrdinal *int) (Diagnostic, error) {
	codes := map[Stage]Code{StageSyntax: CodeInvalidSyntax, StageConfig: CodeInvalidConfig, StageDiscovery: CodeDiscoveryFailed, StageAcquisition: CodeAcquisitionFailed, StageAssembly: CodeAssemblyFailed, StagePublication: CodePublicationFailed, StageCommitted: CodeCommittedDegraded}
	code, ok := codes[stage]
	if !ok || batchOrdinal != nil && (*batchOrdinal < 0 || stage != StageAcquisition) {
		return Diagnostic{}, errors.New("invalid census diagnostic")
	}
	d := Diagnostic{SchemaVersion: DiagnosticSchemaVersion, Status: "FAILED", Stage: stage, Code: code, BatchOrdinal: batchOrdinal, Retry: stage != StageCommitted}
	if stage == StageCommitted {
		d.Status = "SUCCEEDED_DEGRADED"
	}
	return d, nil
}

func MarshalDiagnostic(d Diagnostic) ([]byte, error) {
	expected, err := NewDiagnostic(d.Stage, d.BatchOrdinal)
	if d.Stage == StageCommitted && d.Code == CodeContinuationHostUnavailable {
		expected = NewContinuationHostUnavailableDiagnostic()
	}
	publicationFailureValid := d.PublicationFailure == "" || d.Stage == StagePublication && validPublicationFailureStage(d.PublicationFailure)
	accountingValid := validPublicationAccounting(d.Stage, d.PublicationAccounting)
	if err != nil || !publicationFailureValid || !accountingValid || d.SchemaVersion != expected.SchemaVersion || d.Status != expected.Status || d.Stage != expected.Stage || d.Code != expected.Code || d.Retry != expected.Retry {
		if err == nil {
			err = errors.New("invalid census diagnostic")
		}
		return nil, err
	}
	b, err := json.Marshal(d)
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

func validPublicationAccounting(stage Stage, a *PublicationAccounting) bool {
	if a == nil {
		return true
	}
	if stage != StagePublication {
		return false
	}
	switch a.Category {
	case PublicationAccountingUnknown:
		return a.Observed == nil && a.Limit == nil
	case PublicationAccountingCandidateBytes, PublicationAccountingCodecBytes, PublicationAccountingConstituents:
		if a.Observed == nil || *a.Observed > MaxPublicationDiagnosticCount {
			return false
		}
		return a.Limit == nil || *a.Limit <= MaxPublicationDiagnosticCount
	default:
		return false
	}
}

func DecodeDiagnostic(raw []byte) (Diagnostic, error) {
	if err := strictjson.RejectDuplicates(raw); err != nil {
		return Diagnostic{}, errors.New("invalid census diagnostic")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var d Diagnostic
	if err := dec.Decode(&d); err != nil {
		return Diagnostic{}, errors.New("invalid census diagnostic")
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return Diagnostic{}, errors.New("invalid census diagnostic")
	}
	if _, err := MarshalDiagnostic(d); err != nil {
		return Diagnostic{}, err
	}
	return d, nil
}
