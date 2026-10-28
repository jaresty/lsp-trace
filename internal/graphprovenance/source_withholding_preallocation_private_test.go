package graphprovenance

import (
	"fmt"
	"strings"
	"testing"
)

func TestSourceWithholdingPrivatePreallocationKeys(t *testing.T) {
	members := func(key string, n int) []byte {
		return []byte(`{"source_policy":{"` + key + `":[` + strings.TrimSuffix(strings.Repeat(`{"uri":"u","sha256":"h","role":"r"},`, n), ",") + `]}}`)
	}
	for _, tc := range []struct {
		name string
		raw  []byte
		want bool
	}{
		{"canonical-six", members("members", 6), true},
		{"canonical-seven", members("members", 7), false},
		{"alias-six", members("Members", 6), false},
		{"alias-seven", members("Members", 7), false},
		{"alias-large", members("Members", 20000), false},
		{"unknown-policy", []byte(`{"source_policy":{"unknown":[]}}`), false},
		{"unknown-member", []byte(`{"source_policy":{"members":[{"unknown":[]}]}}`), false},
		{"alias-member-field", []byte(`{"source_policy":{"members":[{"URI":"u"}]}}`), false},
		{"alias-captures", []byte(`{"evidence":{"Captures":[]}}`), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := preflightSourceArraysPrivateV1(tc.raw)
			if (err == nil) != tc.want {
				t.Fatalf("ASSERT_PRIVATE_PREALLOCATION_KEY: %s err=%v", tc.name, err)
			}
		})
	}
	small, large := members("Members", 7), members("Members", 20000)
	a := testing.AllocsPerRun(5, func() { _ = preflightSourceArraysPrivateV1(small) })
	b := testing.AllocsPerRun(5, func() { _ = preflightSourceArraysPrivateV1(large) })
	// A token scan stops at the alias key, before its payload. Allow small runtime
	// measurement variation, not allocation proportional to thousands of members.
	if b > a+16 {
		t.Fatalf("ASSERT_PRIVATE_ALIAS_NO_PAYLOAD_ALLOCATION: small=%g large=%g", a, b)
	}
	t.Log(fmt.Sprintf("ASSERT_PRIVATE_ALIAS_NO_PAYLOAD_ALLOCATION: small=%g large=%g", a, b))
}
