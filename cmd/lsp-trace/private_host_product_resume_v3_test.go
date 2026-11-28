//go:build darwin

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"

	product "lsp-trace/internal/privatehostproductv3"
)

func runHostProductFixtureCommand(t *testing.T, tool, operation, path string, r hostProductRequestV3, cancelAt ...uint64) (product.Report, error) {
	t.Helper()
	var out, errout bytes.Buffer
	if runPrivateHostProductV3([]string{"setup", path}, bytes.NewReader(nil), &out, &errout) != 0 {
		t.Fatalf("setup: %s", errout.String())
	}
	mailbox := hostProductMailboxV3(r).Directory
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	var services sync.WaitGroup
	var cmd *exec.Cmd
	started := make(chan struct{})
	for _, role := range []string{"interpreter", "reviewer"} {
		if product.MarkReady(mailbox, product.Ready{Protocol: product.Protocol, Role: role, LaunchID: operation + "-controlled-host-" + role, PromptSHA256: product.PromptDigest(role)}) != nil {
			t.Fatal("ready")
		}
		services.Add(1)
		go func(role string) {
			defer services.Done()
			var after uint64
			for {
				bundle, e := product.WaitBundle(ctx, mailbox, role, after)
				if e != nil {
					return
				}
				task, e := product.ReadBundle(bundle)
				if e != nil {
					t.Error(e)
					cancel()
					return
				}
				if task.ConfigSHA256 != product.ConfigDigest(r.Configuration) || task.ScopeSHA256 != r.Configuration.ScopeSHA256 || task.Application != r.Configuration.Application || task.Revision != 1 {
					t.Error("ASSERT_TASK_SOURCE_CONFIG_REVISION_PINS")
					cancel()
					return
				}
				if len(cancelAt) > 0 && task.Sequence == cancelAt[0] {
					if operation == "retained-core" {
						cancel()
						return
					}
					<-started
					if cmd.Process != nil {
						cmd.Process.Signal(syscall.SIGTERM)
					}
					return
				}
				response := product.Response{Protocol: product.Protocol, RequestDigest: task.RequestDigest, Label: "Controlled declaration", Explanation: "Own-body declaration; product purpose unresolved.", Alternative: "Technical infrastructure", RejectionCondition: "Own source contradicts the declaration description", Quote: task.Body, Start: 0, End: len(task.Body), Supported: role == "reviewer"}
				raw, _ := json.Marshal(response)
				if e = product.Submit(mailbox, role, task.Sequence, raw); e != nil {
					t.Error(e)
					cancel()
					return
				}
				after = task.Sequence
			}
		}(role)
	}
	if operation == "retained-core" {
		// Hosted fixture only: exercise supported acquisition and retained APIs
		// directly, not a replacement production orchestration command.
		acquire := exec.CommandContext(ctx, tool, "private-host-v3", "acquire", path)
		acquire.Env = append(os.Environ(), "LSP_HOST_PRODUCT_OWNED_SERVER=1")
		if raw, e := acquire.CombinedOutput(); e != nil {
			cancel()
			services.Wait()
			t.Fatalf("retained-core acquisition: %v %s", e, raw)
		}
		transfer, e := product.ReadTransfer(filepath.Join(r.AuxiliaryRoot, "transfer.json"))
		if e == nil {
			e = validateHostProductTransferV3(r, transfer)
		}
		if e != nil {
			cancel()
			services.Wait()
			t.Fatal("retained-core transfer admission", e)
		}
		bootstrap := r.BootstrapAllowance
		owner, closeOwner, e := product.OpenTransfer(ctx, transfer, &bootstrap, r.Configuration.Exchange)
		if e != nil {
			cancel()
			services.Wait()
			t.Fatal("retained-core owner admission", e)
		}
		backend := hostProductMailboxV3(r)
		backend.Observation = transfer.Cleanup
		// Observed above: acquire.CombinedOutput returned after waiting.
		backend.Observation.OuterOriginalProcessWaited = true
		report, runErr := product.RunRetained(ctx, r.Configuration, owner, backend)
		closeErr := closeOwner()
		cancel()
		services.Wait()
		return report, errors.Join(runErr, closeErr)
	}
	cmd = exec.CommandContext(ctx, tool, "private-host-v3", operation, path)
	cmd.Env = append(os.Environ(), "LSP_HOST_PRODUCT_OWNED_SERVER=1")
	out.Reset()
	errout.Reset()
	cmd.Stdout = &out
	cmd.Stderr = &errout
	e := cmd.Start()
	close(started)
	if e == nil {
		e = cmd.Wait()
	}
	cancel()
	services.Wait()
	var report product.Report
	if json.Unmarshal(out.Bytes(), &report) != nil {
		t.Fatalf("missing machine report: %s %s %v", out.String(), errout.String(), e)
	}
	return report, e
}
func TestHostProductRetainedCoreBudgetPauseCLIResume(t *testing.T) {
	r, path, base := hostProductCLIRequestFixture(t)
	r.Configuration.MaxCalls = 2
	r.Configuration.ApprovalSHA256 = product.ConfigDigest(r.Configuration)
	r.RuntimeGrantSHA256 = hostProductRequestDigestV3(r)
	raw, _ := json.Marshal(r)
	os.WriteFile(path, raw, 0600)
	tool := filepath.Join(base, "lsp-trace")
	if raw, e := exec.Command("go", "build", "-o", tool, ".").CombinedOutput(); e != nil {
		t.Fatalf("build %s %v", raw, e)
	}
	first, e := runHostProductFixtureCommand(t, tool, "retained-core", path, r)
	if e == nil || first.Status != "PAUSED_BUDGET" || first.Completed != 1 || first.Pending != 19 || first.Spend.Calls != 2 {
		t.Fatalf("ASSERT_ACTUAL_CLI_PAUSE: %+v %v", first, e)
	}
	machinePath := filepath.Join(r.Configuration.Store, first.MachinePaths[0])
	before, e := os.ReadFile(machinePath)
	if e != nil {
		t.Fatal(e)
	}
	claimPath := filepath.Join(r.Configuration.Store, "claim/claim-00000000.json")
	claim, e := os.ReadFile(claimPath)
	if e != nil {
		t.Fatal(e)
	}
	old := r.Configuration
	r.Configuration.StartBatch = first.NextBatch
	r.Configuration.AllowanceRevision++
	r.Configuration.PreviousConfigSHA256 = product.ConfigDigest(old)
	r.Configuration.MaxCalls = 40
	r.Configuration.ApprovalSHA256 = product.ConfigDigest(r.Configuration)
	r.RuntimeGrantSHA256 = hostProductRequestDigestV3(r)
	path2 := filepath.Join(base, "request-v2.json")
	raw, _ = json.Marshal(r)
	os.WriteFile(path2, raw, 0600)
	final, e := runHostProductFixtureCommand(t, tool, "resume", path2, r)
	if e != nil || final.Completed != 20 || final.Pending != 0 || final.Spend.Calls != 40 || final.Spend.PolicyReservation.LogicalBytes < first.Spend.PolicyReservation.LogicalBytes || final.ReportRevision <= first.ReportRevision {
		t.Fatalf("ASSERT_ACTUAL_CLI_RESTART_NO_REFUND: %+v %v", final, e)
	}
	after, e := os.ReadFile(machinePath)
	if e != nil || !bytes.Equal(before, after) {
		t.Fatal("ASSERT_ACTUAL_CLI_PRIOR_REPORT_BYTE_EXACT")
	}
	after, e = os.ReadFile(claimPath)
	if e != nil || !bytes.Equal(claim, after) {
		t.Fatal("ASSERT_ACTUAL_CLI_ORIGINAL_CLAIM_UNCHANGED")
	}
	if final.Candidates[0].EvidenceRefs[0] != first.Candidates[0].EvidenceRefs[0] {
		t.Fatal("ASSERT_ACTUAL_CLI_PRIOR_PREPARED_REF_PRESERVED")
	}
	t.Log("ASSERT_ACTUAL_CLI_RESTART_NO_REFUND: PASS; fresh OS process, retained-observation admission, explicit new semantic allowance, no acquisition replay")
}
