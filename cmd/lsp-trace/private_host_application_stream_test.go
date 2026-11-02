//go:build darwin

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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
	for _, role := range ticket.RoleTasks {
		if b, e := exec.CommandContext(ctx, tool, "private-host-application", "register", path, ticket.RunID, role.Role, "scripted-existing-host-"+role.Role, role.PromptSHA256).CombinedOutput(); e != nil {
			t.Fatalf("register %v %s", e, b)
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
	var returns [2]product.CohortRoleReturn
	for idx, role := range []string{"interpreter", "reviewer"} {
		wg.Add(1)
		go func(idx int, role string) {
			defer wg.Done()
			var after uint64
			for {
				cmd := exec.CommandContext(ctx, tool, "private-host-application", "wait", path, ticket.RunID, role, strconv.FormatUint(after, 10), "60000")
				b, e := cmd.CombinedOutput()
				if e != nil {
					if ctx.Err() == nil {
						t.Errorf("wait %v %s", e, b)
					}
					return
				}
				var event applicationHostEvent
				if json.Unmarshal(b, &event) != nil {
					t.Error("event JSON")
					return
				}
				if event.Event == "DEADLINE" {
					t.Error("unexpected role deadline")
					return
				}
				if event.Event == "TERMINAL" {
					returns[idx] = product.CohortRoleReturn{Kind: "SCRIPTED_CONTRACT", Role: role, LaunchID: "scripted-existing-host-" + role, TerminalSHA256: event.TerminalSHA256, Status: "RETURNED", ReturnSHA256: pilot.Digest([]byte("scripted result retrieved after completion")), Sequence: after}
					return
				}
				if event.Bundle == nil || event.GlobalSequence <= after {
					t.Error("global sequence binding")
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
	cmd.Env = append(append(os.Environ(), "LSP_HOST_APPLICATION_OWNED_SERVER=1"), extraEnv...)
	var out, errout bytes.Buffer
	cmd.Stdout = &out
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
	if out.Len() == 0 {
		cancel()
	}
	wg.Wait()
	var report applicationTerminal
	if crash && out.Len() == 0 {
		return applicationTerminal{Status: "KILLED_AFTER_PHYSICAL_REPLY"}, e
	}
	if json.Unmarshal(out.Bytes(), &report) != nil {
		t.Fatalf("application output %v %s %s", e, out.String(), errout.String())
	}
	observations, _ := json.Marshal(returns)
	finalize := exec.CommandContext(ctx, tool, "private-host-application", "finalize", path, ticket.RunID)
	finalize.Stdin = bytes.NewReader(observations)
	if b, fe := finalize.CombinedOutput(); fe != nil {
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
