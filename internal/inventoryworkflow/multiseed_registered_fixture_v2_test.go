package inventoryworkflow

import (
	"encoding/json"
	"lsp-trace/internal/programcrepresentative"
	"os"
	"testing"
)

// This private controlled-fixture file is consumed only by the disposable
// declaration-data overlay. No production code reads it or installs a callback.
const controlledActorDeclarationPathV2 = "/Users/schwa/.local/state/lsp-trace-evidence/multiseed-owner-repair-20261104-atlas/controlled-registered-actors.json"

func installControlledActorDeclarationV2(t *testing.T, h controlledCheckpointHarnessV2) {
	t.Helper()
	bindings := []programcrepresentative.ReviewedHarnessActorBindingV2{}
	for _, key := range []string{"PI_SESSION:controlled-test-writer", "PI_ISOLATED_TASK:controlled-test-isolated-reviewer"} {
		actor, ok := h.actors[key]
		if !ok {
			t.Fatal("CONTROLLED_REGISTRATION_NOT_ATTACKER_ACTOR")
		}
		projection := actor.Projection
		actor.Projection = nil
		bindings = append(bindings, programcrepresentative.ReviewedHarnessActorBindingV2{Actor: actor, Projection: controlledArtifactBindingV2("lsp-trace.program-c-representative-review-transcript-jsonl.private.v2", "", projection)})
	}
	raw, err := json.Marshal(bindings)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(controlledActorDeclarationPathV2, raw, 0600); err != nil {
		t.Fatal(err)
	}
}
