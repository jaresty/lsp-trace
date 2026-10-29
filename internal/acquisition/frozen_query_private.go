package acquisition

import (
	"errors"
	"fmt"
	"strings"

	"lsp-trace/internal/graph"
)

const FrozenQueryStopPrivate = "EXTERNAL_TO_FROZEN_QUERYSET"

func (r Request) frozenQueryPrivate() bool {
	return r.FrozenQueryURIsPrivate != [6]string{} || r.frozenQueryDeclarationPrivateV2 != nil
}
func (r Request) queryAllowedPrivate(uri string) bool {
	if r.frozenQueryDeclarationPrivateV2 != nil {
		return r.FrozenQueryURIsPrivate == [6]string{} && r.frozenQueryDeclarationPrivateV2.contains(uri)
	}
	if !r.frozenQueryPrivate() {
		return true
	}
	for _, member := range r.FrozenQueryURIsPrivate {
		if member == uri {
			return true
		}
	}
	return false
}
func (r Request) validateFrozenQueryPrivate() error {
	if !r.frozenQueryPrivate() {
		return nil
	}
	if r.frozenQueryDeclarationPrivateV2 != nil {
		if r.FrozenQueryURIsPrivate != [6]string{} {
			return errors.New("private frozen query V1 and V2 declarations cannot be combined")
		}
		if err := r.frozenQueryDeclarationPrivateV2.validate(); err != nil {
			return err
		}
	} else {
		for i, uri := range r.FrozenQueryURIsPrivate {
			if !canonicalURI(uri) || (i > 0 && r.FrozenQueryURIsPrivate[i-1] >= uri) {
				return errors.New("private frozen query declaration must be sorted canonical six")
			}
		}
	}
	for _, t := range append([]Target{r.Root}, r.RequiredTargets...) {
		if !r.queryAllowedPrivate(t.Locator.URI) {
			return errors.New("private target outside frozen query declaration")
		}
	}
	return nil
}

// The boundary cites real retained observations. It is not a request or a
// fabricated response. For a seed there may be no originating edge observation.
func (c *runner) frozenBoundaryPrivate(target, id string, d Direction) graph.Boundary {
	origins := []string{}
	incident := map[string]bool{}
	for _, e := range c.result.Graph.Edges {
		if e.CallerNodeID == id || e.CalleeNodeID == id {
			incident[e.RelationID] = true
		}
	}
	for _, o := range c.result.EdgeObservations {
		if incident[o.RelationID] {
			origins = append(origins, o.RequestID)
		}
	}
	return graph.Boundary{NodeID: id, Reason: graph.ExternalURI, Provenance: "CLIENT_DERIVED", Message: fmt.Sprintf("external-to-frozen-queryset direction=%s target=%s originating_observations=%s", d, target, strings.Join(origins, ","))}
}
