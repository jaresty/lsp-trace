// Package presentation renders non-authoritative, read-only views of V3 artifacts.
package presentation

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

type Format string
type Detail string

const (
	FormatSummary Format = "summary"
	FormatTree    Format = "tree"
	FormatMermaid Format = "mermaid"
	DetailCompact Detail = "compact"
	DetailFull    Detail = "full"
)

type Options struct {
	Format Format
	Detail Detail
}

type position struct{ Line, Character int }
type rng struct{ Start, End position }
type node struct {
	ID, Name, URI  string
	Range          rng `json:"range"`
	SelectionRange rng `json:"selection_range"`
}
type edge struct {
	CallerNodeID string `json:"caller_node_id"`
	CalleeNodeID string `json:"callee_node_id"`
	CallSites    []rng  `json:"call_sites"`
}
type boundary struct{ NodeID, Reason, Message string }
type artifact struct {
	SchemaVersion string `json:"schema_version"`
	Invocation    struct {
		WorkspaceURI string `json:"workspace_uri"`
		Limits       struct {
			MaxDepth, MaxNodes int
			TimeoutMS          int64 `json:"timeout_ms"`
		} `json:"limits"`
	} `json:"invocation"`
	Nodes     []node     `json:"nodes"`
	Edges     []edge     `json:"edges"`
	Terminals []boundary `json:"terminals"`
	Frontier  []boundary `json:"frontier"`
	Summary   struct {
		NodeCount           int    `json:"node_count"`
		EdgeCount           int    `json:"edge_count"`
		TerminalCount       int    `json:"terminal_count"`
		CycleCount          int    `json:"cycle_count"`
		TraversalComplete   bool   `json:"traversal_complete"`
		SourceGraphComplete string `json:"source_graph_complete"`
		CompletenessScope   string `json:"completeness_scope"`
		Truncated           bool   `json:"truncated"`
	} `json:"summary"`
}

func Render(data []byte, opts Options) (string, error) {
	if opts.Format == "" {
		opts.Format = FormatSummary
	}
	if opts.Detail == "" {
		opts.Detail = DetailCompact
	}
	if opts.Format != FormatSummary && opts.Format != FormatTree && opts.Format != FormatMermaid {
		return "", fmt.Errorf("unsupported format %q", opts.Format)
	}
	if opts.Detail != DetailCompact && opts.Detail != DetailFull {
		return "", fmt.Errorf("unsupported detail %q", opts.Detail)
	}
	var a artifact
	if err := json.Unmarshal(data, &a); err != nil {
		return "", fmt.Errorf("decode V3 artifact: %w", err)
	}
	if a.SchemaVersion != "lsp-trace.graph.v3" {
		return "", fmt.Errorf("presentation requires lsp-trace.graph.v3")
	}
	sort.Slice(a.Nodes, func(i, j int) bool {
		return a.Nodes[i].ID < a.Nodes[j].ID
	})
	sort.Slice(a.Edges, func(i, j int) bool {
		if a.Edges[i].CallerNodeID != a.Edges[j].CallerNodeID {
			return a.Edges[i].CallerNodeID < a.Edges[j].CallerNodeID
		}
		return a.Edges[i].CalleeNodeID < a.Edges[j].CalleeNodeID
	})
	sort.Slice(a.Terminals, func(i, j int) bool {
		if a.Terminals[i].Reason != a.Terminals[j].Reason {
			return a.Terminals[i].Reason < a.Terminals[j].Reason
		}
		return a.Terminals[i].NodeID < a.Terminals[j].NodeID
	})
	sort.Slice(a.Frontier, func(i, j int) bool {
		if a.Frontier[i].Reason != a.Frontier[j].Reason {
			return a.Frontier[i].Reason < a.Frontier[j].Reason
		}
		return a.Frontier[i].NodeID < a.Frontier[j].NodeID
	})
	switch opts.Format {
	case FormatSummary:
		return renderSummary(a, opts), nil
	case FormatTree:
		return renderTree(a, opts), nil
	default:
		return renderMermaid(a, opts), nil
	}
}

func renderSummary(a artifact, opts Options) string {
	var b strings.Builder
	fmt.Fprintln(&b, "lsp-trace presentation only — derived view; not authoritative evidence")
	fmt.Fprintf(&b, "nodes: %d\nedges: %d\nterminals: %d\ncycles: %d\ncomplete: %t\ntruncated: %t\n", a.Summary.NodeCount, a.Summary.EdgeCount, a.Summary.TerminalCount, a.Summary.CycleCount, a.Summary.TraversalComplete, a.Summary.Truncated)
	writeBoundaries(&b, a, opts)
	return b.String()
}

func renderTree(a artifact, opts Options) string {
	var b strings.Builder
	fmt.Fprintln(&b, "lsp-trace presentation only — derived view; not authoritative evidence")
	byID := map[string]node{}
	incoming := map[string]int{}
	children := map[string][]edge{}
	for _, n := range a.Nodes {
		byID[n.ID] = n
	}
	for _, e := range a.Edges {
		children[e.CallerNodeID] = append(children[e.CallerNodeID], e)
		incoming[e.CalleeNodeID]++
	}
	seen := map[string]bool{}
	var walk func(string, string)
	walk = func(id, prefix string) {
		n, ok := byID[id]
		if !ok {
			return
		}
		if seen[id] {
			fmt.Fprintf(&b, "%s↩ %s [%s]\n", prefix, n.Name, id)
			return
		}
		seen[id] = true
		fmt.Fprintf(&b, "%s%s — %s:%d:%d [%s]\n", prefix, n.Name, displayURI(n.URI, a.Invocation.WorkspaceURI), n.SelectionRange.Start.Line+1, n.SelectionRange.Start.Character+1, id)
		for _, e := range children[id] {
			if opts.Detail == DetailFull && len(e.CallSites) > 0 {
				fmt.Fprintf(&b, "%s  call-site %d:%d\n", prefix, e.CallSites[0].Start.Line+1, e.CallSites[0].Start.Character+1)
			}
			walk(e.CalleeNodeID, prefix+"  ")
		}
	}
	for _, n := range a.Nodes {
		if incoming[n.ID] == 0 {
			walk(n.ID, "")
		}
	}
	for _, n := range a.Nodes {
		if !seen[n.ID] {
			walk(n.ID, "")
		}
	}
	if opts.Detail == DetailFull {
		writeBoundaries(&b, a, opts)
	}
	return b.String()
}

func renderMermaid(a artifact, opts Options) string {
	var b strings.Builder
	fmt.Fprintln(&b, "flowchart TD")
	for _, n := range a.Nodes {
		label := mermaidEscape(n.Name)
		if opts.Detail == DetailFull {
			label += "<br/>" + mermaidEscape(displayURI(n.URI, a.Invocation.WorkspaceURI))
		}
		fmt.Fprintf(&b, "  %s[\"%s\"]\n", mermaidID(n.ID), label)
	}
	for _, e := range a.Edges {
		fmt.Fprintf(&b, "  %s --> %s\n", mermaidID(e.CallerNodeID), mermaidID(e.CalleeNodeID))
	}
	return b.String()
}

func writeBoundaries(b *strings.Builder, a artifact, _ Options) {
	all := append(append([]boundary{}, a.Terminals...), a.Frontier...)
	for _, x := range all {
		fmt.Fprintf(b, "boundary: %s", x.Reason)
		if x.NodeID != "" {
			fmt.Fprintf(b, " node=%s", x.NodeID)
		}
		if p := suggestion(x.Reason); p != "" {
			fmt.Fprintf(b, "; try increasing %s", p)
		}
		if x.Message != "" {
			fmt.Fprintf(b, "; %s", x.Message)
		}
		fmt.Fprintln(b)
	}
}
func suggestion(reason string) string {
	switch reason {
	case "MAX_DEPTH":
		return "--max-depth"
	case "MAX_NODES":
		return "--max-nodes"
	case "GLOBAL_TIMEOUT":
		return "--timeout"
	case "REQUEST_TIMEOUT":
		return "--request-timeout"
	}
	return ""
}

func displayURI(raw, workspace string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "file" {
		return raw
	}
	w, err := url.Parse(workspace)
	if err != nil || w.Scheme != "file" || u.Host != w.Host {
		return raw
	}
	base := path.Clean(w.Path)
	target := path.Clean(u.Path)
	rel, err := filepath.Rel(base, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, "../") {
		return raw
	}
	if rel == "." {
		return "."
	}
	return rel
}
func mermaidID(id string) string {
	var b strings.Builder
	b.WriteString("n_")
	for _, r := range id {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' {
			b.WriteRune(r)
		} else {
			fmt.Fprintf(&b, "_%x_", r)
		}
	}
	return b.String()
}
func mermaidEscape(s string) string {
	r := strings.NewReplacer("&", "&#38;", "\"", "&#34;", "<", "&#60;", ">", "&#62;", "#", "&#35;")
	return r.Replace(s)
}
