//go:build darwin || linux

package main

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

func censusCohortFixtureConfig(mailbox, scope string) prep.CohortHandoffConfigPrivateV2 {
	c := prep.CohortHandoffConfigPrivateV2{Enabled: true, Mode: prep.CohortModePrivateV2, Protocol: prep.CohortHandoffProtocolPrivateV2, MaxItems: 6, Directory: mailbox, Invocation: "owned-cohort-cli", InterpreterRole: "owned-interpreter", ReviewerRole: "owned-reviewer", ScopeSHA256: scope, TimeoutMS: 5000, OverallTimeoutMS: 20000, PollMS: 2, Limits: prep.AutomaticLimitsPrivateV2{MaxCorrections: 1, MaxCalls: 16, MaxRequestBytes: 1 << 20, MaxResponseBytes: 64 << 10, MaxTotalBytes: 8 << 20, MaxOutputBytes: 4 << 20}}
	c.ApprovalSHA256 = prep.CohortHandoffApprovalPrivateV2(c)
	return c
}
func TestCensusCohortApprovalAndSourceIdentityPrivate(t *testing.T) {
	c := inventoryConfigPrivate()
	legacy, _ := json.Marshal(c)
	if bytes.Contains(legacy, []byte("CohortAutomation")) {
		t.Fatal("ASSERT_COHORT_DEFAULT_OMITTED: FAIL")
	}
	baseline := censusInventoryScopeBytesPrivate(censusCLIOptions{}, c, nil, nil, nil, nil)
	cfg := censusCohortFixtureConfig("/owned/mailbox", pilot.Digest([]byte("scope")))
	c.CohortAutomation = &cfg
	if c.validateAutomation() != nil || !c.hasAutomation() {
		t.Fatal("ASSERT_COHORT_MODE_APPROVAL: FAIL")
	}
	if !bytes.Equal(baseline, censusInventoryScopeBytesPrivate(censusCLIOptions{}, c, nil, nil, nil, nil)) {
		t.Fatal("ASSERT_COHORT_SOURCE_ONLY_IDENTITY: FAIL")
	}
	for _, kind := range []string{"mode", "protocol", "cap", "scope", "role", "calls", "timeout", "approval"} {
		changed := cfg
		switch kind {
		case "mode":
			changed.Mode = "legacy"
		case "protocol":
			changed.Protocol = prep.AutomaticHandoffProtocolPrivateV2
		case "cap":
			changed.MaxItems++
		case "scope":
			changed.ScopeSHA256 = pilot.Digest([]byte("other"))
		case "role":
			changed.ReviewerRole = "other"
		case "calls":
			changed.Limits.MaxCalls++
		case "timeout":
			changed.TimeoutMS++
		case "approval":
			changed.ApprovalSHA256 = prep.AutomaticHandoffApprovalPrivateV2(prep.AutomaticHandoffConfigPrivateV2{})
		}
		if prep.ValidateCohortHandoffPrivateV2(changed) == nil {
			t.Fatalf("ASSERT_COHORT_MODE_APPROVAL: FAIL %s", kind)
		}
	}
	c.Automation = &prep.AutomaticHandoffConfigPrivateV2{}
	if c.validateAutomation() == nil {
		t.Fatal("ASSERT_COHORT_EXCLUSIVE: FAIL")
	}
	t.Log("ASSERT_COHORT_DEFAULT_OMITTED: PASS")
	t.Log("ASSERT_COHORT_SOURCE_ONLY_IDENTITY: PASS")
	t.Log("ASSERT_COHORT_MODE_APPROVAL: PASS")
	t.Log("ASSERT_COHORT_EXCLUSIVE: PASS")
}
func cohortHostEmulatePrivate(base, mailbox, helper, python string) error {
	deadline := time.Now().Add(25 * time.Second)
	wait := func(path string) error {
		for time.Now().Before(deadline) {
			if _, e := os.Stat(path); e == nil {
				return nil
			}
			time.Sleep(2 * time.Millisecond)
		}
		return errors.New("owned cohort wait bound")
	}
	bindingPath := mailbox + ".census-export.json"
	if e := wait(bindingPath); e != nil {
		return e
	}
	binding, e := os.ReadFile(bindingPath)
	if e != nil {
		return e
	}
	for sequence := 1; sequence <= 4; sequence++ {
		if e := wait(filepath.Join(mailbox, fmt.Sprintf("request-%06d.json", sequence))); e != nil {
			return e
		}
		dest := filepath.Join(base, "exports", fmt.Sprint(sequence))
		if raw, e := exec.Command(python, helper, "read", "--mailbox", mailbox, "--sequence", fmt.Sprint(sequence), "--export-dir", dest, "--config", bindingPath, "--config-sha256", pilot.Digest(binding), "--control", filepath.Join(base, "control")).CombinedOutput(); e != nil {
			return fmt.Errorf("controlled cohort export: %v %s", e, raw)
		}
		manifestRaw, e := os.ReadFile(filepath.Join(dest, "manifest.json"))
		if e != nil {
			return e
		}
		var manifest struct {
			Schema string
			Parts  []struct {
				Name, SHA256           string
				Start, End, ByteLength int
			}
		}
		if json.Unmarshal(manifestRaw, &manifest) != nil || manifest.Schema != "private-v2-cohort-host-export.v1" {
			return errors.New("cohort manifest family")
		}
		var raw []byte
		for _, part := range manifest.Parts {
			p, e := os.ReadFile(filepath.Join(dest, part.Name))
			if e != nil || pilot.Digest(p) != part.SHA256 || part.Start != len(raw) || part.End != len(raw)+len(p) || part.ByteLength != len(p) {
				return errors.New("cohort ordered export")
			}
			raw = append(raw, p...)
		}
		exact, e := os.ReadFile(filepath.Join(dest, "frame.json"))
		if e != nil || !bytes.Equal(exact, raw) {
			return errors.New("cohort exact physical export")
		}
		var frame struct {
			Binding prep.CohortHandoffBindingPrivateV2
			Request prep.CohortRequestPrivateV2
		}
		if json.Unmarshal(raw, &frame) != nil {
			return errors.New("cohort frame parse")
		}
		req := frame.Request
		var payload []byte
		fixture := inventorySemanticFixturePrivate{id: "owned-cohort-host"}
		if req.Navigation != nil {
			payload, e = fixture.Exchange(context.Background(), *req.Navigation, 64<<10)
		} else {
			out := prep.CohortResponsePrivateV2{Protocol: req.Protocol, Stage: req.Stage, PreparationSHA256: req.PreparationSHA256}
			for j, item := range req.Items {
				response := prep.CohortResponseItemPrivateV2{Binding: req.ItemBindings[j]}
				if req.Stage == "INTERPRET_COHORT" {
					p, err := fixture.Exchange(context.Background(), prep.AutomaticRequestPrivateV2{Stage: "INTERPRET", Bodies: item.Bodies}, 64<<10)
					if err != nil {
						return err
					}
					var v prep.AutomaticInterpretationPrivateV2
					if json.Unmarshal(p, &v) != nil {
						return errors.New("cohort typed interpretation")
					}
					response.Interpretation = &v
				} else {
					response.Review = &prep.AutomaticReviewPrivateV2{Digest: item.InterpretationDigest, Verdict: "APPROVE", Findings: "Controlled own-body cohort review; runtime unknown"}
				}
				out.Items = append(out.Items, response)
			}
			payload, e = json.Marshal(out)
		}
		if e != nil {
			return e
		}
		path := filepath.Join(base, "payloads", fmt.Sprintf("%d.json", sequence))
		if e = os.WriteFile(path, payload, 0600); e != nil {
			return e
		}
		if raw, e := exec.Command(python, helper, "respond", "--mailbox", mailbox, "--sequence", fmt.Sprint(sequence), "--payload", path).CombinedOutput(); e != nil {
			return fmt.Errorf("controlled cohort respond: %v %s", e, raw)
		}
	}
	return nil
}
func TestCensusCohortActualCLIProcessPrivate(t *testing.T) {
	python, e := exec.LookPath("python3")
	if e != nil {
		t.Fatal(e)
	}
	base, _, config, args := inventoryFixturePrivate(t)
	binary := filepath.Join(base, "lsp-trace")
	build := exec.Command("go", "build", "-o", binary, ".")
	if out, e := build.CombinedOutput(); e != nil {
		t.Fatalf("controlled CLI setup: %v %s", e, out)
	}
	for j, arg := range args {
		if arg == "--server" {
			args[j+1] = os.Args[0]
		}
	}
	args = append(args, "--server-arg", "-test.run=^TestInventoryOwnedLSPProcessPrivate$")
	run := func() (censusInventoryReportPrivate, int) {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, binary, append([]string{"census"}, args...)...)
		cmd.Env = append(os.Environ(), "LSP_TRACE_OWNED_TWO_ORIGIN_SERVER=1")
		raw, e := cmd.Output()
		if e != nil {
			if x, ok := e.(*exec.ExitError); ok {
				t.Logf("controlled stderr: %s", x.Stderr)
			}
		}
		var meta struct{ AttemptDirectory string }
		if json.Unmarshal(raw, &meta) != nil || meta.AttemptDirectory == "" {
			t.Fatalf("controlled CLI metadata: %v %s", e, raw)
		}
		report, e := os.ReadFile(filepath.Join(meta.AttemptDirectory, "report.json"))
		if e != nil {
			t.Fatal(e)
		}
		var r censusInventoryReportPrivate
		if json.Unmarshal(report, &r) != nil {
			t.Fatal("controlled report")
		}
		if ctx.Err() != nil || cmd.ProcessState == nil {
			t.Fatal("controlled child failed to join")
		}
		return r, cmd.ProcessState.ExitCode()
	}
	first, code := run()
	if code != 0 {
		t.Fatal("controlled source-scope setup", code, first.Failure)
	}
	for _, name := range []string{"mailbox", "exports", "control", "payloads"} {
		if e = os.Mkdir(filepath.Join(base, name), 0700); e != nil {
			t.Fatal(e)
		}
	}
	c := inventoryConfigPrivate()
	cfg := censusCohortFixtureConfig(filepath.Join(base, "mailbox"), first.ScopeSHA256)
	c.CohortAutomation = &cfg
	saveInventoryConfigPrivate(t, config, c)
	helper, e := filepath.Abs(filepath.Join("..", "..", "scripts", "private-variable-v2-host-handoff.py"))
	if e != nil {
		t.Fatal(e)
	}
	done := make(chan error, 1)
	go func() { done <- cohortHostEmulatePrivate(base, cfg.Directory, helper, python) }()
	r, code := run()
	hostErr := <-done
	if hostErr != nil || code != 0 || r.Inventory == nil || r.Inventory.Calls != 4 || r.Inventory.Protocol != prep.CohortProtocolPrivateV2 || r.Outcome == nil || r.Outcome.Status != "VERIFIED_PAIR_PRIVATE_WITH_LIMITS" || r.HostExportBindingSHA256 == "" || !r.CleanupVerified || r.ShutdownCalls != 1 {
		t.Fatalf("ASSERT_COHORT_ACTUAL_CLI_PAIR: FAIL host=%v exit=%d report=%+v", hostErr, code, r)
	}
	t.Logf("ASSERT_COHORT_ACTUAL_CLI_PAIR: PASS physical_exchanges=%d candidates=%d shutdown=%d joined=true", r.Inventory.Calls, len(r.Inventory.Candidates), r.ShutdownCalls)
}
