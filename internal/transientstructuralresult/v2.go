package transientstructuralresult

import (
	"errors"
	"net/url"
	"path/filepath"
	"strings"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphkernel"
	"lsp-trace/internal/transientstructural"
)

type LocatorNodeV2 struct {
	ID               string      `json:"node_id"`
	Name             string      `json:"name"`
	Kind             int         `json:"kind"`
	Path             string      `json:"path"`
	DeclarationRange graph.Range `json:"declaration_range"`
}
type LocatorCallV2 struct {
	CallerID      string      `json:"caller_node_id"`
	CalleeID      string      `json:"callee_node_id"`
	Path          string      `json:"path"`
	CallSiteRange graph.Range `json:"call_site_range"`
}
type CouplingV2 struct {
	NodeID      string  `json:"node_id"`
	Ca          int     `json:"ca"`
	Ce          int     `json:"ce"`
	Instability float64 `json:"instability"`
}

var errOutsideWorkspace = errors.New("server location escapes workspace")

type LocatorResultV2 struct {
	SchemaVersion        string          `json:"schema_version"`
	Authority            int             `json:"authority"`
	SourceGraphComplete  string          `json:"source_graph_complete"`
	PositionEncoding     string          `json:"position_encoding"`
	TargetID             string          `json:"target_node_id"`
	Nodes                []LocatorNodeV2 `json:"nodes"`
	Calls                []LocatorCallV2 `json:"calls"`
	AnalyticsScope       string          `json:"analytics_scope"`
	Coupling             []CouplingV2    `json:"coupling"`
	ExternalNodesOmitted int             `json:"external_nodes_omitted"`
	ExternalCallsOmitted int             `json:"external_calls_omitted"`
}

func ProjectV2(in transientstructural.Result, q transientstructural.Request, transientID, root string) (LocatorResultV2, error) {
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil || !filepath.IsAbs(realRoot) {
		return LocatorResultV2{}, errors.New("invalid workspace root")
	}
	ids := make(map[string]string, len(in.Analysis.Nodes))
	external := make(map[string]bool)
	nodes := make([]LocatorNodeV2, 0, len(in.Analysis.Nodes))
	externalNodesOmitted := 0
	for _, n := range in.Analysis.Nodes {
		id, e := NodeID(transientID, q.Generation, n.ID)
		if e != nil {
			return LocatorResultV2{}, errors.New("invalid node")
		}
		p, e := relativeURI(realRoot, n.URI)
		if errors.Is(e, errOutsideWorkspace) {
			external[n.ID] = true
			externalNodesOmitted++
			continue
		}
		if e != nil {
			return LocatorResultV2{}, e
		}
		ids[n.ID] = id
		nodes = append(nodes, LocatorNodeV2{id, n.Name, n.Kind, p, n.Range})
	}
	target, ok := ids[in.TargetID]
	if !ok {
		return LocatorResultV2{}, errors.New("target absent")
	}
	calls := make([]LocatorCallV2, 0, len(in.Analysis.Occurrences))
	arcs := make([]graphkernel.Arc, 0, len(in.Analysis.Occurrences))
	externalCallsOmitted := 0
	for _, c := range in.Analysis.Occurrences {
		caller, cok := ids[c.CallerID]
		callee, dok := ids[c.CalleeID]
		if !cok || !dok {
			if external[c.CallerID] || external[c.CalleeID] {
				externalCallsOmitted++
				continue
			}
			return LocatorResultV2{}, errors.New("call endpoint absent")
		}
		p, e := relativeURI(realRoot, c.URI)
		if e != nil {
			return LocatorResultV2{}, e
		}
		calls = append(calls, LocatorCallV2{caller, callee, p, c.Range})
		arcs = append(arcs, graphkernel.Arc{From: caller, To: callee, Weight: 1})
	}
	nodeIdentities := make([]string, len(nodes))
	for i := range nodes {
		nodeIdentities[i] = nodes[i].ID
	}
	g, err := graphkernel.NewDirectedWeighted(nodeIdentities, arcs)
	if err != nil {
		return LocatorResultV2{}, errors.New("invalid coupling graph")
	}
	metrics := g.RobertMartinCoupling()
	coupling := make([]CouplingV2, len(metrics))
	for i := range metrics {
		coupling[i] = CouplingV2{NodeID: metrics[i].Node, Ca: metrics[i].Ca, Ce: metrics[i].Ce, Instability: metrics[i].Instability}
	}
	return LocatorResultV2{"lsp-trace.transient-structural-result.v2", 0, "UNKNOWN", in.Qualification.PositionEncoding, target, nodes, calls, "BOUNDED_LOCAL", coupling, externalNodesOmitted, externalCallsOmitted}, nil
}

func relativeURI(realRoot, raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "file" || u.Host != "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("invalid server location")
	}
	path, err := url.PathUnescape(u.Path)
	if err != nil || !filepath.IsAbs(path) {
		return "", errors.New("invalid server location")
	}
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", errors.New("unresolved server location")
	}
	rel, err := filepath.Rel(realRoot, real)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", errOutsideWorkspace
	}
	rel = filepath.ToSlash(filepath.Clean(rel))
	if rel == "." || strings.Contains(rel, "\\") {
		return "", errors.New("invalid relative path")
	}
	return rel, nil
}
