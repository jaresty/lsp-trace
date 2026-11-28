//go:build darwin

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	product "lsp-trace/internal/privatehostproductv3"
)

func TestHostProductRetainedCoreCancelNoOriginalReplay(t *testing.T) {
	r, path, base := hostProductCLIRequestFixture(t)
	tool := filepath.Join(base, "lsp-trace")
	if raw, e := exec.Command("go", "build", "-o", tool, ".").CombinedOutput(); e != nil {
		t.Fatalf("build %s %v", raw, e)
	}
	first, e := runHostProductFixtureCommand(t, tool, "retained-core", path, r, 3)
	if e == nil || first.Status != "CANCELLED" || first.Completed != 1 || first.Pending != 19 || first.Spend.Calls != 3 {
		t.Fatalf("ASSERT_HOST_CANCEL_RETAINS_PRIOR: %+v %v", first, e)
	}
	claimPath := filepath.Join(r.Configuration.Store, "claim/claim-00000000.json")
	claim, e := os.ReadFile(claimPath)
	if e != nil {
		t.Fatal(e)
	}
	// Make fresh source acquisition impossible. Replay must still distinguish the
	// spent original claim BEFORE trying to read these now-absent source bytes.
	options, e := parseCensusCLIOptions(r.CensusArguments)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.Remove(filepath.Join(options.Workspace, "owned.go")); e != nil {
		t.Fatal(e)
	}
	cmd := exec.Command(tool, "private-host-v3", "acquire", path)
	var out, errout bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errout
	if e = cmd.Run(); e == nil {
		t.Fatal("ASSERT_ORIGINAL_REPLAY_REFUSED_AFTER_CANCEL")
	}
	var final product.Report
	if json.Unmarshal(out.Bytes(), &final) != nil || final.Failure == nil || final.Failure.Class != "ORIGINAL_CLAIM_SPENT" || final.Completed != 1 || final.Spend.Calls != 3 {
		t.Fatalf("ASSERT_NO_SOURCE_REPLAY_AFTER_HOST_CANCEL: %+v %s", final, errout.String())
	}
	after, e := os.ReadFile(claimPath)
	if e != nil || !bytes.Equal(after, claim) {
		t.Fatal("ASSERT_CANCEL_ORIGINAL_CLAIM_BYTE_EXACT")
	}
	if final.Candidates[0].EvidenceRefs[0] != first.Candidates[0].EvidenceRefs[0] {
		t.Fatal("ASSERT_CANCEL_PRIOR_PREPARED_REFERENCE_RETAINED")
	}
	t.Log("ASSERT_NO_SOURCE_REPLAY_AFTER_HOST_CANCEL: PASS")
}
func TestHostProductCLIStaleScopeBeforeClaim(t *testing.T) {
	r, path, _ := hostProductCLIRequestFixture(t)
	if runPrivateHostProductV3([]string{"setup", path}, bytes.NewReader(nil), &bytes.Buffer{}, &bytes.Buffer{}) != 0 {
		t.Fatal("setup")
	}
	for _, role := range []string{"interpreter", "reviewer"} {
		if product.MarkReady(hostProductMailboxV3(r).Directory, product.Ready{Protocol: product.Protocol, Role: role, LaunchID: "controlled-" + role, PromptSHA256: product.PromptDigest(role)}) != nil {
			t.Fatal("ready")
		}
	}
	options, _ := parseCensusCLIOptions(r.CensusArguments)
	if os.WriteFile(filepath.Join(options.Workspace, "owned.go"), []byte("package changed\n"), 0600) != nil {
		t.Fatal("source change")
	}
	var out, errout bytes.Buffer
	if runPrivateHostProductV3([]string{"acquire", path}, bytes.NewReader(nil), &out, &errout) == 0 {
		t.Fatal("ASSERT_STALE_SCOPE_REFUSED")
	}
	var report product.Report
	if json.Unmarshal(out.Bytes(), &report) != nil || report.Failure == nil || report.Failure.Class != "STALE_SOURCE_GRANT_REFUSED" {
		t.Fatal("ASSERT_SOURCE_PIN_PRODUCTION_LOADER", out.String(), errout.String())
	}
	if _, e := os.Stat(filepath.Join(r.Configuration.Store, "claim/claim-00000000.json")); !os.IsNotExist(e) {
		t.Fatal("ASSERT_STALE_SCOPE_NO_ORIGINAL_CLAIM")
	}
}
