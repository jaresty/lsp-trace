package programc

import (
	"bytes"
	"errors"

	rg "lsp-trace/internal/referencegrouping"
)

// GroupingPairBindingPrivate is metadata from already-created opaque values.
// It does NOT validate arbitrary serialized results or qualify their policy.
type GroupingPairBindingPrivate struct {
	Case, Result, Projection                                             rg.CompositionCommitment
	Bindings                                                             rg.EvaluationBindingsPrivate
	CaseID, RunID, SelectorID, SelectorDigest, ClaimCeiling              string
	Seed                                                                 uint64
	Nodes, Occurrences, Custody, Members, Communities, ActiveOccurrences int
}

// BindGroupingPairPrivate compares the frozen declaration and exact commitments
// without copying canonical payloads, importing JSON, or evaluating the kernel.
func BindGroupingPairPrivate(c GroupingEvaluationCasePrivate, r GroupingEvaluationResultPrivate, input rg.AdmittedInput) (GroupingPairBindingPrivate, error) {
	bad := func() (GroupingPairBindingPrivate, error) {
		return GroupingPairBindingPrivate{}, errors.New("private opaque case/result/input binding mismatch")
	}
	if !c.Valid() || c.id == "" || r.id == "" || len(r.canonical) == 0 || len(c.canonical) > rg.EvaluationMaxBytesPrivate || len(r.canonical) > rg.EvaluationMaxBytesPrivate {
		return bad()
	}
	m, err := rg.ReadAllModeMetadataPrivate(input)
	if err != nil {
		return bad()
	}
	n, o, u, err := rg.ProjectionCountsPrivate(c.projection)
	if err != nil {
		return bad()
	}
	d, e := c.doc, r.doc.Declaration
	// Every declaration field is compared, including selector bytes. No hash is
	// recomputed: only legitimate in-process opaque constructors establish values.
	if d.Schema != e.Schema || d.CaseID != e.CaseID || d.RunID != e.RunID || d.NumericContract != e.NumericContract || d.UniverseScope != e.UniverseScope || d.SourceNodeUniverse != e.SourceNodeUniverse || d.Seed != e.Seed || d.Bindings != e.Bindings || d.Projection != e.Projection || d.Selector.ID != e.Selector.ID || d.Selector.Digest != e.Selector.Digest || d.Selector.ByteLength != e.Selector.ByteLength || d.Selector.Semantics != e.Selector.Semantics || !bytes.Equal(d.Selector.Bytes, e.Selector.Bytes) {
		return bad()
	}
	cc := rg.CompositionCommitment{ID: c.id, Digest: c.id, ByteLength: len(c.canonical)}
	pc := rg.CompositionCommitment{ID: c.projection.ID(), Digest: c.projection.Digest(), ByteLength: c.projection.ByteLength()}
	if r.doc.Case != cc || d.Projection != pc || d.Bindings != c.projection.Bindings() || d.Bindings.Input != m.Input || r.doc.Schema != GroupingEvaluationSchemaPrivate || r.doc.ClaimCeiling != rg.EvaluationCeilingPrivate || r.doc.Authority != 0 || r.doc.Accepted || r.doc.Partition.Seed != d.Seed || len(r.doc.Partition.Communities) > rg.EvaluationMaxNodesPrivate {
		return bad()
	}
	members := 0
	for _, community := range r.doc.Partition.Communities {
		if len(community.Members) > rg.EvaluationMaxNodesPrivate-members {
			return bad()
		}
		members += len(community.Members)
	}
	if members != n {
		return bad()
	}
	return GroupingPairBindingPrivate{Case: cc, Result: rg.CompositionCommitment{ID: r.id, Digest: r.id, ByteLength: len(r.canonical)}, Projection: pc, Bindings: d.Bindings, CaseID: d.CaseID, RunID: d.RunID, SelectorID: d.Selector.ID, SelectorDigest: d.Selector.Digest, ClaimCeiling: r.doc.ClaimCeiling, Seed: d.Seed, Nodes: n, Occurrences: o, Custody: u, Members: members, Communities: len(r.doc.Partition.Communities), ActiveOccurrences: r.doc.ActiveOccurrenceCount}, nil
}

