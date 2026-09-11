package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"lsp-trace/internal/operation"
)

func TestCommandCustodyLoaderGuidesVerifySelectorShape(t *testing.T) {
	_, failure := (commandCustodyLoader{}).Load(context.Background(), json.RawMessage(`{"artifact":"artifact.json","family":"lsp-trace.retained-relations","version":"v1"}`))
	if failure == nil || failure.Code != operation.FailureInvalidInput {
		t.Fatalf("ASSERT_VERIFY_SELECTOR_GUIDANCE_FAILURE: %#v", failure)
	}
	const want = `verify requires input shaped as {"input":"<generation-selector-path>"}; pass the publication selector, not artifact, family, or version fields`
	if len(failure.Diagnostics) != 1 || !strings.Contains(failure.Diagnostics[0], want) {
		t.Fatalf("ASSERT_VERIFY_SELECTOR_GUIDANCE_ACTIONABLE: %#v", failure.Diagnostics)
	}
}
