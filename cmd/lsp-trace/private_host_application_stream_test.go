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
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	pilot "lsp-trace/internal/censuslineagepilot"
	product "lsp-trace/internal/privatehostproductv3"
)

func writeApplicationFixture(t *testing.T, r hostApplicationRequest, path string) {
	t.Helper()
	policySHA := hostApplicationUnitPolicyDigest(r)
	r.Configuration.UnitPolicySHA256 = policySHA
	r.Configuration.ProducerConfigSHA256 = policySHA
	r.Configuration.ApprovalSHA256 = product.ApplicationConfigDigest(r.Configuration)
	r.ApprovalSHA256 = hostApplicationDigest(r)
	raw, _ := json.Marshal(r)
	if e := os.WriteFile(path, raw, 0600); e != nil {
		t.Fatal(e)
	}
}
func buildApplicationTool(t *testing.T, base string) string {
	t.Helper()
	tool := filepath.Join(base, "tool")
	args := []string{"build", "-o", tool}
	if applicationFixtureRace {
		args = append(args, "-race")
	}
	if os.Getenv("TEST_FIXTURE_WINDOW_BARRIER") == "1" {
		// Test-only phase notification on an inherited pipe. No polling, new
		// deadline, or production hook: the overlaid body only writes one byte
		// after the real bound prepaid window has been read successfully.
		source, e := os.ReadFile("private_host_application.go")
		if e != nil {
			t.Fatal(e)
		}
		needle := "\t\tunitCtx, unitCancel := context.WithDeadline(ctx, unitDeadline)"
		if bytes.Count(source, []byte(needle)) != 1 {
			t.Fatal("window barrier seam changed")
		}
		insert := "\t\tif os.Getenv(\"TEST_FIXTURE_WINDOW_BARRIER\") == \"1\" { f := os.NewFile(3, \"fixture-prepaid-window\"); if f == nil { return report(\"FIXTURE_BARRIER_FAILED\", 1) }; _, err := f.Write([]byte{1}); _ = f.Close(); if err != nil { return report(\"FIXTURE_BARRIER_FAILED\", 1) } }\n"
		overlaid := filepath.Join(base, "application-window-overlay.go")
		if e := os.WriteFile(overlaid, bytes.Replace(source, []byte(needle), []byte(insert+needle), 1), 0600); e != nil {
			t.Fatal(e)
		}
		original, e := filepath.Abs("private_host_application.go")
		if e != nil {
			t.Fatal(e)
		}
		raw, _ := json.Marshal(struct{ Replace map[string]string }{map[string]string{original: overlaid}})
		overlay := filepath.Join(base, "window-overlay.json")
		if e := os.WriteFile(overlay, raw, 0600); e != nil {
			t.Fatal(e)
		}
		args = append(args, "-overlay", overlay)
	}
	if os.Getenv("TEST_FIXTURE_DISCOVERY_DELAY") == "1" {
		// Controlled resistance AFTER each real 3s directory-page prepayment.
		// Four deterministic 2.1s reservation-bound barriers exceed the shared
		// eager 8s startup pool without any free post-window waiting facility.
		source, er := os.ReadFile("private_host_application_literal.go")
		if er != nil {
			t.Fatal(er)
		}
		needle := "\t\t\tend := offset + 32"
		if bytes.Count(source, []byte(needle)) != 1 {
			t.Fatal("discovery delay seam changed")
		}
		insert := `
			if os.Getenv("TEST_FIXTURE_DISCOVERY_DELAY") == "1" {
				f, er := os.OpenFile(os.Getenv("TEST_FIXTURE_DISCOVERY_MARKER"), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
				if er != nil { return er }; _, er = f.Write([]byte{1}); ce := f.Close(); if er != nil || ce != nil { return product.ErrInvalid }
				timer := time.NewTimer(2100*time.Millisecond)
				select { case <-timer.C: case <-ctx.Done(): timer.Stop(); return ctx.Err() }
			}
`
		original, er := filepath.Abs("private_host_application_literal.go")
		if er != nil {
			t.Fatal(er)
		}
		replacement := filepath.Join(base, "literal-delay.go")
		if er = os.WriteFile(replacement, bytes.Replace(source, []byte(needle), []byte(insert+needle), 1), 0600); er != nil {
			t.Fatal(er)
		}
		raw, _ := json.Marshal(struct{ Replace map[string]string }{map[string]string{original: replacement}})
		overlay := filepath.Join(base, "literal-delay-overlay.json")
		if er = os.WriteFile(overlay, raw, 0600); er != nil {
			t.Fatal(er)
		}
		args = append(args, "-overlay", overlay)
	}
	args = append(args, ".")
	if b, e := exec.Command("go", args...).CombinedOutput(); e != nil {
		t.Fatalf("fixture build %v %s", e, b)
	}
	return tool
}

// Existing HOST tool contract adapter. It launches no Agents/models. Every wait,
// ready registration, response, terminal and observation uses the actual CLI.
func controlledApplicationRun(t *testing.T, tool, path string, extraEnv ...string) (applicationTerminal, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 240*time.Second)
	defer cancel()
	setup := exec.CommandContext(ctx, tool, "private-host-application", "setup", path)
	setupRaw, e := setup.CombinedOutput()
	if e != nil {
		t.Fatalf("setup %v %s", e, setupRaw)
	}
	var ticket struct {
		RunID     string
		RoleTasks []struct{ Role, Prompt, PromptSHA256 string }
	}
	if json.Unmarshal(setupRaw, &ticket) != nil || len(ticket.RoleTasks) != 2 {
		t.Fatal("invalid setup")
	}
	fixtureRequest, readErr := readHostApplication(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	applicationSHA256 := product.ApplicationConfigDigest(fixtureRequest.Configuration)
	activationGate := false
	hostWrapper := false
	hostBridge := false
	for _, marker := range extraEnv {
		if marker == "TEST_HOST_BRIDGE=1" {
			hostBridge = true
			hostWrapper = true
			activationGate = true
		}
		if marker == "TEST_HOST_WRAPPER=1" {
			hostWrapper = true
			activationGate = true
		}
		if marker == "TEST_ACTIVATION_GATE=1" {
			activationGate = true
		}
	}
	registerRoles := func() error {
		for _, role := range ticket.RoleTasks {
			if b, e := exec.CommandContext(ctx, tool, "private-host-application", "register", path, ticket.RunID, role.Role, "scripted-existing-host-"+role.Role, role.PromptSHA256).CombinedOutput(); e != nil {
				return fmt.Errorf("register %v %s", e, b)
			}
		}
		return nil
	}
	if !activationGate {
		if e := registerRoles(); e != nil {
			t.Fatal(e)
		}
	}
	crash := false
	for _, v := range extraEnv {
		if v == "TEST_CRASH_PHYSICAL_REPLY=1" {
			crash = true
		}
	}
	processReady := make(chan *os.Process, 1)
	var wg sync.WaitGroup
	rolesStart := make(chan struct{})
	barrierMode := os.Getenv("TEST_FIXTURE_WINDOW_BARRIER") == "1"
	var windowRead, windowWrite *os.File
	if barrierMode || activationGate {
		windowRead, windowWrite, e = os.Pipe()
		if e != nil {
			t.Fatal(e)
		}
		defer windowRead.Close()
		defer windowWrite.Close()
	} else {
		close(rolesStart)
	}
	var returns [2]product.CohortRoleReturn
	var terminalObserved [2]bool // disjoint role writers, joined before finalize
	var physical [2][3]int       // disjoint role writers; read only after wg.Wait
	for idx, role := range []string{"interpreter", "reviewer"} {
		wg.Add(1)
		go func(idx int, role string) {
			defer wg.Done()
			select {
			case <-rolesStart:
			case <-ctx.Done():
				return
			}
			var after uint64
			for {
				cmd := exec.CommandContext(ctx, tool, "private-host-application", "wait", path, ticket.RunID, role, strconv.FormatUint(after, 10), "60000")
				b, e := cmd.CombinedOutput()
				if e != nil {
					if ctx.Err() == nil {
						t.Errorf("SAFE_SCRIPTED_WAIT_ERROR schema=scripted-application-wait-error.v1 status=CLI_ERROR role=%s after=%d bytes=%d output_sha256=%s", role, after, len(b), pilot.Digest(b))
					}
					return
				}
				var event applicationHostEvent
				if json.Unmarshal(b, &event) != nil {
					t.Errorf("SAFE_SCRIPTED_WAIT_ERROR schema=scripted-application-wait-error.v1 status=INVALID_JSON role=%s after=%d bytes=%d output_sha256=%s", role, after, len(b), pilot.Digest(b))
					return
				}
				action, observation := applicationFixtureEventAction(event, applicationSHA256, ticket.RunID, role, after)
				t.Logf("SAFE_SCRIPTED_WAIT_OBSERVATION %s", observation)
				switch action {
				case "TERMINAL":
					terminalObserved[idx] = true
					returns[idx] = product.CohortRoleReturn{Kind: "SCRIPTED_CONTRACT", Role: role, LaunchID: "scripted-existing-host-" + role, TerminalSHA256: event.TerminalSHA256, Status: "RETURNED", ReturnSHA256: pilot.Digest([]byte("scripted result retrieved after completion")), Sequence: after}
					return
				case "NEEDS_HOST":
					// ReadCohortBundle independently validates the full retained bundle.
				default:
					t.Errorf("ASSERT_SCRIPTED_WAIT_STOP %s", observation)
					return
				}
				task, e := product.ReadCohortBundle(*event.Bundle)
				if e != nil {
					t.Error(e)
					return
				}
				response := product.CohortResponse{Protocol: product.CohortProtocol, RequestDigest: task.RequestDigest, Items: []product.CohortDecision{}}
				for _, item := range task.Items {
					response.Items = append(response.Items, product.CohortDecision{ItemID: item.ItemID, InputID: item.InputID, Target: item.Target, Attempt: item.Attempt, Predecessor: item.Predecessor, OwnBodySHA256: item.OwnBodySHA256, Revision: item.Revision, Disposition: "SUPPORTED", Label: "Controlled declaration", Explanation: "Scripted own-source description", Alternative: "Scaffolding", Falsifier: "Declaration differs", Quote: item.Body, Start: 0, End: len(item.Body)})
				}
				var stopped *os.Process
				if crash && role == "interpreter" {
					stopped = <-processReady
					if e = stopped.Signal(syscall.SIGSTOP); e != nil {
						t.Error(e)
						return
					}
				}
				raw, _ := json.Marshal(response)
				submit := exec.CommandContext(ctx, tool, "private-host-application", "respond", path, ticket.RunID, role, strconv.FormatUint(event.GlobalSequence, 10))
				submit.Stdin = bytes.NewReader(raw)
				if b, e = submit.CombinedOutput(); e != nil {
					t.Errorf("respond %v %s", e, b)
					return
				}
				physical[idx][0]++
				physical[idx][1] += event.Bundle.Bytes
				physical[idx][2] += len(raw)
				if stopped != nil {
					forged := product.Ready{Protocol: product.CohortProtocol, Role: role, LaunchID: "foreign-after-return", PromptSHA256: pilot.Digest([]byte(product.CohortRolePrompt(role)))}
					fb, _ := json.Marshal(forged)
					if e = os.WriteFile(filepath.Join(event.Bundle.Directory, "ready-"+role+".json"), fb, 0600); e != nil {
						t.Error(e)
					}
					_ = stopped.Kill()
					return
				}
				after = event.GlobalSequence
			}
		}(idx, role)
	}
	cmd := exec.CommandContext(ctx, tool, "private-host-application", "run", path, ticket.RunID)
	var out, errout bytes.Buffer
	cmd.Stdout = &out
	wrapperLog := ""
	bridgeControl := ""
	bridgePath := ""
	var bridgeOutput bytes.Buffer
	var lifecycleWrite *os.File
	if hostBridge {
		bridgePath, e = filepath.Abs("../../.pi/skills/private-host-streaming-application/scripts/activation-bridge.py")
		if e != nil {
			t.Fatal(e)
		}
		bridgeControl = filepath.Join(filepath.Dir(path), "host-bridge-"+ticket.RunID[7:])
		prepare := exec.CommandContext(ctx, "python3", bridgePath, "prepare", bridgeControl, tool, path, ticket.RunID, applicationSHA256)
		if raw, er := prepare.CombinedOutput(); er != nil {
			t.Fatalf("bridge prepare %v %s", er, raw)
		}
		wrapperLog = filepath.Join(bridgeControl, "wrapper")
		cmd = exec.CommandContext(ctx, "python3", bridgePath, "owner", bridgeControl)
		cmd.Stdout = &bridgeOutput
		cmd.Cancel = func() error { return cmd.Process.Signal(os.Interrupt) }
		cmd.WaitDelay = 3 * time.Second
	} else if hostWrapper {
		wrapper, er := filepath.Abs("../../.pi/skills/private-host-streaming-application/scripts/activation-host.sh")
		if er != nil {
			t.Fatal(er)
		}
		wrapperLog = filepath.Join(filepath.Dir(path), "host-wrapper-"+ticket.RunID[7:])
		var lifecycleRead *os.File
		lifecycleRead, lifecycleWrite, e = os.Pipe()
		if e != nil {
			t.Fatal(e)
		}
		defer lifecycleRead.Close()
		defer lifecycleWrite.Close()
		cmd = exec.CommandContext(ctx, "/bin/sh", wrapper, tool, path, ticket.RunID, applicationSHA256, wrapperLog)
		cmd.Stdin = lifecycleRead
		cmd.Stdout = windowWrite
		cmd.Cancel = func() error { return cmd.Process.Signal(os.Interrupt) }
		cmd.WaitDelay = 3 * time.Second
	} else {
		if barrierMode || activationGate {
			cmd.ExtraFiles = []*os.File{windowWrite}
		}
		if activationGate {
			cmd.Args = append(cmd.Args, "--startup-notify-fd", "3")
		}
	}
	cmd.Env = append(append(os.Environ(), "LSP_HOST_APPLICATION_OWNED_SERVER=1"), extraEnv...)
	cmd.Stderr = &errout
	cancelled := false
	for _, v := range extraEnv {
		if v == "TEST_CANCEL_PARENT=1" {
			cancelled = true
		}
	}
	cancelPattern := ""
	priorClaims := 0
	if cancelled {
		request, er := readHostApplication(path)
		if er != nil {
			t.Fatal(er)
		}
		cancelPattern = filepath.Join(request.Configuration.Store, "units", "*", "application-original-claim.json")
		old, _ := filepath.Glob(cancelPattern)
		priorClaims = len(old)
	}
	if e = cmd.Start(); e == nil {
		processReady <- cmd.Process
		if activationGate {
			_ = windowWrite.Close()
			go func() {
				deadline, _ := ctx.Deadline()
				_ = windowRead.SetReadDeadline(deadline)
				var raw []byte
				var err error
				if hostBridge {
					await := exec.CommandContext(ctx, "python3", bridgePath, "await", bridgeControl)
					var framed []byte
					framed, err = await.CombinedOutput()
					var ready struct {
						Event  string
						Packet json.RawMessage
					}
					if err == nil && json.Unmarshal(framed, &ready) == nil && ready.Event == "ACTIVATION_READY" {
						raw = ready.Packet
					} else {
						err = fmt.Errorf("bridge startup %v %s", err, framed)
					}
				} else {
					raw, err = io.ReadAll(io.LimitReader(windowRead, 513))
				}
				if err == nil {
					err = verifyStartupFixturePacket(raw, fixtureRequest, ticket.RunID)
				}
				if err == nil {
					slots, slotErr := filepath.Glob(filepath.Join(fixtureRequest.Configuration.Store, "host-wait-slice-"+ticket.RunID[7:]+"-*.json"))
					if slotErr != nil || len(slots) != 0 {
						err = fmt.Errorf("startup consumed wait identities before activation: %d", len(slots))
					}
				}
				if err == nil {
					err = registerRoles()
				}
				if err != nil {
					t.Errorf("ASSERT_PRODUCTION_ACTIVATION_GATE A-fail bytes=%d error=%v", len(raw), err)
					cancel()
					return
				}
				t.Logf("ASSERT_PRODUCTION_ACTIVATION_GATE A-pass bytes=%d schedule=DURABLE_ACTIVATION_BEFORE_ROLE_REGISTER", len(raw))
				close(rolesStart)
			}()
		} else if barrierMode {
			_ = windowWrite.Close()
			go func() {
				var b [1]byte
				if _, err := windowRead.Read(b[:]); err != nil || b[0] != 1 {
					t.Error("fixture window barrier notification failed")
					return
				}
				deadline, err := applicationWaitChildDeadline(fixtureRequest.Configuration.Store, applicationSHA256, ticket.RunID)
				if err != nil || !deadline.After(time.Now()) {
					t.Errorf("ASSERT_WINDOW_BEFORE_ROLE_START A-fail lookup=%v", err)
					return
				}
				t.Log("ASSERT_WINDOW_BEFORE_ROLE_START A-pass slots=0 lookup=BOUND_PREPAID_WINDOW schedule=AFTER_UNIT_WINDOW")
				close(rolesStart)
			}()
		}
		if cancelled {
			go func() {
				for n := 0; n < 400; n++ {
					claims, _ := filepath.Glob(cancelPattern)
					if len(claims) > priorClaims {
						_ = cmd.Process.Signal(os.Interrupt)
						return
					}
					select {
					case <-ctx.Done():
						return
					case <-time.After(50 * time.Millisecond):
					}
				}
				t.Error("cancel fixture never observed new durable original claim")
			}()
		}
		e = cmd.Wait()
	}
	if hostWrapper {
		if lifecycleWrite != nil {
			_ = lifecycleWrite.Close()
		}
		if hostBridge {
			var completion struct {
				Event         string
				WrapperJoined bool
				WrapperExit   int
			}
			if json.Unmarshal(bridgeOutput.Bytes(), &completion) != nil || completion.Event != "OWNER_COMMAND_FINISHED" || !completion.WrapperJoined || completion.WrapperExit != 0 {
				t.Errorf("ASSERT_BRIDGE_NATIVE_JOIN A-fail %s", bridgeOutput.String())
			} else {
				t.Log("ASSERT_BRIDGE_NATIVE_JOIN A-pass")
			}
		}
		raw, readErr := os.ReadFile(filepath.Join(wrapperLog, "terminal.json"))
		if readErr != nil {
			t.Fatal(readErr)
		}
		_, _ = out.Write(raw)
		receipt, readErr := os.ReadFile(filepath.Join(wrapperLog, "process.json"))
		var process struct {
			Joined, ActivationRelayed bool
			CoordinatorExit           int
		}
		if readErr != nil || json.Unmarshal(receipt, &process) != nil || !process.Joined || !process.ActivationRelayed || process.CoordinatorExit != 0 {
			t.Errorf("ASSERT_HOST_WRAPPER_JOINED A-fail receipt=%s error=%v", receipt, readErr)
		} else {
			t.Log("ASSERT_HOST_WRAPPER_JOINED A-pass")
		}
	}
	if out.Len() == 0 {
		cancel()
	}
	wg.Wait()
	if e != nil {
		t.Logf("controlled run failure: %v %s", e, errout.String())
	}
	var report applicationTerminal
	if crash && out.Len() == 0 {
		return applicationTerminal{Status: "KILLED_AFTER_PHYSICAL_REPLY"}, e
	}
	if json.Unmarshal(out.Bytes(), &report) != nil {
		t.Fatalf("application output %v %s %s", e, out.String(), errout.String())
	}
	for _, marker := range extraEnv {
		if marker == "TEST_COUNT_PHYSICAL=1" {
			calls, req, resp := physical[0][0]+physical[1][0], physical[0][1]+physical[1][1], physical[0][2]+physical[1][2]
			if report.Calls != calls || report.RequestBytes != req || report.ResponseBytes != resp {
				t.Errorf("ASSERT_INDEPENDENT_SCRIPTED_PHYSICAL_BYTES calls=%d request=%d response=%d report=%+v", calls, req, resp, report)
			}
			t.Logf("ASSERT_INDEPENDENT_SCRIPTED_PHYSICAL_BYTES A-pass calls=%d request=%d response=%d", calls, req, resp)
		}
	}
	if !applicationFixtureCompleted(terminalObserved) {
		t.Logf("SAFE_SCRIPTED_INCOMPLETE_ROLES interpreter_terminal=%t reviewer_terminal=%t status=UNKNOWN finalize=NOT_ATTEMPTED", terminalObserved[0], terminalObserved[1])
		return report, fmt.Errorf("scripted role observations incomplete: UNKNOWN")
	}
	observations, _ := json.Marshal(returns)
	finalize := exec.CommandContext(ctx, tool, "private-host-application", "finalize", path, ticket.RunID)
	finalize.Stdin = bytes.NewReader(observations)
	if b, fe := finalize.CombinedOutput(); fe != nil {
		var diagnostic struct{ Schema, Class, Reason, Path string }
		if json.NewDecoder(bytes.NewReader(b)).Decode(&diagnostic) == nil && diagnostic.Schema == "private-application-finalize-refusal.v1" && diagnostic.Class == "HOST_RETURN_REFUSED" && filepath.Base(diagnostic.Path) == diagnostic.Path && strings.HasPrefix(diagnostic.Path, "host-refusal-") {
			if request, re := readHostApplication(path); re == nil {
				if record, re := applicationReadBounded(filepath.Join(request.Configuration.Store, diagnostic.Path), 4096); re == nil {
					t.Logf("SAFE_FINALIZE_OPERAND_OBSERVATION %s", record)
				}
			}
		}
		t.Errorf("host observation %v %s; run=%v %s", fe, b, e, errout.String())
	}
	return report, e
}
func captureApplicationPair(t *testing.T, r hostApplicationRequest, id string) map[string][]byte {
	t.Helper()
	a, e := product.OpenApplication(r.Configuration, time.Now())
	if e != nil {
		t.Fatal(e)
	}
	defer a.Close()
	cp, e := a.VerifyInventory(id)
	if e != nil {
		t.Fatal(e)
	}
	out := map[string][]byte{}
	for _, p := range []string{cp.MachinePaths[0], cp.MachinePaths[1], cp.HumanPaths[0], cp.HumanPaths[1]} {
		b, e := os.ReadFile(filepath.Join(r.Configuration.Store, p))
		if e != nil {
			t.Fatal(e)
		}
		out[p] = b
	}
	return out
}
func assertApplicationPairUnchanged(t *testing.T, r hostApplicationRequest, pair map[string][]byte) {
	t.Helper()
	for p, raw := range pair {
		b, e := os.ReadFile(filepath.Join(r.Configuration.Store, p))
		if e != nil || !bytes.Equal(raw, b) {
			t.Fatalf("ASSERT_APPLICATION_PRIOR_PAIRS_BYTE_EXACT %s", p)
		}
	}
}

func TestHostApplicationCLIHostResume(t *testing.T) {
	r, path, base := hostApplicationCLIFixture(t)
	r.MaxUnitsPerRun = 1
	writeApplicationFixture(t, r, path)
	r, e := readHostApplication(path)
	if e != nil {
		t.Fatal(e)
	}
	tool := buildApplicationTool(t, base)
	first, e := controlledApplicationRun(t, tool, path)
	if e != nil || first.Status != "PAUSED_UNIT_SLICE" || first.Units != 1 || first.Calls != 2 {
		t.Fatalf("ASSERT_APPLICATION_FRESH_PROCESS_PAUSE %+v %v", first, e)
	}
	pair := captureApplicationPair(t, r, first.InventoryHead)
	second, e := controlledApplicationRun(t, tool, path)
	if e != nil || second.Units != 2 || second.Calls != 4 {
		t.Fatalf("ASSERT_APPLICATION_FRESH_PROCESS_RESUME %+v %v", second, e)
	}
	assertApplicationPairUnchanged(t, r, pair)
	third, e := controlledApplicationRun(t, tool, path)
	if e != nil || third.Units != 3 || third.Targets != 20 || third.Calls != 6 {
		t.Fatalf("ASSERT_APPLICATION_HOST_ENTRY20_3 %+v %v", third, e)
	}
	assertApplicationPairUnchanged(t, r, pair)
	if third.KnownOwnedWork.ReadHashBytes < first.KnownOwnedWork.ReadHashBytes || third.Reserved.ActiveNS > uint64(240*time.Second) {
		t.Fatal("ASSERT_APPLICATION_CUMULATIVE_POLICY_AND_OWNED_WORK")
	}
	t.Logf("HOST SCRIPTED_CONTRACT actual CLI: 3 fresh processes, targets=%d calls=%d", third.Targets, third.Calls)
}
func TestHostApplicationCLILateFailureNoReplay(t *testing.T) {
	r, path, base := hostApplicationCLIFixture(t)
	r.MaxUnitsPerRun = 1
	writeApplicationFixture(t, r, path)
	r, e := readHostApplication(path)
	if e != nil {
		t.Fatal(e)
	}
	tool := buildApplicationTool(t, base)
	first, e := controlledApplicationRun(t, tool, path)
	if e != nil || first.Units != 1 {
		t.Fatal("first", first, e)
	}
	pair := captureApplicationPair(t, r, first.InventoryHead)
	failed, e := controlledApplicationRun(t, tool, path, "LSP_HOST_APPLICATION_FAIL_DOC=1")
	if e == nil || failed.Units != 1 || failed.Calls != 2 || failed.InventoryHead != first.InventoryHead {
		t.Fatalf("ASSERT_APPLICATION_LATE_PROVIDER_FAILURE %+v %v", failed, e)
	}
	assertApplicationPairUnchanged(t, r, pair)
	again, e := controlledApplicationRun(t, tool, path)
	if e == nil || again.Status != "PAUSED_AMBIGUOUS_SPENT_UNIT_NO_REPLAY" || again.Calls != 2 || again.InventoryHead != first.InventoryHead {
		t.Fatalf("ASSERT_APPLICATION_SPENT_ORIGINAL_NO_REPLAY %+v %v", again, e)
	}
	assertApplicationPairUnchanged(t, r, pair)
}
func TestHostApplicationRealCatalogPagesOverFourMiB(t *testing.T) {
	r, path, _ := hostApplicationCLIFixture(t)
	source := r.Configuration.SourceRoot
	for n := 0; n < 3; n++ {
		if e := os.Remove(filepath.Join(source, fmt.Sprintf("unit%d.go", n))); e != nil {
			t.Fatal(e)
		}
	}
	deep := source
	for n := 0; n < 4; n++ {
		deep = filepath.Join(deep, strings.Repeat(string(rune('a'+n)), 180))
		if e := os.Mkdir(deep, 0700); e != nil {
			t.Fatal(e)
		}
	}
	for n := 0; n < 2000; n++ {
		if e := os.WriteFile(filepath.Join(deep, fmt.Sprintf("entry-%05d.go", n)), []byte("package observed\n"), 0600); e != nil {
			t.Fatal(e)
		}
	}
	var out, diagnostic bytes.Buffer
	code := runPrivateHostApplication([]string{"discover", path}, bytes.NewReader(nil), &out, &diagnostic)
	var report applicationTerminal
	if code != 0 || json.Unmarshal(out.Bytes(), &report) != nil || report.CatalogBytes+report.ViewBytes <= 4<<20 || report.OwnedIO.MaxReadBuffer > product.ApplicationRecordLimit || report.OwnedIO.PriorPageReads != 0 {
		t.Fatalf("ASSERT_APPLICATION_REAL_CATALOG_PAGES_OVER_4MIB code=%d report=%+v diagnostic=%s", code, report, diagnostic.String())
	}
	if report.Status == "PAUSED_BUDGET" && report.Denominator != "UNKNOWN" {
		t.Fatal("ASSERT_APPLICATION_UNFINISHED_DENOMINATOR_UNKNOWN")
	}
	// Reopen and pause again without rewriting an earlier admitted metadata page.
	a, e := product.OpenApplication(r.Configuration, time.Now())
	if e != nil {
		t.Fatal(e)
	}
	var state applicationState
	if e = a.GetRecord(a.Head().CursorSHA256, &state); e != nil {
		t.Fatal(e)
	}
	var selected applicationCatalogEntry
	if e = a.GetRecord(state.Selected, &selected); e != nil {
		t.Fatal(e)
	}
	raw, _ := json.Marshal(selected)
	entryID := pilot.Digest(raw)
	id, lookupErr := a.CatalogPageForEntry(entryID)
	if lookupErr != nil {
		t.Fatal(lookupErr)
	}
	metadata := filepath.Join(r.Configuration.Store, "objects", id[7:9], "catalog-a-"+id+".json")
	before, e := os.ReadFile(metadata)
	a.Close()
	if e != nil {
		t.Fatal(e)
	}
	out.Reset()
	_ = runPrivateHostApplication([]string{"discover", path}, bytes.NewReader(nil), &out, &diagnostic)
	after, e := os.ReadFile(metadata)
	if e != nil || !bytes.Equal(before, after) {
		t.Fatal("ASSERT_APPLICATION_REAL_CATALOG_PRIOR_PAGE_NOT_REWRITTEN")
	}
	t.Logf("REAL canonical catalog=%d view bytes=%d total=%d max record read=%d prior page reads=%d status=%s denominator=%s; no padding or fabricated totals", report.CatalogBytes, report.ViewBytes, report.CatalogBytes+report.ViewBytes, report.OwnedIO.MaxReadBuffer, report.OwnedIO.PriorPageReads, report.Status, report.Denominator)
}
