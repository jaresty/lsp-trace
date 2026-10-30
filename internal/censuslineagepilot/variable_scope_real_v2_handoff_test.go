//go:build darwin || linux

package censuslineagepilot_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	pilot "lsp-trace/internal/censuslineagepilot"
	prep "lsp-trace/internal/privatedescribeprep"
	"lsp-trace/internal/publication"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

// This child owns the original production orchestrator and mailbox adapter.
// Only acquisition/Git use the existing package-local owned fixture seams.
func TestVariableCallerHandoffNoModelTransport(t *testing.T) {
	repo, e := filepath.Abs("../..")
	if e != nil {
		t.Fatal(e)
	}
	file, e := parser.ParseFile(token.NewFileSet(), filepath.Join(repo, "internal/privatedescribeprep/automatic_handoff_private_v2.go"), nil, parser.ImportsOnly)
	if e != nil {
		t.Fatal(e)
	}
	for _, imp := range file.Imports {
		p, _ := strconv.Unquote(imp.Path.Value)
		if strings.HasPrefix(p, "net") || p == "os/exec" {
			t.Fatal("NO_GO_MODEL_TRANSPORT: FAIL", p)
		}
	}
	typ := reflect.TypeOf(prep.AutomaticHandoffConfigPrivateV2{})
	for _, name := range []string{"Endpoint", "InterpreterModel", "ReviewerModel", "APIKeyEnvironment"} {
		if _, ok := typ.FieldByName(name); ok {
			t.Fatal("NO_GO_MODEL_TRANSPORT: FAIL", name)
		}
	}
	t.Log("NO_GO_MODEL_TRANSPORT: PASS adapter imports and configuration; owned execution uses host file IPC only")
}
func TestVariableCallerHandoffRetiredAndAlias(t *testing.T) {
	for _, kind := range []string{"v3", "source", "producer", "journal", "output"} {
		t.Run(kind, func(t *testing.T) {
			c, path, git := variableAutomaticConfig(t, 1)
			switch kind {
			case "v3":
				c.Schema = "private-variable-scope-v2-caller.v3"
			case "source":
				c.Automation.Directory = c.Scope.Root
			case "producer":
				c.Automation.Directory = c.ProducerOutputs[0]
			case "journal":
				c.Automation.Directory = c.OperationDirectory
			case "output":
				c.Automation.Directory = c.PostOutputs[0]
			}
			c.Automation.ApprovalSHA256 = prep.AutomaticHandoffApprovalPrivateV2(*c.Automation)
			variableSave(t, path, c)
			factory, starts, _, _ := variableOwnedFactory(t, "")
			if _, e := variableRunWithCode(context.Background(), "execute-once", path, git, factory, variableClose, variableOwnedCodeInputs); e == nil || starts.Load() != 0 {
				t.Fatal("HANDOFF_REFUSE_BEFORE_CLAIM: FAIL", kind)
			}
			variableEmpty(t, c)
			t.Log("HANDOFF_REFUSE_BEFORE_CLAIM: PASS", kind)
		})
	}
}
func TestVariableCallerHostWorker(t *testing.T) {
	path := os.Getenv("LSP_TRACE_PRIVATE_V2_CALLER_CONFIG")
	if path == "" {
		t.Skip("owned subprocess only")
	}
	raw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	var c variableCallerConfig
	if e = variableJSON(raw, &c); e != nil {
		t.Fatal(e)
	}
	git := func(ctx context.Context, root string, args ...string) ([]byte, error) {
		switch strings.Join(args, " ") {
		case "rev-parse --show-toplevel":
			return []byte(c.CodeRoot + "\n"), nil
		case "rev-parse HEAD":
			if root == c.CodeRoot {
				return []byte(c.CodeHEAD + "\n"), nil
			}
			return []byte(c.Scope.RevisionAssertion + "\n"), nil
		case "diff --cached --quiet", "ls-files --others --exclude-standard":
			return nil, nil
		}
		return nil, errors.New("owned Git allowlist")
	}
	factory, starts, child, _ := variableOwnedFactory(t, "")
	r, e := variableHostRunWithCode(context.Background(), "execute-once", path, os.Getenv("LSP_TRACE_PRIVATE_V2_CALLER_CONFIG_SHA256"), git, factory, variableClose, variableOwnedCodeInputs)
	if e != nil || r.Inventory == nil || r.Phase != "PROVISIONAL_COMPLETE" || starts.Load() != 1 || r.Cleanup.StopCalls != 1 || r.Cleanup.ShutdownCalls != 1 || r.Cleanup.TerminalReads != 1 {
		t.Fatalf("HOST_ORIGINAL_OWNER_PAIR: FAIL %v phase=%s cleanup=%+v", e, r.Phase, r.Cleanup)
	}
	traffic := child.traffic.Load()
	if _, e = variableRunWithCode(context.Background(), "execute-once", path, git, factory, variableClose, variableOwnedCodeInputs); e == nil || starts.Load() != 1 || traffic != child.traffic.Load() {
		t.Fatal("HOST_SPENT_NO_REPLAY: FAIL")
	}
	t.Log("HOST_ORIGINAL_OWNER_PAIR: PASS; one Stop/Shutdown/Operation; replay refused; no post-shutdown traffic")
}

// This single composite predicate is used for both success and late failure.
// Exact decoding also refuses omitted fields, duplicates and unknown fields.
func variableHostCompositeSuccess(raw []byte, paired bool) error {
	var receipt struct {
		ExitCode int
		Stopped  bool
		Joined   bool
	}
	if e := variableJSON(raw, &receipt); e != nil {
		return errors.New("invalid host exit receipt")
	}
	if !paired || !receipt.Joined || receipt.ExitCode != 0 || receipt.Stopped {
		return errors.New("host completion not joined-zero-not-stopped with verified pair")
	}
	return nil
}

func TestVariableCallerHostCompositeReceipt(t *testing.T) {
	for _, raw := range []string{"", "{", `{}`, `{"ExitCode":0,"Stopped":false,"Joined":false}`, `{"ExitCode":0,"Stopped":true,"Joined":true}`, `{"ExitCode":7,"Stopped":false,"Joined":true}`, `{"ExitCode":0,"Joined":true}`} {
		if variableHostCompositeSuccess([]byte(raw), true) == nil {
			t.Fatalf("HOST_STRICT_EXIT: FAIL admitted %s", raw)
		}
	}
	if variableHostCompositeSuccess([]byte(`{"ExitCode":0,"Stopped":false,"Joined":true}`), false) == nil {
		t.Fatal("HOST_STRICT_EXIT: FAIL no pair")
	}
	t.Log("HOST_STRICT_EXIT: PASS")
}

func TestVariableCallerHandoffOwnedProcesses(t *testing.T) {
	python, e := exec.LookPath("python3")
	if e != nil {
		t.Fatal(e)
	}
	repo, e := filepath.Abs("../..")
	if e != nil {
		t.Fatal(e)
	}
	helper := filepath.Join(repo, "scripts/private-variable-v2-host-handoff.py")
	emulator := filepath.Join(repo, "scripts/tests/private_variable_v2_owned_host.py")
	for _, fixture := range []struct{ batches, exit int }{{1, 0}, {3, 0}, {1, 7}} {
		n, exit := fixture.batches, fixture.exit
		t.Run(fmt.Sprintf("batches%d-exit%d", n, exit), func(t *testing.T) {
			c, path, _ := variableAutomaticConfig(t, n)
			c.Automation.TimeoutMS = 10000
			c.Automation.OverallTimeoutMS = 25000
			// Exact caller assertion is recomputed after changing finite fixture timing.
			c.Automation.ApprovalSHA256 = prep.AutomaticHandoffApprovalPrivateV2(*c.Automation)
			variableSave(t, path, c)
			base := filepath.Dir(path)
			control := filepath.Join(base, "control")
			payloads := filepath.Join(base, "payloads")
			for _, p := range []string{control, payloads} {
				if e := os.Mkdir(p, 0700); e != nil {
					t.Fatal(e)
				}
			}
			binary, e := os.Executable()
			if e != nil {
				t.Fatal(e)
			}
			// Test-only reviewed worker fixture: production helper still supplies ONLY
			// the exact existing entry flags. This fixture routes them to the owned seam.
			worker := filepath.Join(base, "worker")
			script := []byte("#!/bin/sh\n'" + binary + "' '-test.run=^TestVariableCallerHostWorker$' -test.count=1 -test.v\ncode=$?\nif [ $code -ne 0 ]; then exit $code; fi\nexit " + strconv.Itoa(exit) + "\n")
			if e = os.WriteFile(worker, script, 0700); e != nil {
				t.Fatal(e)
			}
			cfg, _ := os.ReadFile(path)
			ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
			defer cancel()
			host := exec.CommandContext(ctx, python, emulator, helper, c.Automation.Directory, payloads)
			host.Env = []string{"PATH=/usr/bin:/bin"}
			var hostOut bytes.Buffer
			host.Stdout = &hostOut
			host.Stderr = &hostOut
			if e = host.Start(); e != nil {
				t.Fatal(e)
			}
			defer func() {
				if host.ProcessState == nil {
					host.Process.Kill()
					host.Wait()
				}
			}()
			launch := exec.CommandContext(ctx, python, helper, "launch", "--config", path, "--config-sha256", pilot.Digest(cfg), "--worker", worker, "--worker-sha256", pilot.Digest(script), "--control", control)
			out, e := launch.CombinedOutput()
			if e != nil {
				t.Fatalf("host launch: %v %s", e, out)
			}
			if e = host.Wait(); e != nil {
				t.Fatalf("owned host: %v %s", e, hostOut.String())
			}
			for {
				if _, e = os.Stat(filepath.Join(control, "exit.json")); e == nil {
					break
				}
				select {
				case <-ctx.Done():
					t.Fatal("supervisor not joined")
				case <-time.After(10 * time.Millisecond):
				}
			}
			exitRaw, e := os.ReadFile(filepath.Join(control, "exit.json"))
			if e != nil {
				t.Fatal(e)
			}
			output, e := os.ReadFile(filepath.Join(control, "worker-output.txt"))
			if e != nil || !bytes.Contains(output, []byte("HOST_ORIGINAL_OWNER_PAIR: PASS")) {
				t.Fatalf("original worker failed: %s %v", output, e)
			}
			reportRaw, e := os.ReadFile(c.ReportPath)
			if e != nil {
				t.Fatal(e)
			}
			var report variableCallerReport
			if e = json.Unmarshal(reportRaw, &report); e != nil {
				t.Fatal(e)
			}
			if report.Outcome == nil || report.Inventory == nil || len(report.Inventory.Candidates) != n {
				t.Fatal("HOST_PAIRED_READBACK: FAIL")
			}
			var first []byte
			for _, p := range c.PostOutputs {
				root, e := publication.OpenContentRetentionRoot(context.Background(), p)
				if e != nil {
					t.Fatal(e)
				}
				raw, e := root.ReadPostPreparationPrivateV2(context.Background(), report.Outcome.EnvelopeSHA256, int64(report.Outcome.ByteLength))
				ce := root.Close()
				if e != nil || ce != nil {
					t.Fatal(e, ce)
				}
				if first == nil {
					first = raw
				} else if !bytes.Equal(first, raw) {
					t.Fatal("HOST_PAIRED_READBACK: FAIL")
				}
			}
			if !bytes.Contains(first, []byte(`"HumanInventory"`)) || !bytes.Contains(first, []byte(`"Inventory"`)) {
				t.Fatal("HOST_PAIRED_READBACK: FAIL")
			}
			// All actual native paired bytes and the original-worker PASS marker
			// have been checked above, even in the late-nonzero vector.
			completionErr := variableHostCompositeSuccess(exitRaw, true)
			if exit != 0 {
				if completionErr == nil {
					t.Fatal("HOST_LATE_EXIT_REJECTED: FAIL paired success masked nonzero exit")
				}
				t.Logf("HOST_LATE_EXIT_REJECTED: PASS paired bytes and original PASS preserved; exit=%s", exitRaw)
			} else {
				if completionErr != nil {
					t.Fatalf("HOST_COMPOSITE_SUCCESS: FAIL %v receipt=%s", completionErr, exitRaw)
				}
				t.Logf("HOST_COMPOSITE_SUCCESS: PASS batches=%d receipt=%s", n, exitRaw)
			}
			t.Logf("HOST_PAIRED_READBACK: PASS batches=%d original-worker+host-emulator+supervisor; %s", n, hostOut.String())
		})
	}
}
