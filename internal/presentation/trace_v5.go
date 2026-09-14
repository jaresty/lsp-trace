package presentation

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"sort"
	"strings"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
)

const traceV5PresentationMarker = "lsp-trace presentation only — derived view; not authoritative evidence"

// TraceV5Options controls presentation of retained target-centered graph output.
type TraceV5Options struct {
	// Targets may only select exact node IDs already retained as authoritative
	// targets. Names, locators, and arbitrary retained nodes are rejected.
	Targets             []string
	ExpandTestNodes     bool
	ExpandExternalNodes bool
}

type traceV5Status string

const (
	traceStatusComplete traceV5Status = "COMPLETE"
	traceStatusPartial  traceV5Status = "PARTIAL"
	traceStatusEmpty    traceV5Status = "EMPTY"
)

type traceV5TargetRelation struct {
	peer   graph.Node
	caller graph.Node
	edge   graph.Edge
}

type traceNodeClass struct{ test, external bool }

// RenderTraceV5 renders a deterministic, target-centered view over graph-v5
// evidence embedded in a canonically admitted graph-provenance-v5 envelope.
func RenderTraceV5(data []byte, opts TraceV5Options) (string, error) {
	if _, err := graphprovenance.ValidateFor(data, graphprovenance.Family, "v5"); err != nil {
		return "", fmt.Errorf("provenance envelope: %w", err)
	}
	var env graphprovenance.EvidenceV5
	if err := json.Unmarshal(data, &env); err != nil {
		return "", fmt.Errorf("decode provenance envelope: %w", err)
	}
	native, err := base64.StdEncoding.DecodeString(env.GraphV5)
	if err != nil {
		return "", fmt.Errorf("decode embedded graph_v5: %w", err)
	}
	g, err := graph.DecodeNativeV3(native)
	if err != nil {
		return "", fmt.Errorf("decode embedded graph_v5: %w", err)
	}

	// Seed memberships are already covered by canonical V5 admission. They are
	// decoded only to recover PREPARED_TARGET endpoints when the graph target list
	// is empty; all displayed graph context comes from the canonical decoder.
	var membershipCarrier struct {
		SeedMemberships []graph.SeedMembership `json:"seed_memberships"`
	}
	if err := json.Unmarshal(native, &membershipCarrier); err != nil {
		return "", fmt.Errorf("decode embedded target memberships: %w", err)
	}

	nodes := make(map[string]graph.Node, len(g.Nodes))
	for _, n := range g.Nodes {
		if _, exists := nodes[n.ID]; exists {
			return "", fmt.Errorf("embedded graph has duplicate node %q", n.ID)
		}
		nodes[n.ID] = n
	}
	authoritative := append([]string(nil), g.Targets...)
	for _, m := range membershipCarrier.SeedMemberships {
		if m.EvidenceKind == "PREPARED_TARGET" {
			authoritative = append(authoritative, m.EndpointID)
		}
	}
	targetIDs, err := resolveTraceTargets(opts.Targets, authoritative, nodes)
	if err != nil {
		return "", err
	}

	incoming := map[string][]traceV5TargetRelation{}
	outgoing := map[string][]traceV5TargetRelation{}
	for _, e := range g.Edges {
		caller, callerOK := nodes[e.CallerNodeID]
		callee, calleeOK := nodes[e.CalleeNodeID]
		if !callerOK {
			return "", fmt.Errorf("embedded graph has unknown caller node %q", e.CallerNodeID)
		}
		if !calleeOK {
			return "", fmt.Errorf("embedded graph has unknown callee node %q", e.CalleeNodeID)
		}
		incoming[e.CalleeNodeID] = append(incoming[e.CalleeNodeID], traceV5TargetRelation{peer: caller, caller: caller, edge: e})
		outgoing[e.CallerNodeID] = append(outgoing[e.CallerNodeID], traceV5TargetRelation{peer: callee, caller: caller, edge: e})
	}

	sort.Slice(targetIDs, func(i, j int) bool { return lessTraceNode(nodes[targetIDs[i]], nodes[targetIDs[j]]) })
	nameCounts := map[string]int{}
	for _, id := range targetIDs {
		nameCounts[nodes[id].Name]++
	}

	var out strings.Builder
	fmt.Fprintln(&out, traceV5PresentationMarker)
	fmt.Fprintf(&out, "status: %s\n", traceV5StatusFromGraph(g))
	fmt.Fprintf(&out, "nodes: %d\nedges: %d\nterminals: %d\ncycles: %d\n", g.Summary.NodeCount, g.Summary.EdgeCount, g.Summary.TerminalCount, g.Summary.CycleCount)
	fmt.Fprintf(&out, "traversal_complete: %t\ntruncated: %t\n", g.Summary.Complete, g.Summary.Truncated)
	fmt.Fprintf(&out, "source_graph_complete: %s\n", valueOrUnknown(graph.Unknown))
	fmt.Fprintf(&out, "completeness_scope: %s\n", valueOrUnknown(graph.CompletenessScope))
	fmt.Fprintf(&out, "dependency_completeness: %s\n", valueOrUnknown(env.DependencyCompleteness))
	fmt.Fprintf(&out, "analyzed_version: %s\n", valueOrUnknown(env.AnalyzedVersion))
	fmt.Fprintf(&out, "source_policy: %s\n", valueOrUnknown(env.SourcePolicy))
	fmt.Fprintln(&out, "COMPLETE means bounded traversal complete only; it never means workspace or source complete")
	formatBoundaries(&out, "terminals", g.Terminals)
	formatBoundaries(&out, "frontier", g.Frontier)
	formatDiagnostics(&out, g.Diagnostics)
	if len(targetIDs) == 0 {
		fmt.Fprintln(&out, "targets: none")
		return out.String(), nil
	}

	fmt.Fprintf(&out, "targets: %d\n", len(targetIDs))
	for _, id := range targetIDs {
		n := nodes[id]
		targetName := n.Name
		if nameCounts[targetName] > 1 {
			targetName = fmt.Sprintf("%s (%s)", n.Name, formatNodeLocation(g.Invocation.WorkspaceURI, n))
		}
		fmt.Fprintf(&out, "\nTARGET %s (%s)\n", targetName, id)
		fmt.Fprintf(&out, "  %s @ %s\n", n.Name, formatNodeLocation(g.Invocation.WorkspaceURI, n))
		formatTraceRelations(&out, g.Invocation.WorkspaceURI, "direct incoming", incoming[id], opts)
		formatTraceRelations(&out, g.Invocation.WorkspaceURI, "direct outgoing", outgoing[id], opts)
	}
	return out.String(), nil
}

func valueOrUnknown(s string) string {
	if strings.TrimSpace(s) == "" {
		return graph.Unknown
	}
	return s
}

func traceV5StatusFromGraph(g graph.Result) traceV5Status {
	if incompleteTrace(g) {
		return traceStatusPartial
	}
	if len(g.Nodes) == 0 {
		return traceStatusEmpty
	}
	return traceStatusComplete
}

func incompleteTrace(g graph.Result) bool {
	if !g.Summary.Complete || g.Summary.Truncated || len(g.Frontier) > 0 {
		return true
	}
	for _, b := range g.Terminals {
		switch b.Reason {
		case graph.NoIncomingCalls, graph.ServerReportedNoIncoming, graph.PrepareReturnedNoItem, graph.IncomingReturnedNull, graph.ExternalURI:
		default:
			return true
		}
	}
	for _, d := range g.Diagnostics {
		if d.Category == graph.UnresolvedCall {
			return true
		}
		switch d.Phase {
		case "invocation", "source", "trace", "spawn", "initialize", "prepare", "didOpen", "open", "shutdown":
			return true
		}
	}
	return false
}

func resolveTraceTargets(requested, authoritative []string, nodes map[string]graph.Node) ([]string, error) {
	auth := map[string]struct{}{}
	for _, id := range authoritative {
		if _, ok := nodes[id]; !ok {
			return nil, fmt.Errorf("authoritative target references unknown node %q", id)
		}
		auth[id] = struct{}{}
	}
	specs := requested
	if len(specs) == 0 {
		specs = authoritative
	}
	selected := make([]string, 0, len(specs))
	seen := map[string]struct{}{}
	for _, id := range specs {
		if strings.TrimSpace(id) != id || id == "" {
			return nil, fmt.Errorf("target must be an exact authoritative node ID: %q", id)
		}
		if _, ok := auth[id]; !ok {
			return nil, fmt.Errorf("target is not an authoritative retained target: %q", id)
		}
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			selected = append(selected, id)
		}
	}
	return selected, nil
}

func lessTraceNode(a, b graph.Node) bool {
	if a.Name != b.Name {
		return a.Name < b.Name
	}
	if a.URI != b.URI {
		return a.URI < b.URI
	}
	if a.SelectionRange.Start.Line != b.SelectionRange.Start.Line {
		return a.SelectionRange.Start.Line < b.SelectionRange.Start.Line
	}
	if a.SelectionRange.Start.Character != b.SelectionRange.Start.Character {
		return a.SelectionRange.Start.Character < b.SelectionRange.Start.Character
	}
	return a.ID < b.ID
}

func formatSelectionStart(n graph.Node) string {
	return fmt.Sprintf("%d:%d", n.SelectionRange.Start.Line+1, n.SelectionRange.Start.Character+1)
}
func formatNodeLocation(workspaceURI string, n graph.Node) string {
	if n.URI == "" {
		return ""
	}
	return fmt.Sprintf("%s:%s", displayURI(n.URI, workspaceURI), formatSelectionStart(n))
}
func formatRange(r graph.Range) string {
	return fmt.Sprintf("%d:%d-%d:%d", r.Start.Line+1, r.Start.Character+1, r.End.Line+1, r.End.Character+1)
}

func lessTraceRange(a, b graph.Range) bool {
	if a.Start.Line != b.Start.Line {
		return a.Start.Line < b.Start.Line
	}
	if a.Start.Character != b.Start.Character {
		return a.Start.Character < b.Start.Character
	}
	if a.End.Line != b.End.Line {
		return a.End.Line < b.End.Line
	}
	return a.End.Character < b.End.Character
}

func formatTraceRelations(out *strings.Builder, workspace, heading string, relations []traceV5TargetRelation, opts TraceV5Options) {
	visible := make([]traceV5TargetRelation, 0, len(relations))
	collapsedPeers := map[string]traceNodeClass{}
	collapsedRelations, testPeers, externalPeers := 0, map[string]struct{}{}, map[string]struct{}{}
	for _, rel := range relations {
		class := classifyTraceNode(rel.peer, workspace)
		collapse := (class.external && !opts.ExpandExternalNodes) || (class.test && !opts.ExpandTestNodes)
		if !collapse {
			visible = append(visible, rel)
			continue
		}
		collapsedRelations++
		collapsedPeers[rel.peer.ID] = class
		if class.external {
			externalPeers[rel.peer.ID] = struct{}{}
		} else if class.test {
			testPeers[rel.peer.ID] = struct{}{}
		}
	}
	fmt.Fprintf(out, "  %s: %d\n", heading, len(relations))
	sort.Slice(visible, func(i, j int) bool {
		if lessTraceNode(visible[i].peer, visible[j].peer) {
			return true
		}
		if lessTraceNode(visible[j].peer, visible[i].peer) {
			return false
		}
		return visible[i].edge.RelationID < visible[j].edge.RelationID
	})
	if len(visible) == 0 {
		fmt.Fprintln(out, "    none")
	}
	for _, rel := range visible {
		fmt.Fprintf(out, "    %s@%s\n", rel.peer.Name, formatNodeLocation(workspace, rel.peer))
		sites := append([]graph.Range(nil), rel.edge.CallSites...)
		sort.Slice(sites, func(i, j int) bool { return lessTraceRange(sites[i], sites[j]) })
		callerURI := displayURI(rel.caller.URI, workspace)
		for _, site := range sites {
			fmt.Fprintf(out, "      call-site: %s:%s\n", callerURI, formatRange(site))
		}
	}
	if collapsedRelations > 0 {
		fmt.Fprintf(out, "    collapsed peer nodes: %d\n", len(collapsedPeers))
		fmt.Fprintf(out, "    collapsed direct relations: %d\n", collapsedRelations)
		if len(testPeers) > 0 {
			fmt.Fprintf(out, "    test peer nodes: %d\n", len(testPeers))
		}
		if len(externalPeers) > 0 {
			fmt.Fprintf(out, "    external/outside-workspace peer nodes: %d\n", len(externalPeers))
		}
	}
}

func classifyTraceNode(node graph.Node, workspace string) traceNodeClass {
	if isOutsideWorkspaceNode(node.URI, workspace) {
		return traceNodeClass{external: true}
	}
	return traceNodeClass{test: isTestNode(node)}
}

func isTestNode(node graph.Node) bool {
	u, err := url.Parse(node.URI)
	if err != nil {
		return false
	}
	clean := path.Clean(u.Path)
	for _, segment := range strings.Split(strings.Trim(clean, "/"), "/") {
		if strings.EqualFold(segment, "test") || strings.EqualFold(segment, "tests") {
			return true
		}
	}
	base := strings.ToLower(path.Base(clean))
	// Closed presentation-only heuristic: test/tests path segments, Go _test.go,
	// and language-neutral .test. / .spec. filename infixes.
	return strings.HasSuffix(base, "_test.go") || strings.Contains(base, ".test.") || strings.Contains(base, ".spec.")
}

func isOutsideWorkspaceNode(nodeURI, workspaceURI string) bool {
	n, err := url.Parse(nodeURI)
	if err != nil || n.Scheme != "file" || n.Host != "" || n.Path == "" {
		return true
	}
	w, err := url.Parse(workspaceURI)
	if err != nil || w.Scheme != "file" || w.Host != "" || w.Path == "" {
		return true
	}
	nodePath, workspacePath := path.Clean(n.Path), path.Clean(w.Path)
	return nodePath != workspacePath && !strings.HasPrefix(nodePath, workspacePath+"/")
}

func formatBoundaries(out *strings.Builder, heading string, boundaries []graph.Boundary) {
	items := append([]graph.Boundary(nil), boundaries...)
	sort.Slice(items, func(i, j int) bool {
		if items[i].NodeID != items[j].NodeID {
			return items[i].NodeID < items[j].NodeID
		}
		if items[i].Reason != items[j].Reason {
			return items[i].Reason < items[j].Reason
		}
		return items[i].Message < items[j].Message
	})
	fmt.Fprintf(out, "%s: %d\n", heading, len(items))
	for _, b := range items {
		fmt.Fprintf(out, "  node=%s reason=%s", b.NodeID, b.Reason)
		if b.Message != "" {
			fmt.Fprintf(out, " message=%q", b.Message)
		}
		if b.Provenance != "" {
			fmt.Fprintf(out, " provenance=%s", b.Provenance)
		}
		fmt.Fprintln(out)
	}
}

func formatDiagnostics(out *strings.Builder, diagnostics []graph.Diagnostic) {
	items := append([]graph.Diagnostic(nil), diagnostics...)
	sort.Slice(items, func(i, j int) bool {
		a, _ := json.Marshal(items[i])
		b, _ := json.Marshal(items[j])
		return bytes.Compare(a, b) < 0
	})
	fmt.Fprintf(out, "graph diagnostics: %d\n", len(items))
	for _, d := range items {
		fmt.Fprintf(out, "  phase=%s method=%s node=%s category=%s message=%q\n", d.Phase, d.Method, d.NodeID, d.Category, d.Message)
	}
}
