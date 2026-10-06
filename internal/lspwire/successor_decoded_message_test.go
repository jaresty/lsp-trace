package lspwire

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
)

func decodedMessageCapacity(m Message) uint64 {
	total := uint64(len(m.JSONRPC) + len(m.Method) + len(m.ID) + len(m.Params) + len(m.Result))
	if m.Error != nil {
		total += uint64(len(m.Error.Message) + len(m.Error.Data))
	}
	return total
}

func TestCloneSuccessorMessageOwnedReservesExactCapacityBeforeIndependentClone(t *testing.T) {
	original := Message{
		JSONRPC: Version,
		ID:      json.RawMessage(`17`),
		Method:  "m\u00e9thod",
		Params:  json.RawMessage(`{"p":true}`),
		Result:  json.RawMessage(`[{"uri":"file:///x"}]`),
		Error:   &RPCError{Code: -1, Message: "failure", Data: json.RawMessage(`{"retry":false}`)},
	}
	owner := &successorAllocationOwnerProbe{}
	clone, lease, err := CloneSuccessorMessageOwned(original, owner)
	if err != nil {
		t.Fatal(err)
	}
	if len(owner.roles) != 1 || owner.roles[0] != SuccessorAllocationDecodedMessage || owner.capacities[0] != decodedMessageCapacity(original) {
		t.Fatalf("ASSERT_C15_DECODED_EXACT_RESERVATION roles=%v capacities=%v want=%d", owner.roles, owner.capacities, decodedMessageCapacity(original))
	}
	if lease == nil || owner.leases[0].releases != 0 {
		t.Fatal("ASSERT_C15_DECODED_OWNER_TRANSFER")
	}
	before := append([]byte(nil), clone.Result...)
	original.Result[0] ^= 1
	original.Params[0] ^= 1
	original.Error.Data[0] ^= 1
	if !bytes.Equal(clone.Result, before) || bytes.Equal(clone.Params, original.Params) || bytes.Equal(clone.Error.Data, original.Error.Data) {
		t.Fatal("ASSERT_C15_DECODED_INDEPENDENT")
	}
	lease.Release()
	if owner.leases[0].releases != 1 {
		t.Fatalf("ASSERT_C15_DECODED_RELEASE releases=%d", owner.leases[0].releases)
	}
}

func TestCloneSuccessorMessageOwnedRefusalHasZeroClone(t *testing.T) {
	original := Message{JSONRPC: Version, ID: json.RawMessage(`1`), Result: json.RawMessage(`[]`)}
	owner := &successorAllocationOwnerProbe{rejectRole: SuccessorAllocationDecodedMessage}
	clone, lease, err := CloneSuccessorMessageOwned(original, owner)
	if !errors.Is(err, ErrSuccessorAllocationRefused) || clone.JSONRPC != "" || clone.ID != nil || clone.Method != "" || clone.Params != nil || clone.Result != nil || clone.Error != nil || lease != nil {
		t.Fatalf("ASSERT_C15_DECODED_PLUS_ONE clone=%+v lease=%v err=%v", clone, lease, err)
	}
	if len(owner.roles) != 1 || owner.capacities[0] != decodedMessageCapacity(original) {
		t.Fatalf("ASSERT_C15_DECODED_REFUSAL_EXACT roles=%v capacities=%v", owner.roles, owner.capacities)
	}
}
