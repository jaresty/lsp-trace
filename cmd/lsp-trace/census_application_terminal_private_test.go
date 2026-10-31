//go:build darwin || linux

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"lsp-trace/internal/censusacquisition"
	pilot "lsp-trace/internal/censuslineagepilot"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/lspwire"
	prep "lsp-trace/internal/privatedescribeprep"
)

// Explicit fixture-authored protocol results, NOT an AST projection or actual
// application provider. All 84 callables are returned by original discovery.
func terminalOwnedResponsePrivate(m lspwire.Message) (json.RawMessage, bool) {
	var p lsp.PrepareCallHierarchyParams
	if json.Unmarshal(m.Params, &p) != nil {
		return nil, false
	}
	item := func(i int, kind int) lsp.CallHierarchyItem {
		name := fmt.Sprintf("C%d", i)
		line := uint32(2 + 2*i)
		return lsp.CallHierarchyItem{Name: name, Kind: kind, URI: p.TextDocument.URI, Range: lsp.Range{Start: lsp.Position{Line: line}, End: lsp.Position{Line: line, Character: uint32(len("func " + name + "() {}"))}}, SelectionRange: lsp.Range{Start: lsp.Position{Line: line, Character: 5}, End: lsp.Position{Line: line, Character: uint32(5 + len(name))}}}
	}
	var value any
	switch m.Method {
	case "textDocument/documentSymbol":
		if !strings.HasSuffix(p.TextDocument.URI, "/a.go") && !strings.HasSuffix(p.TextDocument.URI, "/b.go") {
			return nil, false
		}
		count, kind := 84, 12
		if strings.HasSuffix(p.TextDocument.URI, "/b.go") {
			count, kind = 8, 13
		}
		symbols := []lsp.DocumentSymbol{}
		for i := 0; i < count; i++ {
			v := item(i, kind)
			symbols = append(symbols, lsp.DocumentSymbol{Name: v.Name, Kind: v.Kind, Range: v.Range, SelectionRange: v.SelectionRange})
		}
		value = symbols
	case "textDocument/prepareCallHierarchy":
		if !strings.HasSuffix(p.TextDocument.URI, "/a.go") {
			return nil, false
		}
		value = []lsp.CallHierarchyItem{item(int((p.Position.Line-2)/2), 12)}
	default:
		return nil, false
	}
	raw, _ := json.Marshal(value)
	return raw, true
}
func terminalApplicationFixturePrivate(t *testing.T) (string, string, string, []string) {
	t.Helper()
	base, src, path, args := inventoryFixturePrivate(t)
	for _, entry := range []struct {
		name     string
		count    int
		callable bool
	}{{"a.go", 84, true}, {"b.go", 8, false}} {
		body := "package owned\n\n"
		for i := 0; i < entry.count; i++ {
			if entry.callable {
				body += fmt.Sprintf("func C%d() {}\n\n", i)
			} else {
				body += fmt.Sprintf("var C%d = 1\n\n", i)
			}
		}
		if e := os.WriteFile(filepath.Join(src, entry.name), []byte(body), 0600); e != nil {
			t.Fatal(e)
		}
	}
	directory := filepath.Join(base, "application")
	if e := os.Mkdir(directory, 0700); e != nil {
		t.Fatal(e)
	}
	c, e := loadCensusInventoryConfigPrivate(path)
	if e != nil {
		t.Fatal(e)
	}
	c.Policy.MaxNominationFiles = 1
	c.Policy.Discovery.MaxFiles = 1
	c.Application = &censusApplicationConfigPrivate{censusApplicationTerminalProfilePrivate, directory, 1 << 20, 2 << 20, 128 << 20, 128}
	saveInventoryConfigPrivate(t, path, c)
	return base, src, path, args
}

type terminalProcessRunnerPrivate struct {
	t            *testing.T
	base, binary string
	args         []string
}

func newTerminalProcessRunnerPrivate(t *testing.T, base string, args []string) terminalProcessRunnerPrivate {
	t.Helper()
	binary := filepath.Join(base, "lsp-trace")
	if out, e := exec.Command("go", "build", "-o", binary, ".").CombinedOutput(); e != nil {
		t.Fatalf("build setup: %v %s", e, out)
	}
	t.Cleanup(func() {
		if e := os.Chmod(binary, 0600); e != nil && !os.IsNotExist(e) {
			t.Error(e)
		}
	})
	args = append([]string(nil), args...)
	for i, a := range args {
		if a == "--server" {
			args[i+1] = os.Args[0]
		}
	}
	args = append(args, "--server-arg", "-test.run=^TestInventoryOwnedLSPProcessPrivate$")
	return terminalProcessRunnerPrivate{t, base, binary, args}
}
func (x terminalProcessRunnerPrivate) run() (censusInventoryReportPrivate, int, int) {
	t := x.t
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, x.binary, append([]string{"census"}, x.args...)...)
	cmd.Env = append(os.Environ(), "LSP_TRACE_OWNED_TWO_ORIGIN_SERVER=1", "LSP_TRACE_OWNED_TERMINAL_SERVER=1")
	out, e := cmd.Output()
	code := -1
	if cmd.ProcessState != nil {
		code = cmd.ProcessState.ExitCode()
	}
	if ee, ok := e.(*exec.ExitError); ok {
		t.Logf("CLI stderr=%s", ee.Stderr)
	}
	var meta struct{ AttemptDirectory string }
	if json.Unmarshal(out, &meta) != nil || meta.AttemptDirectory == "" {
		t.Fatalf("CLI report setup: %v %s", e, out)
	}
	raw, e := os.ReadFile(filepath.Join(meta.AttemptDirectory, "report.json"))
	if e != nil {
		t.Fatal(e)
	}
	var r censusInventoryReportPrivate
	if json.Unmarshal(raw, &r) != nil {
		t.Fatal("report decode")
	}
	wait := struct {
		ExitCode, ProcessID int
		Joined, Stopped     bool
	}{code, cmd.Process.Pid, cmd.ProcessState != nil, ctx.Err() != nil}
	if e = writeCensusInventoryFilePrivate(filepath.Join(x.base, "wait-"+filepath.Base(r.AttemptDirectory)+".json"), wait); e != nil {
		t.Fatal(e)
	}
	t.Logf("ACTUAL_TERMINAL_CLI_WAIT exit=%d pid=%d joined=%t stopped=%t status=%s attempt=%s failure=%s", code, wait.ProcessID, wait.Joined, wait.Stopped, r.Status, r.AttemptDirectory, r.Failure)
	if !wait.Joined || wait.Stopped {
		t.Fatal("actual host Wait bound")
	}
	return r, code, cmd.Process.Pid
}
func TestCensusTerminalApplicationCLIPrivate(t *testing.T) {
	base, _, path, args := terminalApplicationFixturePrivate(t)
	runner := newTerminalProcessRunnerPrivate(t, base, args)
	c, e := loadCensusInventoryConfigPrivate(path)
	if e != nil {
		t.Fatal(e)
	}
	for segment, want := range []string{pilot.TerminalTargetCapPrivateV2, pilot.TerminalNoTargetsPrivateV2} {
		r, code, pid := runner.run()
		assertInventoryPrivate(t, "ASSERT_TERMINAL_CLI_DELIVERY", code == 0 && r.Status == censusTerminalSegmentStatusPrivate && r.ApplicationBinding != nil && r.ApplicationBinding.Segment == segment && len(r.Origins) == 1 && r.Origins[0].Status == want && r.Origins[0].Terminal != nil && r.Outcome == nil && r.Inventory == nil && r.PreparationBytes == 0 && r.StopCalls == 1 && r.ShutdownJoined && r.CleanupVerified)
		terminalPairProofPrivate(t, r)
		applicationHostObservationPrivate(t, r, code, pid, true, false)
	}
	// Ordinary CLI exports exact scope metadata before any normal generation
	// claim/startup. This is an additional NONDELIVERY process, not a fourth
	// delivery or a private Go hash reconstruction by the host.
	normalScope, previewPlan := terminalScopePreviewCLIPrivate(t, runner)
	python, e := exec.LookPath("python3")
	if e != nil {
		t.Fatal(e)
	}
	helper, e := filepath.Abs(filepath.Join("..", "..", "scripts", "private-variable-v2-host-handoff.py"))
	if e != nil {
		t.Fatal(e)
	}
	hostBase := filepath.Join(base, "host")
	if e = os.Mkdir(hostBase, 0700); e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"mailbox", "exports", "control", "payloads"} {
		if e = os.Mkdir(filepath.Join(hostBase, name), 0700); e != nil {
			t.Fatal(e)
		}
	}
	c.Automation = &prep.AutomaticHandoffConfigPrivateV2{Enabled: true, Directory: filepath.Join(hostBase, "mailbox"), Invocation: "owned-terminal-successor-normal", InterpreterRole: "owned-interpreter", ReviewerRole: "owned-reviewer", ScopeSHA256: normalScope, TimeoutMS: 5000, OverallTimeoutMS: 20000, PollMS: 2, Limits: prep.AutomaticLimitsPrivateV2{MaxCalls: 64, MaxRequestBytes: 4 << 20, MaxResponseBytes: 64 << 10, MaxTotalBytes: 64 << 20, MaxOutputBytes: 8 << 20}}
	c.Automation.ApprovalSHA256 = prep.AutomaticHandoffApprovalPrivateV2(*c.Automation)
	saveInventoryConfigPrivate(t, path, c)
	host := make(chan error, 1)
	go func() {
		host <- inventoryHostEmulatePrivate(hostBase, c.Automation.Directory, helper, python, 4)
	}()
	r, code, pid := runner.run()
	hostErr := <-host
	assertInventoryPrivate(t, "ASSERT_TERMINAL_POST_GAP_NORMAL_NATIVE", code == 0 && hostErr == nil && r.Outcome != nil && r.Inventory != nil && r.ApplicationBinding.Segment == 2 && r.ApplicationBinding.FullPlanSHA256 == previewPlan && r.ScopeSHA256 == normalScope && len(r.Index) == 1 && r.CleanupVerified)
	applicationHostObservationPrivate(t, r, code, pid, true, false)
	c.Automation = nil
	saveInventoryConfigPrivate(t, path, c)
	final, code, _ := runner.run()
	raw, e := os.ReadFile(filepath.Join(final.AttemptDirectory, "application-accounting.json"))
	if e != nil {
		t.Fatal(e)
	}
	var view censusApplicationAccountingPrivate
	if json.Unmarshal(raw, &view) != nil {
		t.Fatal("accounting")
	}
	assertInventoryPrivate(t, "ASSERT_TERMINAL_EXACT_DISJOINT_ACCOUNTING", code == 0 && len(view.Verified) == 3 && view.FileDenominator == 4 && view.CompletedFiles == 1 && view.TerminalAccountedFiles == 2 && view.TerminalAccountedTargets == 84 && view.NativePreparedTargets == 1 && view.GapFiles == 2 && view.ExcludedFiles == 1 && view.UnattemptedFiles == 0 && view.CallableTargets == 85 && view.NonCallableSymbols == 8 && view.ObservedSymbolEntries == 93)
	wantRows := []struct{ path, disposition string }{{"a.go", "TERMINAL_ACCOUNTED_GAP"}, {"b.go", "TERMINAL_ACCOUNTED_GAP"}, {"c.go", "COMPLETED_NATIVE"}, {"excluded.go", "EXCLUDED"}}
	identitiesOK := len(view.Files) == len(wantRows)
	for i, want := range wantRows {
		if i >= len(view.Files) || view.Files[i].File.Path != want.path || view.Files[i].Disposition != want.disposition {
			identitiesOK = false
		}
	}
	assertInventoryPrivate(t, "ASSERT_TERMINAL_EXACT_FINAL_ROW_IDENTITIES", identitiesOK)
	t.Logf("TERMINAL_APPLICATION_EVIDENCE base=%s final=%s", base, final.AttemptDirectory)
}

func TestCensusTerminalApplicationUnverifiedStopPrivate(t *testing.T) {
	for _, kind := range []string{"missing-host", "failed-host", "missing-join", "source-drift", "config-drift", "missing-terminal", "wrong-reason", "malformed-pages", "unknown-control", "cleanup", "wrong-count", "missing-pair", "terminal-fifo", "terminal-symlink", "page-fifo"} {
		t.Run(kind, func(t *testing.T) {
			base, src, path, args := terminalApplicationFixturePrivate(t)
			// Same actual original pipeline; this test does not claim an outer CLI Wait.
			deps, _, _ := inventoryTwoDepsPrivate(t)
			t.Setenv("LSP_TRACE_OWNED_TERMINAL_SERVER", "1")
			r, code := runInventoryCLIPrivate(t, args, deps)
			if code != 0 || r.Status != censusTerminalSegmentStatusPrivate {
				t.Fatalf("terminal control setup: %d %s", code, r.Failure)
			}
			c, _ := loadCensusInventoryConfigPrivate(path)
			// Existing exact reseal helper rebinds all hashes, not opaque ownership.
			switch kind {
			case "source-drift":
				if e := os.WriteFile(filepath.Join(src, "c.go"), []byte("package owned\n\nfunc A1() {}\n"), 0600); e != nil {
					t.Fatal(e)
				}
			case "config-drift":
				c.Seeds = []uint64{44, 45}
				saveInventoryConfigPrivate(t, path, c)
			case "cleanup":
				r.Origins[0].Cleanup.Reap = ""
			case "wrong-count":
				var d censusacquisition.Discovery
				_ = json.Unmarshal(r.Origins[0].Discovery, &d)
				d.Targets = nil
				r.Origins[0].Discovery, _ = json.Marshal(d)
			case "terminal-fifo", "terminal-symlink", "page-fifo":
				copy := r.Origins[0].Terminal.Copies[1].Receipt
				leaf := filepath.Join(r.AttemptDirectory, "terminal-0-1", copy.FinalSelector)
				if kind == "page-fifo" {
					leaf = filepath.Join(r.AttemptDirectory, r.FilePages[0].Selector)
				}
				if e := os.Remove(leaf); e != nil {
					t.Fatal(e)
				}
				if kind == "terminal-symlink" {
					if e := os.Symlink(filepath.Join(r.AttemptDirectory, "terminal-0-0", copy.FinalSelector), leaf); e != nil {
						t.Fatal(e)
					}
				} else if e := syscall.Mkfifo(leaf, 0600); e != nil {
					t.Fatal(e)
				}
			case "missing-pair":
				copy := r.Origins[0].Terminal.Copies[1].Receipt
				if e := os.Remove(filepath.Join(r.AttemptDirectory, "terminal-0-1", copy.FinalSelector)); e != nil {
					t.Fatal(e)
				}
			case "missing-terminal":
				r.Origins[0].Terminal = nil
			case "wrong-reason":
				r.Origins[0].Status = pilot.TerminalNoTargetsPrivateV2
				r.Origins[0].Terminal.Reason = pilot.TerminalNoTargetsPrivateV2
			case "malformed-pages":
				r.FilePages[0].Entries++
			}
			if kind == "missing-terminal" || kind == "wrong-reason" || kind == "malformed-pages" || kind == "unknown-control" || kind == "cleanup" || kind == "wrong-count" {
				raw, _ := json.Marshal(r)
				if kind == "unknown-control" {
					raw = bytes.Replace(raw, []byte(`"Reason":`), []byte(`"ExtraControl":true,"Reason":`), 1)
				}
				if e := os.WriteFile(filepath.Join(r.AttemptDirectory, "report.json"), raw, 0600); e != nil {
					t.Fatal(e)
				}
				resultPath := filepath.Join(c.Application.Directory, censusApplicationLeafPrivate(0, "result"))
				b, e := os.ReadFile(resultPath)
				if e != nil {
					t.Fatal(e)
				}
				var result censusApplicationResultPrivate
				_ = json.Unmarshal(b, &result)
				result.ReportSHA256 = pilot.Digest(raw)
				b, _ = json.Marshal(result)
				if e = os.WriteFile(resultPath, b, 0600); e != nil {
					t.Fatal(e)
				}
			}
			if kind != "missing-host" {
				exit := 0
				if kind == "failed-host" {
					exit = 1
				}
				applicationHostObservationPrivate(t, r, exit, r.ApplicationBinding.ProcessID, kind != "missing-join", false)
			}
			later, laterCode := runInventoryCLIPrivate(t, args, deps)
			assertInventoryPrivate(t, "ASSERT_TERMINAL_UNVERIFIED_HARD_STOP", laterCode == 1 && later.StopCalls == 0 && later.ApplicationBinding != nil && later.ApplicationBinding.Segment == 0)
			_ = base
		})
	}
}
