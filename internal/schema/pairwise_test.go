package schema

import (
	"reflect"
	"testing"
)

func TestPartitionTypedReferences(t *testing.T) {
	global := []TypedReference{
		{Namespace: ReferenceNode, Value: "outside", Ordinal: 0},
		{Namespace: ReferenceNode, Value: "shared", Ordinal: 1},
		{Namespace: ReferenceCallRelation, Value: "shared", Ordinal: 2},
		{Namespace: ReferenceNode, Value: "left", Ordinal: 3},
		{Namespace: ReferenceNode, Value: "right", Ordinal: 4},
	}
	left := []TypedReferenceKey{
		{Namespace: ReferenceNode, Value: "left"},
		{Namespace: ReferenceCallRelation, Value: "shared"},
		{Namespace: ReferenceNode, Value: "shared"},
	}
	right := []TypedReferenceKey{
		{Namespace: ReferenceNode, Value: "right"},
		{Namespace: ReferenceNode, Value: "shared"},
	}

	got := PartitionTypedReferences(global, left, right)
	want := TypedReferencePartition{
		Shared: []TypedReference{{Namespace: ReferenceNode, Value: "shared", Ordinal: 1}},
		LeftOnly: []TypedReference{
			{Namespace: ReferenceCallRelation, Value: "shared", Ordinal: 2},
			{Namespace: ReferenceNode, Value: "left", Ordinal: 3},
		},
		RightOnly: []TypedReference{{Namespace: ReferenceNode, Value: "right", Ordinal: 4}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ASSERT_PAIRWISE_TYPED_IDENTITY_PARTITIONS_AND_GLOBAL_ORDER: got=%#v want=%#v", got, want)
	}
}

func TestPartitionTypedReferencesOperandReversal(t *testing.T) {
	global := []TypedReference{
		{Namespace: ReferenceNode, Value: "shared", Ordinal: 0},
		{Namespace: ReferenceNode, Value: "left", Ordinal: 1},
		{Namespace: ReferenceNode, Value: "right", Ordinal: 2},
	}
	left := []TypedReferenceKey{{Namespace: ReferenceNode, Value: "left"}, {Namespace: ReferenceNode, Value: "shared"}}
	right := []TypedReferenceKey{{Namespace: ReferenceNode, Value: "right"}, {Namespace: ReferenceNode, Value: "shared"}}

	forward := PartitionTypedReferences(global, left, right)
	reverse := PartitionTypedReferences(global, right, left)
	if !reflect.DeepEqual(forward.Shared, reverse.Shared) || !reflect.DeepEqual(forward.LeftOnly, reverse.RightOnly) || !reflect.DeepEqual(forward.RightOnly, reverse.LeftOnly) {
		t.Fatalf("ASSERT_PAIRWISE_OPERAND_REVERSAL_SWAPS_ONLY_DIRECTIONAL_PARTITIONS: forward=%#v reverse=%#v", forward, reverse)
	}
}
