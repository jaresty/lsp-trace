package graph

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// NativeV5RepresentativeView is the narrow, immutable Stage 6 projection of a
// strictly verified native Graph V5 bundle. Its zero value is invalid.
type NativeV5RepresentativeView struct {
	valid             bool
	executionBundleID string
	nodeIDs           []string
	edges             []Edge
	invocationSeeds   []InvocationSeed
	seedMemberships   []SeedMembership
}

// NewNativeV5RepresentativeView validates exact canonical Graph V5 bytes before
// retaining only the fields required by the private representative adapter.
func NewNativeV5RepresentativeView(raw []byte) (NativeV5RepresentativeView, error) {
	if len(raw) == 0 || ValidateSemanticBundle(raw) != nil {
		return NativeV5RepresentativeView{}, fmt.Errorf("invalid native Graph V5 representative input")
	}
	var bundle bundleV3
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&bundle); err != nil {
		return NativeV5RepresentativeView{}, fmt.Errorf("decode native Graph V5 representative input: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF || bundle.SchemaVersion != SchemaVersionV5 || bundle.ExecutionBundleID == "" {
		return NativeV5RepresentativeView{}, fmt.Errorf("invalid native Graph V5 representative envelope")
	}
	if !strictRepresentativeEdges(bundle.Edges) {
		return NativeV5RepresentativeView{}, fmt.Errorf("noncanonical native Graph V5 representative edges")
	}
	view := NativeV5RepresentativeView{
		valid:             true,
		executionBundleID: bundle.ExecutionBundleID,
		nodeIDs:           make([]string, len(bundle.Nodes)),
		edges:             cloneRepresentativeEdges(bundle.Edges),
		invocationSeeds:   append([]InvocationSeed(nil), bundle.Invocation.Seeds...),
		seedMemberships:   append([]SeedMembership(nil), bundle.SeedMemberships...),
	}
	for i := range bundle.Nodes {
		view.nodeIDs[i] = bundle.Nodes[i].ID
	}
	return view, nil
}

func (v NativeV5RepresentativeView) Valid() bool { return v.valid }
func (v NativeV5RepresentativeView) ExecutionBundleID() string {
	if !v.valid {
		return ""
	}
	return v.executionBundleID
}
func (v NativeV5RepresentativeView) NodeIDs() []string {
	if !v.valid {
		return []string{}
	}
	return append([]string(nil), v.nodeIDs...)
}
func (v NativeV5RepresentativeView) Edges() []Edge {
	if !v.valid {
		return []Edge{}
	}
	return cloneRepresentativeEdges(v.edges)
}
func (v NativeV5RepresentativeView) InvocationSeeds() []InvocationSeed {
	if !v.valid {
		return []InvocationSeed{}
	}
	return append([]InvocationSeed(nil), v.invocationSeeds...)
}
func (v NativeV5RepresentativeView) SeedMemberships() []SeedMembership {
	if !v.valid {
		return []SeedMembership{}
	}
	return append([]SeedMembership(nil), v.seedMemberships...)
}

func strictRepresentativeEdges(edges []Edge) bool {
	seen := make(map[string]struct{}, len(edges))
	for i, edge := range edges {
		key := edge.RelationID + "\x00" + edge.CallerNodeID + "\x00" + edge.CalleeNodeID
		if _, ok := seen[key]; ok {
			return false
		}
		seen[key] = struct{}{}
		if i > 0 {
			previous := edges[i-1]
			if previous.CallerNodeID > edge.CallerNodeID || (previous.CallerNodeID == edge.CallerNodeID && previous.CalleeNodeID >= edge.CalleeNodeID) {
				return false
			}
		}
		for j := 1; j < len(edge.CallSites); j++ {
			if !lessRange(edge.CallSites[j-1], edge.CallSites[j]) {
				return false
			}
		}
	}
	return true
}

func cloneRepresentativeEdges(in []Edge) []Edge {
	out := append([]Edge(nil), in...)
	for i := range out {
		out[i].CallSites = append([]Range(nil), in[i].CallSites...)
	}
	return out
}
