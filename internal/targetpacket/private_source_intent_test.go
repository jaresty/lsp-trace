package targetpacket

import (
	"strings"
	"testing"
)

func TestPrivateSourceIntentTypedUnresolved(t *testing.T) {
	for _, tc := range []struct{ kind, role string }{{"CALLS", "CALLER_OCCURRENCE"}, {"REFERENCES_SYMBOL", "REFERENCING_OCCURRENCE"}, {"RESOLVES_TO_DEFINITION", "DEFINITION_TARGET"}} {
		n := PrivateNomination{ID: "nomination", Mode: "CRD", RunID: "run", Lineage: "lineage", Members: []string{"node"}, Endpoints: []PrivateTypedEndpoint{{Node: "node", Kind: tc.kind, Role: tc.role, Occurrence: "occurrence", Ordinal: 2, Predecessor: "selector", Custody: "CALLER_ASSERTED"}}}
		out, err := BuildPrivateSourceIntents(n, 1)
		if err != nil || len(out) != 1 || out[0].Endpoint != n.Endpoints[0] || out[0].BindingState != "UNRESOLVED" || !strings.Contains(out[0].ClaimCeiling, "NO_DESCRIBE") {
			t.Fatal("ASSERT_ALLMODE_TYPED_SOURCE_INTENT_UNRESOLVED", out, err)
		}
		n.Endpoints[0].Role = "FROM"
		if _, err = BuildPrivateSourceIntents(n, 1); err == nil {
			t.Fatal("ASSERT_ALLMODE_NO_FLATTENED_ENDPOINT_ROLE")
		}
	}
	t.Log("ASSERT_ALLMODE_TYPED_SOURCE_INTENT_UNRESOLVED: PASS")
	t.Log("ASSERT_ALLMODE_NO_FLATTENED_ENDPOINT_ROLE: PASS")
	n := PrivateNomination{ID: "missing", Mode: "RD", RunID: "run", Lineage: "lineage", Gap: "DECLARED_MISSING_UNRESOLVED"}
	out, err := BuildPrivateSourceIntents(n, 1)
	if err != nil || len(out) != 1 || out[0].Endpoint.Node != "" || out[0].CaseID != "" || out[0].Gap != n.Gap {
		t.Fatal("ASSERT_ALLMODE_MISSING_GAP_NOT_SOURCE")
	}
	if _, err = BuildPrivateSourceIntents(n, 0); err == nil {
		t.Fatal("ASSERT_ALLMODE_INTENT_FINITE_BOUND")
	}
	t.Log("ASSERT_ALLMODE_MISSING_GAP_NOT_SOURCE: PASS")
}
