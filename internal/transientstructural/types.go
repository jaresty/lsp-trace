// Package transientstructural computes bounded, privacy-projected structural
// claims directly from one exact managed language-server session generation.
// Results are transient observations: they are not retained evidence, are not
// replayable, and confer no authority beyond the qualified execution.
package transientstructural

import (
	"fmt"

	"lsp-trace/internal/graph"
	"lsp-trace/sessionruntime"
)

type Phase string

const (
	PhasePreflight     Phase = "PREFLIGHT"
	PhaseTraversal     Phase = "TRAVERSAL"
	PhaseAdmission     Phase = "ADMISSION"
	PhaseAnalysis      Phase = "ANALYSIS"
	PhaseDeliveryCheck Phase = "DELIVERY_CHECK"
)

type TerminalState string

const (
	StateComplete              TerminalState = "COMPLETE"
	StateEmpty                 TerminalState = "EMPTY"
	StateUnsupported           TerminalState = "UNSUPPORTED"
	StateAmbiguousTarget       TerminalState = "AMBIGUOUS_TARGET"
	StateTargetNotFound        TerminalState = "TARGET_NOT_FOUND"
	StatePartial               TerminalState = "PARTIAL"
	StateTruncated             TerminalState = "TRUNCATED"
	StateResourceLimit         TerminalState = "RESOURCE_LIMIT"
	StateTimeout               TerminalState = "TIMEOUT"
	StateCancelled             TerminalState = "CANCELLED"
	StateGenerationChanged     TerminalState = "GENERATION_CHANGED"
	StateInvalidServerResponse TerminalState = "INVALID_SERVER_RESPONSE"
	StateAnalysisFailed        TerminalState = "ANALYSIS_FAILED"
)

type AnalysisKind string

const (
	AnalysisNeighborhood AnalysisKind = "NEIGHBORHOOD"
	AnalysisImpact       AnalysisKind = "IMPACT"
)

type Direction string

const (
	DirectionRoot     Direction = "ROOT"
	DirectionIncoming Direction = "INCOMING"
	DirectionOutgoing Direction = "OUTGOING"
)

type OmissionReason string

const (
	OmissionDepthBound      OmissionReason = "DEPTH_BOUND"
	OmissionNodeBound       OmissionReason = "NODE_BOUND"
	OmissionRequestBound    OmissionReason = "REQUEST_BOUND"
	OmissionTimeout         OmissionReason = "TIMEOUT"
	OmissionCancellation    OmissionReason = "CANCELLATION"
	OmissionUnsupported     OmissionReason = "UNSUPPORTED_RESPONSE"
	OmissionInvalidResponse OmissionReason = "MALFORMED_RESPONSE"
	OmissionDuplicate       OmissionReason = "DEDUPLICATION"
)

type RegexLocator struct {
	Pattern          string
	MatchIndex       int
	CaptureGroup     int
	ExpectedDigest   string
	MaxDocumentBytes int
	MaxMatches       int
	MaxPatternBytes  int
	MaxWork          int
}

type Target struct {
	URI       string
	Symbol    string
	Line      *uint32
	Character *uint32
	Regex     *RegexLocator
}

type AnalysisRequest struct {
	Kind      AnalysisKind
	Direction Direction
	MaxDepth  int
}

type Request struct {
	SessionID        string
	Generation       uint64
	LanguageID       string
	Target           Target
	UpDepth          int
	DownDepth        int
	MaxNodes         int
	TimeoutMS        int64
	RequestTimeoutMS int64
	MaxMessages      int
	MaxBytes         int64
	CaptureSupply    bool
	SourceOnlyTarget bool
	Analysis         AnalysisRequest
}

type Witness struct {
	Direction Direction `json:"direction"`
	Depth     int       `json:"depth"`
}

type NodeFact struct {
	ID             string      `json:"id"`
	Witnesses      []Witness   `json:"witnesses"`
	Name           string      `json:"-"`
	Kind           int         `json:"-"`
	URI            string      `json:"-"`
	Range          graph.Range `json:"-"`
	ItemRange      graph.Range `json:"-"`
	SelectionRange graph.Range `json:"-"`
}

type OccurrenceFact struct {
	ID        string      `json:"id"`
	CallerID  string      `json:"caller_id"`
	CalleeID  string      `json:"callee_id"`
	Witnesses []Witness   `json:"witnesses"`
	URI       string      `json:"-"`
	Range     graph.Range `json:"-"`
}

type AnalysisResult struct {
	Kind        AnalysisKind     `json:"kind"`
	Direction   Direction        `json:"direction,omitempty"`
	MaxDepth    int              `json:"max_depth"`
	Nodes       []NodeFact       `json:"nodes"`
	Occurrences []OccurrenceFact `json:"occurrences"`
}

type RequestAccounting struct {
	Attempted int `json:"attempted"`
	Succeeded int `json:"succeeded"`
	Failed    int `json:"failed"`
	Cancelled int `json:"cancelled"`
}

type PreparationAccounting struct {
	Attempted int `json:"attempted"`
	Returned  int `json:"returned"`
	Empty     int `json:"empty"`
	Failed    int `json:"failed"`
}

type AdmissionAccounting struct {
	Observed int `json:"observed"`
	Admitted int `json:"admitted"`
	Rejected int `json:"rejected"`
	Omitted  int `json:"omitted"`
}

type FrontierAccounting struct {
	Observed   int `json:"observed"`
	Expanded   int `json:"expanded"`
	Unexpanded int `json:"unexpanded"`
}

type OmissionCount struct {
	Reason OmissionReason `json:"reason"`
	Count  int            `json:"count"`
}

type Accounting struct {
	Requests    RequestAccounting     `json:"requests"`
	Preparation PreparationAccounting `json:"preparation"`
	Nodes       AdmissionAccounting   `json:"nodes"`
	Occurrences AdmissionAccounting   `json:"occurrences"`
	Frontier    FrontierAccounting    `json:"frontier"`
	Omissions   []OmissionCount       `json:"omissions"`
}

type PolicyBinding struct {
	LifecycleID      string `json:"lifecycle_id"`
	LifecycleVersion string `json:"lifecycle_version"`
	LifecycleSHA256  string `json:"lifecycle_sha256"`
	PrivacyID        string `json:"privacy_id"`
	PrivacyVersion   string `json:"privacy_version"`
	PrivacySHA256    string `json:"privacy_sha256"`
	IdentityID       string `json:"identity_id"`
	IdentityVersion  string `json:"identity_version"`
	IdentitySHA256   string `json:"identity_sha256"`
	AnalysisID       string `json:"analysis_id"`
	AnalysisVersion  string `json:"analysis_version"`
	AnalysisSHA256   string `json:"analysis_sha256"`
}

type Qualification struct {
	SessionID        string `json:"session_id"`
	Generation       uint64 `json:"generation"`
	PositionEncoding string `json:"position_encoding"`
}

type Result struct {
	SchemaVersion       string                         `json:"schema_version"`
	EvidenceClass       string                         `json:"evidence_class"`
	Authority           int                            `json:"authority"`
	SourceGraphComplete string                         `json:"source_graph_complete"`
	Retained            bool                           `json:"retained"`
	Replayable          bool                           `json:"replayable"`
	PublicationEligible bool                           `json:"publication_eligible"`
	HydrationEligible   bool                           `json:"hydration_eligible"`
	ClaimCeiling        string                         `json:"claim_ceiling"`
	Phase               Phase                          `json:"phase"`
	State               TerminalState                  `json:"state"`
	Qualification       Qualification                  `json:"qualification"`
	TargetID            string                         `json:"target_id"`
	GraphDigest         string                         `json:"graph_digest"`
	Policy              PolicyBinding                  `json:"policy"`
	Bounds              BoundsBinding                  `json:"bounds"`
	Accounting          Accounting                     `json:"accounting"`
	Analysis            AnalysisResult                 `json:"analysis"`
	SourceSupply        *sessionruntime.DocumentSupply `json:"-"`
}

type BoundsBinding struct {
	UpDepth          int          `json:"up_depth"`
	DownDepth        int          `json:"down_depth"`
	MaxNodes         int          `json:"max_nodes"`
	TimeoutMS        int64        `json:"timeout_ms"`
	RequestTimeoutMS int64        `json:"request_timeout_ms"`
	MaxMessages      int          `json:"max_messages"`
	MaxBytes         int64        `json:"max_bytes"`
	AnalysisKind     AnalysisKind `json:"analysis_kind"`
	Direction        Direction    `json:"direction,omitempty"`
	AnalysisMaxDepth int          `json:"analysis_max_depth"`
}

type TraversalStage string

const (
	TraversalStagePrepare  TraversalStage = "PREPARE"
	TraversalStageOutgoing TraversalStage = "OUTGOING"
	TraversalStageIncoming TraversalStage = "INCOMING"
)

type TraversalDiagnostic struct {
	Stage             TraversalStage `json:"stage"`
	Method            string         `json:"method"`
	Direction         Direction      `json:"direction,omitempty"`
	Depth             *int           `json:"depth,omitempty"`
	ProviderMethod    string         `json:"provider_method,omitempty"`
	ItemIndex         *int           `json:"item_index,omitempty"`
	FailedField       string         `json:"failed_field,omitempty"`
	FailedInvariant   string         `json:"failed_invariant,omitempty"`
	ProviderVariant   string         `json:"provider_variant,omitempty"`
	ProjectionEntered *bool          `json:"projection_entered,omitempty"`
	Guidance          string         `json:"guidance,omitempty"`
}

type TargetAction string

const (
	TargetActionFailAbsent           TargetAction = "FAIL_ABSENT"
	TargetActionEnumerationTruncated TargetAction = "ENUMERATION_TRUNCATED"
	TargetActionFailAmbiguous        TargetAction = "FAIL_AMBIGUOUS"
	TargetActionFailMalformed        TargetAction = "FAIL_MALFORMED"
	TargetActionFailMismatch         TargetAction = "FAIL_MISMATCH"
	TargetActionFailUnsupported      TargetAction = "FAIL_UNSUPPORTED"
	TargetActionFailDocument         TargetAction = "FAIL_DOCUMENT_SYMBOLS"
	TargetActionFailUnpreparable     TargetAction = "FAIL_UNPREPARABLE"
)

type TargetCandidatePosition struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type TargetCandidateRange struct {
	Start TargetCandidatePosition `json:"start"`
	End   TargetCandidatePosition `json:"end"`
}

// TargetCandidate contains only server-returned workspace-contained location metadata.
type TargetCandidate struct {
	URI       string               `json:"uri"`
	Name      string               `json:"name"`
	Kind      int                  `json:"kind"`
	Container string               `json:"container,omitempty"`
	Range     TargetCandidateRange `json:"range"`
}

type TargetCandidateAccounting struct {
	Observed     *int `json:"observed,omitempty"`
	Accepted     int  `json:"accepted"`
	Returned     int  `json:"returned"`
	Excluded     int  `json:"excluded"`
	Deduplicated int  `json:"deduplicated"`
	Truncated    int  `json:"truncated"`
}

type TargetRecovery struct {
	Kind              string         `json:"kind"`
	URI               string         `json:"uri,omitempty"`
	Line              *int           `json:"line,omitempty"`
	Character         *int           `json:"character,omitempty"`
	RequiredPattern   bool           `json:"required_pattern,omitempty"`
	MatchIndex        *int           `json:"match_index,omitempty"`
	RequiredFields    []string       `json:"required_fields,omitempty"`
	Limits            map[string]int `json:"limits,omitempty"`
	UnavailableReason string         `json:"unavailable_reason,omitempty"`
	Complete          *bool          `json:"complete,omitempty"`
	RequestFragment   map[string]any `json:"request_fragment,omitempty"`
	OmittedFields     []string       `json:"omitted_fields,omitempty"`
}

type TargetDiagnostic struct {
	ExactMatches        int                        `json:"exact_matches"`
	TotalSymbols        int                        `json:"total_symbols"`
	OmittedSymbols      int                        `json:"omitted_symbols"`
	Action              TargetAction               `json:"action"`
	Candidates          []TargetCandidate          `json:"candidates,omitempty"`
	CandidateAccounting *TargetCandidateAccounting `json:"candidate_accounting,omitempty"`
	ProviderMethod      string                     `json:"provider_method,omitempty"`
	ProviderAdapter     string                     `json:"provider_adapter,omitempty"`
	ItemIndex           *int                       `json:"item_index,omitempty"`
	NormalizationStage  string                     `json:"normalization_stage,omitempty"`
	FailedField         string                     `json:"failed_field,omitempty"`
	FailedInvariant     string                     `json:"failed_invariant,omitempty"`
	ProjectionEntered   *bool                      `json:"projection_entered,omitempty"`
	LocatorScope        string                     `json:"locator_scope,omitempty"`
	Guidance            string                     `json:"guidance,omitempty"`
	Recovery            *TargetRecovery            `json:"recovery,omitempty"`
	Recoveries          []TargetRecovery           `json:"recoveries,omitempty"`
	Completeness        string                     `json:"completeness,omitempty"`
}

type ResourceReason string

const (
	ResourceReasonNodeBound             ResourceReason = "NODE_BOUND"
	ResourceReasonRegexMaxDocumentBytes ResourceReason = "REGEX_MAX_DOCUMENT_BYTES"
	ResourceReasonRegexMaxPatternBytes  ResourceReason = "REGEX_MAX_PATTERN_BYTES"
	ResourceReasonRegexMaxWork          ResourceReason = "REGEX_MAX_WORK"
	ResourceReasonRegexMaxMatches       ResourceReason = "REGEX_MAX_MATCHES"
)

type ResourceField string

const (
	ResourceFieldMaxNodes              ResourceField = "max_nodes"
	ResourceFieldRegexMaxDocumentBytes ResourceField = "regex_locator.limits.max_document_bytes"
	ResourceFieldRegexMaxPatternBytes  ResourceField = "regex_locator.limits.max_pattern_bytes"
	ResourceFieldRegexMaxWork          ResourceField = "regex_locator.limits.max_work"
	ResourceFieldRegexMaxMatches       ResourceField = "regex_locator.limits.max_matches"
)

type SuggestedLimit struct {
	Field     ResourceField `json:"field"`
	Current   int           `json:"current"`
	Observed  int           `json:"observed"`
	Suggested int           `json:"suggested"`
}

type RegexLimitsFragment struct {
	MaxDocumentBytes int `json:"max_document_bytes,omitempty"`
	MaxPatternBytes  int `json:"max_pattern_bytes,omitempty"`
	MaxWork          int `json:"max_work,omitempty"`
	MaxMatches       int `json:"max_matches,omitempty"`
}

type RegexLocatorFragment struct {
	Limits RegexLimitsFragment `json:"limits"`
}

type BoundedRequestFragment struct {
	MaxNodes     int                   `json:"max_nodes,omitempty"`
	RegexLocator *RegexLocatorFragment `json:"regex_locator,omitempty"`
}

type ResourceDiagnostic struct {
	Reason          ResourceReason          `json:"reason"`
	Field           ResourceField           `json:"field"`
	Allowed         int                     `json:"allowed"`
	Observed        *int                    `json:"observed,omitempty"`
	MaximumAllowed  int                     `json:"maximum_allowed,omitempty"`
	SuggestedLimit  *int                    `json:"suggested_limit,omitempty"`
	SuggestedLimits []SuggestedLimit        `json:"suggested_limits,omitempty"`
	CallerAction    string                  `json:"caller_action,omitempty"`
	RequestFragment *BoundedRequestFragment `json:"request_fragment,omitempty"`
}

type FailureReason string

const (
	FailureReasonNoRegexMatch      FailureReason = "NO_REGEX_MATCH"
	FailureReasonSourceUnavailable FailureReason = "SOURCE_UNAVAILABLE"
	FailureReasonPrepareFailed     FailureReason = "PREPARE_FAILED"
	FailureReasonTraversalFailed   FailureReason = "TRAVERSAL_FAILED"
)

type DomainFailure struct {
	Phase               Phase                `json:"phase"`
	State               TerminalState        `json:"state"`
	Reason              FailureReason        `json:"reason,omitempty"`
	Accounting          Accounting           `json:"accounting"`
	TraversalDiagnostic *TraversalDiagnostic `json:"traversal_diagnostic,omitempty"`
	TargetDiagnostic    *TargetDiagnostic    `json:"target_diagnostic,omitempty"`
	ResourceDiagnostic  *ResourceDiagnostic  `json:"resource_diagnostic,omitempty"`
}

func (f *DomainFailure) Error() string {
	if f == nil {
		return ""
	}
	return fmt.Sprintf("transient structural execution: %s/%s", f.Phase, f.State)
}
