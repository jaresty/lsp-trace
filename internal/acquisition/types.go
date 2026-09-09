// Package acquisition coordinates bounded, multi-target native call-hierarchy
// acquisition. It is preparatory infrastructure, not a public FR20 operation.
package acquisition

import (
	"context"
	"encoding/json"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/retainedpath"
	"time"
)

type Mode string

const (
	Slice    Mode = "SLICE"
	Incoming Mode = "INCOMING"
)

type Direction string

const (
	Outgoing          Direction = "OUTGOING"
	IncomingDirection Direction = "INCOMING"
)

type ResolutionStatus string

const (
	Resolved              ResolutionStatus = "RESOLVED"
	Missing               ResolutionStatus = "MISSING"
	Ambiguous             ResolutionStatus = "AMBIGUOUS"
	ResolutionFailed      ResolutionStatus = "FAILED"
	ResolutionUnsupported ResolutionStatus = "UNSUPPORTED"
	ResolutionBlocked     ResolutionStatus = "BUDGET_BLOCKED"
)

type AdmissionStatus string

const (
	Admitted         AdmissionStatus = "ADMITTED"
	AdmissionBlocked AdmissionStatus = "BUDGET_BLOCKED"
	NotApplicable    AdmissionStatus = "NOT_APPLICABLE"
)

type ExpansionStatus string

const (
	SuccessEmpty           ExpansionStatus = "SUCCESS_EMPTY"
	SuccessNonempty        ExpansionStatus = "SUCCESS_NONEMPTY"
	Partial                ExpansionStatus = "PARTIAL"
	Frontier               ExpansionStatus = "FRONTIER"
	Failed                 ExpansionStatus = "FAILED"
	Unsupported            ExpansionStatus = "UNSUPPORTED"
	BudgetBlocked          ExpansionStatus = "BUDGET_BLOCKED"
	ExpansionNotApplicable ExpansionStatus = "NOT_APPLICABLE"
)

// Locator positions are zero-based in the declared session position encoding.
// Symbol and the complete Line/Character pair are mutually exclusive.
type Locator struct {
	URI        string  `json:"uri"`
	Symbol     string  `json:"symbol,omitempty"`
	Line       *uint32 `json:"line,omitempty"`
	Character  *uint32 `json:"character,omitempty"`
	LanguageID string  `json:"language_id,omitempty"`
}
type Target struct {
	ID        string  `json:"id"`
	Locator   Locator `json:"locator"`
	DownDepth int     `json:"down_depth"`
	UpDepth   int     `json:"up_depth"`
}
type AcquisitionContext struct {
	ID               string `json:"id"`
	SessionID        string `json:"session_id"`
	Generation       uint64 `json:"generation"`
	PositionEncoding string `json:"position_encoding"`
}
type Limits struct {
	MaxNodes         int           `json:"max_nodes"`
	MaxRequests      int           `json:"max_requests"`
	MaxEvidenceBytes int           `json:"max_evidence_bytes"`
	MaxPathWork      int           `json:"max_path_work"`
	Timeout          time.Duration `json:"timeout"`
	RequestTimeout   time.Duration `json:"request_timeout"`
	MaxResponseBytes int           `json:"max_response_bytes"`
	MaxMessages      int           `json:"max_messages"`
}
type Request struct {
	Mode            Mode               `json:"mode"`
	Context         AcquisitionContext `json:"context"`
	Root            Target             `json:"root"`
	RequiredTargets []Target           `json:"required_targets"`
	Limits          Limits             `json:"limits"`
	TopmostSiblings bool               `json:"topmost_siblings,omitempty"`
}

// Client must be bound to Request.Context, enforce wire byte/message limits and
// honor context cancellation. A provider null response is successful-empty.
// Each call row requires nonnil FromRanges: [] is valid zero-site evidence,
// nil is malformed (a direct typed client's row is retained only as partial
// response accounting, never as a supported edge). Wire clients must distinguish
// missing/null/scalar/object fields before decoding into Go slices.
// NewWireClient supplies the neutral bounded adapter; an existing SessionClient
// also implements this interface when constructed with the declared wire limits.
type Client interface {
	DocumentSymbols(context.Context, lsp.DocumentSymbolParams) ([]lsp.DocumentSymbol, error)
	PrepareCallHierarchy(context.Context, lsp.PrepareCallHierarchyParams) ([]lsp.CallHierarchyItem, error)
	IncomingCalls(context.Context, lsp.CallHierarchyItem) ([]lsp.CallHierarchyIncomingCall, bool, error)
	OutgoingCalls(context.Context, lsp.CallHierarchyItem) ([]lsp.CallHierarchyOutgoingCall, bool, error)
}

// Supply is an optional observation of document supply, NOT an analyzed-source
// receipt. Different observations for the same URI are not coalesced by URI.
type Supply struct {
	URI         string          `json:"uri"`
	LanguageID  string          `json:"language_id,omitempty"`
	Observation json.RawMessage `json:"observation,omitempty"`
}
type DocumentSupplier interface {
	PrepareDocument(context.Context, AcquisitionContext, Locator) (Supply, error)
}
type Usage struct {
	Nodes         int `json:"nodes"`
	Requests      int `json:"requests"`
	EvidenceBytes int `json:"evidence_bytes"`
	PathWork      int `json:"path_work"`
	PrepareProbes int `json:"prepare_probes"`
}
type RequestRecord struct {
	ID              string             `json:"id"`
	TargetID        string             `json:"target_id"`
	Method          string             `json:"method"`
	NodeID          string             `json:"node_id,omitempty"`
	Context         AcquisitionContext `json:"context"`
	Before          Usage              `json:"before"`
	Attempted       bool               `json:"attempted"`
	Outcome         string             `json:"outcome"`
	Reason          string             `json:"reason,omitempty"`
	Params          json.RawMessage    `json:"params,omitempty"`
	Response        json.RawMessage    `json:"response,omitempty"`
	CaptureComplete bool               `json:"capture_complete"`
	EvidenceBytes   int                `json:"evidence_bytes"`
}
type Resolution struct {
	Status     ResolutionStatus       `json:"status"`
	Reason     string                 `json:"reason,omitempty"`
	Identity   *graph.Node            `json:"identity,omitempty"`
	Prepared   *lsp.CallHierarchyItem `json:"prepared,omitempty"`
	Position   *lsp.Position          `json:"position,omitempty"`
	RequestIDs []string               `json:"request_ids"`
}
type Expansion struct {
	NodeID    string          `json:"node_id"`
	Depth     int             `json:"depth"`
	Status    ExpansionStatus `json:"status"`
	RequestID string          `json:"request_id,omitempty"`
	Cached    bool            `json:"cached"`
	Reason    string          `json:"reason,omitempty"`
}
type DirectionResult struct {
	Status             ExpansionStatus    `json:"status"`
	Expansions         []Expansion        `json:"expansions"`
	Layers             []graph.SliceLayer `json:"layers"`
	FrontierIDs        []string           `json:"frontier_ids"`
	SuccessfulEmptyIDs []string           `json:"successful_empty_ids"`
	StartIDs           []string           `json:"start_ids"`
}
type Connection struct {
	From      string            `json:"from,omitempty"`
	To        string            `json:"to,omitempty"`
	Direction string            `json:"direction"`
	Status    string            `json:"status"`
	Reason    string            `json:"reason,omitempty"`
	Path      retainedpath.Path `json:"path"`
	Work      int               `json:"work"`
}
type TargetResult struct {
	Requested  Target          `json:"requested"`
	Resolution Resolution      `json:"resolution"`
	Admission  AdmissionStatus `json:"admission"`
	Outgoing   DirectionResult `json:"outgoing"`
	Incoming   DirectionResult `json:"incoming"`
	Connection Connection      `json:"connection"`
}

// EdgeObservation links one actual response to its admitted native edge/ranges.
// Cache replay adds membership, never additional observations or support.
type EdgeObservation struct {
	RequestID  string        `json:"request_id"`
	RelationID string        `json:"relation_id"`
	CallSites  []graph.Range `json:"call_sites"`
}
type Result struct {
	Policy              string            `json:"policy"`
	PathProjection      string            `json:"path_projection"`
	Request             Request           `json:"request"`
	Graph               graph.Result      `json:"graph"`
	Targets             []TargetResult    `json:"targets"`
	Requests            []RequestRecord   `json:"requests"`
	Supplies            []Supply          `json:"supplies"`
	EdgeObservations    []EdgeObservation `json:"edge_observations"`
	Usage               Usage             `json:"usage"`
	AcquisitionComplete bool              `json:"acquisition_complete"`
}
