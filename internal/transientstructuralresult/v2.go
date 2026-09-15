package transientstructuralresult

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/url"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphkernel"
	"lsp-trace/internal/strictjson"
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
type StrongComponentV2 struct {
	Nodes  []string `json:"node_ids"`
	Cyclic bool     `json:"cyclic"`
}
type WeakBridgeV2 struct {
	NodeA string `json:"node_a"`
	NodeB string `json:"node_b"`
}
type NodeScoreV2 struct {
	NodeID string  `json:"node_id"`
	Score  float64 `json:"score"`
}
type HubAuthorityV2 struct {
	NodeID    string  `json:"node_id"`
	Hub       float64 `json:"hub"`
	Authority float64 `json:"authority"`
}

var errOutsideWorkspace = errors.New("server location escapes workspace")

type LocatorResultV2 struct {
	SchemaVersion        string              `json:"schema_version"`
	Authority            int                 `json:"authority"`
	SourceGraphComplete  string              `json:"source_graph_complete"`
	PositionEncoding     string              `json:"position_encoding"`
	TargetID             string              `json:"target_node_id"`
	Nodes                []LocatorNodeV2     `json:"nodes"`
	Calls                []LocatorCallV2     `json:"calls"`
	AnalyticsScope       string              `json:"analytics_scope"`
	Coupling             []CouplingV2        `json:"coupling"`
	StrongComponents     []StrongComponentV2 `json:"strong_components"`
	WeakProjection       string              `json:"weak_projection"`
	WeakBridges          []WeakBridgeV2      `json:"weak_bridges"`
	ArticulationPoints   []string            `json:"articulation_points"`
	PageRankDamping      float64             `json:"pagerank_damping"`
	AnalyticsTolerance   float64             `json:"analytics_tolerance"`
	PageRank             []NodeScoreV2       `json:"pagerank"`
	HITS                 []HubAuthorityV2    `json:"hits"`
	ExternalNodesOmitted int                 `json:"external_nodes_omitted"`
	ExternalCallsOmitted int                 `json:"external_calls_omitted"`
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
	components := g.StrongComponents()
	strong := make([]StrongComponentV2, len(components))
	for i, component := range components {
		strong[i] = StrongComponentV2{Nodes: component.Nodes, Cyclic: component.Cyclic}
	}
	bridges, articulation := g.WeakCritical()
	weak := make([]WeakBridgeV2, len(bridges))
	for i, bridge := range bridges {
		weak[i] = WeakBridgeV2{NodeA: bridge.A, NodeB: bridge.B}
	}
	pageRankMetrics := g.PageRank()
	pageRank := make([]NodeScoreV2, len(pageRankMetrics))
	for i, metric := range pageRankMetrics {
		pageRank[i] = NodeScoreV2{NodeID: metric.Node, Score: metric.Score}
	}
	hitsMetrics := g.HITS()
	hits := make([]HubAuthorityV2, len(hitsMetrics))
	for i, metric := range hitsMetrics {
		hits[i] = HubAuthorityV2{NodeID: metric.Node, Hub: metric.Hub, Authority: metric.Authority}
	}
	result := LocatorResultV2{
		SchemaVersion: "lsp-trace.transient-structural-result.v2", Authority: 0, SourceGraphComplete: "UNKNOWN",
		PositionEncoding: in.Qualification.PositionEncoding, TargetID: target, Nodes: nodes, Calls: calls,
		AnalyticsScope: "BOUNDED_LOCAL", Coupling: coupling, StrongComponents: strong,
		WeakProjection: graphkernel.WeakProjectionPolicy, WeakBridges: weak, ArticulationPoints: articulation,
		PageRankDamping: graphkernel.PageRankDamping, AnalyticsTolerance: graphkernel.AnalyticsTolerance,
		PageRank: pageRank, HITS: hits, ExternalNodesOmitted: externalNodesOmitted, ExternalCallsOmitted: externalCallsOmitted,
	}
	if err := ValidateV2(result); err != nil {
		return LocatorResultV2{}, err
	}
	return result, nil
}

func DecodeV2Artifact(raw []byte) (LocatorResultV2, error) {
	if len(raw) == 0 || len(raw) > 1048576 || strictjson.RejectDuplicates(raw) != nil {
		return LocatorResultV2{}, errors.New("V2 artifact must be strict bounded JSON")
	}
	var r LocatorResultV2
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&r); err != nil {
		return LocatorResultV2{}, err
	}
	if err := d.Decode(&struct{}{}); err != io.EOF {
		return LocatorResultV2{}, errors.New("trailing JSON")
	}
	if err := ValidateV2(r); err != nil {
		return LocatorResultV2{}, err
	}
	return r, nil
}

func ValidateV2(r LocatorResultV2) error {
	if r.SchemaVersion != "lsp-trace.transient-structural-result.v2" {
		return errors.New("invalid V2 schema version")
	}
	if r.Authority != 0 || r.SourceGraphComplete != "UNKNOWN" || r.AnalyticsScope != "BOUNDED_LOCAL" {
		return errors.New("invalid V2 qualification")
	}
	if r.WeakProjection != graphkernel.WeakProjectionPolicy || r.PageRankDamping != graphkernel.PageRankDamping || r.AnalyticsTolerance != graphkernel.AnalyticsTolerance {
		return errors.New("invalid V2 analytics policy")
	}
	if r.ExternalNodesOmitted < 0 || r.ExternalCallsOmitted < 0 {
		return errors.New("negative omission count")
	}
	if r.PositionEncoding != "utf-8" && r.PositionEncoding != "utf-16" && r.PositionEncoding != "utf-32" {
		return errors.New("invalid position encoding")
	}
	if len(r.Nodes) < 1 || len(r.Nodes) > 10000 || len(r.Calls) > 100000 {
		return errors.New("invalid graph bounds")
	}
	ids := make(map[string]bool, len(r.Nodes))
	for _, n := range r.Nodes {
		if !validV2NodeID(n.ID) || ids[n.ID] || strings.TrimSpace(n.Name) == "" || !validV2Path(n.Path) || !validV2Range(n.DeclarationRange) {
			return errors.New("invalid or duplicate node")
		}
		ids[n.ID] = true
	}
	if !ids[r.TargetID] {
		return errors.New("target absent")
	}
	for _, c := range r.Calls {
		if !ids[c.CallerID] || !ids[c.CalleeID] || !validV2Path(c.Path) || !validV2Range(c.CallSiteRange) {
			return errors.New("invalid call")
		}
	}
	seen := make(map[string]bool, len(ids))
	previousComponent := ""
	for _, c := range r.StrongComponents {
		if len(c.Nodes) == 0 || !sort.StringsAreSorted(c.Nodes) {
			return errors.New("invalid component ordering")
		}
		componentKey := strings.Join(c.Nodes, "\x00")
		if previousComponent != "" && previousComponent >= componentKey {
			return errors.New("invalid component ordering")
		}
		previousComponent = componentKey
		for _, id := range c.Nodes {
			if !ids[id] || seen[id] {
				return errors.New("component membership mismatch")
			}
			seen[id] = true
		}
	}
	if len(seen) != len(ids) || len(r.Coupling) != len(ids) || len(r.PageRank) != len(ids) || len(r.HITS) != len(ids) {
		return errors.New("analytics node coverage mismatch")
	}
	for i, id := range sortedIDs(ids) {
		c, p, h := r.Coupling[i], r.PageRank[i], r.HITS[i]
		wantInstability := 0.0
		if c.Ca < 0 || c.Ce < 0 {
			return errors.New("negative coupling")
		}
		if c.Ca+c.Ce > 0 {
			wantInstability = float64(c.Ce) / float64(c.Ca+c.Ce)
		}
		if c.NodeID != id || p.NodeID != id || h.NodeID != id || !finiteNumber(c.Instability) || math.Abs(c.Instability-wantInstability) > r.AnalyticsTolerance || !finiteNumber(p.Score) || p.Score < 0 || !finiteNumber(h.Hub) || h.Hub < 0 || !finiteNumber(h.Authority) || h.Authority < 0 {
			return errors.New("analytics ordering, formula, or number invalid")
		}
	}
	lastBridge := ""
	for _, b := range r.WeakBridges {
		key := b.NodeA + "\x00" + b.NodeB
		if !ids[b.NodeA] || !ids[b.NodeB] || b.NodeA >= b.NodeB || (lastBridge != "" && lastBridge >= key) {
			return errors.New("invalid weak bridge reference or ordering")
		}
		lastBridge = key
	}
	if !sort.StringsAreSorted(r.ArticulationPoints) {
		return errors.New("invalid articulation ordering")
	}
	last := ""
	for _, id := range r.ArticulationPoints {
		if !ids[id] || id == last {
			return errors.New("invalid articulation reference")
		}
		last = id
	}
	return nil
}

func validV2NodeID(id string) bool {
	if len(id) != 35 || !strings.HasPrefix(id, "tn_") {
		return false
	}
	for _, c := range id[3:] {
		if !strings.ContainsRune("0123456789abcdef", c) {
			return false
		}
	}
	return true
}
func validV2Path(p string) bool {
	return p != "" && len(p) < 4096 && !path.IsAbs(p) && !strings.Contains(p, "\\") && path.Clean(p) == p && p != "." && p != ".." && !strings.HasPrefix(p, "../")
}
func validV2Range(r graph.Range) bool {
	return r.Start.Line < r.End.Line || (r.Start.Line == r.End.Line && r.Start.Character <= r.End.Character)
}
func sortedIDs(ids map[string]bool) []string {
	out := make([]string, 0, len(ids))
	for id := range ids {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}
func finiteNumber(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

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
