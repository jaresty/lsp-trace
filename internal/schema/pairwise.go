package schema

import "sort"

type ReferenceNamespace string

const (
	ReferenceNode                  ReferenceNamespace = "NODE"
	ReferenceCallRelation          ReferenceNamespace = "CALL_RELATION"
	ReferenceDispatchRelationship  ReferenceNamespace = "DISPATCH_RELATIONSHIP"
	ReferenceSiblingCandidate      ReferenceNamespace = "SIBLING_CANDIDATE"
	ReferenceDiagnosticCorrelation ReferenceNamespace = "DIAGNOSTIC_CORRELATION"
)

type TypedReferenceKey struct {
	Namespace ReferenceNamespace
	Value     string
}

type TypedReference struct {
	Namespace ReferenceNamespace
	Value     string
	Ordinal   int
}

type TypedReferencePartition struct {
	Shared    []TypedReference
	LeftOnly  []TypedReference
	RightOnly []TypedReference
}

// PartitionTypedReferences partitions already-admitted references in canonical global order.
func PartitionTypedReferences(global []TypedReference, left, right []TypedReferenceKey) TypedReferencePartition {
	ordered := append([]TypedReference(nil), global...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Ordinal < ordered[j].Ordinal })

	leftSet := referenceSet(left)
	rightSet := referenceSet(right)
	var out TypedReferencePartition
	for _, ref := range ordered {
		key := TypedReferenceKey{Namespace: ref.Namespace, Value: ref.Value}
		_, inLeft := leftSet[key]
		_, inRight := rightSet[key]
		switch {
		case inLeft && inRight:
			out.Shared = append(out.Shared, ref)
		case inLeft:
			out.LeftOnly = append(out.LeftOnly, ref)
		case inRight:
			out.RightOnly = append(out.RightOnly, ref)
		}
	}
	return out
}

func referenceSet(refs []TypedReferenceKey) map[TypedReferenceKey]struct{} {
	set := make(map[TypedReferenceKey]struct{}, len(refs))
	for _, ref := range refs {
		set[ref] = struct{}{}
	}
	return set
}
