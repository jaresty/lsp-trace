package graph

import (
	"encoding/json"
	"fmt"
)

// DecodeNativeV3 recovers the native CALLS model through the same bundleV3
// projection used by ValidateSemanticBundle. Callers must preflight byte/depth
// limits and structurally validate before calling. It does not authenticate the
// producer or recover process secrets omitted by native serialization.
func DecodeNativeV3(data []byte) (Result, error) {
	if err := ValidateSemanticBundle(data); err != nil {
		return Result{}, err
	}
	var b bundleV3
	if err := json.Unmarshal(data, &b); err != nil {
		return Result{}, err
	}
	if len(b.SiblingCandidates) != 0 || len(b.DispatchRelationships) != 0 {
		return Result{}, fmt.Errorf("native CALLS decoder excludes discovery relations")
	}
	r := Result{SchemaVersion: b.SchemaVersion, Invocation: b.Invocation, Tool: b.Tool, Targets: b.Targets, Nodes: b.Nodes, Edges: b.Edges, Terminals: b.Terminals, Frontier: b.Frontier, Diagnostics: b.Diagnostics, Seeds: b.Seeds, Slice: b.Slice, Capabilities: b.Capabilities, CapabilityQuality: b.CapabilityQuality, Summary: Summary{NodeCount: b.Summary.NodeCount, EdgeCount: b.Summary.EdgeCount, TerminalCount: b.Summary.TerminalCount, CycleCount: b.Summary.CycleCount, Complete: b.Summary.TraversalComplete, Truncated: b.Summary.Truncated}}
	for i := range r.Seeds {
		if r.Seeds[i].ReachedRelationIDs == nil {
			r.Seeds[i].ReachedRelationIDs = []string{}
		}
	}
	return r, nil
}
