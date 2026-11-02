//go:build darwin

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
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

// None_Model native stdio fixture. Positions and CALLS are controlled protocol
// answers, not gopls or evidence of independent semantic Agent judgments.
func TestHostApplicationOwnedServer(t *testing.T) {
	if os.Getenv("LSP_HOST_APPLICATION_OWNED_SERVER") != "1" {
		t.Skip("controlled child only")
	}
	rd := lspwire.NewReader(os.Stdin, lspwire.DefaultLimits())
	wr := lspwire.NewWriter(os.Stdout, lspwire.DefaultLimits())
	lastFile := ""
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
			if os.Getenv("LSP_HOST_APPLICATION_CORRUPT_LEDGER") == "1" {
				wd, _ := os.Getwd()
				files, _ := filepath.Glob(filepath.Join(filepath.Dir(wd), "parent", "units", "*", "source.json"))
				for _, p := range files {
					b, er := os.ReadFile(p)
					var source hostProductRequestV3
					if er != nil || json.Unmarshal(b, &source) != nil {
						continue
					}
					match := false
					for i, arg := range source.CensusArguments {
						if arg == "--source" && i+1 < len(source.CensusArguments) && filepath.Join(wd, source.CensusArguments[i+1]) == lastFile {
							match = true
						}
					}
					if match {
						ledger := filepath.Join(source.AuxiliaryRoot, "ledger")
						if os.Rename(ledger, ledger+".controlled-corruption") == nil {
							_ = os.Mkdir(ledger, 0700)
						}
					}
				}
			}
			result = nil
		case "textDocument/documentSymbol":
			if os.Getenv("LSP_HOST_APPLICATION_BLOCK_DOC") == "1" {
				time.Sleep(5 * time.Second)
			}
			if os.Getenv("LSP_HOST_APPLICATION_FAIL_DOC") == "1" {
				os.Exit(6)
			}
			var p struct {
				TextDocument struct {
					URI string `json:"uri"`
				} `json:"textDocument"`
			}
			if json.Unmarshal(m.Params, &p) != nil {
				os.Exit(2)
			}
			u, e := url.Parse(p.TextDocument.URI)
			if e != nil {
				os.Exit(2)
			}
			lastFile = u.Path
			raw, e := os.ReadFile(u.Path)
			if e != nil {
				os.Exit(2)
			}
			symbols := []lsp.DocumentSymbol{}
			for n, line := range strings.Split(string(raw), "\n") {
				if !strings.HasPrefix(line, "func Owned") {
					continue
				}
				name := strings.Split(strings.TrimPrefix(line, "func "), "(")[0]
				pos := uint32(n)
				symbols = append(symbols, lsp.DocumentSymbol{Name: name, Kind: 12, Range: lsp.Range{Start: lsp.Position{Line: pos}, End: lsp.Position{Line: pos, Character: uint32(len(line))}}, SelectionRange: lsp.Range{Start: lsp.Position{Line: pos, Character: 5}, End: lsp.Position{Line: pos, Character: 12}}})
			}
			result = symbols
		case "callHierarchy/incomingCalls":
			if os.Getenv("LSP_HOST_APPLICATION_LARGE_REFERENCES") == "1" {
				var p lsp.CallHierarchyIncomingCallsParams
				if json.Unmarshal(m.Params, &p) != nil {
					os.Exit(2)
				}
				if p.Item.Name == "Owned00" {
					u, e := url.Parse(p.Item.URI)
					if e != nil {
						os.Exit(2)
					}
					body, e := os.ReadFile(u.Path)
					if e != nil {
						os.Exit(2)
					}
					lines := strings.Split(string(body), "\n")
					ranges := []lsp.Range{}
					for line, text := range lines {
						if text == "\tOwned00()" {
							ranges = append(ranges, lsp.Range{Start: lsp.Position{Line: uint32(line), Character: 1}, End: lsp.Position{Line: uint32(line), Character: 8}})
						}
					}
					from := lsp.CallHierarchyItem{Name: "manyReferences", Kind: 12, URI: p.Item.URI, Range: lsp.Range{Start: lsp.Position{Line: 22}, End: lsp.Position{Line: uint32(len(lines) - 2), Character: 1}}, SelectionRange: lsp.Range{Start: lsp.Position{Line: 22, Character: 5}, End: lsp.Position{Line: 22, Character: 19}}}
					result = []lsp.CallHierarchyIncomingCall{{From: from, FromRanges: ranges}}
				}
			}
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

func hostApplicationCLIFixture(t *testing.T) (hostApplicationRequest, string, string) {
	t.Helper()
	source, _, base := hostProductCLIRequestFixture(t)
	workspace := filepath.Join(base, "source")
	if e := os.Remove(filepath.Join(workspace, "owned.go")); e != nil {
		t.Fatal(e)
	}
	for n, count := range []int{7, 7, 6} {
		var b strings.Builder
		b.WriteString("package owned\n\n")
		for i := 0; i < count; i++ {
			fmt.Fprintf(&b, "func Owned%02d() {}\n", i)
		}
		if e := os.WriteFile(filepath.Join(workspace, fmt.Sprintf("unit%d.go", n)), []byte(b.String()), 0600); e != nil {
			t.Fatal(e)
		}
	}
	for n, arg := range source.CensusArguments {
		if strings.Contains(arg, "TestHostProductCLIOwnedServer") {
			source.CensusArguments[n] = "-test.run=^TestHostApplicationOwnedServer$"
		}
	}
	source.Configuration.Requested = 0
	source.Configuration.ApprovalSHA256 = ""
	source.Configuration.ScopeSHA256 = ""
	source.Configuration.ProducerConfigSHA256 = ""
	source.RuntimeGrantSHA256 = ""
	allowance := pilot.PageCostPrivateV3{LogicalBytes: 32 << 30, ReadHashBytes: 32 << 30, WriteBytes: 4 << 30, Work: 4 << 30}
	// Explicit finite synthetic unit policy: the 1 MiB inherited compute
	// maximum amplified every retained operation. 32 KiB refused explicitly;
	// these controlled units use 256 KiB; oversized units still refuse.
	source.ComputationUnitBytes = 256 << 10
	source.PayloadBytes = 16 << 10
	source.Configuration.Exchange = allowance
	source.ApplicationAllowance = allowance
	source.BootstrapAllowance = pilot.PageCostPrivateV3{LogicalBytes: 256 << 20, ReadHashBytes: 256 << 20, WriteBytes: 16 << 20, Work: 64 << 20}
	d := pilot.Digest([]byte("controlled application selection: native Census *.go, one nominated file per unit"))
	templateRaw, _ := json.Marshal(source)
	cfg := product.ApplicationConfig{Schema: "private-host-application-config.v1", Profile: product.ApplicationProfile, Store: filepath.Join(base, "parent"), SourceRoot: workspace, RevisionAssertion: source.RevisionAssertion, SelectionRuleSHA256: d, UnitPolicySHA256: pilot.Digest(templateRaw), ProducerConfigSHA256: pilot.Digest(templateRaw), ProducerExecutableSHA256: source.ServerSHA256, StorageIdentitySHA256: pilot.Digest([]byte(base)), RolePromptSHA256: [2]string{pilot.Digest([]byte(product.CohortRolePrompt("interpreter"))), pilot.Digest([]byte(product.CohortRolePrompt("reviewer")))}, Grant: product.ApplicationGrant{Revision: 1, ExpiresUnixNano: time.Now().Add(10 * time.Minute).UnixNano(), BootstrapAttempts: 32, BootstrapEach: product.ApplicationBounds{ReadHashBytes: 4 * product.ApplicationRecordLimit, WriteBytes: product.ApplicationRecordLimit, Records: 8, ParserBytes: 3 * product.ApplicationRecordLimit, Work: 32, ActiveNS: 1000000}, Ceiling: product.ApplicationBounds{LogicalBytes: 128 << 30, ReadHashBytes: 128 << 30, WriteBytes: 32 << 30, Records: 1 << 20, ParserBytes: 4 << 30, ProviderRequests: 10000, ProviderMessages: 100000, Work: 32 << 30, ActiveNS: uint64(240 * time.Second), Calls: 16, RequestBytes: 1 << 20, ResponseBytes: 64 << 10, WireBytes: 8 << 20, OutputBytes: 4 << 20}}}
	cfg.ApprovalSHA256 = product.ApplicationConfigDigest(cfg)
	request := hostApplicationRequest{Schema: "private-host-application-request.v1", Configuration: cfg, Template: source, MaxUnitsPerRun: 8, UnitResponseBytes: 8192, UnitTimeoutMS: 60000}
	policySHA := hostApplicationUnitPolicyDigest(request)
	request.Configuration.UnitPolicySHA256 = policySHA
	request.Configuration.ProducerConfigSHA256 = policySHA
	request.Configuration.ApprovalSHA256 = product.ApplicationConfigDigest(request.Configuration)
	request.ApprovalSHA256 = hostApplicationDigest(request)
	raw, _ := json.Marshal(request)
	path := filepath.Join(base, "application.json")
	if e := os.WriteFile(path, raw, 0600); e != nil {
		t.Fatal(e)
	}
	return request, path, base
}
func TestHostApplicationCLIActualTwentyThreeUnits(t *testing.T) {
	request, path, base := hostApplicationCLIFixture(t)
	cfg := request.Configuration
	tool := buildApplicationTool(t, base)
	ctx, cancel := context.WithTimeout(context.Background(), 240*time.Second)
	defer cancel()
	var wg sync.WaitGroup
	var mu sync.Mutex
	calls, requests, responses := 0, 0, 0
	for _, role := range []string{"interpreter", "reviewer"} {
		wg.Add(1)
		go func(role string) {
			defer wg.Done()
			for unit := 1; unit <= 8; unit++ {
				var activation struct {
					Mailbox string
					UnitID  string
					Base    uint64
				}
				activationPath := filepath.Join(cfg.Store, fmt.Sprintf("activation-%02d.json", unit))
				for {
					b, e := os.ReadFile(activationPath)
					if e == nil && json.Unmarshal(b, &activation) == nil {
						break
					}
					select {
					case <-ctx.Done():
						return
					case <-time.After(2 * time.Millisecond):
					}
				}
				if product.MarkCohortReady(activation.Mailbox, product.Ready{Protocol: product.CohortProtocol, Role: role, LaunchID: "scripted-application-" + role, PromptSHA256: pilot.Digest([]byte(product.CohortRolePrompt(role)))}) != nil {
					t.Error("ready")
					return
				}
				var after uint64
				for {
					event, e := product.WaitCohort(ctx, activation.Mailbox, role, after)
					if e != nil {
						return
					}
					if event.Event == "TERMINAL" {
						break
					}
					if event.Bundle == nil {
						t.Error("bundle absent")
						return
					}
					task, e := product.ReadCohortBundle(*event.Bundle)
					if e != nil {
						t.Error(e)
						return
					}
					response := product.CohortResponse{Protocol: product.CohortProtocol, RequestDigest: task.RequestDigest, Items: []product.CohortDecision{}}
					for _, item := range task.Items {
						response.Items = append(response.Items, product.CohortDecision{ItemID: item.ItemID, InputID: item.InputID, Target: item.Target, Attempt: item.Attempt, Predecessor: item.Predecessor, OwnBodySHA256: item.OwnBodySHA256, Revision: item.Revision, Disposition: "SUPPORTED", Label: "Controlled declaration", Explanation: "Predetermined own-source lead; identity unresolved", Alternative: "Scaffolding", Falsifier: "Declaration differs", Quote: item.Body, Start: 0, End: len(item.Body)})
					}
					b, _ := json.Marshal(response)
					if e = product.SubmitCohort(activation.Mailbox, role, event.Bundle.Sequence, b); e != nil {
						t.Error(e)
						return
					}
					mu.Lock()
					calls++
					requests += event.Bundle.Bytes
					responses += len(b)
					mu.Unlock()
					after = event.Bundle.Sequence
				}
			}
		}(role)
	}
	cmd := exec.CommandContext(ctx, tool, "private-host-application", "run", path)
	cmd.Env = append(os.Environ(), "LSP_HOST_APPLICATION_OWNED_SERVER=1")
	var out, errout bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errout
	e := cmd.Run()
	cancel()
	wg.Wait()
	var head struct {
		Status                                             string
		Units, Targets, Calls, RequestBytes, ResponseBytes int
		InventoryHead                                      string
		Authority                                          int
		Accepted                                           bool
	}
	if e != nil || json.Unmarshal(out.Bytes(), &head) != nil || head.Units != 3 || head.Targets != 20 || head.Calls != 6 || head.InventoryHead == "" || head.Authority != 0 || head.Accepted {
		t.Fatalf("ASSERT_APPLICATION_CLI20_3_PIPELINE: error=%v output=%s diagnostics=%s", e, out.String(), errout.String())
	}
	if head.Calls != calls || head.RequestBytes != requests || head.ResponseBytes != responses {
		t.Fatalf("ASSERT_APPLICATION_GLOBAL_DIRECT_UTF8: head=%+v fixture=%d/%d/%d", head, calls, requests, responses)
	}
	t.Logf("ASSERT_APPLICATION_CLI20_3_PIPELINE PASS: units=%d targets=%d calls=%d direct request=%d response=%d; None_Model/SCRIPTED_CONTRACT", head.Units, head.Targets, calls, requests, responses)
}
