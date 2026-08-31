package graph

import "sort"

const SchemaVersion = "lsp-trace.graph.v1"

type Reason string

const (
	NoIncomingCalls          Reason = "NO_INCOMING_CALLS"
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
	NodeID  string `json:"node_id,omitempty"`
	Reason  Reason `json:"reason"`
	Message string `json:"message,omitempty"`
}
type Diagnostic struct {
	Phase   string `json:"phase"`
	Method  string `json:"method,omitempty"`
	NodeID  string `json:"node_id,omitempty"`
	Message string `json:"message"`
}
type Summary struct {
	NodeCount     int  `json:"node_count"`
	EdgeCount     int  `json:"edge_count"`
	TerminalCount int  `json:"terminal_count"`
	CycleCount    int  `json:"cycle_count"`
	Complete      bool `json:"complete"`
	Truncated     bool `json:"truncated"`
}

type Result struct {
	SchemaVersion string       `json:"schema_version"`
	Invocation    Invocation   `json:"invocation"`
	Capabilities  Capabilities `json:"capabilities"`
	Targets       []string     `json:"targets"`
	Nodes         []Node       `json:"nodes"`
	Edges         []Edge       `json:"edges"`
	Terminals     []Boundary   `json:"terminals"`
	Frontier      []Boundary   `json:"frontier"`
	Diagnostics   []Diagnostic `json:"diagnostics"`
	Summary       Summary      `json:"summary"`
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
		return a.Message < b.Message
	})
	r.Summary.NodeCount = len(r.Nodes)
	r.Summary.EdgeCount = len(r.Edges)
	r.Summary.TerminalCount = len(r.Terminals)
	r.Summary.CycleCount = cycleCount(r.Nodes, r.Edges)
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
