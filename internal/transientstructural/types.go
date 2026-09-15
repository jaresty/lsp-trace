// Package transientstructural computes bounded, privacy-projected structural
// claims directly from one exact managed language-server session generation.
// Results are transient observations: they are not retained evidence, are not
// replayable, and confer no authority beyond the qualified execution.
package transientstructural

import (
	"fmt"

	"lsp-trace/internal/graph"
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

type Target struct {
	URI       string
	Symbol    string
	Line      *uint32
	Character *uint32
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
	Analysis         AnalysisRequest
}

type Witness struct {
	Direction Direction `json:"direction"`
	Depth     int       `json:"depth"`
}

type NodeFact struct {
	ID        string      `json:"id"`
	Witnesses []Witness   `json:"witnesses"`
	Name      string      `json:"-"`
	Kind      int         `json:"-"`
	URI       string      `json:"-"`
	Range     graph.Range `json:"-"`
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
	SchemaVersion       string         `json:"schema_version"`
	EvidenceClass       string         `json:"evidence_class"`
	Authority           int            `json:"authority"`
	SourceGraphComplete string         `json:"source_graph_complete"`
	Retained            bool           `json:"retained"`
	Replayable          bool           `json:"replayable"`
	PublicationEligible bool           `json:"publication_eligible"`
	HydrationEligible   bool           `json:"hydration_eligible"`
	ClaimCeiling        string         `json:"claim_ceiling"`
	Phase               Phase          `json:"phase"`
	State               TerminalState  `json:"state"`
	Qualification       Qualification  `json:"qualification"`
	TargetID            string         `json:"target_id"`
	GraphDigest         string         `json:"graph_digest"`
	Policy              PolicyBinding  `json:"policy"`
	Bounds              BoundsBinding  `json:"bounds"`
	Accounting          Accounting     `json:"accounting"`
	Analysis            AnalysisResult `json:"analysis"`
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

type DomainFailure struct {
	Phase      Phase         `json:"phase"`
	State      TerminalState `json:"state"`
	Accounting Accounting    `json:"accounting"`
}

func (f *DomainFailure) Error() string {
	if f == nil {
		return ""
	}
	return fmt.Sprintf("transient structural execution: %s/%s", f.Phase, f.State)
}
