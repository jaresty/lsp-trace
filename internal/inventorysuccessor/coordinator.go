// Package inventorysuccessor owns the private, additive ADR 0012 successor head.
package inventorysuccessor

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math"
	"regexp"

	"lsp-trace/internal/censuscontinuation"
	"lsp-trace/internal/censussourceassembly"
	"lsp-trace/internal/censussourcecontinuation"
	"lsp-trace/internal/publication"
)

const (
	headVersion   = "lsp-trace.inventory-successor-head.private.v1"
	headNamespace = "inventory-successor/heads/private/v1"
)

// NewHeads binds successor state to its additive private namespace.
func NewHeads(root *publication.Root) (censussourcecontinuation.HeadStore, error) {
	return censussourceassembly.NewHeadsWithParser(root, headNamespace, ParseHeadRevision)
}

var ErrInvalid = errors.New("invalid private inventory successor")
var sha256Pattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

type Stage string

const (
	StageProgramC            Stage = "PROGRAM_C"
	StageDescribePreparation Stage = "DESCRIBE_PREPARATION"
)

type Outcome string

const (
	OutcomeAdvanced               Outcome = "ADVANCED"
	OutcomeCompleteProvisional    Outcome = "COMPLETE_PROVISIONAL"
	OutcomeWaitingEvidence        Outcome = "WAITING_EVIDENCE"
	OutcomeWaitingExternal        Outcome = "WAITING_EXTERNAL"
	OutcomeRetainedFailure        Outcome = "RETAINED_FAILURE"
	OutcomeConflict               Outcome = "CONFLICT"
	OutcomeIndeterminateExecution Outcome = "INDETERMINATE_EXECUTION"
	OutcomeCancelled              Outcome = "CANCELLED"
)

type EvidenceGap string

const (
	GapRepresentativeNominationDenominator EvidenceGap = "REPRESENTATIVE_NOMINATION_DENOMINATOR"
	GapCommunityRepresentativeLineage      EvidenceGap = "COMMUNITY_TO_REPRESENTATIVE_LINEAGE"
	GapRetainedSourceClosure               EvidenceGap = "RETAINED_SOURCE_CLOSURE"
	GapFullDefinitionBindings              EvidenceGap = "FULL_DEFINITION_DISPLAY_BINDINGS"
	GapCallerOccurrenceV3Custody           EvidenceGap = "CALLER_OCCURRENCE_V3_CUSTODY"
)

var describePreparationGaps = []EvidenceGap{
	GapRepresentativeNominationDenominator,
	GapCommunityRepresentativeLineage,
	GapRetainedSourceClosure,
	GapFullDefinitionBindings,
	GapCallerOccurrenceV3Custody,
}

type WorkflowIdentity struct {
	Profile                   string `json:"profile"`
	Contract                  string `json:"contract"`
	CensusReceiptSHA256       string `json:"census_receipt_sha256"`
	AvailabilityLedgerSHA256  string `json:"availability_ledger_sha256"`
	SourcePinSHA256           string `json:"source_pin_sha256"`
	ProgramCPlanSHA256        string `json:"program_c_plan_sha256"`
	ProgramCBindingsSHA256    string `json:"program_c_bindings_sha256"`
	PrivacyPolicySHA256       string `json:"privacy_policy_sha256"`
	RetentionPolicySHA256     string `json:"retention_policy_sha256"`
	ResourcePolicySHA256      string `json:"resource_policy_sha256"`
	ReplayPolicySHA256        string `json:"replay_policy_sha256"`
	QualificationPolicySHA256 string `json:"qualification_policy_sha256"`
}

type Request struct {
	WorkflowKey string
	HeadKey     string
	Identity    WorkflowIdentity
	ProgramC    censuscontinuation.ProgramCTransitionVerificationRequest
	Heads       censussourcecontinuation.HeadStore
}

type programCVerifier func(context.Context, censuscontinuation.ProgramCTransitionVerificationRequest) (censuscontinuation.ProgramCTransitionResult, error)

type Result struct {
	Outcome                Outcome
	Head                   censussourcecontinuation.HeadRevision
	PredecessorHead        string
	CurrentStage           Stage
	LastCompletedStage     Stage
	MissingEvidence        []EvidenceGap
	Authority              uint8
	Accepted               bool
	FeatureIdentity        string
	Completeness           string
	Effects                string
	ImplementationDecision string
	ProductionAdmission    string
	SourceGraphComplete    string
	AbsenceInference       string
}

type head struct {
	SchemaVersion          string           `json:"schema_version"`
	Selector               string           `json:"selector"`
	WorkflowKey            string           `json:"workflow_key"`
	Identity               WorkflowIdentity `json:"identity"`
	Authority              uint8            `json:"authority"`
	Generation             uint64           `json:"generation"`
	PredecessorHead        string           `json:"predecessor_head"`
	CurrentStage           Stage            `json:"current_stage"`
	LastCompletedStage     Stage            `json:"last_completed_stage"`
	Outcome                Outcome          `json:"outcome"`
	MissingEvidence        []EvidenceGap    `json:"missing_evidence"`
	Accepted               bool             `json:"accepted"`
	FeatureIdentity        string           `json:"feature_identity"`
	Completeness           string           `json:"completeness"`
	Effects                string           `json:"effects"`
	ImplementationDecision string           `json:"implementation_decision"`
	ProductionAdmission    string           `json:"production_admission"`
	SourceGraphComplete    string           `json:"source_graph_complete"`
	AbsenceInference       string           `json:"absence_inference"`
}

func digest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func strict(raw []byte, value any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return ErrInvalid
	}
	canonical, err := json.Marshal(value)
	if err != nil || !bytes.Equal(canonical, raw) {
		return ErrInvalid
	}
	return nil
}

func encodeHead(value *head) ([]byte, error) {
	value.Selector = ""
	unsigned, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	value.Selector = digest(unsigned)
	return json.Marshal(value)
}

func ParseHeadRevision(raw []byte) (censussourcecontinuation.HeadRevision, error) {
	if len(raw) == 0 {
		return censussourcecontinuation.HeadRevision{}, nil
	}
	var value head
	if strict(raw, &value) != nil || validateHead(value) != nil {
		return censussourcecontinuation.HeadRevision{}, ErrInvalid
	}
	copyValue := value
	canonical, err := encodeHead(&copyValue)
	if err != nil || !bytes.Equal(canonical, raw) {
		return censussourcecontinuation.HeadRevision{}, ErrInvalid
	}
	return censussourcecontinuation.HeadRevision{Generation: value.Generation, Selector: value.Selector}, nil
}

func validIdentity(value WorkflowIdentity) bool {
	return value.Profile == "CALLS_ONLY_FROZEN_CENSUS_V1" && value.Contract != "" &&
		sha256Pattern.MatchString(value.CensusReceiptSHA256) && sha256Pattern.MatchString(value.AvailabilityLedgerSHA256) &&
		sha256Pattern.MatchString(value.SourcePinSHA256) && sha256Pattern.MatchString(value.ProgramCPlanSHA256) &&
		sha256Pattern.MatchString(value.ProgramCBindingsSHA256) && sha256Pattern.MatchString(value.PrivacyPolicySHA256) &&
		sha256Pattern.MatchString(value.RetentionPolicySHA256) && sha256Pattern.MatchString(value.ResourcePolicySHA256) &&
		sha256Pattern.MatchString(value.ReplayPolicySHA256) && sha256Pattern.MatchString(value.QualificationPolicySHA256)
}

func bindingsDigest(value censuscontinuation.ProgramCTransitionBindings) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return digest(raw)
}

func identityBindsProgramC(identity WorkflowIdentity, request censuscontinuation.ProgramCTransitionVerificationRequest) bool {
	return request.Bindings.CensusReceiptSHA256 == identity.CensusReceiptSHA256 &&
		request.Bindings.LedgerSHA256 == identity.AvailabilityLedgerSHA256 && request.Bindings.PlanSHA256 == identity.ProgramCPlanSHA256 &&
		request.Bindings.ResourcePolicySHA256 == identity.ResourcePolicySHA256 && bindingsDigest(request.Bindings) == identity.ProgramCBindingsSHA256
}

func equalGapList(actual, expected []EvidenceGap) bool {
	if len(actual) != len(expected) {
		return false
	}
	for index := range actual {
		if actual[index] != expected[index] {
			return false
		}
	}
	return true
}

func validateHead(value head) error {
	if value.SchemaVersion != headVersion || value.WorkflowKey == "" || !validIdentity(value.Identity) || value.Authority != 0 ||
		value.Generation != 1 || value.Generation == math.MaxUint64 || !sha256Pattern.MatchString(value.Selector) ||
		!sha256Pattern.MatchString(value.PredecessorHead) || value.CurrentStage != StageDescribePreparation ||
		value.LastCompletedStage != StageProgramC || value.Outcome != OutcomeWaitingEvidence || !equalGapList(value.MissingEvidence, describePreparationGaps) ||
		value.Accepted || value.FeatureIdentity != "UNRESOLVED" || value.Completeness != "UNKNOWN" || value.Effects != "NONE" ||
		value.ImplementationDecision != "UNISSUED" || value.ProductionAdmission != "REFUSED_MISSING_EVIDENCE" ||
		value.SourceGraphComplete != "UNKNOWN" || value.AbsenceInference != "PROHIBITED" {
		return ErrInvalid
	}
	return nil
}

func resultFromHead(value head) Result {
	return Result{
		Outcome: value.Outcome, Head: censussourcecontinuation.HeadRevision{Generation: value.Generation, Selector: value.Selector},
		PredecessorHead: value.PredecessorHead, CurrentStage: value.CurrentStage, LastCompletedStage: value.LastCompletedStage,
		MissingEvidence: append([]EvidenceGap(nil), value.MissingEvidence...), Authority: value.Authority, Accepted: value.Accepted,
		FeatureIdentity: value.FeatureIdentity, Completeness: value.Completeness, Effects: value.Effects,
		ImplementationDecision: value.ImplementationDecision, ProductionAdmission: value.ProductionAdmission,
		SourceGraphComplete: value.SourceGraphComplete, AbsenceInference: value.AbsenceInference,
	}
}

func stopResult(outcome Outcome) Result {
	return Result{Outcome: outcome, Authority: 0, Accepted: false, FeatureIdentity: "UNRESOLVED", Completeness: "UNKNOWN", Effects: "NONE", ImplementationDecision: "UNISSUED", ProductionAdmission: "REFUSED_MISSING_EVIDENCE", SourceGraphComplete: "UNKNOWN", AbsenceInference: "PROHIBITED"}
}

// CoordinateNext performs at most one durable successor-head transition. It
// never constructs Describe packets and callers supply no stage or CAS head.
func CoordinateNext(ctx context.Context, request Request) (Result, error) {
	return coordinateNext(ctx, request, censuscontinuation.VerifyProgramCTransitionCompletion)
}

func coordinateNext(ctx context.Context, request Request, verify programCVerifier) (Result, error) {
	if ctx == nil || request.WorkflowKey == "" || request.HeadKey == "" || request.Heads == nil || verify == nil || !validIdentity(request.Identity) || !identityBindsProgramC(request.Identity, request.ProgramC) {
		return Result{}, ErrInvalid
	}
	if ctx.Err() != nil {
		return stopResult(OutcomeCancelled), nil
	}
	verified, err := verify(ctx, request.ProgramC)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return stopResult(OutcomeCancelled), nil
		}
		return Result{}, err
	}
	if verified.Status != censuscontinuation.ProgramCTransitionComplete || verified.CommittedPrefix != verified.ConstituentCount || verified.ConstituentCount <= 0 || !sha256Pattern.MatchString(verified.Head.Selector) {
		return Result{}, ErrInvalid
	}
	predecessorHead := verified.Head.Selector
	raw, err := request.Heads.Load(ctx, request.HeadKey)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return stopResult(OutcomeCancelled), nil
		}
		return Result{}, err
	}
	if len(raw) != 0 {
		var existing head
		if strict(raw, &existing) != nil || validateHead(existing) != nil || existing.WorkflowKey != request.WorkflowKey || existing.Identity != request.Identity || existing.PredecessorHead != predecessorHead {
			return Result{}, ErrInvalid
		}
		copyValue := existing
		canonical, encodeErr := encodeHead(&copyValue)
		if encodeErr != nil || !bytes.Equal(canonical, raw) {
			return Result{}, ErrInvalid
		}
		return resultFromHead(existing), nil
	}
	value := head{
		SchemaVersion: headVersion, WorkflowKey: request.WorkflowKey, Identity: request.Identity, Authority: 0, Generation: 1,
		PredecessorHead: predecessorHead, CurrentStage: StageDescribePreparation, LastCompletedStage: StageProgramC,
		Outcome: OutcomeWaitingEvidence, MissingEvidence: append([]EvidenceGap(nil), describePreparationGaps...), Accepted: false,
		FeatureIdentity: "UNRESOLVED", Completeness: "UNKNOWN", Effects: "NONE", ImplementationDecision: "UNISSUED",
		ProductionAdmission: "REFUSED_MISSING_EVIDENCE", SourceGraphComplete: "UNKNOWN", AbsenceInference: "PROHIBITED",
	}
	if ctx.Err() != nil {
		return stopResult(OutcomeCancelled), nil
	}
	encoded, err := encodeHead(&value)
	if err != nil || validateHead(value) != nil {
		return Result{}, ErrInvalid
	}
	cas := request.Heads.CompareAndSwap(ctx, request.HeadKey, censussourcecontinuation.HeadRevision{}, encoded)
	switch cas.Outcome {
	case censussourcecontinuation.CASCommitted:
		if cas.Err != nil {
			return Result{}, ErrInvalid
		}
		return resultFromHead(value), nil
	case censussourcecontinuation.CASNotCommitted:
		if errors.Is(cas.Err, context.Canceled) || errors.Is(cas.Err, context.DeadlineExceeded) {
			return stopResult(OutcomeCancelled), nil
		}
		if cas.Err != nil {
			return Result{}, cas.Err
		}
		return stopResult(OutcomeConflict), nil
	case censussourcecontinuation.CASCommitUncertain:
		return stopResult(OutcomeIndeterminateExecution), nil
	default:
		if errors.Is(cas.Err, context.Canceled) || errors.Is(cas.Err, context.DeadlineExceeded) {
			return stopResult(OutcomeIndeterminateExecution), nil
		}
		if cas.Err != nil {
			return Result{}, cas.Err
		}
		return Result{}, ErrInvalid
	}
}
