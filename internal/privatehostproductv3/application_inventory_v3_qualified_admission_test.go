//go:build darwin || linux

package privatehostproductv3

import (
	"bytes"
	"encoding/json"
	"testing"
)

// Pure DATA only: no root/opener, Manager, native execution or grants.
func TestQualifiedOwnerSourceBindingV1(t *testing.T) {
	q := ownerDataFixtureV3(t)
	d := q.Inventory.NativeOwners[0]
	legacy, _ := json.Marshal(d)
	legacyInput, e := d.DescriptorInput()
	if e != nil {
		t.Fatal(e)
	}
	legacyTarget := InventoryNativeTargetV3(legacyInput)
	if bytes.Contains(legacy, []byte("ExpectedSourceAuthorization")) {
		t.Fatal("ASSERT absent owner binding legacy bytes: FAIL")
	}
	t.Log("ASSERT absent owner binding legacy bytes: PASS")
	expected := hash([]byte("complete qualified header"))
	patch, _ := json.Marshal(map[string]string{"ExpectedSourceAuthorization": expected})
	if e = json.Unmarshal(patch, &d); e != nil {
		t.Fatal(e)
	}
	input, e := d.DescriptorInput()
	if e != nil {
		t.Fatal("ASSERT owner expected source propagated: FAIL", e)
	}
	raw, _ := json.Marshal(input.Configuration)
	var fields map[string]json.RawMessage
	json.Unmarshal(raw, &fields)
	var got string
	json.Unmarshal(fields["ExpectedSourceAuthorization"], &got)
	if got != expected {
		t.Fatal("ASSERT owner expected source propagated: FAIL")
	}
	t.Log("ASSERT owner expected source propagated: PASS")
	if InventoryNativeTargetV3(input) == legacyTarget {
		t.Fatal("ASSERT source binding changes native configuration identity: FAIL")
	}
	t.Log("ASSERT source binding changes native configuration identity: PASS")
	if e = json.Unmarshal([]byte(`{"ExpectedSourceAuthorization":"not-a-digest"}`), &d); e != nil {
		t.Fatal(e)
	}
	if _, e = d.DescriptorInput(); e == nil {
		t.Fatal("ASSERT malformed expected source refused: FAIL")
	}
	t.Log("ASSERT malformed expected source refused: PASS")
}
