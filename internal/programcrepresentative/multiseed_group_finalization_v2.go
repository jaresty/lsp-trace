package programcrepresentative

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"

	"lsp-trace/internal/graph"
)

// computeGroupRecordsV2 is private pure computation, not qualification issuance.
// Its opaque input must already have repository-reconstructed Native custody.
// No v1 singleton grouping, repeated community, or aggregate winner is made.
func computeGroupRecordsV2(m MechanicsV2) ([]GroupResultV2, GroupAccountingV2, error) {
	zero := GroupAccountingV2{}
	n, err := CanonicalSizeMechanicsIdentityDTOv2(m.dto)
	if err != nil {
		return nil, zero, err
	}
	if !m.Valid() {
		return nil, zero, fmt.Errorf("untrusted multiseed mechanics")
	}
	raw, err := marshalSizedV2(m.dto, n, nil)
	if err != nil || !bytes.Equal(raw, m.canonical) {
		return nil, zero, fmt.Errorf("multiseed mechanics retained-byte mismatch")
	}
	return groupRecordsFromAdmittedDTOv2(m.dto)
}

// Caller has already checked the complete typed size and mechanics identity.
// This computation carries no custody or qualification authority.
func groupRecordsFromAdmittedDTOv2(dto MechanicsIdentityDTOv2) ([]GroupResultV2, GroupAccountingV2, error) {
	zero := GroupAccountingV2{}
	groups := dto.SelectionContext.SeedGroups
	out := make([]GroupResultV2, 0, len(groups))
	accounting := GroupAccountingV2{Expected: uint64(len(groups))}
	groupSize := canonicalSizerV2{}
	groupSize.add(2)
	for i, g := range groups {
		relations := stringSet(g.RelationIDs)
		closure := make([]string, 0, len(g.OccurrenceIDs))
		calls := make([]RepresentativeEvidence, 0, len(g.OccurrenceIDs))
		allCalls := 0
		for ordinal, e := range dto.Evidence {
			if !relations[e.RelationID] {
				continue
			}
			closure = append(closure, e.Identity)
			allCalls++ // Included self-loops count before deriving ineligibility.
			if e.FromNodeID == e.ToNodeID {
				continue
			}
			calls = append(calls, RepresentativeEvidence{Identity: e.Identity, RelationID: e.RelationID, Kind: "CALLS", From: e.FromNodeID, To: e.ToNodeID, Disposition: "INCLUDED", SourcePredecessor: e.ConstituentSelector, Custody: dto.CustodyIdentity, Ordinal: ordinal, CallSite: graph.Range{Start: graph.Position{Line: e.CallSite.Start.Line, Character: e.CallSite.Start.Character}, End: graph.Position{Line: e.CallSite.End.Line, Character: e.CallSite.End.Character}}})
		}
		sort.Strings(closure)
		if i != g.InvocationSeedOrdinal || !equalMechanicsStrings(closure, g.OccurrenceIDs) {
			return nil, zero, fmt.Errorf("multiseed group closure mismatch")
		}
		context := RepresentativeSelectionContext{Identity: g.Identity, ConstituentIdentity: dto.SelectionContext.ConstituentSelector, ExecutionBundleID: g.ExecutionBundleID, SeedLabel: g.SeedLabel, SeedAt: g.SeedAt, CommunityMembers: dto.CommunityMembers, PreparedTargets: g.PreparedTargets, ReachedNodes: g.ReachedNodes, RelationIDs: g.RelationIDs, OccurrenceIDs: g.OccurrenceIDs}
		record := RecordV2{SCCMembers: []string{}, IncomingPredecessors: []PredecessorV2{}}
		if allCalls == 0 {
			record.Outcome = "NO_CALLS_REPRESENTATIVE"
			accounting.NoCalls++
		} else {
			selected, ok := selectRecord(context, calls)
			if !ok {
				record.Outcome = "CALLS_EVIDENCE_INELIGIBLE"
				accounting.CallsIneligible++
			} else {
				if err := admitSelectedRecordCopyV2(selected); err != nil {
					return nil, zero, err
				}
				record.Outcome = selected.Outcome
				record.SelectedNode = selected.SelectedNode
				record.Distance = selected.Distance
				record.SCCMembers = append([]string{}, selected.SCCMembers...)
				for _, p := range selected.IncomingPredecessors {
					record.IncomingPredecessors = append(record.IncomingPredecessors, PredecessorV2{OccurrenceID: p.OccurrenceID, RelationID: p.RelationID, CallerID: p.CallerID, TargetID: p.TargetID, CallSite: RangeDTO{Start: PositionDTO{Line: p.CallSite.Start.Line, Character: p.CallSite.Start.Character}, End: PositionDTO{Line: p.CallSite.End.Line, Character: p.CallSite.End.Character}}})
				}
				accounting.Selected++
			}
		}
		if _, err := CanonicalSizeRecordV2(record); err != nil {
			return nil, zero, err
		}
		group := GroupResultV2{InvocationSeedOrdinal: i, SeedGroupIdentity: g.Identity, Record: record}
		size, err := CanonicalSizeGroupResultV2(group)
		if err != nil {
			return nil, zero, err
		}
		if i > 0 {
			groupSize.add(1)
		}
		groupSize.add(size)
		if groupSize.err != nil {
			return nil, zero, groupSize.err
		}
		out = append(out, group)
	}
	return out, accounting, nil
}

// marshalSizedV2 is reachable only after a concrete typed walk has admitted
// this exact value. The independent length comparison catches walk drift.
func marshalSizedV2(value any, n uint64, sizeErr error) ([]byte, error) {
	if sizeErr != nil {
		return nil, sizeErr
	}
	if n > MaxArtifactCanonicalBytesV2 {
		return nil, fmt.Errorf("multiseed canonical artifact exceeds 64 MiB")
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	if uint64(len(raw)) != n {
		return nil, fmt.Errorf("multiseed typed size mismatch")
	}
	return raw, nil
}
