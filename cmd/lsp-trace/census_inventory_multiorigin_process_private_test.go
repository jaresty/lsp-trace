//go:build darwin || linux

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	pilot "lsp-trace/internal/censuslineagepilot"
	prep "lsp-trace/internal/privatedescribeprep"
	"lsp-trace/internal/publication"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Owned stdio fixture executable; never starts an application language server.
func TestInventoryOwnedLSPProcessPrivate(t *testing.T) {
	if os.Getenv("LSP_TRACE_OWNED_TWO_ORIGIN_SERVER") != "1" {
		t.Skip("owned subprocess only")
	}
	a, b := io.Pipe()
	c, d := io.Pipe()
	child := &inventoryPipePrivate{in: a, input: b, out: c, output: d, done: make(chan struct{})}
	if os.Getenv("LSP_TRACE_OWNED_SHUTDOWN_DELAY") == "late" {
		child.shutdownDelay = 600 * time.Millisecond
	}
	if os.Getenv("LSP_TRACE_OWNED_SHUTDOWN_DELAY") == "absent" {
		child.shutdownDelay = 4 * time.Second
	}
	go func() { _, _ = io.Copy(b, os.Stdin); _ = b.Close() }()
	copied := make(chan struct{})
	go func() { _, _ = io.Copy(os.Stdout, c); close(copied) }()
	child.serve()
	_ = d.Close()
	<-copied
	child.Close()
	os.Exit(0)
}

func TestCensusPrivateInventoryTwoOriginCLIProcess(t *testing.T) {
	python, e := exec.LookPath("python3")
	if e != nil {
		t.Fatal(e)
	}
	base, src, config, args := inventoryTwoFixturePrivate(t)
	binary := filepath.Join(base, "lsp-trace")
	build := exec.Command("go", "build", "-o", binary, ".")
	if out, e := build.CombinedOutput(); e != nil {
		t.Fatalf("owned CLI build setup: %v %s", e, out)
	}
	for i, a := range args {
		if a == "--server" {
			args[i+1] = os.Args[0]
		}
	}
	args = append(args, "--server-arg", "-test.run=^TestInventoryOwnedLSPProcessPrivate$")
	var completion struct {
		ExitCode        int
		Joined, Stopped bool
	}
	run := func() (censusInventoryReportPrivate, int) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, binary, append([]string{"census"}, args...)...)
		cmd.Env = append(os.Environ(), "LSP_TRACE_OWNED_TWO_ORIGIN_SERVER=1")
		out, e := cmd.Output()
		code := -1
		if cmd.ProcessState != nil {
			code = cmd.ProcessState.ExitCode()
		}
		if e != nil {
			t.Logf("CLI process error: %v stdout=%s", e, out)
			if ee, ok := e.(*exec.ExitError); ok {
				t.Logf("stderr=%s", ee.Stderr)
			}
		}
		var meta struct{ AttemptDirectory string }
		if json.Unmarshal(out, &meta) != nil || meta.AttemptDirectory == "" {
			t.Fatalf("CLI process omitted report: %s", out)
		}
		raw, e := os.ReadFile(filepath.Join(meta.AttemptDirectory, "report.json"))
		if e != nil {
			t.Fatal(e)
		}
		var r censusInventoryReportPrivate
		if e = json.Unmarshal(raw, &r); e != nil {
			t.Fatal(e)
		}
		completion.ExitCode = code
		completion.Joined = cmd.ProcessState != nil // Output returned only after this exact command's Wait.
		completion.Stopped = ctx.Err() != nil       // CommandContext is the only fixture stop mechanism.
		t.Logf("CLI process Wait returned; native command completion=%+v attempt=%s", completion, r.AttemptDirectory)
		if code != 0 {
			for _, o := range r.Origins {
				t.Logf("terminal origin ordinal=%d status=%s cleanup=%+v", o.Ordinal, o.Status, o.Cleanup)
			}
		}
		completionRaw, _ := json.Marshal(completion)
		if e = os.WriteFile(filepath.Join(base, fmt.Sprintf("process-%s.json", filepath.Base(r.AttemptDirectory))), completionRaw, 0600); e != nil {
			t.Fatal(e)
		}
		return r, code
	}
	first, code := run()
	if code != 0 {
		t.Fatal("preparation process failed", first.Failure)
	}
	second, code := run()
	assertInventoryPrivate(t, "ASSERT_TWO_ORIGIN_FRESH_SCOPE_REPRODUCIBLE", code == 0 && first.ScopeSHA256 == second.ScopeSHA256 && first.AttemptDirectory != second.AttemptDirectory)
	c, e := loadCensusInventoryConfigPrivate(config)
	if e != nil {
		t.Fatal(e)
	}
	mailbox := filepath.Join(base, "mailbox")
	for _, name := range []string{"mailbox", "exports", "control", "payloads"} {
		if e = os.Mkdir(filepath.Join(base, name), 0700); e != nil {
			t.Fatal(e)
		}
	}
	c.Automation = &prep.AutomaticHandoffConfigPrivateV2{Enabled: true, Directory: mailbox, Invocation: "owned-two-original-origins", InterpreterRole: "owned-interpreter", ReviewerRole: "owned-reviewer", ScopeSHA256: first.ScopeSHA256, TimeoutMS: 5000, OverallTimeoutMS: 20000, PollMS: 2, Limits: prep.AutomaticLimitsPrivateV2{MaxCalls: 32, MaxRequestBytes: 4 << 20, MaxResponseBytes: 64 << 10, MaxTotalBytes: 64 << 20, MaxOutputBytes: 8 << 20}}
	c.Automation.ApprovalSHA256 = prep.AutomaticHandoffApprovalPrivateV2(*c.Automation)
	saveInventoryConfigPrivate(t, config, c)
	helper, e := filepath.Abs(filepath.Join("..", "..", "scripts", "private-variable-v2-host-handoff.py"))
	if e != nil {
		t.Fatal(e)
	}
	host := make(chan error, 1)
	go func() { host <- inventoryHostEmulatePrivate(base, mailbox, helper, python, 18) }()
	r, code := run()
	hostErr := <-host
	pair := true
	var prior []byte
	var stats [2]os.FileInfo
	if r.Outcome == nil {
		pair = false
	} else {
		for i := 0; i < 2; i++ {
			rootPath := filepath.Join(r.AttemptDirectory, fmt.Sprintf("inventory-%d", i))
			root, e := publication.OpenContentRetentionRoot(context.Background(), rootPath)
			if e != nil {
				t.Fatal(e)
			}
			raw, e := root.ReadPostPreparationPrivateV2(context.Background(), r.Outcome.EnvelopeSHA256, int64(r.Outcome.ByteLength))
			ce := root.Close()
			var envelope struct {
				Inventory      *prep.AutomaticInventoryPrivateV2
				HumanInventory string
			}
			if e != nil || ce != nil || json.Unmarshal(raw, &envelope) != nil || envelope.Inventory == nil || len(envelope.Inventory.Candidates) != 8 || !strings.Contains(envelope.HumanInventory, "Declares an empty function body") || pilot.Digest(raw) != r.Outcome.EnvelopeSHA256 {
				pair = false
			}
			if i > 0 && !bytes.Equal(prior, raw) {
				pair = false
			}
			prior = raw
			entries, e := os.ReadDir(rootPath)
			if e != nil || len(entries) != 1 {
				pair = false
			} else {
				stats[i], _ = os.Stat(filepath.Join(rootPath, entries[0].Name()))
			}
		}
	}
	pair = pair && stats[0] != nil && stats[1] != nil && !os.SameFile(stats[0], stats[1])
	ids := map[string]bool{}
	for _, cov := range r.Coverage {
		if cov.PrivateV2 != nil {
			ids[cov.PrivateV2.OriginID] = true
		}
	}
	assertInventoryPrivate(t, "ASSERT_TWO_ORIGIN_CLI_FILE_HANDOFF_PAIR", code == 0 && completion.Joined && completion.ExitCode == 0 && !completion.Stopped && hostErr == nil && r.Inventory != nil && len(r.Inventory.Candidates) == 8 && len(ids) == 2 && r.ShutdownCalls == 2 && r.StopCalls == 2 && r.CleanupVerified && pair)
	t.Logf("two-origin original CLI fixture evidence=%s source=%s; authority=0 accepted=false; host=%v; envelope-bytes=%d", base, src, hostErr, len(prior))
	// Source drift with the old scope must refuse before either startup/disclosure.
	if e = os.WriteFile(filepath.Join(src, "a.go"), []byte("package owned\n\nfunc A0() { /* changed */ }\n"), 0600); e != nil {
		t.Fatal(e)
	}
	c.Automation.Directory = filepath.Join(base, "drift-mailbox")
	if e = os.Mkdir(c.Automation.Directory, 0700); e != nil {
		t.Fatal(e)
	}
	c.Automation.Invocation = "owned-two-origin-drift-refusal"
	c.Automation.ApprovalSHA256 = prep.AutomaticHandoffApprovalPrivateV2(*c.Automation)
	saveInventoryConfigPrivate(t, config, c)
	drift, code := run()
	assertInventoryPrivate(t, "ASSERT_TWO_ORIGIN_SOURCE_APPROVAL_DRIFT", code == 1 && drift.Failure == "DISCLOSURE_SCOPE_MISMATCH" && drift.ScopeSHA256 != first.ScopeSHA256 && drift.StopCalls == 0 && drift.Outcome == nil)
}
