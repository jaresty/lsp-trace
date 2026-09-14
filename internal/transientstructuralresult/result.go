// Package transientstructuralresult defines the transport-neutral, normalized
// public result boundary for the still-unregistered transient structural operation.
package transientstructuralresult

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
)

const (
	DefaultMaxMessages uint64 = 64
	DefaultMaxBytes    uint64 = 4194304
	MaxMessages        uint64 = 4096
	MaxBytes           uint64 = 16777216
	Calls                     = "CALLS"
	Neighborhood              = "NEIGHBORHOOD"
	Impact                    = "IMPACT"
	Incoming                  = "INCOMING"
	Outgoing                  = "OUTGOING"
	Complete                  = "COMPLETE"
	Empty                     = "EMPTY"
	Unknown                   = "UNKNOWN"
	policyVersion             = "1"
	policyStatus              = "PROVISIONAL_NONCERTIFIED"
	claimCeiling              = "Under the named managed session generation, exact target, server responses, traversal bounds, and analysis policy, this bounded server-reported call graph has the reported structural properties."
)

var (
	managerPattern = regexp.MustCompile(`^ts_[0-9a-f]{32}$`)
	nodePattern    = regexp.MustCompile(`^tn_[0-9a-f]{32}$`)
	edgePattern    = regexp.MustCompile(`^te_[0-9a-f]{32}$`)
	ErrInvalid     = errors.New("transient structural result: invalid")
)

type AnalysisRequest struct {
	Kind, Direction string
	Depth           uint64
}
type Request struct {
	Generation, DownDepth, UpDepth, MaxNodes           uint64
	TimeoutMS, RequestTimeoutMS, MaxMessages, MaxBytes uint64
	Analysis                                           AnalysisRequest
}

func NormalizeRequest(q Request) (Request, error) {
	if q.MaxMessages == 0 {
		q.MaxMessages = DefaultMaxMessages
	}
	if q.MaxBytes == 0 {
		q.MaxBytes = DefaultMaxBytes
	}
	if q.Generation < 1 || q.DownDepth > 64 || q.UpDepth > 64 || q.MaxNodes < 1 || q.MaxNodes > 10000 || q.TimeoutMS < 1 || q.TimeoutMS > 60000 || q.RequestTimeoutMS < 1 || q.RequestTimeoutMS > q.TimeoutMS || q.MaxMessages < 1 || q.MaxMessages > MaxMessages || q.MaxBytes < 1 || q.MaxBytes > MaxBytes {
		return Request{}, ErrInvalid
	}
	switch q.Analysis.Kind {
	case Neighborhood:
		if q.Analysis.Direction != "" || q.Analysis.Depth != 0 {
			return Request{}, ErrInvalid
		}
	case Impact:
		if q.Analysis.Depth < 1 || q.Analysis.Depth > 64 {
			return Request{}, ErrInvalid
		}
		if q.Analysis.Direction == Incoming {
			if q.Analysis.Depth > q.UpDepth {
				return Request{}, ErrInvalid
			}
		} else if q.Analysis.Direction == Outgoing {
			if q.Analysis.Depth > q.DownDepth {
				return Request{}, ErrInvalid
			}
		} else {
			return Request{}, ErrInvalid
		}
	default:
		return Request{}, ErrInvalid
	}
	return q, nil
}

func opaqueID(prefix, domain, managerID string, generation uint64, parts ...string) (string, error) {
	if !managerPattern.MatchString(managerID) || generation < 1 {
		return "", ErrInvalid
	}
	h := sha256.New()
	fmt.Fprintf(h, "transient-structural-v1\x00%s\x00%s\x00%d", domain, managerID, generation)
	for _, p := range parts {
		h.Write([]byte{0})
		h.Write([]byte(p))
	}
	return prefix + hex.EncodeToString(h.Sum(nil)[:16]), nil
}
func NodeID(managerID string, generation uint64, structuralIdentity ...string) (string, error) {
	return opaqueID("tn_", "node", managerID, generation, structuralIdentity...)
}
func EdgeID(managerID string, generation uint64, structuralIdentity ...string) (string, error) {
	return opaqueID("te_", "edge", managerID, generation, structuralIdentity...)
}

type OmissionReason string

const (
	DepthBound          OmissionReason = "DEPTH_BOUND"
	NodeBound           OmissionReason = "NODE_BOUND"
	RequestBound        OmissionReason = "REQUEST_BOUND"
	Timeout             OmissionReason = "TIMEOUT"
	Cancellation        OmissionReason = "CANCELLATION"
	UnsupportedResponse OmissionReason = "UNSUPPORTED_RESPONSE"
	MalformedResponse   OmissionReason = "MALFORMED_RESPONSE"
	Deduplication       OmissionReason = "DEDUPLICATION"
)

var omissionReasons = []OmissionReason{DepthBound, NodeBound, RequestBound, Timeout, Cancellation, UnsupportedResponse, MalformedResponse, Deduplication}

func OmissionReasons() []OmissionReason { return append([]OmissionReason(nil), omissionReasons...) }

type ReasonMap map[OmissionReason]uint64

func EmptyReasonMap() ReasonMap {
	r := make(ReasonMap, len(omissionReasons))
	for _, reason := range omissionReasons {
		r[reason] = 0
	}
	return r
}

func validReasons(r ReasonMap) bool {
	if len(r) != len(omissionReasons) {
		return false
	}
	for _, reason := range omissionReasons {
		if _, ok := r[reason]; !ok {
			return false
		}
	}
	return true
}

func normalizedReasons(r ReasonMap) map[string]uint64 {
	out := map[string]uint64{}
	for _, k := range omissionReasons {
		out[string(k)] = r[k]
	}
	return out
}
func reasonSum(r ReasonMap) uint64 {
	var n uint64
	for _, k := range omissionReasons {
		n += r[k]
	}
	return n
}

type Accounting struct {
	RequestAttempted          uint64    `json:"request_attempted"`
	RequestSucceeded          uint64    `json:"request_succeeded"`
	RequestFailed             uint64    `json:"request_failed"`
	RequestCancelled          uint64    `json:"request_cancelled"`
	RequestOmissionReasons    ReasonMap `json:"-"`
	PreparedAttempted         uint64    `json:"prepared_attempted"`
	PreparedReturned          uint64    `json:"prepared_returned"`
	PreparedEmpty             uint64    `json:"prepared_empty"`
	PreparedFailed            uint64    `json:"prepared_failed"`
	NodeObserved              uint64    `json:"node_observed"`
	NodeAdmitted              uint64    `json:"node_admitted"`
	NodeRejected              uint64    `json:"node_rejected"`
	NodeOmitted               uint64    `json:"node_omitted"`
	NodeOmissionReasons       ReasonMap `json:"-"`
	OccurrenceObserved        uint64    `json:"occurrence_observed"`
	OccurrenceAdmitted        uint64    `json:"occurrence_admitted"`
	OccurrenceRejected        uint64    `json:"occurrence_rejected"`
	OccurrenceOmitted         uint64    `json:"occurrence_omitted"`
	OccurrenceOmissionReasons ReasonMap `json:"-"`
	FrontierObserved          uint64    `json:"frontier_observed"`
	FrontierExpanded          uint64    `json:"frontier_expanded"`
	FrontierUnexpanded        uint64    `json:"frontier_unexpanded"`
	FrontierOmissionReasons   ReasonMap `json:"-"`
	DeduplicatedNodes         uint64    `json:"deduplicated_nodes"`
	DeduplicatedOccurrences   uint64    `json:"deduplicated_occurrences"`
	Truncated                 bool      `json:"truncated"`
}

func (a Accounting) Validate() error {
	if a.Truncated || !validReasons(a.RequestOmissionReasons) || !validReasons(a.NodeOmissionReasons) || !validReasons(a.OccurrenceOmissionReasons) || !validReasons(a.FrontierOmissionReasons) || a.RequestAttempted != a.RequestSucceeded+a.RequestFailed+a.RequestCancelled || a.PreparedAttempted != a.PreparedReturned+a.PreparedEmpty+a.PreparedFailed || a.NodeObserved != a.NodeAdmitted+a.NodeRejected+a.NodeOmitted || a.OccurrenceObserved != a.OccurrenceAdmitted+a.OccurrenceRejected+a.OccurrenceOmitted || a.FrontierObserved != a.FrontierExpanded+a.FrontierUnexpanded || reasonSum(a.RequestOmissionReasons) != a.RequestFailed+a.RequestCancelled || reasonSum(a.NodeOmissionReasons) != a.NodeOmitted || reasonSum(a.OccurrenceOmissionReasons) != a.OccurrenceOmitted || reasonSum(a.FrontierOmissionReasons) != a.FrontierUnexpanded || a.NodeOmissionReasons[Deduplication] != a.DeduplicatedNodes || a.OccurrenceOmissionReasons[Deduplication] != a.DeduplicatedOccurrences {
		return ErrInvalid
	}
	return nil
}
func (a Accounting) MarshalJSON() ([]byte, error) {
	type alias Accounting
	return json.Marshal(struct {
		alias
		Request    map[string]uint64 `json:"request_omission_reasons"`
		Node       map[string]uint64 `json:"node_omission_reasons"`
		Occurrence map[string]uint64 `json:"occurrence_omission_reasons"`
		Frontier   map[string]uint64 `json:"frontier_omission_reasons"`
	}{alias(a), normalizedReasons(a.RequestOmissionReasons), normalizedReasons(a.NodeOmissionReasons), normalizedReasons(a.OccurrenceOmissionReasons), normalizedReasons(a.FrontierOmissionReasons)})
}

type Node struct {
	ID string `json:"node_id"`
}
type Edge struct {
	ID           string `json:"edge_id"`
	CallerNodeID string `json:"caller_node_id"`
	CalleeNodeID string `json:"callee_node_id"`
}
type NeighborhoodResult struct {
	Kind          string `json:"kind"`
	RootNodeID    string `json:"root_node_id"`
	Nodes         []Node `json:"nodes"`
	Edges         []Edge `json:"edges"`
	IncomingCount uint64 `json:"incoming_count"`
	OutgoingCount uint64 `json:"outgoing_count"`
}

func (n NeighborhoodResult) Validate() error {
	if n.Kind != "" && n.Kind != Neighborhood {
		return ErrInvalid
	}
	nodes, edges, err := validateGraph(n.RootNodeID, n.Nodes, n.Edges)
	if err != nil {
		return err
	}
	var in, out uint64
	for _, e := range edges {
		if e.CalleeNodeID == n.RootNodeID {
			in++
		}
		if e.CallerNodeID == n.RootNodeID {
			out++
		}
	}
	_ = nodes
	if in != n.IncomingCount || out != n.OutgoingCount {
		return ErrInvalid
	}
	return nil
}

type ImpactResult struct {
	Kind             string   `json:"kind"`
	RootNodeID       string   `json:"root_node_id"`
	Direction        string   `json:"direction"`
	Depth            uint64   `json:"depth"`
	Nodes            []Node   `json:"nodes"`
	Edges            []Edge   `json:"edges"`
	ReachableNodeIDs []string `json:"reachable_node_ids"`
	WitnessEdgeIDs   []string `json:"witness_edge_ids"`
}

func (i ImpactResult) Validate(up, down uint64) error {
	if i.Kind != "" && i.Kind != Impact {
		return ErrInvalid
	}
	if i.Depth < 1 {
		return ErrInvalid
	}
	limit := down
	if i.Direction == Incoming {
		limit = up
	} else if i.Direction != Outgoing {
		return ErrInvalid
	}
	if i.Depth > limit {
		return ErrInvalid
	}
	nodes, edges, err := validateGraph(i.RootNodeID, i.Nodes, i.Edges)
	if err != nil {
		return err
	}
	reachable := map[string]bool{i.RootNodeID: true}
	witness := map[string]bool{}
	for d := uint64(0); d < i.Depth; d++ {
		changed := false
		for _, e := range edges {
			from, to := e.CallerNodeID, e.CalleeNodeID
			if i.Direction == Incoming {
				from, to = to, from
			}
			if reachable[from] && !reachable[to] {
				reachable[to] = true
				witness[e.ID] = true
				changed = true
			}
		}
		if !changed {
			break
		}
	}
	if err := exactSet(i.ReachableNodeIDs, reachable, i.RootNodeID); err != nil {
		return err
	}
	expectedWitness := map[string]bool{}
	for _, id := range i.WitnessEdgeIDs {
		if expectedWitness[id] || !witness[id] {
			return ErrInvalid
		}
		expectedWitness[id] = true
	}
	if len(expectedWitness) != len(witness) || len(nodes) != len(reachable) {
		return ErrInvalid
	}
	return nil
}
func exactSet(got []string, want map[string]bool, exclude string) error {
	seen := map[string]bool{}
	for _, x := range got {
		if seen[x] || !want[x] || x == exclude {
			return ErrInvalid
		}
		seen[x] = true
	}
	if len(seen) != len(want)-1 {
		return ErrInvalid
	}
	return nil
}
func validateGraph(root string, nodes []Node, edges []Edge) (map[string]bool, []Edge, error) {
	if !nodePattern.MatchString(root) || len(nodes) > 10000 || len(edges) > 100000 {
		return nil, nil, ErrInvalid
	}
	ns := map[string]bool{}
	for _, n := range nodes {
		if !nodePattern.MatchString(n.ID) || ns[n.ID] {
			return nil, nil, ErrInvalid
		}
		ns[n.ID] = true
	}
	if !ns[root] {
		return nil, nil, ErrInvalid
	}
	es := map[string]bool{}
	for _, e := range edges {
		if !edgePattern.MatchString(e.ID) || es[e.ID] || !ns[e.CallerNodeID] || !ns[e.CalleeNodeID] {
			return nil, nil, ErrInvalid
		}
		es[e.ID] = true
	}
	return ns, edges, nil
}

type Policy struct {
	PolicyID         string `json:"policy_id"`
	PolicyVersion    string `json:"policy_version"`
	PolicyStatus     string `json:"policy_status"`
	PolicyDigest     string `json:"policy_digest"`
	DownDepth        uint64 `json:"down_depth,omitempty"`
	UpDepth          uint64 `json:"up_depth,omitempty"`
	MaxNodes         uint64 `json:"max_nodes,omitempty"`
	TimeoutMS        uint64 `json:"timeout_ms,omitempty"`
	RequestTimeoutMS uint64 `json:"request_timeout_ms,omitempty"`
	MaxMessages      uint64 `json:"max_messages,omitempty"`
	MaxBytes         uint64 `json:"max_bytes,omitempty"`
}
type Result struct {
	SchemaVersion       string     `json:"schema_version"`
	Phase               string     `json:"phase"`
	State               string     `json:"state"`
	EvidenceClass       string     `json:"evidence_class"`
	Authority           uint64     `json:"authority"`
	SourceGraphComplete string     `json:"source_graph_complete"`
	Retained            bool       `json:"retained"`
	Replayable          bool       `json:"replayable"`
	PublicationEligible bool       `json:"publication_eligible"`
	HydrationEligible   bool       `json:"hydration_eligible"`
	CaptureSupply       bool       `json:"-"`
	Relation            string     `json:"-"`
	ClaimCeiling        string     `json:"claim_ceiling"`
	CanonicalSessionID  string     `json:"canonical_session_id"`
	Generation          uint64     `json:"generation"`
	TargetNodeID        string     `json:"target_node_id"`
	PositionEncoding    string     `json:"position_encoding"`
	TraversalPolicy     Policy     `json:"traversal_policy"`
	ResourcePolicy      Policy     `json:"resource_policy"`
	AnalysisPolicy      Policy     `json:"analysis_policy"`
	GraphDigest         string     `json:"graph_digest"`
	Accounting          Accounting `json:"accounting"`
	Analysis            any        `json:"analysis"`
	request             Request
}

var policyDocs = map[string]string{"transient-calls-traversal.v1": `{"policy_id":"transient-calls-traversal.v1","policy_version":"1","policy_status":"PROVISIONAL_NONCERTIFIED","relation":"CALLS","down_depth":{"minimum":0,"maximum":64},"up_depth":{"minimum":0,"maximum":64},"max_nodes":{"minimum":1,"maximum":10000}}`, "transient-structural-resources.v1": `{"policy_id":"transient-structural-resources.v1","policy_version":"1","policy_status":"PROVISIONAL_NONCERTIFIED","timeout_ms":{"minimum":1,"maximum":60000},"request_timeout_ms":{"minimum":1,"maximum":60000,"maximum_relation":"request_timeout_ms<=timeout_ms"},"max_messages":{"minimum":1,"maximum":4096,"default":64},"max_bytes":{"minimum":1,"maximum":16777216,"default":4194304}}`, "transient-neighborhood.v1": `{"policy_id":"transient-neighborhood.v1","policy_version":"1","policy_status":"PROVISIONAL_NONCERTIFIED","kind":"NEIGHBORHOOD","relation":"CALLS","semantics":"target_rooted_admitted_nodes_and_call_edges_within_exact_independent_incoming_and_outgoing_traversal_bounds"}`, "transient-impact.v1": `{"policy_id":"transient-impact.v1","policy_version":"1","policy_status":"PROVISIONAL_NONCERTIFIED","kind":"IMPACT","relation":"CALLS","directions":["INCOMING","OUTGOING"],"depth":{"minimum":1,"maximum":64},"semantics":"target_rooted_admitted_directed_reachability_within_requested_direction_and_depth_without_traversal_expansion"}`}

func policy(id string) Policy {
	sum := sha256.Sum256([]byte(policyDocs[id]))
	return Policy{PolicyID: id, PolicyVersion: policyVersion, PolicyStatus: policyStatus, PolicyDigest: "sha256:" + hex.EncodeToString(sum[:])}
}
func NewResult(managerID string, generation uint64, target, encoding string, q Request, a Accounting, analysis any) Result {
	q, _ = NormalizeRequest(q)
	tp := policy("transient-calls-traversal.v1")
	tp.DownDepth, tp.UpDepth, tp.MaxNodes = q.DownDepth, q.UpDepth, q.MaxNodes
	rp := policy("transient-structural-resources.v1")
	rp.TimeoutMS, rp.RequestTimeoutMS, rp.MaxMessages, rp.MaxBytes = q.TimeoutMS, q.RequestTimeoutMS, q.MaxMessages, q.MaxBytes
	apID := "transient-neighborhood.v1"
	state := Empty
	switch x := analysis.(type) {
	case NeighborhoodResult:
		x.Kind = Neighborhood
		analysis = x
		if len(x.Edges) > 0 {
			state = Complete
		}
	case ImpactResult:
		x.Kind = Impact
		analysis = x
		apID = "transient-impact.v1"
		if len(x.Edges) > 0 {
			state = Complete
		}
	}
	ids := []string{}
	switch x := analysis.(type) {
	case NeighborhoodResult:
		for _, n := range x.Nodes {
			ids = append(ids, n.ID)
		}
		for _, e := range x.Edges {
			ids = append(ids, e.ID)
		}
	case ImpactResult:
		for _, n := range x.Nodes {
			ids = append(ids, n.ID)
		}
		for _, e := range x.Edges {
			ids = append(ids, e.ID)
		}
	}
	sort.Strings(ids)
	g := sha256.Sum256([]byte(fmt.Sprint(ids)))
	return Result{"lsp-trace.transient-structural-result.v1", "DELIVERY_CHECK", state, "TRANSIENT_LIVE", 0, Unknown, false, false, false, false, false, Calls, claimCeiling, managerID, generation, target, encoding, tp, rp, policy(apID), "sha256:" + hex.EncodeToString(g[:]), a, analysis, q}
}
func (r Result) Validate() error {
	q, err := NormalizeRequest(r.request)
	if err != nil {
		return err
	}
	if !managerPattern.MatchString(r.CanonicalSessionID) || r.Generation != q.Generation || !nodePattern.MatchString(r.TargetNodeID) || r.SchemaVersion != "lsp-trace.transient-structural-result.v1" || r.Phase != "DELIVERY_CHECK" || r.EvidenceClass != "TRANSIENT_LIVE" || r.Authority != 0 || r.SourceGraphComplete != Unknown || r.Retained || r.Replayable || r.PublicationEligible || r.HydrationEligible || r.CaptureSupply || r.Relation != Calls || r.ClaimCeiling != claimCeiling {
		return ErrInvalid
	}
	if r.PositionEncoding != "utf-8" && r.PositionEncoding != "utf-16" && r.PositionEncoding != "utf-32" {
		return ErrInvalid
	}
	tp := policy("transient-calls-traversal.v1")
	tp.DownDepth, tp.UpDepth, tp.MaxNodes = q.DownDepth, q.UpDepth, q.MaxNodes
	rp := policy("transient-structural-resources.v1")
	rp.TimeoutMS, rp.RequestTimeoutMS, rp.MaxMessages, rp.MaxBytes = q.TimeoutMS, q.RequestTimeoutMS, q.MaxMessages, q.MaxBytes
	if r.TraversalPolicy != tp || r.ResourcePolicy != rp || r.AnalysisPolicy != policy(r.AnalysisPolicy.PolicyID) {
		return ErrInvalid
	}
	if err := r.Accounting.Validate(); err != nil {
		return err
	}
	var edgeCount int
	switch x := r.Analysis.(type) {
	case NeighborhoodResult:
		if r.AnalysisPolicy.PolicyID != "transient-neighborhood.v1" || x.RootNodeID != r.TargetNodeID {
			return ErrInvalid
		}
		if err := x.Validate(); err != nil {
			return err
		}
		edgeCount = len(x.Edges)
	case ImpactResult:
		if r.AnalysisPolicy.PolicyID != "transient-impact.v1" || x.RootNodeID != r.TargetNodeID {
			return ErrInvalid
		}
		if err := x.Validate(q.UpDepth, q.DownDepth); err != nil {
			return err
		}
		edgeCount = len(x.Edges)
	default:
		return ErrInvalid
	}
	want := Empty
	if edgeCount > 0 {
		want = Complete
	}
	if r.State != want {
		return ErrInvalid
	}
	return nil
}
