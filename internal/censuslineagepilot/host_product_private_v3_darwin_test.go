//go:build darwin

package censuslineagepilot_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	pilot "lsp-trace/internal/censuslineagepilot"
	prep "lsp-trace/internal/privatedescribeprep"
	product "lsp-trace/internal/privatehostproductv3"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Predetermined ordinary-host tool-contract fixture, NOT a model or evidence of
// fresh independent semantic judgment. All source units still come from the real
// owned stdio/Census/page/checkpoint/preparation fixture, not fabricated DTOs.
type productHostFixture struct {
	t     *testing.T
	calls atomic.Int32
	fault bool
}

func (f *productHostFixture) Exchange(ctx context.Context, task product.Task) (product.Response, error) {
	n := f.calls.Add(1)
	if task.Body == "" || task.Protocol != product.Protocol || task.Revision != 1 || task.ConfigSHA256 == "" || task.Application == "" || task.ScopeSHA256 == "" || task.MaxRequestBytes <= 0 || task.MaxResponseBytes <= 0 {
		return product.Response{}, errors.New("own body protocol")
	}
	quote := task.Body
	if f.fault && n == 3 {
		quote = "foreign item body"
	}
	return product.Response{Protocol: product.Protocol, RequestDigest: task.RequestDigest, Label: "Controlled declaration", Explanation: "This bounded body declares a function; product purpose is unresolved.", Alternative: "Infrastructure declaration, not established product capability", RejectionCondition: "Own body does not support the bounded declaration description", Quote: quote, Start: 0, End: len(task.Body), Supported: true}, nil
}

type productContinuationFixture struct {
	Transfer      product.Transfer
	Config        product.Config
	Report        product.Report
	FirstPrepared []byte
}

func runProductScenario(t *testing.T, mode string) {
	t.Helper()
	dir, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	if os.Chmod(dir, 0700) != nil {
		t.Fatal("mode")
	}
	exe, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	transfer := filepath.Join(dir, "transfer.json")
	roles := []string{"A", "B"}
	if mode == "pause" {
		roles = append(roles, "C")
	}
	for _, role := range roles {
		cmd := exec.Command(exe, "-test.run=^TestHostProductPrivateV3Child$", "-test.v")
		cmd.Env = []string{"LSP_PRODUCT_ROLE=" + role, "LSP_PRODUCT_MODE=" + mode, "LSP_PRODUCT_TRANSFER=" + transfer, "HOME=" + dir, "TMPDIR=" + dir}
		raw, e := cmd.CombinedOutput()
		t.Logf("PRODUCT_%s_%s:\n%s", mode, role, raw)
		if e != nil {
			t.Fatalf("ASSERT_PRODUCT_TWENTY_AUTOMATIC_TERMINAL: %s %v", role, e)
		}
	}
}
func TestHostProductPrivateV3TwentyOneInvocation(t *testing.T) { runProductScenario(t, "normal") }
func TestHostProductPrivateV3BudgetPauseFreshProcessResume(t *testing.T) {
	runProductScenario(t, "pause")
}
func TestHostProductPrivateV3LaterUnitFailureRetainsCandidate(t *testing.T) {
	runProductScenario(t, "fault")
}
func TestHostProductPrivateV3Child(t *testing.T) {
	role := os.Getenv("LSP_PRODUCT_ROLE")
	if role == "" {
		t.Skip("controlled child only")
	}
	mode := os.Getenv("LSP_PRODUCT_MODE")
	path := os.Getenv("LSP_PRODUCT_TRANSFER")
	if role == "A" {
		pilot.ControlledRetainedCheckpointExportPrivateV3(t, path)
		os.Exit(0)
	}
	if role != "B" && role != "C" {
		t.Fatal("role")
	}
	transferRaw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	var transfer struct {
		Manifest   []byte
		Spec       pilot.ScopeSpecPrivateV2
		LedgerPath string
	}
	if json.Unmarshal(transferRaw, &transfer) != nil {
		t.Fatal("transfer")
	}
	var manifest pilot.RetainedPageCheckpointManifestPrivateV3
	if json.Unmarshal(transfer.Manifest, &manifest) != nil {
		t.Fatal("manifest")
	}
	c := product.Config{Profile: product.Profile, Application: manifest.Application, ScopeSHA256: manifest.ScopeSHA256, ProducerConfigSHA256: manifest.Revision.Policy.ConfigSHA256, Store: filepath.Join(filepath.Dir(path), "application"), Requested: 20, Exchange: pipelineGrantV3(), Preparation: prep.Limits{MaxTargets: 64, MaxPartBytes: 64 << 10, MaxTotalBytes: 8 << 20, MaxParts: 256, MaxSourceBytes: 2 << 20}, Seeds: []uint64{3, 7}, MaxCalls: 80, MaxRequestBytes: 1 << 20, MaxResponseBytes: 64 << 10, MaxTotalBytes: 8 << 20, AllowanceRevision: 1}
	if mode == "pause" && role == "B" {
		c.MaxCalls = 2
	}
	c.RolePromptSHA256 = [2]string{product.PromptDigest("interpreter"), product.PromptDigest("reviewer")}
	c.ApprovalSHA256 = product.ConfigDigest(c)
	var owner pilot.RetainedPageCheckpointOwnerPrivateV3
	var previous productContinuationFixture
	if role == "C" {
		raw, e := os.ReadFile(filepath.Join(filepath.Dir(path), "continuation.json"))
		if e != nil || json.Unmarshal(raw, &previous) != nil {
			t.Fatal("continuation")
		}
		c = previous.Config
		bootstrap := pipelineGrantV3()
		var close func() error
		owner, close, e = product.OpenTransferCurrent(context.Background(), previous.Transfer, previous.Report.LedgerGeneration, previous.Report.LedgerTip, &bootstrap, c.Exchange)
		if e != nil {
			t.Fatal("ASSERT_PRODUCT_FRESH_PROCESS_RETAINED_ADMISSION", e)
		}
		defer close()
		now := owner.BudgetSpent()
		old := previous.Report.Spend.PolicyReservation
		if now.LogicalBytes < old.LogicalBytes || now.ReadHashBytes < old.ReadHashBytes || now.Work < old.Work {
			t.Fatal("ASSERT_PRODUCT_CUMULATIVE_NO_REFUND")
		}
		raw, e = owner.ReadOutput(context.Background(), previous.Report.Candidates[0].EvidenceRefs[0], 1<<20, c.Exchange)
		if e != nil || !bytes.Equal(raw, previous.FirstPrepared) {
			t.Fatal("ASSERT_PRODUCT_PREVIOUS_REFS_BYTE_EXACT", e)
		}
	} else {
		owner, _ = pilot.ControlledRetainedCheckpointOpenPrivateV3(t, path)
	}
	if product.ValidateConfig(c) != nil {
		t.Fatal("valid config")
	}
	host := &productHostFixture{t: t, fault: mode == "fault"}
	mailboxPath := filepath.Join(filepath.Dir(path), "product-mailbox-"+role)
	if product.PrepareMailbox(mailboxPath) != nil {
		t.Fatal("mailbox")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	var services sync.WaitGroup
	for _, ownRole := range []string{"interpreter", "reviewer"} {
		if product.MarkReady(mailboxPath, product.Ready{Protocol: product.Protocol, Role: ownRole, LaunchID: "controlled-tool-contract-" + role + "-" + ownRole, PromptSHA256: product.PromptDigest(ownRole)}) != nil {
			t.Fatal("ready")
		}
		services.Add(1)
		go func(ownRole string) {
			defer services.Done()
			var after uint64
			for {
				bundle, e := product.WaitBundle(ctx, mailboxPath, ownRole, after)
				if e != nil {
					return
				}
				task, e := product.ReadBundle(bundle)
				if e != nil {
					t.Error("ASSERT_PRODUCT_COMPLETE_PARTS", e)
					cancel()
					return
				}
				response, e := host.Exchange(ctx, task)
				if e != nil {
					t.Error(e)
					cancel()
					return
				}
				raw, _ := json.Marshal(response)
				if e = product.Submit(mailboxPath, ownRole, task.Sequence, raw); e != nil {
					t.Error("ASSERT_PRODUCT_IMMEDIATE_RESPONSE", e)
					cancel()
					return
				}
				after = task.Sequence
			}
		}(ownRole)
	}
	backend := product.Mailbox{Directory: mailboxPath, MaxRequestBytes: c.MaxRequestBytes, MaxResponseBytes: c.MaxResponseBytes, ExchangeTimeout: 5 * time.Second}
	r, runErr := product.RunRetained(ctx, c, owner, backend)
	cancel()
	services.Wait()
	if mode == "pause" && role == "B" {
		if !errors.Is(runErr, product.ErrPaused) || r.Status != "PAUSED_BUDGET" || r.Completed != 1 || r.Pending != 19 || r.Spend.Calls != 2 {
			t.Fatalf("ASSERT_PRODUCT_PAUSE_AFTER_ADMITTED_SUBSET: %+v %v", r, runErr)
		}
		m, _, _, _, e := owner.HostFrontierPrivateV3()
		if e != nil {
			t.Fatal(e)
		}
		raw, e := owner.ReadOutput(context.Background(), r.Candidates[0].EvidenceRefs[0], 1<<20, c.Exchange)
		if e != nil {
			t.Fatal(e)
		}
		_, g, tip, spent, e := owner.HostFrontierPrivateV3()
		if e != nil {
			t.Fatal(e)
		}
		r.LedgerGeneration = g
		r.LedgerTip = tip
		r.Spend.PolicyReservation = spent
		next := c
		next.PreviousConfigSHA256 = product.ConfigDigest(c)
		next.AllowanceRevision++
		next.MaxCalls = 80
		next.StartBatch = m.NextBatch
		next.ApprovalSHA256 = product.ConfigDigest(next)
		mraw, _ := json.Marshal(m)
		saved := productContinuationFixture{Transfer: product.Transfer{Manifest: mraw, Spec: transfer.Spec, LedgerPath: transfer.LedgerPath}, Config: next, Report: r, FirstPrepared: raw}
		b, _ := json.Marshal(saved)
		if os.WriteFile(filepath.Join(filepath.Dir(path), "continuation.json"), b, 0600) != nil {
			t.Fatal("continuation save")
		}
		fmt.Println("ASSERT_PRODUCT_PAUSE_AFTER_ADMITTED_SUBSET: PASS")
		return
	}
	if mode == "fault" {
		if runErr == nil || r.Status != "FAILED" || r.Completed != 1 || r.Pending != 18 || r.Gaps != 1 || r.Failure == nil || r.Failure.Phase != "HOST_SEMANTIC_EXCHANGE" {
			t.Fatalf("ASSERT_PRODUCT_LATER_FAILURE_RETAINS_PRIOR: %+v %v", r, runErr)
		}
		raw, e := owner.ReadOutput(context.Background(), r.Candidates[0].EvidenceRefs[0], 1<<20, c.Exchange)
		if e != nil || pilot.Digest(raw) != r.Candidates[0].EvidenceRefs[0] {
			t.Fatal("ASSERT_PRODUCT_LATER_FAILURE_PAIRED_PRIOR", e)
		}
		calls := host.calls.Load()
		if _, e = product.RunRetained(context.Background(), c, owner, backend); e == nil || host.calls.Load() != calls {
			t.Fatal("ASSERT_PRODUCT_SPENT_EXCHANGE_NOT_AUTORETRIED")
		}
		fmt.Println("ASSERT_PRODUCT_LATER_FAILURE_RETAINS_PRIOR: PASS")
		return
	}
	expectedCalls := int32(40)
	if role == "C" {
		expectedCalls = 38
	}
	if runErr != nil || r.Completed != 20 || r.Pending != 0 || r.Gaps != 0 || host.calls.Load() != expectedCalls || r.Spend.Calls != 40 {
		t.Fatalf("ASSERT_PRODUCT_TWENTY_AUTOMATIC_TERMINAL: %+v calls=%d err=%v", r, host.calls.Load(), runErr)
	}
	for _, name := range []string{"report.json", "inventory.md"} {
		if _, e := os.Stat(filepath.Join(c.Store, "result", name)); e != nil {
			t.Fatal("ASSERT_PRODUCT_AUTOMATIC_VIEWS", e)
		}
	}
	fmt.Println("ASSERT_PRODUCT_TWENTY_AUTOMATIC_TERMINAL: PASS")
}
