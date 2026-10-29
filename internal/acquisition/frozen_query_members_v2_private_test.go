package acquisition

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestFrozenQueryMembersPrivateV2(t *testing.T) {
	a := item("A", 0)
	plain := request(a)
	if members, err := plain.FrozenQueryMembersPrivateV2(); err != nil || members != nil {
		t.Fatal("ASSERT_QUERY_ABSENT", members, err)
	}
	r := bindPrivateV2(t, plain, []string{a.URI, "file:///b.go"})
	members, err := r.FrozenQueryMembersPrivateV2()
	if err != nil {
		t.Fatal(err)
	}
	members[0] = "file:///changed.go"
	again, err := r.FrozenQueryMembersPrivateV2()
	if err != nil || !reflect.DeepEqual(again, []string{a.URI, "file:///b.go"}) {
		t.Fatal("ASSERT_QUERY_DETACHED", again, err)
	}
	mixed := r
	mixed.FrozenQueryURIsPrivate[0] = a.URI
	if _, err = mixed.FrozenQueryMembersPrivateV2(); err == nil {
		t.Fatal("ASSERT_QUERY_MIXED")
	}
	invalid := plain
	invalid.frozenQueryDeclarationPrivateV2 = &FrozenQueryDeclarationPrivateV2{}
	if _, err = invalid.FrozenQueryMembersPrivateV2(); err == nil {
		t.Fatal("ASSERT_QUERY_INVALID")
	}
	raw, _ := json.Marshal(r)
	var fresh Request
	if err = json.Unmarshal(raw, &fresh); err != nil {
		t.Fatal(err)
	}
	if members, err = fresh.FrozenQueryMembersPrivateV2(); err != nil || members != nil {
		t.Fatal("ASSERT_QUERY_JSON_NO_AUTHORITY", members, err)
	}
}
