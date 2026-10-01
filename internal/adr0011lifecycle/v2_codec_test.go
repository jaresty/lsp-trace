package adr0011lifecycle

import (
	"testing"
)

func TestV2ExpectedRefRejectsCoherentReplacement(t *testing.T) {
	expected := map[string]any{"role_uri": "https://jaresty.github.io/lsp-trace/schemas/adr0011-lifecycle-v2.proposed.schema.json#/$defs/trustedHostRoot", "selector": "adr0011-trusted-host-root-v2-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.json", "path": "file:///private/adr0011-trusted-host-root-v2-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.json", "digest": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "byte_length": int64(1)}
	replacement := map[string]any{"role_uri": expected["role_uri"], "selector": "adr0011-trusted-host-root-v2-bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb.json", "path": "file:///private/adr0011-trusted-host-root-v2-bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb.json", "digest": "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "byte_length": int64(1)}
	if err := v2MatchHeldRef(expected, replacement); err == nil {
		t.Fatal("ASSERT_COHERENT_SELF_REHASH_DENIED: trusted replacement ref without held original")
	}
}
