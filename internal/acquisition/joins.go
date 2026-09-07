package acquisition

import (
	"encoding/json"
	"errors"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/lsp"
	"reflect"
	"sort"
)

func validateTargetJoins(r Result, records map[string]RequestRecord) error {
	seeds := map[string]graph.SeedResult{}
	for _, s := range r.Graph.Seeds {
		seeds[s.Label] = s
	}
	edges := map[string]graph.Edge{}
	for _, e := range r.Graph.Edges {
		edges[e.RelationID] = e
	}
	observed := map[string][]string{}
	for _, o := range r.EdgeObservations {
		observed[o.RequestID] = append(observed[o.RequestID], o.RelationID)
	}
	for _, t := range r.Targets {
		if t.Resolution.Status == Resolved {
			rr := t.Resolution
			matched := false
			for _, id := range rr.RequestIDs {
				rec := records[id]
				if rec.Method != "textDocument/prepareCallHierarchy" || rec.Outcome != "SUCCESS" {
					continue
				}
				var p lsp.PrepareCallHierarchyParams
				var items []lsp.CallHierarchyItem
				if json.Unmarshal(rec.Params, &p) != nil || json.Unmarshal(rec.Response, &items) != nil {
					return errors.New("invalid resolution capture")
				}
				if p.TextDocument.URI != t.Requested.Locator.URI || p.Position != *rr.Position {
					continue
				}
				for _, item := range items {
					if graph.SameNodeIdentity(node(item), *rr.Identity) {
						matched = true
					}
				}
			}
			if !matched {
				return errors.New("resolution lacks prepared response")
			}
			if t.Requested.Locator.Symbol != "" && t.Requested.Locator.Symbol != rr.Identity.Name {
				return errors.New("symbol name substitution")
			}
			if t.Requested.Locator.Line != nil && (*t.Requested.Locator.Line != rr.Position.Line || *t.Requested.Locator.Character != rr.Position.Character) {
				return errors.New("position substitution")
			}
		}
		members, relations := map[string]bool{}, map[string]bool{}
		if t.Admission == Admitted {
			members[t.Resolution.Identity.ID] = true
			initial := t.Outgoing
			if r.Request.Mode == Incoming {
				initial = t.Incoming
			}
			if len(initial.StartIDs) != 1 || initial.StartIDs[0] != t.Resolution.Identity.ID || len(initial.Expansions) == 0 {
				return errors.New("admitted seed lacks initial directional accounting")
			}
		}
		for direction, d := range []DirectionResult{t.Outgoing, t.Incoming} {
			limit := t.Requested.UpDepth
			if direction == 0 {
				limit = t.Requested.DownDepth
			}
			for _, e := range d.Expansions {
				if e.Status == Frontier && e.Depth != limit {
					return errors.New("frontier depth mismatch")
				}
				if e.Status != Frontier && e.Depth >= limit {
					return errors.New("expanded beyond depth")
				}
				if e.Status != SuccessNonempty && e.Status != Partial {
					continue
				}
				for _, id := range observed[e.RequestID] {
					edge := edges[id]
					relations[id] = true
					members[edge.CallerNodeID] = true
					members[edge.CalleeNodeID] = true
				}
			}
		}
		seed := seeds[t.Requested.ID]
		if !reflect.DeepEqual(keys(members), seed.ReachedNodeIDs) || !reflect.DeepEqual(keys(relations), seed.ReachedRelationIDs) {
			return errors.New("native seed membership does not match target expansion")
		}
		if r.Request.Mode == Incoming {
			if t.Outgoing.Status != ExpansionNotApplicable || len(t.Outgoing.Expansions) != 0 {
				return errors.New("incoming-only has outgoing acquisition")
			}
		} else if t.Admission == Admitted {
			union := append(append([]string{}, t.Outgoing.FrontierIDs...), t.Outgoing.SuccessfulEmptyIDs...)
			sort.Strings(union)
			union = unique(union)
			if len(union) != len(t.Incoming.StartIDs) {
				return errors.New("incoming starts not frontier/empty union")
			}
			for i, id := range union {
				if t.Incoming.StartIDs[i] != id {
					return errors.New("incoming starts not frontier/empty union")
				}
			}
		}
	}
	return nil
}
