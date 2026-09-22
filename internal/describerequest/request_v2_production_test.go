package describerequest

import (
	"strings"
	"testing"
)

func TestVariantEProductionPromptAndGrammarArePinned(t *testing.T) {
	if !strings.HasPrefix(VariantEPromptPrefix, "Task: describe what C1 provides, the semantic need it serves if resolved, and its one-layer boundary contribution; derive each value from this packet, never copy generic wording. Exact JSON schema: ") {
		t.Fatal("ASSERT_VARIANT_E_TASK_FIRST_PREFIX")
	}
	for _, required := range []string{
		`{"verdict":"COMPLETE|ABSTAINED","target_role":"string","consumer_need":{"status":"RESOLVED|UNRESOLVED","value":"string; empty exactly when UNRESOLVED"},"provided_behavior":{"value":"string","consumer_relative":true|false},"boundary_contribution":"string","limitations":["zero to eight strings"]}`,
		"Host admissible evidence handles grounding.",
		"citation_suggestions is optional, non-authoritative, and not required.",
		"\n\nPACKET:\n",
	} {
		if !strings.Contains(VariantEPromptPrefix, required) {
			t.Fatalf("ASSERT_VARIANT_E_EXACT_COMPONENT missing=%q", required)
		}
	}
	for _, required := range []string{`root ::= base | suggested`, `ws ::= [ \t\n\r]*`, `id ::= "\"C1\"" | "\"C2\"" | "\"PACKET_SCOPE\"" | "\"UNRESOLVED_CUSTODY\""`} {
		if !strings.Contains(PinnedRuntimeGrammarV2, required) {
			t.Fatalf("ASSERT_V2_CORRECTED_PINNED_GRAMMAR missing=%q", required)
		}
	}
}
