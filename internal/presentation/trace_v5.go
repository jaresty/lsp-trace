package presentation

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/schema"
)

const traceV5PresentationMarker = "lsp-trace presentation only — derived view; not authoritative evidence"

// TraceV5Options controls presentation of retained target-centered graph output.
type TraceV5Options struct {
	// Targets identifies nodes by exact node ID or by node name. A bare name may
	// match multiple nodes and fail with an explicit ambiguity error.
	// To disambiguate a name, provide "name@uri:line:column" using one-based
	// selection coordinates.
	Targets []string

	// ExpandTestNodes controls whether test-only files are shown by default.
	// By default test nodes are omitted and counted as collapsed.
	ExpandTestNodes bool
	// ExpandExternalNodes controls whether nodes outside the traced workspace are
	// shown by default. By default external and outside-workspace nodes are omitted
	// and counted as collapsed.
	ExpandExternalNodes bool
}

type traceV5Status string

const (
	traceStatusComplete traceV5Status = "COMPLETE"
	traceStatusPartial  traceV5Status = "PARTIAL"
	traceStatusEmpty    traceV5Status = "EMPTY"
)

type traceV5TargetRelation struct {
	peer graph.Node
	edge graph.Edge
}

type traceV5Summary struct {
	NodeCount           int    `json:"node_count"`
	EdgeCount           int    `json:"edge_count"`
	TerminalCount       int    `json:"terminal_count"`
	CycleCount          int    `json:"cycle_count"`
	TraversalComplete   bool   `json:"traversal_complete"`
	SourceGraphComplete string `json:"source_graph_complete"`
	CompletenessScope   string `json:"completeness_scope"`
	Truncated           bool   `json:"truncated"`
}

// RenderTraceV5 renders a deterministic, target-centered view over graph-v5
// evidence embedded in a valid graph-provenance-v5 envelope.
func RenderTraceV5(data []byte, opts TraceV5Options) (string, error) {
	if _, err := graphprovenance.ValidateFor(data, graphprovenance.Family, "v5"); err != nil {
		return "", fmt.Errorf("provenance envelope: %w", err)
	}

	var env graphprovenance.EvidenceV5
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(&env); err != nil {
		return "", fmt.Errorf("decode provenance envelope: %w", err)
	}
	native, err := base64.StdEncoding.DecodeString(env.GraphV5)
	if err != nil {
		return "", fmt.Errorf("decode embedded graph_v5: %w", err)
	}
	if expected, got := env.GraphV5SHA256, fmt.Sprintf("sha256:%x", sha256.Sum256(native)); expected != got {
		return "", fmt.Errorf("embedded graph_v5 digest mismatch")
	}
	if _, err := schema.ValidateStructure(native, schema.FamilyGraph, graph.SchemaVersionV5); err != nil {
		return "", fmt.Errorf("validate embedded graph_v5: %w", err)
	}

	type traceV5Graph struct {
		Invocation struct {
			WorkspaceURI string `json:"workspace_uri"`
		} `json:"invocation"`
		Targets []string       `json:"targets"`
		Nodes   []graph.Node   `json:"nodes"`
		Edges   []graph.Edge   `json:"edges"`
		Summary traceV5Summary `json:"summary"`
	}
	var g traceV5Graph
	d = json.NewDecoder(bytes.NewReader(native))
	if err := d.Decode(&g); err != nil {
		return "", fmt.Errorf("decode embedded graph_v5: %w", err)
	}

	nodes := map[string]graph.Node{}
	for _, n := range g.Nodes {
		if _, ok := nodes[n.ID]; ok {
			return "", fmt.Errorf("embedded graph has duplicate node %q", n.ID)
		}
		nodes[n.ID] = n
	}

	targetIDs, err := resolveTraceTargets(opts.Targets, g.Targets, nodes)
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
		incoming[e.CalleeNodeID] = append(incoming[e.CalleeNodeID], traceV5TargetRelation{peer: caller, edge: e})
		outgoing[e.CallerNodeID] = append(outgoing[e.CallerNodeID], traceV5TargetRelation{peer: callee, edge: e})
	}

	sort.Slice(targetIDs, func(i, j int) bool {
		a, b := nodes[targetIDs[i]], nodes[targetIDs[j]]
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		if a.URI != b.URI {
			return a.URI < b.URI
		}
		if a.SelectionRange.Start != b.SelectionRange.Start {
			if a.SelectionRange.Start.Line != b.SelectionRange.Start.Line {
				return a.SelectionRange.Start.Line < b.SelectionRange.Start.Line
			}
			return a.SelectionRange.Start.Character < b.SelectionRange.Start.Character
		}
		return targetIDs[i] < targetIDs[j]
	})

	nameCounts := map[string]int{}
	for _, id := range targetIDs {
		nameCounts[nodes[id].Name]++
	}

	var out strings.Builder
	fmt.Fprintln(&out, traceV5PresentationMarker)
	fmt.Fprintf(&out, "status: %s\n", traceV5StatusFromSummary(g.Summary))
	fmt.Fprintf(&out, "nodes: %d\nedges: %d\nterminals: %d\ncycles: %d\n", g.Summary.NodeCount, g.Summary.EdgeCount, g.Summary.TerminalCount, g.Summary.CycleCount)
	fmt.Fprintf(&out, "traversal_complete: %t\ntruncated: %t\n", g.Summary.TraversalComplete, g.Summary.Truncated)
	if g.Summary.SourceGraphComplete != "" {
		fmt.Fprintf(&out, "source_graph_complete: %s\n", g.Summary.SourceGraphComplete)
	}
	if g.Summary.CompletenessScope != "" {
		fmt.Fprintf(&out, "completeness_scope: %s\n", g.Summary.CompletenessScope)
	}
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
		formatTraceRelations(&out, g.Invocation.WorkspaceURI, "direct incoming", incoming[id], targetName, opts)
		formatTraceRelations(&out, g.Invocation.WorkspaceURI, "direct outgoing", outgoing[id], targetName, opts)
	}

	return out.String(), nil
}

func traceV5StatusFromSummary(summary traceV5Summary) traceV5Status {
	if summary.NodeCount == 0 {
		return traceStatusEmpty
	}
	if !summary.TraversalComplete || summary.Truncated {
		return traceStatusPartial
	}
	return traceStatusComplete
}

func resolveTraceTargets(requested, fallback []string, nodes map[string]graph.Node) ([]string, error) {
	specs := requested
	if len(specs) == 0 {
		specs = fallback
	}
	if len(specs) == 0 {
		return nil, nil
	}
	nameIndex := map[string][]string{}
	for id, node := range nodes {
		nameIndex[node.Name] = append(nameIndex[node.Name], id)
	}
	for _, ids := range nameIndex {
		sort.Strings(ids)
	}

	selected := make([]string, 0, len(specs))
	seen := map[string]struct{}{}
	for _, spec := range specs {
		spec = strings.TrimSpace(spec)
		if spec == "" {
			return nil, fmt.Errorf("empty target spec")
		}
		if _, ok := nodes[spec]; ok {
			if _, ok := seen[spec]; !ok {
				selected = append(selected, spec)
				seen[spec] = struct{}{}
			}
			continue
		}
		parsed, err := parseTargetSelector(spec)
		if err != nil {
			return nil, err
		}
		cands := append([]string(nil), nameIndex[parsed.Name]...)
		if len(cands) == 0 {
			return nil, fmt.Errorf("target not found: %q", spec)
		}
		if parsed.URI != "" {
			filtered := make([]string, 0, len(cands))
			for _, id := range cands {
				n := nodes[id]
				if n.URI == parsed.URI && (!parsed.HasLineCol || (int(n.SelectionRange.Start.Line)+1 == parsed.Line && int(n.SelectionRange.Start.Character)+1 == parsed.Character)) {
					filtered = append(filtered, id)
				}
			}
			cands = filtered
		}
		if len(cands) == 0 {
			return nil, fmt.Errorf("target not found: %q", spec)
		}
		if len(cands) > 1 && !parsed.HasLineCol && parsed.URI == "" {
			return nil, fmt.Errorf("ambiguous target %q; specify disambiguating @uri:line:column", spec)
		}
		if len(cands) > 1 {
			parts := make([]string, 0, len(cands))
			for _, id := range cands {
				n := nodes[id]
				parts = append(parts, fmt.Sprintf("%s (%s)", n.URI, formatSelectionStart(n)))
			}
			sort.Strings(parts)
			return nil, fmt.Errorf("ambiguous target %q: %s", spec, strings.Join(parts, ", "))
		}
		id := cands[0]
		if _, ok := seen[id]; !ok {
			selected = append(selected, id)
			seen[id] = struct{}{}
		}
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("no matching targets")
	}
	return selected, nil
}

type traceTargetSpec struct {
	Name       string
	URI        string
	Line       int
	Character  int
	HasLineCol bool
}

func parseTargetSelector(spec string) (traceTargetSpec, error) {
	selector := traceTargetSpec{Name: spec}
	at := strings.Index(spec, "@")
	if at < 0 {
		return selector, nil
	}
	name := strings.TrimSpace(spec[:at])
	locator := spec[at+1:]
	if name == "" || locator == "" {
		return traceTargetSpec{}, fmt.Errorf("invalid target specifier %q", spec)
	}
	lineIdx := strings.LastIndex(locator, ":")
	if lineIdx < 0 || lineIdx == len(locator)-1 {
		return traceTargetSpec{}, fmt.Errorf("invalid target specifier %q", spec)
	}
	colIdx := strings.LastIndex(locator[:lineIdx], ":")
	if colIdx < 0 || colIdx >= len(locator)-2 {
		return traceTargetSpec{}, fmt.Errorf("invalid target specifier %q", spec)
	}
	lineText := locator[colIdx+1 : lineIdx]
	charText := locator[lineIdx+1:]
	line, err := strconv.Atoi(lineText)
	if err != nil || line < 1 {
		return traceTargetSpec{}, fmt.Errorf("invalid target specifier %q", spec)
	}
	char, err := strconv.Atoi(charText)
	if err != nil || char < 1 {
		return traceTargetSpec{}, fmt.Errorf("invalid target specifier %q", spec)
	}
	selector = traceTargetSpec{Name: name, URI: locator[:colIdx], Line: line, Character: char, HasLineCol: true}
	return selector, nil
}

func formatSelectionStart(n graph.Node) string {
	return fmt.Sprintf("%d:%d", n.SelectionRange.Start.Line+1, n.SelectionRange.Start.Character+1)
}

func formatNodeLocation(workspaceURI string, n graph.Node) string {
	if n.URI == "" {
		return ""
	}
	if offset := formatSelectionStart(n); offset != "" {
		return fmt.Sprintf("%s:%s", displayURI(n.URI, workspaceURI), offset)
	}
	return n.URI
}

func formatRange(r graph.Range) string {
	return fmt.Sprintf("%d:%d-%d:%d", r.Start.Line+1, r.Start.Character+1, r.End.Line+1, r.End.Character+1)
}

func formatTraceRelations(out *strings.Builder, workspace string, heading string, relations []traceV5TargetRelation, _ string, opts TraceV5Options) {
	visible := make([]traceV5TargetRelation, 0, len(relations))
	omittedTest := 0
	omittedExternal := 0
	omittedTotal := 0
	for _, rel := range relations {
		if shouldCollapseNode(rel.peer, workspace, opts) {
			omittedTotal++
			if isTestNode(rel.peer) {
				omittedTest++
			}
			if isOutsideWorkspaceNode(rel.peer.URI, workspace) {
				omittedExternal++
			}
			continue
		}
		visible = append(visible, rel)
	}
	fmt.Fprintf(out, "  %s: %d\n", heading, len(relations))
	sort.Slice(visible, func(i, j int) bool {
		a, b := visible[i].peer, visible[j].peer
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		if a.URI != b.URI {
			return a.URI < b.URI
		}
		if a.SelectionRange.Start != b.SelectionRange.Start {
			if a.SelectionRange.Start.Line != b.SelectionRange.Start.Line {
				return a.SelectionRange.Start.Line < b.SelectionRange.Start.Line
			}
			return a.SelectionRange.Start.Character < b.SelectionRange.Start.Character
		}
		if visible[i].edge.RelationID != visible[j].edge.RelationID {
			return visible[i].edge.RelationID < visible[j].edge.RelationID
		}
		if len(visible[i].edge.CallSites) == 0 {
			return false
		}
		if len(visible[j].edge.CallSites) == 0 {
			return true
		}
		return formatRange(visible[i].edge.CallSites[0]) < formatRange(visible[j].edge.CallSites[0])
	})
	if len(visible) == 0 {
		fmt.Fprintln(out, "    none")
	} else {
		for _, rel := range visible {
			fmt.Fprintf(out, "    %s@%s\n", rel.peer.Name, formatNodeLocation(workspace, rel.peer))
			for _, callSite := range rel.edge.CallSites {
				fmt.Fprintf(out, "      call-site: %s\n", formatRange(callSite))
			}
		}
	}
	if omittedTotal > 0 {
		parts := make([]string, 0, 2)
		if omittedTest > 0 {
			parts = append(parts, fmt.Sprintf("%d test", omittedTest))
		}
		if omittedExternal > 0 {
			parts = append(parts, fmt.Sprintf("%d external/outside-workspace", omittedExternal))
		}
		fmt.Fprintf(out, "    omitted collapsed nodes (%s): %d\n", strings.Join(parts, ", "), omittedTotal)
	}
}

func shouldCollapseNode(node graph.Node, workspace string, opts TraceV5Options) bool {
	if isTestNode(node) && !opts.ExpandTestNodes {
		return true
	}
	if isOutsideWorkspaceNode(node.URI, workspace) && !opts.ExpandExternalNodes {
		return true
	}
	return false
}

func isTestNode(node graph.Node) bool {
	u, err := url.Parse(node.URI)
	if err != nil {
		return false
	}
	base := path.Base(u.Path)
	return strings.HasSuffix(base, "_test.go")
}

func isOutsideWorkspaceNode(nodeURI, workspaceURI string) bool {
	n, err := url.Parse(nodeURI)
	if err != nil {
		return true
	}
	if n.Scheme != "file" {
		return true
	}
	if n.Host != "" {
		return true
	}
	if n.Path == "" {
		return true
	}
	w, err := url.Parse(workspaceURI)
	if err != nil || w.Path == "" || w.Scheme == "" {
		return true
	}
	if w.Scheme != "file" || w.Host != "" {
		return true
	}
	nodePath := path.Clean(n.Path)
	workspacePath := path.Clean(w.Path)
	if nodePath == workspacePath {
		return false
	}
	return !(strings.HasPrefix(nodePath, workspacePath+"/") && nodePath != workspacePath)
}
