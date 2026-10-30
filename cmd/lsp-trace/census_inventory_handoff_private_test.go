//go:build darwin || linux

package main

// Owned host emulator only: ordinary helper subprocesses read/export/respond;
// the fixture authors typed JSON, not an LLM or production inference fallback.
import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	pilot "lsp-trace/internal/censuslineagepilot"
	prep "lsp-trace/internal/privatedescribeprep"
)

func inventoryHostEmulatePrivate(base, mailbox, helper, python string, exchanges ...int) error {
	count := 4
	if len(exchanges) == 1 {
		count = exchanges[0]
	}
	bindingPath := mailbox + ".census-export.json"
	deadline := time.Now().Add(20 * time.Second)
	wait := func(path string) error {
		for time.Now().Before(deadline) {
			if _, e := os.Stat(path); e == nil {
				return nil
			}
			time.Sleep(2 * time.Millisecond)
		}
		return errors.New("owned file wait bound")
	}
	if e := wait(bindingPath); e != nil {
		return e
	}
	binding, e := os.ReadFile(bindingPath)
	if e != nil {
		return e
	}
	for sequence := 1; sequence <= count; sequence++ {
		if e := wait(filepath.Join(mailbox, fmt.Sprintf("request-%06d.json", sequence))); e != nil {
			return e
		}
		destination := filepath.Join(base, "exports", fmt.Sprint(sequence))
		command := exec.Command(python, helper, "read", "--mailbox", mailbox, "--sequence", fmt.Sprint(sequence), "--export-dir", destination, "--config", bindingPath, "--config-sha256", pilot.Digest(binding), "--control", filepath.Join(base, "control"))
		if _, e := command.CombinedOutput(); e != nil {
			return errors.New("owned actual helper export refused")
		}
		manifestRaw, e := os.ReadFile(filepath.Join(destination, "manifest.json"))
		if e != nil {
			return e
		}
		var manifest struct {
			Frame struct {
				SHA256     string
				ByteLength int
			}
			Parts []struct {
				Name, SHA256           string
				Start, End, ByteLength int
			}
		}
		if json.Unmarshal(manifestRaw, &manifest) != nil {
			return errors.New("owned manifest parse")
		}
		var frameBytes []byte
		for _, part := range manifest.Parts {
			raw, e := os.ReadFile(filepath.Join(destination, part.Name))
			if e != nil || pilot.Digest(raw) != part.SHA256 || part.ByteLength != len(raw) || part.Start != len(frameBytes) || part.End != len(frameBytes)+len(raw) {
				return errors.New("owned incomplete ordered export")
			}
			frameBytes = append(frameBytes, raw...)
		}
		exact, e := os.ReadFile(filepath.Join(destination, "frame.json"))
		if e != nil || !bytes.Equal(exact, frameBytes) || pilot.Digest(frameBytes) != manifest.Frame.SHA256 || len(frameBytes) != manifest.Frame.ByteLength {
			return errors.New("owned frame byte mismatch")
		}
		var frame struct {
			Request prep.AutomaticRequestPrivateV2
		}
		if json.Unmarshal(frameBytes, &frame) != nil {
			return errors.New("owned request parse")
		}
		payload, e := (inventorySemanticFixturePrivate{id: "owned-host-emulator"}).Exchange(context.Background(), frame.Request, 64<<10)
		if e != nil {
			return e
		}
		payloadPath := filepath.Join(base, "payloads", fmt.Sprintf("%d.json", sequence))
		if e = os.WriteFile(payloadPath, payload, 0600); e != nil {
			return e
		}
		if _, e = exec.Command(python, helper, "respond", "--mailbox", mailbox, "--sequence", fmt.Sprint(sequence), "--payload", payloadPath).CombinedOutput(); e != nil {
			return errors.New("owned actual helper respond refused")
		}
	}
	return nil
}
func TestCensusPrivateInventoryActualFileHandoff(t *testing.T) {
	python, e := exec.LookPath("python3")
	if e != nil {
		t.Skip("existing Python host helper unavailable")
	}
	base, _, config, args := inventoryFixturePrivate(t)
	deps, _ := inventoryDepsPrivate(t)
	first, code := runInventoryCLIPrivate(t, args, deps)
	if code != 0 {
		t.Fatal("owned preparation setup failed")
	}
	mailbox := filepath.Join(base, "mailbox")
	for _, name := range []string{"mailbox", "exports", "control", "payloads"} {
		if e = os.Mkdir(filepath.Join(base, name), 0700); e != nil {
			t.Fatal(e)
		}
	}
	c := inventoryConfigPrivate()
	c.Automation = &prep.AutomaticHandoffConfigPrivateV2{Enabled: true, Directory: mailbox, Invocation: "owned-actual-helper", InterpreterRole: "owned-interpreter", ReviewerRole: "owned-reviewer", ScopeSHA256: first.ScopeSHA256, TimeoutMS: 3000, OverallTimeoutMS: 15000, PollMS: 2, Limits: prep.AutomaticLimitsPrivateV2{MaxCorrections: 0, MaxCalls: 8, MaxRequestBytes: 1 << 20, MaxResponseBytes: 64 << 10, MaxTotalBytes: 8 << 20, MaxOutputBytes: 4 << 20}}
	c.Automation.ApprovalSHA256 = prep.AutomaticHandoffApprovalPrivateV2(*c.Automation)
	saveInventoryConfigPrivate(t, config, c)
	deps, _ = inventoryDepsPrivate(t)
	// No injected semantic backend: the production file adapter keeps its original
	// opaque preparation owner alive while the separate host helper exchanges.
	var out, errout bytes.Buffer
	done := make(chan int, 1)
	go func() { done <- runCensusWithDependencies(args, &out, &errout, deps) }()
	helper, e := filepath.Abs(filepath.Join("..", "..", "scripts", "private-variable-v2-host-handoff.py"))
	if e != nil {
		t.Fatal(e)
	}
	hostErr := inventoryHostEmulatePrivate(base, mailbox, helper, python)
	select {
	case code = <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("owned CLI did not join within declared adapter bounds")
	}
	var meta struct{ AttemptDirectory string }
	_ = json.Unmarshal(out.Bytes(), &meta)
	var r censusInventoryReportPrivate
	raw, _ := os.ReadFile(filepath.Join(meta.AttemptDirectory, "report.json"))
	_ = json.Unmarshal(raw, &r)
	t.Logf("procedure: actual Census CLI -> production NewAutomaticHandoffPrivateV2 -> Python read/export exact ordered parts -> owned typed payload -> Python respond -> native paired Inventory/HumanInventory; host-error=%v; attempt=%s", hostErr, meta.AttemptDirectory)
	assertInventoryPrivate(t, "ASSERT_CENSUS_FILE_HELPER_CONNECTED", hostErr == nil && code == 0 && r.Outcome != nil && r.Outcome.Status == "VERIFIED_PAIR_PRIVATE_WITH_LIMITS" && r.Inventory != nil && len(r.Inventory.Candidates) == 1 && len(r.Inventory.Navigation) == 1 && r.ShutdownCalls == 1 && r.CleanupVerified && r.HostExportBindingSHA256 != "")
}
