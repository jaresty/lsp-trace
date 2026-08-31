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
	sort.Slice(r.Terminals, func(i, j int) bool {
		if r.Terminals[i].NodeID != r.Terminals[j].NodeID {
			return r.Terminals[i].NodeID < r.Terminals[j].NodeID
		}
		return r.Terminals[i].Reason < r.Terminals[j].Reason
	})
	sort.Slice(r.Frontier, func(i, j int) bool {
		if r.Frontier[i].NodeID != r.Frontier[j].NodeID {
			return r.Frontier[i].NodeID < r.Frontier[j].NodeID
		}
		return r.Frontier[i].Reason < r.Frontier[j].Reason
	})
	r.Summary.NodeCount = len(r.Nodes)
	r.Summary.EdgeCount = len(r.Edges)
	r.Summary.TerminalCount = len(r.Terminals)
}
