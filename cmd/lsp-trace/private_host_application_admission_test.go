//go:build darwin

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	pilot "lsp-trace/internal/censuslineagepilot"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestHostApplicationCLIConfigFailuresSpendBootstrap(t *testing.T) {
	r, path, _ := hostApplicationCLIFixture(t)
	var out, diagnostic bytes.Buffer
	if code := runPrivateHostApplication([]string{"setup", path}, bytes.NewReader(nil), &out, &diagnostic); code != 0 {
		t.Fatal(diagnostic.String())
	}
	before, e := os.Stat(filepath.Join(r.Configuration.Store, "bootstrap"))
	if e != nil {
		t.Fatal(e)
	}
	r.Template.Policy.MaxRequests++ // stale template; DO NOT repair caller approval.
	raw, _ := json.Marshal(r)
	if e = os.WriteFile(path, raw, 0600); e != nil {
		t.Fatal(e)
	}
	for n := 0; n < 2; n++ {
		if code := runPrivateHostApplication([]string{"run", path}, bytes.NewReader(nil), &out, &diagnostic); code == 0 {
			t.Fatal("invalid request accepted")
		}
	}
	after, e := os.Stat(filepath.Join(r.Configuration.Store, "bootstrap"))
	if e != nil {
		t.Fatal(e)
	}
	if after.Size() != before.Size()+2 {
		t.Fatalf("ASSERT_APPLICATION_CLI_FAILED_BOOTSTRAP_DURABLE: before=%d after=%d", before.Size(), after.Size())
	}
}
func TestHostApplicationCLICancellationPreservesPriorPairs(t *testing.T) {
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
		t.Fatal(first, e)
	}
	pair := captureApplicationPair(t, r, first.InventoryHead)
	cancelled, e := controlledApplicationRun(t, tool, path, "LSP_HOST_APPLICATION_BLOCK_DOC=1", "TEST_CANCEL_PARENT=1")
	if e == nil || cancelled.InventoryHead != first.InventoryHead || cancelled.Calls != first.Calls {
		t.Fatalf("ASSERT_APPLICATION_CANCEL_PRIOR_VIEWS_AND_SPEND %+v %v", cancelled, e)
	}
	assertApplicationPairUnchanged(t, r, pair)
	again, e := controlledApplicationRun(t, tool, path)
	if e == nil || again.Status != "PAUSED_AMBIGUOUS_SPENT_UNIT_NO_REPLAY" || again.Calls != first.Calls {
		t.Fatalf("ASSERT_APPLICATION_CANCELLED_ORIGINAL_SPENT %+v %v", again, e)
	}
	assertApplicationPairUnchanged(t, r, pair)
}
func TestHostApplicationCLIGlobalSixteenBeforeNinthDisclosure(t *testing.T) {
	r, path, base := hostApplicationCLIFixture(t)
	r.MaxUnitsPerRun = 16
	r.UnitResponseBytes = 2048
	r.UnitTimeoutMS = 20000
	for n := 0; n < 3; n++ {
		if e := os.Remove(filepath.Join(r.Configuration.SourceRoot, fmt.Sprintf("unit%d.go", n))); e != nil {
			t.Fatal(e)
		}
	}
	for n := 0; n < 10; n++ {
		if e := os.WriteFile(filepath.Join(r.Configuration.SourceRoot, fmt.Sprintf("part%d.go", n)), []byte("package owned\n\nfunc Owned00() {}\nfunc Owned01() {}\n"), 0600); e != nil {
			t.Fatal(e)
		}
	}
	allowance := pilot.PageCostPrivateV3{LogicalBytes: 8 << 30, ReadHashBytes: 8 << 30, WriteBytes: 1 << 30, Work: 1 << 30}
	r.Template.ApplicationAllowance = allowance
	r.Template.Configuration.Exchange = allowance
	// Narrower two-target controlled computation envelope, not more allowance.
	r.Template.ComputationUnitBytes = 128 << 10
	writeApplicationFixture(t, r, path)
	r, e := readHostApplication(path)
	if e != nil {
		t.Fatal(e)
	}
	tool := buildApplicationTool(t, base)
	report, e := controlledApplicationRun(t, tool, path)
	if e != nil || report.Status != "PAUSED_BUDGET" || report.Units != 8 || report.Calls != 16 || report.Reserved.Calls != 16 || report.Targets != 16 {
		t.Fatalf("ASSERT_APPLICATION_CLI_GLOBAL_CALL16_BOUNDARY %+v %v", report, e)
	}
	// No ninth original-capture ledger exists. Filesystem discovery itself may
	// have observed all ten files; it does not authorize another semantic slot.
	ledgers, e := filepath.Glob(filepath.Join(r.Configuration.Store, "units", "*", "auxiliary", "ledger"))
	if e != nil || len(ledgers) != 8 {
		t.Fatalf("ASSERT_APPLICATION_CALL17_BEFORE_PROVIDER_START: ledgers=%d %v", len(ledgers), e)
	}
	if report.Reserved.ActiveNS > uint64(240*time.Second) {
		t.Fatal("ASSERT_APPLICATION_CUMULATIVE_ACTIVE_TIME")
	}
	t.Logf("GLOBAL16 actual CLI: units=%d targets=%d calls=%d reserved active=%s", report.Units, report.Targets, report.Calls, time.Duration(report.Reserved.ActiveNS))
}
func TestHostApplicationCLIZeroAndOversizedAreGaps(t *testing.T) {
	r, path, base := hostApplicationCLIFixture(t)
	for n := 0; n < 3; n++ {
		os.Remove(filepath.Join(r.Configuration.SourceRoot, fmt.Sprintf("unit%d.go", n)))
	}
	if e := os.WriteFile(filepath.Join(r.Configuration.SourceRoot, "empty.go"), []byte("package owned\n"), 0600); e != nil {
		t.Fatal(e)
	}
	large, e := os.OpenFile(filepath.Join(r.Configuration.SourceRoot, "oversized.go"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		t.Fatal(e)
	}
	if e = large.Truncate(int64(r.Template.Policy.FileBytes) + 1); e != nil {
		t.Fatal(e)
	}
	large.Close()
	tool := buildApplicationTool(t, base)
	cmd := exec.Command(tool, "private-host-application", "run", path)
	cmd.Env = append(os.Environ(), "LSP_HOST_APPLICATION_OWNED_SERVER=1")
	out, e := cmd.CombinedOutput()
	var report applicationTerminal
	if e != nil || json.Unmarshal(out, &report) != nil || report.Targets != 0 || report.Calls != 0 || report.SourceGaps != 2 || report.Selected != 2 || report.AbsenceInference != "PROHIBITED" || report.SourceCompleteness != "UNKNOWN" {
		t.Fatalf("ASSERT_APPLICATION_ZERO_OVERSIZE_EXPLICIT_GAPS %s %v", out, e)
	}
}

func TestHostApplicationCLICorruptChildLedgerBeforeRetainedFreeze(t *testing.T) {
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
		t.Fatal(first, e)
	}
	pair := captureApplicationPair(t, r, first.InventoryHead)
	bad, e := controlledApplicationRun(t, tool, path, "LSP_HOST_APPLICATION_CORRUPT_LEDGER=1")
	if e == nil || bad.Status != "FAILED_VALIDATED_RETAINED_ADMISSION" || bad.Calls != first.Calls || bad.InventoryHead != first.InventoryHead {
		t.Fatalf("ASSERT_APPLICATION_CORRUPT_CHILD_LEDGER_BEFORE_FRESH_FREEZE %+v %v", bad, e)
	}
	assertApplicationPairUnchanged(t, r, pair)
}

func TestHostApplicationCLIRawReplyCrashInvalidReadyRecovery(t *testing.T) {
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
		t.Fatal(first, e)
	}
	pair := captureApplicationPair(t, r, first.InventoryHead)
	crashed, e := controlledApplicationRun(t, tool, path, "TEST_CRASH_PHYSICAL_REPLY=1")
	if e == nil || crashed.Status != "KILLED_AFTER_PHYSICAL_REPLY" {
		t.Fatalf("crash fixture %v %+v", e, crashed)
	}
	recovered, e := controlledApplicationRun(t, tool, path)
	if e == nil || recovered.Status != "PAUSED_AMBIGUOUS_SPENT_UNIT_NO_REPLAY" || recovered.Calls != first.Calls+1 || recovered.ResponseBytes <= first.ResponseBytes || recovered.InventoryHead != first.InventoryHead {
		t.Fatalf("ASSERT_APPLICATION_RAW_REPLY_CRASH_INVALID_READY_KNOWN_BYTES: before=%+v after=%+v err=%v", first, recovered, e)
	}
	assertApplicationPairUnchanged(t, r, pair)
	again, e := controlledApplicationRun(t, tool, path)
	if e == nil || again.Calls != recovered.Calls || again.RequestBytes != recovered.RequestBytes || again.ResponseBytes != recovered.ResponseBytes || again.InventoryHead != first.InventoryHead {
		t.Fatalf("ASSERT_APPLICATION_REPEATED_UNKNOWN_NEVER_ZERO: %+v %v", again, e)
	}
	assertApplicationPairUnchanged(t, r, pair)
	t.Logf("actual SIGSTOP/reply/READY-substitution/SIGKILL/fresh CLI twice: known calls=%d request=%d response=%d; prior pairs immutable; no semantic adoption/replay", again.Calls, again.RequestBytes, again.ResponseBytes)
}
