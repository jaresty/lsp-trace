//go:build darwin

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	pilot "lsp-trace/internal/censuslineagepilot"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/lspwire"
	product "lsp-trace/internal/privatehostproductv3"
)

// Own controlled stdio producer, never gopls or an external application. This
// supplies native documentSymbol/CALLS responses for twenty exact own bodies.
func TestHostProductCLIOwnedServer(t *testing.T) {
	if os.Getenv("LSP_HOST_PRODUCT_OWNED_SERVER") != "1" {
		t.Skip("controlled stdio child only")
	}
	rd := lspwire.NewReader(os.Stdin, lspwire.DefaultLimits())
	wr := lspwire.NewWriter(os.Stdout, lspwire.DefaultLimits())
	for {
		m, e := rd.Read()
		if e != nil {
			os.Exit(1)
		}
		if m.Method == "exit" {
			os.Exit(0)
		}
		if m.Method == "initialized" || strings.HasPrefix(m.Method, "textDocument/did") {
			continue
		}
		var result any = []any{}
		switch m.Method {
		case "initialize":
			result = map[string]any{"capabilities": map[string]any{"positionEncoding": "utf-16", "documentSymbolProvider": true, "callHierarchyProvider": true}}
		case "shutdown":
			result = nil
		case "textDocument/documentSymbol":
			symbols := []lsp.DocumentSymbol{}
			for i := 0; i < 20; i++ {
				line := uint32(2 + i)
				symbols = append(symbols, lsp.DocumentSymbol{Name: fmt.Sprintf("Owned%02d", i), Kind: 12, Range: lsp.Range{Start: lsp.Position{Line: line}, End: lsp.Position{Line: line, Character: 17}}, SelectionRange: lsp.Range{Start: lsp.Position{Line: line, Character: 5}, End: lsp.Position{Line: line, Character: 12}}})
			}
			result = symbols
		case "textDocument/prepareCallHierarchy":
			var p lsp.PrepareCallHierarchyParams
			if json.Unmarshal(m.Params, &p) != nil {
				os.Exit(2)
			}
			result = []lsp.CallHierarchyItem{{Name: fmt.Sprintf("Owned%02d", p.Position.Line-2), Kind: 12, URI: p.TextDocument.URI, Range: lsp.Range{Start: lsp.Position{Line: p.Position.Line}, End: lsp.Position{Line: p.Position.Line, Character: 17}}, SelectionRange: lsp.Range{Start: lsp.Position{Line: p.Position.Line, Character: 5}, End: lsp.Position{Line: p.Position.Line, Character: 12}}}}
		}
		raw, _ := json.Marshal(result)
		if wr.Write(lspwire.Message{JSONRPC: lspwire.Version, ID: m.ID, Result: raw}) != nil {
			os.Exit(3)
		}
	}
}
func hostProductCLIRequestFixture(t *testing.T) (hostProductRequestV3, string, string) {
	t.Helper()
	base, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	os.Chmod(base, 0700)
	source, pub := filepath.Join(base, "source"), filepath.Join(base, "publication")
	os.Mkdir(source, 0700)
	os.Mkdir(pub, 0700)
	var body strings.Builder
	body.WriteString("package owned\n\n")
	for i := 0; i < 20; i++ {
		fmt.Fprintf(&body, "func Owned%02d() {}\n", i)
	}
	if os.WriteFile(filepath.Join(source, "owned.go"), []byte(body.String()), 0600) != nil {
		t.Fatal("source")
	}
	exe, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	server, e := os.ReadFile(exe)
	if e != nil {
		t.Fatal(e)
	}
	old := inventoryConfigPrivate()
	policy := old.Policy
	policy.MaxNominationFiles = 1
	policy.Discovery.MaxFiles = 1
	grant := pilot.PageCostPrivateV3{LogicalBytes: 1 << 40, ReadHashBytes: 1 << 40, WriteBytes: 1 << 34, Work: 1 << 32}
	c := product.Config{Profile: product.Profile, Application: pilot.Digest([]byte("actual-controlled-cli-twenty")), Store: filepath.Join(base, "application"), Requested: 20, Exchange: grant, Preparation: old.Preparation, Seeds: []uint64{3, 7}, MaxCalls: 40, MaxRequestBytes: 1 << 20, MaxResponseBytes: 64 << 10, MaxTotalBytes: 8 << 20, AllowanceRevision: 1}
	r := hostProductRequestV3{Schema: "private-existing-host-request.v3.1", Configuration: c, CensusArguments: []string{"--workspace", source, "--server", exe, "--server-arg", "-test.run=^TestHostProductCLIOwnedServer$", "--publication-root", pub, "--source", ".", "--include", "*.go", "--batch-targets", "1", "--max-nodes", "128", "--down-depth", "1", "--up-depth", "1", "--timeout", "30s", "--request-timeout", "5s", "--machine", "--private-enumeration-projection"}, Policy: policy, RevisionAssertion: strings.Repeat("a", 40), AuxiliaryRoot: filepath.Join(base, "auxiliary"), ServerSHA256: pilot.Digest(server), ApplicationAllowance: grant, BootstrapAllowance: grant, PayloadBytes: 64 << 10, ComputationUnitBytes: 1 << 20, OperationTimeoutMS: 240000, ExchangeTimeoutMS: 10000, FileDenominator: 1}
	f, e := hostProductPreviewV3(context.Background(), r)
	if e != nil {
		t.Fatal("preview", e)
	}
	r.Configuration.ScopeSHA256 = pilot.Digest(f.ManifestBytes())
	r.Configuration.ProducerConfigSHA256 = hostProductProducerDigestV3(r)
	r.Configuration.RolePromptSHA256 = [2]string{product.PromptDigest("interpreter"), product.PromptDigest("reviewer")}
	r.Configuration.ApprovalSHA256 = product.ConfigDigest(r.Configuration)
	r.RuntimeGrantSHA256 = hostProductRequestDigestV3(r)
	path := filepath.Join(base, "request.json")
	raw, _ := json.Marshal(r)
	if os.WriteFile(path, raw, 0600) != nil {
		t.Fatal("request")
	}
	return r, path, base
}
func TestHostProductCLIProductionAdmissionBeforeClaim(t *testing.T) {
	r, path, _ := hostProductCLIRequestFixture(t)
	var out, errout bytes.Buffer
	if runPrivateHostProductV3([]string{"setup", path}, bytes.NewReader(nil), &out, &errout) != 0 {
		t.Fatal(errout.String())
	}
	if os.WriteFile(filepath.Join(r.Configuration.Store, "foreign-scope.json"), []byte("{}"), 0600) != nil {
		t.Fatal("foreign")
	}
	if runPrivateHostProductV3([]string{"run", path}, bytes.NewReader(nil), io.Discard, io.Discard) == 0 {
		t.Fatal("ASSERT_PRODUCTION_CLOSED_STORE_REFUSAL")
	}
	if _, e := os.Stat(filepath.Join(r.Configuration.Store, "claim/claim-00000000.json")); !os.IsNotExist(e) {
		t.Fatal("ASSERT_NO_CLAIM_BEFORE_PRODUCTION_ADMISSION")
	}
}
func TestHostProductCLIActualTwentyOneInvocation(t *testing.T) {
	r, path, base := hostProductCLIRequestFixture(t)
	// The real executable is deliberately local to this controlled test. No global
	// install, live provider/model, SDK or daemon is introduced.
	tool := filepath.Join(base, "lsp-trace")
	build := exec.Command("go", "build", "-o", tool, ".")
	if raw, e := build.CombinedOutput(); e != nil {
		t.Fatalf("build %v %s", e, raw)
	}
	var out, errout bytes.Buffer
	if runPrivateHostProductV3([]string{"setup", path}, bytes.NewReader(nil), &out, &errout) != 0 {
		t.Fatal(errout.String())
	}
	mailbox := hostProductMailboxV3(r).Directory
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	var services sync.WaitGroup
	for _, role := range []string{"interpreter", "reviewer"} {
		if product.MarkReady(mailbox, product.Ready{Protocol: product.Protocol, Role: role, LaunchID: "ordinary-host-tool-contract-fixture-" + role, PromptSHA256: product.PromptDigest(role)}) != nil {
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
				// Predetermined transport response ONLY. It claims no model judgment.
				response := product.Response{Protocol: product.Protocol, RequestDigest: task.RequestDigest, Label: "Controlled declaration", Explanation: "Observed own-body function declaration; product purpose unresolved.", Alternative: "Technical scaffolding rather than product capability", RejectionCondition: "Own body fails to support declaration description", Quote: task.Body, Start: 0, End: len(task.Body), Supported: role == "reviewer"}
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
	cmd := exec.CommandContext(ctx, tool, "private-host-v3", "run", path)
	cmd.Env = append(os.Environ(), "LSP_HOST_PRODUCT_OWNED_SERVER=1")
	out.Reset()
	errout.Reset()
	cmd.Stdout = &out
	cmd.Stderr = &errout
	e := cmd.Run()
	cancel()
	services.Wait()
	if e != nil {
		initial, _ := os.ReadFile(filepath.Join(r.Configuration.Store, "result/report.json"))
		var safe product.Report
		json.Unmarshal(initial, &safe)
		t.Fatalf("ASSERT_ACTUAL_CLI_TWENTY_PRODUCT: %v diagnostic=%s safeFailure=%+v stdout=%s", e, errout.String(), safe.Failure, out.String())
	}
	var report product.Report
	if json.Unmarshal(out.Bytes(), &report) != nil || report.Completed != 20 || report.Pending != 0 || report.Status != "PROVISIONAL" || report.Spend.Calls != 40 || len(report.Candidates) != 20 || report.Accepted || report.Authority != 0 {
		t.Fatalf("ASSERT_ACTUAL_CLI_TWENTY_PRODUCT: %+v", report)
	}
	for _, paths := range [][2]string{report.MachinePaths, report.HumanPaths} {
		a, e := os.ReadFile(filepath.Join(r.Configuration.Store, paths[0]))
		if e != nil {
			t.Fatal(e)
		}
		b, e := os.ReadFile(filepath.Join(r.Configuration.Store, paths[1]))
		if e != nil || !bytes.Equal(a, b) {
			t.Fatal("ASSERT_ACTUAL_CLI_PAIRED_VIEWS", e)
		}
	}
	if _, e := os.Stat(filepath.Join(r.Configuration.Store, "claim/claim-00000000.json")); e != nil {
		t.Fatal("ASSERT_ACTUAL_CLI_ORIGINAL_CLAIM", e)
	}
	t.Log("ASSERT_ACTUAL_CLI_TWENTY_PRODUCT: PASS; controlled stdio plus predetermined host tool-contract payloads, not live semantic inference")
}
