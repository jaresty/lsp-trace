//go:build darwin || linux

package censuslineagepilot_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	pilot "lsp-trace/internal/censuslineagepilot"
	prep "lsp-trace/internal/privatedescribeprep"
	"lsp-trace/sessionruntime"
)

// An actual worker process, with only owned preflight dependencies. It invokes
// the identical pinned Go entry used by TestPrivateVariableScopeRealV2.
func TestVariableCallerHostConfigPinWorker(t *testing.T) {
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
	entries := 0
	factory := func() (*sessionruntime.Manager, error) {
		entries++
		return nil, errors.New("owned factory must remain unreached")
	}
	_, e = variableHostRunWithCode(context.Background(), "execute-once", path, os.Getenv("LSP_TRACE_PRIVATE_V2_CALLER_CONFIG_SHA256"), git, factory, variableClose, variableOwnedCodeInputs)
	_, markerErr := os.Stat(c.AttemptMarkerPath)
	if e == nil || e.Error() != "supervisor config pin mismatch or missing" || entries != 0 || !os.IsNotExist(markerErr) {
		t.Fatalf("HOST_CONFIG_PIN: FAIL starts=%d markerAbsent=%t error=%v", entries, os.IsNotExist(markerErr), e)
	}
	variableEmpty(t, c)
	t.Log("HOST_CONFIG_PIN: PASS valid substituted config refused before claim/start; starts=0 marker=0")
}

func TestVariableCallerHostConfigSwap(t *testing.T) {
	for _, schema := range []string{"v4", "v2"} {
		t.Run(schema, func(t *testing.T) { variableHostConfigSwap(t, schema) })
	}
}

func variableHostConfigSwap(t *testing.T, schema string) {
	t.Helper()
	python, e := exec.LookPath("python3")
	if e != nil {
		t.Fatal(e)
	}
	repo, _ := filepath.Abs("../..")
	helper := filepath.Join(repo, "scripts/private-variable-v2-host-handoff.py")
	c, path, _ := variableAutomaticConfig(t, 1)
	raw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	// Both substitutions are canonical VALID configs. A supplied trusted pin
	// must not let the consumed schema choose whether its comparison applies.
	if schema == "v2" {
		c.Schema = variableCallerSchema
		c.Purpose = variableCallerPurpose
		c.Automation = nil
	} else {
		c.Automation.Invocation += "-substituted"
		c.Automation.ApprovalSHA256 = prep.AutomaticHandoffApprovalPrivateV2(*c.Automation)
	}
	replacement := filepath.Join(filepath.Dir(path), "replacement.json")
	variableSave(t, replacement, c)
	// Prove the replacement passes the actual owned preflight before the swap.
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
	if _, e = variablePreflightWithCode(context.Background(), replacement, git, variableOwnedCodeInputs); e != nil {
		t.Fatalf("replacement not valid: %v", e)
	}
	control := filepath.Join(filepath.Dir(path), "control")
	if e = os.Mkdir(control, 0700); e != nil {
		t.Fatal(e)
	}
	binary, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	worker := filepath.Join(filepath.Dir(path), "worker")
	script := []byte("#!/bin/sh\nexec '" + binary + "' '-test.run=^TestVariableCallerHostConfigPinWorker$' -test.count=1 -test.v\n")
	if e = os.WriteFile(worker, script, 0700); e != nil {
		t.Fatal(e)
	}
	// Only replace the pathname immediately before actual Popen. The real child
	// receives the supervisor's original pin and reads the valid replacement.
	runner := `import importlib.util,sys,os
from types import SimpleNamespace
spec=importlib.util.spec_from_file_location('handoff',sys.argv[1]);h=importlib.util.module_from_spec(spec);spec.loader.exec_module(h)
a=SimpleNamespace(config=sys.argv[2],config_sha256=sys.argv[3],worker=sys.argv[4],worker_sha256=sys.argv[5],control=sys.argv[6])
actual=h.subprocess.Popen
def swap(*args,**kwargs):
 os.replace(sys.argv[7],a.config)
 return actual(*args,**kwargs)
h.subprocess.Popen=swap
h.supervise(a)
`
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, python, "-c", runner, helper, path, pilot.Digest(raw), worker, pilot.Digest(script), control, replacement)
	cmd.Env = []string{"PATH=/usr/bin:/bin"}
	if out, e := cmd.CombinedOutput(); e != nil {
		t.Fatalf("supervisor: %v %s", e, out)
	}
	output, e := os.ReadFile(filepath.Join(control, "worker-output.txt"))
	if e != nil {
		t.Fatal(e)
	}
	receipt, e := os.ReadFile(filepath.Join(control, "exit.json"))
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(string(output), "HOST_CONFIG_PIN: PASS valid substituted config refused before claim/start; starts=0 marker=0") || variableHostCompositeSuccess(receipt, true) != nil {
		t.Fatalf("HOST_CONFIG_SWAP: FAIL replacement=%s child did not observe pinned refusal; output=%s receipt=%s", schema, output, receipt)
	}
	t.Logf("HOST_CONFIG_SWAP: PASS replacement=%s actual supervisor/Popen + valid replacement + shared Go consumed-byte check; starts=0 marker=0", schema)
}

func TestVariableCallerHostLegacyPinModes(t *testing.T) {
	for _, mode := range []string{"preflight", "execute-once"} {
		for _, kind := range []string{"empty", "wrong", "matching", "nil"} {
			t.Run(mode+"/"+kind, func(t *testing.T) {
				c, path, git := variableAutomaticConfig(t, 1)
				c.Schema, c.Purpose, c.Automation = variableCallerSchema, variableCallerPurpose, nil
				variableSave(t, path, c)
				raw, e := os.ReadFile(path)
				if e != nil {
					t.Fatal(e)
				}
				pin := ""
				if kind == "wrong" {
					pin = pilot.Digest([]byte("different"))
				}
				if kind == "matching" {
					pin = pilot.Digest(raw)
				}
				entries := 0
				boundary := errors.New("owned legacy factory boundary")
				factory := func() (*sessionruntime.Manager, error) { entries++; return nil, boundary }
				if kind == "nil" {
					// Existing dependency-injected nil seam, not a new dispatch path.
					_, e = variableRunWithAutomaticPinned(context.Background(), mode, path, git, factory, variableClose, variableOwnedCodeInputs, variableRealAutomatic, nil)
				} else {
					_, e = variableHostRunWithCode(context.Background(), mode, path, pin, git, factory, variableClose, variableOwnedCodeInputs)
				}
				_, markerErr := os.Stat(c.AttemptMarkerPath)
				if mode == "preflight" {
					if e != nil || entries != 0 || !os.IsNotExist(markerErr) {
						t.Fatal("HOST_LEGACY_PIN_MODES: FAIL readonly preflight", e, entries, markerErr)
					}
				} else if kind == "wrong" {
					if e == nil || e.Error() != "supervisor config pin mismatch or missing" || entries != 0 || !os.IsNotExist(markerErr) {
						t.Fatal("HOST_LEGACY_PIN_MODES: FAIL pin refusal", e, entries, markerErr)
					}
				} else if !errors.Is(e, boundary) || entries != 1 || markerErr != nil {
					t.Fatal("HOST_LEGACY_PIN_MODES: FAIL compatibility boundary", e, entries, markerErr)
				}
				t.Log("HOST_LEGACY_PIN_MODES: PASS", mode, kind)
			})
		}
	}
}

func TestVariableCallerHostPinModes(t *testing.T) {
	for _, mode := range []string{"preflight", "execute-once"} {
		for _, kind := range []string{"missing", "wrong", "matching"} {
			t.Run(mode+"/"+kind, func(t *testing.T) {
				c, path, git := variableAutomaticConfig(t, 1)
				raw, _ := os.ReadFile(path)
				pin := ""
				if kind == "wrong" {
					pin = pilot.Digest([]byte("different"))
				}
				if kind == "matching" {
					pin = pilot.Digest(raw)
				}
				entries := 0
				factory := func() (*sessionruntime.Manager, error) { entries++; return nil, errors.New("owned factory boundary") }
				_, e := variableHostRunWithCode(context.Background(), mode, path, pin, git, factory, variableClose, variableOwnedCodeInputs)
				_, markerErr := os.Stat(c.AttemptMarkerPath)
				if mode == "preflight" {
					if e != nil || entries != 0 || !os.IsNotExist(markerErr) {
						t.Fatal("readonly preflight changed", e, entries)
					}
				} else if kind != "matching" {
					if e == nil || !strings.Contains(e.Error(), "supervisor config pin") || entries != 0 || !os.IsNotExist(markerErr) {
						t.Fatal("HOST_PIN_MODES: FAIL", e, entries)
					}
				} else if entries != 1 || os.IsNotExist(markerErr) {
					t.Fatal("matching pin did not reach owned boundary", e, entries)
				}
				t.Log("HOST_PIN_MODES: PASS", mode, kind)
			})
		}
	}
}
