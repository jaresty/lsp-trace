package graph

import (
	"encoding/json"
	"sort"
)

const (
	SchemaVersionV1   = "lsp-trace.graph.v1"
	SchemaVersionV2   = "lsp-trace.graph.v2"
	SchemaVersion     = SchemaVersionV2
	CompletenessScope = "SERVER_REPORTED_CALL_HIERARCHY"
	Unknown           = "UNKNOWN"
)

type Reason string

const (
	NoIncomingCalls          Reason = "NO_INCOMING_CALLS"
	ServerReportedNoIncoming Reason = "SERVER_REPORTED_NO_INCOMING_CALLS"
	PrepareReturnedNoItem    Reason = "PREPARE_RETURNED_NO_ITEM"
	IncomingReturnedNull     Reason = "INCOMING_RETURNED_NULL"
	ExternalURI              Reason = "EXTERNAL_URI"
	UnsupportedCallHierarchy Reason = "UNSUPPORTED_CALL_HIERARCHY"
	ServerError              Reason = "SERVER_ERROR"
	InvalidServerResponse    Reason = "INVALID_SERVER_RESPONSE"
	RequestTimeout           Reason = "REQUEST_TIMEOUT"
	GlobalTimeout            Reason = "GLOBAL_TIMEOUT"
	Cancelled                Reason = "CANCELLED"
	MaxDepth                 Reason = "MAX_DEPTH"
	MaxNodes                 Reason = "MAX_NODES"
	NodeIDCollision          Reason = "NODE_ID_COLLISION"
)

type Limits struct {
	MaxDepth  int   `json:"max_depth"`
	MaxNodes  int   `json:"max_nodes"`
	TimeoutMS int64 `json:"timeout_ms"`
}
type Target struct {
	URI    string `json:"uri"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
}
type ServerInvocation struct {
	Command   string   `json:"command"`
	Arguments []string `json:"arguments"`
}
type Invocation struct {
	WorkspaceURI string           `json:"workspace_uri"`
	Target       Target           `json:"target"`
	Server       ServerInvocation `json:"server"`
	Limits       Limits           `json:"limits"`
}
type Capabilities struct {
	CallHierarchyProvider bool `json:"call_hierarchy_provider"`
}
type Boundary struct {
	NodeID     string `json:"node_id,omitempty"`
	Reason     Reason `json:"reason"`
	Message    string `json:"message,omitempty"`
	Provenance string `json:"provenance,omitempty"`
}
type DiagnosticCategory string

const (
	UnresolvedCall DiagnosticCategory = "UNRESOLVED_CALL"
	DynamicCall    DiagnosticCategory = "DYNAMIC_CALL"
)

type Diagnostic struct {
	Phase    string             `json:"phase"`
	Method   string             `json:"method,omitempty"`
	NodeID   string             `json:"node_id,omitempty"`
	Category DiagnosticCategory `json:"category,omitempty"`
	Message  string             `json:"message"`
}
type CapabilityQuality struct {
	Advertised               bool   `json:"advertised"`
	PrepareSucceeded         bool   `json:"prepare_succeeded"`
	IncomingRequestSuccesses int    `json:"incoming_request_successes"`
	IncomingEdges            int    `json:"incoming_edges"`
	CrossFileEdges           int    `json:"cross_file_edges"`
	CrossModuleEdges         string `json:"cross_module_edges"`
	UnresolvedCalls          int    `json:"unresolved_calls"`
	DynamicCalls             int    `json:"dynamic_calls"`
}

type Summary struct {
	NodeCount     int  `json:"node_count"`
	EdgeCount     int  `json:"edge_count"`
	TerminalCount int  `json:"terminal_count"`
	CycleCount    int  `json:"cycle_count"`
	Complete      bool `json:"complete"`
	Truncated     bool `json:"truncated"`
}

type SeedFailure struct {
	Phase   string `json:"phase"`
	Message string `json:"message"`
}

type SeedResult struct {
	Label             string       `json:"label"`
	Requested         Target       `json:"requested_position"`
	PreparedTargetIDs []string     `json:"prepared_target_ids"`
	ReachedNodeIDs    []string     `json:"reached_node_ids"`
	Failure           *SeedFailure `json:"failure,omitempty"`
}

type Result struct {
	SchemaVersion     string            `json:"schema_version"`
	Invocation        Invocation        `json:"invocation"`
	Capabilities      Capabilities      `json:"capabilities"`
	Targets           []string          `json:"targets"`
	Nodes             []Node            `json:"nodes"`
	Edges             []Edge            `json:"edges"`
	Terminals         []Boundary        `json:"terminals"`
	Frontier          []Boundary        `json:"frontier"`
	Diagnostics       []Diagnostic      `json:"diagnostics"`
	Summary           Summary           `json:"summary"`
	CapabilityQuality CapabilityQuality `json:"capability_quality,omitempty"`
	Seeds             []SeedResult      `json:"-"`
}

func (r Result) MarshalJSON() ([]byte, error) {
	type legacyBoundary struct {
		NodeID  string `json:"node_id,omitempty"`
		Reason  Reason `json:"reason"`
		Message string `json:"message,omitempty"`
	}
	type legacyDiagnostic struct {
		Phase   string `json:"phase"`
		Method  string `json:"method,omitempty"`
		NodeID  string `json:"node_id,omitempty"`
		Message string `json:"message"`
	}
	if r.SchemaVersion == SchemaVersionV1 {
		project := func(in []Boundary) []legacyBoundary {
			if in == nil {
				return nil
			}
			out := make([]legacyBoundary, len(in))
			for i, b := range in {
				reason := b.Reason
				if reason == ServerReportedNoIncoming {
					reason = NoIncomingCalls
				}
				out[i] = legacyBoundary{b.NodeID, reason, b.Message}
			}
			return out
		}
		projectDiagnostics := func(in []Diagnostic) []legacyDiagnostic {
			if in == nil {
				return nil
			}
			out := make([]legacyDiagnostic, len(in))
			for i, d := range in {
				out[i] = legacyDiagnostic{d.Phase, d.Method, d.NodeID, d.Message}
			}
			return out
		}
		return json.Marshal(struct {
			SchemaVersion string             `json:"schema_version"`
			Invocation    Invocation         `json:"invocation"`
			Capabilities  Capabilities       `json:"capabilities"`
			Targets       []string           `json:"targets"`
			Nodes         []Node             `json:"nodes"`
			Edges         []Edge             `json:"edges"`
			Terminals     []legacyBoundary   `json:"terminals"`
			Frontier      []legacyBoundary   `json:"frontier"`
			Diagnostics   []legacyDiagnostic `json:"diagnostics"`
			Summary       Summary            `json:"summary"`
		}{r.SchemaVersion, r.Invocation, r.Capabilities, r.Targets, r.Nodes, r.Edges, project(r.Terminals), project(r.Frontier), projectDiagnostics(r.Diagnostics), r.Summary})
	}
	type summaryV2 struct {
		NodeCount           int    `json:"node_count"`
		EdgeCount           int    `json:"edge_count"`
		TerminalCount       int    `json:"terminal_count"`
		CycleCount          int    `json:"cycle_count"`
		TraversalComplete   bool   `json:"traversal_complete"`
		SourceGraphComplete string `json:"source_graph_complete"`
		CompletenessScope   string `json:"completeness_scope"`
		Truncated           bool   `json:"truncated"`
	}
	return json.Marshal(struct {
		SchemaVersion     string            `json:"schema_version"`
		Invocation        Invocation        `json:"invocation"`
		Capabilities      Capabilities      `json:"capabilities"`
		CapabilityQuality CapabilityQuality `json:"capability_quality"`
		Targets           []string          `json:"targets"`
		Nodes             []Node            `json:"nodes"`
		Edges             []Edge            `json:"edges"`
		Terminals         []Boundary        `json:"terminals"`
		Frontier          []Boundary        `json:"frontier"`
		Diagnostics       []Diagnostic      `json:"diagnostics"`
		Seeds             []SeedResult      `json:"seeds,omitempty"`
		Summary           summaryV2         `json:"summary"`
	}{r.SchemaVersion, r.Invocation, r.Capabilities, r.CapabilityQuality, r.Targets, r.Nodes, r.Edges, r.Terminals, r.Frontier, r.Diagnostics, r.Seeds, summaryV2{r.Summary.NodeCount, r.Summary.EdgeCount, r.Summary.TerminalCount, r.Summary.CycleCount, r.Summary.Complete, Unknown, CompletenessScope, r.Summary.Truncated}})
}

func (r *Result) Canonicalize() {
	for i := range r.Edges {
		r.Edges[i].CallSites = mergeRanges(nil, r.Edges[i].CallSites)
	}
	sort.Slice(r.Nodes, func(i, j int) bool {
		a, b := r.Nodes[i], r.Nodes[j]
		if a.URI != b.URI {
			return a.URI < b.URI
		}
		if a.SelectionRange != b.SelectionRange {
			return lessRange(a.SelectionRange, b.SelectionRange)
		}
		if a.Range != b.Range {
			return lessRange(a.Range, b.Range)
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		if a.Detail != b.Detail {
			return a.Detail < b.Detail
		}
		return a.ID < b.ID
	})
	sort.Slice(r.Edges, func(i, j int) bool {
		a, b := r.Edges[i], r.Edges[j]
		if a.CallerNodeID != b.CallerNodeID {
			return a.CallerNodeID < b.CallerNodeID
		}
		return a.CalleeNodeID < b.CalleeNodeID
	})
	sort.Strings(r.Targets)
	r.Targets = uniqueStrings(r.Targets)
	for i := range r.Seeds {
		sort.Strings(r.Seeds[i].PreparedTargetIDs)
		r.Seeds[i].PreparedTargetIDs = uniqueStrings(r.Seeds[i].PreparedTargetIDs)
		sort.Strings(r.Seeds[i].ReachedNodeIDs)
		r.Seeds[i].ReachedNodeIDs = uniqueStrings(r.Seeds[i].ReachedNodeIDs)
	}
	sort.Slice(r.Seeds, func(i, j int) bool { return r.Seeds[i].Label < r.Seeds[j].Label })
	sort.Slice(r.Terminals, func(i, j int) bool { return lessBoundary(r.Terminals[i], r.Terminals[j]) })
	sort.Slice(r.Frontier, func(i, j int) bool { return lessBoundary(r.Frontier[i], r.Frontier[j]) })
	sort.Slice(r.Diagnostics, func(i, j int) bool {
		a, b := r.Diagnostics[i], r.Diagnostics[j]
		if a.Phase != b.Phase {
			return a.Phase < b.Phase
		}
		if a.Method != b.Method {
			return a.Method < b.Method
		}
		if a.NodeID != b.NodeID {
			return a.NodeID < b.NodeID
		}
		if a.Category != b.Category {
			return a.Category < b.Category
		}
		return a.Message < b.Message
	})
	r.Summary.NodeCount = len(r.Nodes)
	r.Summary.EdgeCount = len(r.Edges)
	r.Summary.TerminalCount = len(r.Terminals)
	r.Summary.CycleCount = cycleCount(r.Nodes, r.Edges)
	for i := range r.Terminals {
		if r.Terminals[i].Provenance == "" {
			r.Terminals[i].Provenance = boundaryProvenance(r.Terminals[i].Reason)
		}
	}
	for i := range r.Frontier {
		if r.Frontier[i].Provenance == "" {
			r.Frontier[i].Provenance = "CLIENT_DERIVED"
		}
	}
	if r.SchemaVersion != SchemaVersionV1 {
		r.CapabilityQuality.IncomingEdges = len(r.Edges)
		r.CapabilityQuality.UnresolvedCalls = 0
		r.CapabilityQuality.DynamicCalls = 0
		for _, diagnostic := range r.Diagnostics {
			switch diagnostic.Category {
			case UnresolvedCall:
				r.CapabilityQuality.UnresolvedCalls++
			case DynamicCall:
				r.CapabilityQuality.DynamicCalls++
			}
		}
		if r.CapabilityQuality.UnresolvedCalls > 0 {
			r.Summary.Complete = false
		}
		if r.CapabilityQuality.CrossModuleEdges == "" {
			r.CapabilityQuality.CrossModuleEdges = Unknown
		}
	}
}

func uniqueStrings(in []string) []string {
	if len(in) == 0 {
		return in
	}
	out := in[:1]
	for _, value := range in[1:] {
		if value != out[len(out)-1] {
			out = append(out, value)
		}
	}
	return out
}

func MergeResults(results ...Result) Result {
	out := Result{SchemaVersion: SchemaVersion, Summary: Summary{Complete: true}, CapabilityQuality: CapabilityQuality{CrossModuleEdges: Unknown}}
	nodes := map[string]Node{}
	for _, result := range results {
		if result.SchemaVersion != "" {
			out.SchemaVersion = result.SchemaVersion
		}
		out.Capabilities.CallHierarchyProvider = out.Capabilities.CallHierarchyProvider || result.Capabilities.CallHierarchyProvider
		out.CapabilityQuality.Advertised = out.CapabilityQuality.Advertised || result.CapabilityQuality.Advertised
		out.CapabilityQuality.PrepareSucceeded = out.CapabilityQuality.PrepareSucceeded || result.CapabilityQuality.PrepareSucceeded
		out.CapabilityQuality.IncomingRequestSuccesses += result.CapabilityQuality.IncomingRequestSuccesses
		out.CapabilityQuality.CrossFileEdges += result.CapabilityQuality.CrossFileEdges
		out.Summary.Complete = out.Summary.Complete && result.Summary.Complete
		out.Summary.Truncated = out.Summary.Truncated || result.Summary.Truncated
		out.Targets = append(out.Targets, result.Targets...)
		out.Seeds = append(out.Seeds, result.Seeds...)
		out.Terminals = append(out.Terminals, result.Terminals...)
		out.Frontier = append(out.Frontier, result.Frontier...)
		out.Diagnostics = append(out.Diagnostics, result.Diagnostics...)
		for _, node := range result.Nodes {
			if existing, ok := nodes[node.ID]; !ok || SameNodeIdentity(existing, node) {
				nodes[node.ID] = node
			} else {
				out.Terminals = append(out.Terminals, Boundary{NodeID: node.ID, Reason: NodeIDCollision})
				out.Summary.Complete = false
			}
		}
		for _, edge := range result.Edges {
			out.Edges = MergeEdge(out.Edges, edge)
		}
	}
	for _, node := range nodes {
		out.Nodes = append(out.Nodes, node)
	}
	out.Terminals = uniqueBoundaries(out.Terminals)
	out.Frontier = uniqueBoundaries(out.Frontier)
	out.Diagnostics = uniqueDiagnostics(out.Diagnostics)
	out.Canonicalize()
	return out
}

func uniqueBoundaries(in []Boundary) []Boundary {
	seen := map[Boundary]struct{}{}
	out := make([]Boundary, 0, len(in))
	for _, item := range in {
		if _, ok := seen[item]; !ok {
			seen[item] = struct{}{}
			out = append(out, item)
		}
	}
	return out
}

func uniqueDiagnostics(in []Diagnostic) []Diagnostic {
	seen := map[Diagnostic]struct{}{}
	out := make([]Diagnostic, 0, len(in))
	for _, item := range in {
		if _, ok := seen[item]; !ok {
			seen[item] = struct{}{}
			out = append(out, item)
		}
	}
	return out
}

func boundaryProvenance(reason Reason) string {
	if reason == ServerReportedNoIncoming || reason == IncomingReturnedNull {
		return "SERVER_REPORTED"
	}
	return "CLIENT_DERIVED"
}

func lessBoundary(a, b Boundary) bool {
	if a.NodeID != b.NodeID {
		return a.NodeID < b.NodeID
	}
	if a.Reason != b.Reason {
		return a.Reason < b.Reason
	}
	return a.Message < b.Message
}

func cycleCount(nodes []Node, edges []Edge) int {
	adj := make(map[string][]string, len(nodes))
	selfLoop := make(map[string]bool)
	for _, edge := range edges {
		adj[edge.CallerNodeID] = append(adj[edge.CallerNodeID], edge.CalleeNodeID)
		if edge.CallerNodeID == edge.CalleeNodeID {
			selfLoop[edge.CallerNodeID] = true
		}
	}
	index := 0
	indices := make(map[string]int, len(nodes))
	lowlink := make(map[string]int, len(nodes))
	onStack := make(map[string]bool, len(nodes))
	stack := make([]string, 0, len(nodes))
	cycles := 0
	var visit func(string)
	visit = func(v string) {
		indices[v], lowlink[v] = index, index
		index++
		stack = append(stack, v)
		onStack[v] = true
		for _, w := range adj[v] {
			if _, ok := indices[w]; !ok {
				visit(w)
				if lowlink[w] < lowlink[v] {
					lowlink[v] = lowlink[w]
				}
			} else if onStack[w] && indices[w] < lowlink[v] {
				lowlink[v] = indices[w]
			}
		}
		if lowlink[v] != indices[v] {
			return
		}
		size := 0
		for {
			w := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			onStack[w] = false
			size++
			if w == v {
				break
			}
		}
		if size > 1 || selfLoop[v] {
			cycles++
		}
	}
	for _, node := range nodes {
		if _, ok := indices[node.ID]; !ok {
			visit(node.ID)
		}
	}
	return cycles
}
