package main

import (
	"bytes"
	"encoding/json"
	product "lsp-trace/internal/privatehostproductv3"
	cat "lsp-trace/internal/provisionalfeaturecatalog"
	"os"
	"path/filepath"
	"testing"
)

func reviewModeControllerV3(t *testing.T, mode cat.ReviewModeV3, scope, assertion string) (inventoryTestSession, applicationInventoryTerminalRecordV3) {
	t.Helper()
	s := inventoryTestSetupWith(t, func(q *inventoryTestRequest) {
		q.Inventory.ReviewMode = mode
		q.Inventory.Manifest.Scope = scope
	})
	// The existing fixture has zero leads and no native owner. This exercises
	// the real private controller and scripted IPC, not live semantic inference.
	return s, inventoryTestRun(t, s, nil, assertion)
}

func TestCensusReviewModeControllerFinalizeV3(t *testing.T) {
	s, terminal := reviewModeControllerV3(t, cat.ReviewModeSameThreadV3, "same-thread-terminal-fixture", "ASSERT_REVIEW_CONTROLLER_FINALIZE")
	n, out, diagnostic := inventoryTestEntry([]string{"finalize", s.path, s.runID}, inventoryTestReturns(s, terminal))
	if n != 0 || !bytes.Contains(out, []byte("SCRIPTED_CONTRACT_RETURNS_NOT_AGENTS")) {
		t.Fatal("ASSERT_REVIEW_CONTROLLER_FINALIZE", n, string(out), diagnostic)
	}
	var publication product.InventoryPublicationV3
	if product.ReadApplicationRecord(s.q.Configuration.Store, terminal.InventoryHead, &publication) != nil || product.VerifyInventoryPublicationV3(publication, inventoryTestTyped(t, s.q)) != nil {
		t.Fatal("ASSERT_REVIEW_CONTROLLER_FINALIZE", "published result did not reopen")
	}
	entries, err := os.ReadDir(s.q.Configuration.SourceRoot)
	if err != nil || len(entries) != 0 {
		t.Fatal("ASSERT_REVIEW_CONTROLLER_FINALIZE", "source changed", err)
	}
	t.Log("ASSERT_REVIEW_CONTROLLER_FINALIZE: PASS")
}

func TestCensusReviewModeFinalReportingV3(t *testing.T) {
	for _, mode := range []cat.ReviewModeV3{cat.ReviewModeSameThreadV3, cat.ReviewModeSubagentsV3} {
		s, terminal := reviewModeControllerV3(t, mode, "declared-mode-reporting-fixture", "ASSERT_REVIEW_FINAL_REPORT_MODE")
		n, out, diagnostic := inventoryTestEntry([]string{"finalize", s.path, s.runID}, inventoryTestReturns(s, terminal))
		if n != 0 {
			t.Fatal("ASSERT_REVIEW_FINAL_REPORT_MODE", mode, n, diagnostic)
		}
		var view struct {
			ReviewMode           cat.ReviewModeV3
			Status, Completeness string
			Authority            int
			Accepted             bool
		}
		if json.Unmarshal(out, &view) != nil || view.ReviewMode != mode || view.Status != "SCRIPTED_CONTRACT_RETURNS_NOT_AGENTS" || view.Authority != 0 || view.Accepted || view.Completeness != "UNKNOWN" {
			t.Fatal("ASSERT_REVIEW_FINAL_REPORT_MODE", mode, string(out))
		}
	}
	t.Log("ASSERT_REVIEW_FINAL_REPORT_MODE: PASS")
}

func reviewModeRewriteFinalReadyV3(t *testing.T, s inventoryTestSession, terminal applicationInventoryTerminalRecordV3, mode cat.ReviewModeV3) []byte {
	t.Helper()
	var returns [2]product.CohortRoleReturn
	if json.Unmarshal(inventoryTestReturns(s, terminal), &returns) != nil {
		t.Fatal("return fixture")
	}
	for i, role := range []string{"interpreter", "reviewer"} {
		ready := product.InventoryReadyV3{Ready: product.Ready{Protocol: product.InventoryProtocolV3, Role: role, LaunchID: "synthetic-one-context", PromptSHA256: s.q.Configuration.RolePromptSHA256[i]}, ReviewMode: mode}
		// Fault injection into this test's disposable control store only. Rebuild
		// the admission witness too: a corrupt witness must not masquerade as
		// evidence that the role/mode predicate rejected the supplied metadata.
		path := applicationRoleReadyPath(s.q.Configuration.Store, s.runID, role)
		if err := os.Remove(path); err != nil {
			t.Fatal("ready fixture removal", err)
		}
		if err := os.RemoveAll(filepath.Join(s.q.Configuration.Store, applicationPublicationIntent(filepath.Base(path)))); err != nil {
			t.Fatal("ready fixture witness removal", err)
		}
		raw, _ := json.Marshal(ready)
		if err := applicationWriteOnce(path, raw); err != nil {
			t.Fatal("ready fixture admission", err)
		}
		root, err := os.OpenRoot(s.q.Configuration.Store)
		if err != nil {
			t.Fatal(err)
		}
		admitted, readErr := applicationFinalizeRead(root, filepath.Base(path), 4096)
		closeErr := root.Close()
		if readErr != nil || closeErr != nil || !bytes.Equal(admitted, raw) {
			t.Fatal("ready fixture is not admitted", readErr, closeErr)
		}
		returns[i].LaunchID = ready.LaunchID
	}
	raw, _ := json.Marshal(returns)
	return raw
}

func TestCensusReviewModeStrictFinalizeV3(t *testing.T) {
	for _, mode := range []cat.ReviewModeV3{"", cat.ReviewModeSubagentsV3} {
		s, terminal := reviewModeControllerV3(t, mode, "strict-terminal-fixture", "ASSERT_REVIEW_STRICT_FINALIZATION")
		returns := reviewModeRewriteFinalReadyV3(t, s, terminal, mode)
		n, _, diagnostic := inventoryTestEntry([]string{"finalize", s.path, s.runID}, returns)
		if n == 0 || !bytes.Contains([]byte(diagnostic), []byte("FINALIZE_ROLE_BINDING_REFUSED_CHARGED")) {
			t.Fatal("ASSERT_REVIEW_STRICT_FINALIZATION", mode, n, diagnostic)
		}
	}
	t.Log("ASSERT_REVIEW_STRICT_FINALIZATION: PASS")
}

func TestCensusReviewModeFinalizeBindingV3(t *testing.T) {
	s, terminal := reviewModeControllerV3(t, cat.ReviewModeSameThreadV3, "mode-mismatch-terminal-fixture", "ASSERT_REVIEW_FINALIZATION_MODE_BINDING")
	returns := reviewModeRewriteFinalReadyV3(t, s, terminal, cat.ReviewModeSubagentsV3)
	n, _, diagnostic := inventoryTestEntry([]string{"finalize", s.path, s.runID}, returns)
	if n == 0 || !bytes.Contains([]byte(diagnostic), []byte("FINALIZE_ROLE_BINDING_REFUSED_CHARGED")) {
		t.Fatal("ASSERT_REVIEW_FINALIZATION_MODE_BINDING", n, diagnostic)
	}
	t.Log("ASSERT_REVIEW_FINALIZATION_MODE_BINDING: PASS")
}

func TestCensusReviewModeLegacyFinalizeV3(t *testing.T) {
	s, terminal := reviewModeControllerV3(t, "", "legacy-terminal-fixture", "ASSERT_REVIEW_LEGACY_FINALIZATION_ENCODING")
	n, out, diagnostic := inventoryTestEntry([]string{"finalize", s.path, s.runID}, inventoryTestReturns(s, terminal))
	// Re-encode the same values with the pre-mode observation shape, not merely
	// a key-absence check. This also detects order, extra-field and LF changes.
	var legacy struct {
		Schema, Status, ApplicationSHA256, RunID, TerminalSHA256, InventoryHead string
		Authority                                                               int
		Accepted                                                                bool
		Completeness, OriginalCleanup                                           string
	}
	if n != 0 || json.Unmarshal(out, &legacy) != nil {
		t.Fatal("ASSERT_REVIEW_LEGACY_FINALIZATION_ENCODING", n, string(out), diagnostic)
	}
	want, err := json.Marshal(legacy)
	if err != nil || !bytes.Equal(out, append(want, '\n')) {
		t.Fatal("ASSERT_REVIEW_LEGACY_FINALIZATION_ENCODING", "canonical legacy observation changed", string(out), err)
	}
	if _, err := os.Stat(filepath.Join(s.q.Configuration.Store, "host-finalize-attempt-00.json")); err != nil {
		t.Fatal("ASSERT_REVIEW_LEGACY_FINALIZATION_ENCODING", "finalization not charged", err)
	}
	t.Log("ASSERT_REVIEW_LEGACY_FINALIZATION_ENCODING: PASS")
}
