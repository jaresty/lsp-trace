package inventoryworkflow

import (
	"bytes"
	"encoding/json"
	"lsp-trace/internal/programcrepresentative"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// Context selection is test-only and independent of any qualification result.
// Only the disposable declaration overlay may read the fresh destination.
func controlledLegacyContextV2(t *testing.T) bool {
	t.Helper()
	selected := os.Getenv("MULTISEED_CONTROLLED_CONTEXT")
	if selected == "" {
		implementation, err := programcrepresentative.BuildStage6ImplementationV2(programcrepresentative.MaxArtifactCanonicalBytesV2)
		if err == nil && implementation.Identity == "sha256:f53fcf986d1cfdf52b07bfb5119cd9cf05f83fb44ffdbfca6c3c710bd57343a3" {
			t.Fatal("CONTROLLED_CONTEXT_SETUP_LEGACY_SELECTOR_REQUIRED")
		}
		return false
	}
	if selected != "legacy-overlay" {
		t.Fatal("CONTROLLED_CONTEXT_SETUP_UNKNOWN_SELECTOR")
	}
	destination := os.Getenv("MULTISEED_CONTROLLED_DECLARATION_DESTINATION")
	if destination == "" || !filepath.IsAbs(destination) || filepath.Clean(destination) != destination {
		t.Fatal("CONTROLLED_CONTEXT_SETUP_FRESH_DESTINATION_REQUIRED")
	}
	dir := filepath.Dir(destination)
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil || resolved != dir {
		t.Fatal("CONTROLLED_CONTEXT_SETUP_FRESH_DESTINATION_REQUIRED")
	}
	info, err := os.Stat(dir)
	if err != nil || info.Mode().Perm() != 0700 {
		t.Fatal("CONTROLLED_CONTEXT_SETUP_PRIVATE_DESTINATION_REQUIRED")
	}
	if info, err := os.Lstat(destination); err == nil && (!info.Mode().IsRegular() || info.Mode().Perm() != 0600) {
		t.Fatal("CONTROLLED_CONTEXT_SETUP_REGULAR_DESTINATION_REQUIRED")
	} else if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	implementation, err := programcrepresentative.BuildStage6ImplementationV2(programcrepresentative.MaxArtifactCanonicalBytesV2)
	wantImplementation := programcrepresentative.Stage6ImplementationDTO{RepositoryCommit: "11b381b945a311cc2a6fbde1dbfc2e2aad18105c", RepositoryTree: "9b3e1389969aab2b575869dfa0261ed328ab5712", Files: []programcrepresentative.ImplementationFileDTO{{Path: "internal/programcrepresentative/test-only.go", GitBlobSHA1: "1542ea2a1ab8938d742906ecd8110a4412e26e93", SHA256: workflowDigest([]byte("controlled test-only implementation source\n")), ByteLength: 43}}, Identity: "sha256:f53fcf986d1cfdf52b07bfb5119cd9cf05f83fb44ffdbfca6c3c710bd57343a3"}
	if err != nil || !reflect.DeepEqual(implementation, wantImplementation) {
		t.Fatal("CONTROLLED_CONTEXT_SETUP_EXACT_LEGACY_OVERLAY_REQUIRED")
	}
	// Check the reader's destination before any retained-corpus fixture operation.
	prior, err := programcrepresentative.ReviewedHarnessActorBindingsV2()
	if err != nil {
		pathErr, ok := err.(*os.PathError)
		if !ok || pathErr.Path != destination || !os.IsNotExist(err) {
			t.Fatalf("CONTROLLED_CONTEXT_SETUP_OVERLAY_DESTINATION_MISMATCH: %v", err)
		}
	} else {
		disk, readErr := os.ReadFile(destination)
		var diskBindings []programcrepresentative.ReviewedHarnessActorBindingV2
		if readErr != nil || json.Unmarshal(disk, &diskBindings) != nil || !reflect.DeepEqual(prior, diskBindings) || len(prior) != 2 || prior[0].Actor.ID != "controlled-test-writer" || prior[1].Actor.ID != "controlled-test-isolated-reviewer" || prior[0].Actor.Kind != "PI_SESSION" || prior[1].Actor.Kind != "PI_ISOLATED_TASK" || prior[0].Actor.IsolationKind != "PI_SESSION" || prior[1].Actor.IsolationKind != "PI_ISOLATED_TASK" || prior[0].Actor.ParentID != "" || prior[1].Actor.ParentID != prior[0].Actor.ID {
			t.Fatal("CONTROLLED_CONTEXT_SETUP_OVERLAY_DESTINATION_MISMATCH")
		}
	}
	return true
}

func installControlledActorDeclarationV2(t *testing.T, h controlledCheckpointHarnessV2) {
	t.Helper()
	if !controlledLegacyContextV2(t) {
		return
	} // Default never writes declarations.
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
	destination := os.Getenv("MULTISEED_CONTROLLED_DECLARATION_DESTINATION")
	if err = os.WriteFile(destination, raw, 0600); err != nil {
		t.Fatal(err)
	}
	installed, err := programcrepresentative.ReviewedHarnessActorBindingsV2()
	disk, readErr := os.ReadFile(destination)
	if err != nil || readErr != nil || !bytes.Equal(raw, disk) || !reflect.DeepEqual(installed, bindings) {
		t.Fatalf("CONTROLLED_CONTEXT_SETUP_EXACT_ACTOR_PROJECTION_BINDINGS_REQUIRED: %v", err)
	}
	t.Log("CONTROLLED_LEGACY_CONTEXT_VALIDATED: exact actor roles, parents, projection hashes and lengths; disposable synthetic archive only")
}

func TestMultiSeedV2DefaultDeclarationNoWrite(t *testing.T) {
	if controlledLegacyContextV2(t) {
		return
	} // This guard is meaningful only in the explicitly default context.
	destination := filepath.Join(t.TempDir(), "must-not-exist.json")
	t.Setenv("MULTISEED_CONTROLLED_DECLARATION_DESTINATION", destination)
	installControlledActorDeclarationV2(t, controlledCheckpointHarnessV2{})
	if _, err := os.Lstat(destination); !os.IsNotExist(err) {
		t.Fatalf("DEFAULT_DECLARATION_NO_WRITE: %v", err)
	}
	t.Log("DEFAULT_DECLARATION_NO_WRITE PASS")
}

func TestMultiSeedV2ControlledContextSetup(t *testing.T) {
	// This small entrypoint lets subprocess controls prove named setup failures
	// before fixture retention or checkpoint operations.
	legacy := controlledLegacyContextV2(t)
	t.Logf("CONTROLLED_CONTEXT_SETUP_PASS legacy=%t", legacy)
}
