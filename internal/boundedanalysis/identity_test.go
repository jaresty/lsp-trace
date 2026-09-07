package boundedanalysis

import (
	"bytes"
	"encoding/json"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/retainedcalls"
	"testing"
)

func TestFixedIndependentBasisVectors(t *testing.T) {
	// Independently calculated using Python hashlib + json.dumps(sort_keys=True,
	// separators=(',', ':')), with literal UTF-8 domain + NUL and base64 bytes.
	p := Parameters{Operation: "PROJECT", MaxWork: 1000000}
	for raw, want := range map[string]string{"{}": "sha256:39150d8b4583a6399158e8e80ca5d1c4aaf5ddc7435bb2c1f249e1e27e3b6a3e", "{}\n": "sha256:ffb4122728cf22e4025bb0974d9fbc7cf775165208793d0eb3c7190573d3d0dd"} {
		if got := basis([]byte(raw), p); got != want {
			t.Fatal("ASSERT_FIXED_BASIS_VECTOR", got, want)
		}
	}
}
func TestContextMutationRemainsUnverified(t *testing.T) {
	raw, _ := retainedFixture(t)
	base := evidence(t, raw, Parameters{Operation: "PROJECT"})
	var input retainedcalls.Evidence
	_ = json.Unmarshal(raw, &input)
	var provenance graphprovenance.Evidence
	_ = json.Unmarshal(input.InputBytes, &provenance)
	provenance.Generation = 9007199254740993
	provenance.SessionID = "publicly-resealed-not-authenticated"
	changed, _ := json.Marshal(provenance)
	exported, err := retainedcalls.Export(changed)
	if err != nil {
		t.Fatal(err)
	}
	next := evidence(t, exported, Parameters{Operation: "PROJECT"})
	if next.BasisDigest == base.BasisDigest || next.Edges[0].ContextID == base.Edges[0].ContextID || next.Edges[0].GroupID != base.Edges[0].GroupID || next.Scope != Scope || !bytes.Contains(next.InputBytes, []byte("9007199254740993")) {
		t.Fatal("ASSERT_CONTEXT_IDENTITY_NO_AUTH_UPGRADE")
	}
}
