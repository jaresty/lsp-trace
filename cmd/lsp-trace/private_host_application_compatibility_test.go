//go:build darwin

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	product "lsp-trace/internal/privatehostproductv3"
)

func compatibilityEnvelope(t *testing.T, r hostApplicationRequest, calls int) hostApplicationRequest {
	t.Helper()
	raw, _ := json.Marshal(r)
	var fields map[string]json.RawMessage
	if e := json.Unmarshal(raw, &fields); e != nil {
		t.Fatal(e)
	}
	fields["UnitEnvelope"] = json.RawMessage(fmt.Sprintf(`{"Schema":"private-application-unit-envelope.v1","MaxCalls":%d,"MaxRequestBytes":32768,"MaxTotalBytes":%d,"MaxOutputBytes":262144}`, calls, calls*40960))
	raw, _ = json.Marshal(fields)
	if e := json.Unmarshal(raw, &r); e != nil {
		t.Fatal(e)
	}
	return r
}

func TestHostApplicationCompatibilityExplicitPrepay(t *testing.T) {
	r, _, _ := hostApplicationCLIFixture(t)
	old, e := applicationChildAllowance(r, r.Template)
	if e != nil || old.Calls != 2 {
		t.Fatal("legacy", old, e)
	}
	r = compatibilityEnvelope(t, r, 4)
	next, e := applicationChildAllowance(r, r.Template)
	if e != nil || next.Calls != 4 || next.RequestBytes != 4*32768 || next.ResponseBytes != 4*8192 || next.WireBytes != 4*40960 || next.OutputBytes != 262144 {
		t.Fatalf("ASSERT_EXPLICIT_UNIT_PREPAID_ENVELOPE got=%+v err=%v", next, e)
	}
	t.Log("ASSERT_EXPLICIT_UNIT_PREPAID_ENVELOPE A-pass")
}

func compatibilityOneFile(t *testing.T) (hostApplicationRequest, string, string) {
	r, path, base := hostApplicationCLIFixture(t)
	for i := 0; i < 3; i++ {
		if e := os.Remove(filepath.Join(r.Configuration.SourceRoot, fmt.Sprintf("unit%d.go", i))); e != nil {
			t.Fatal(e)
		}
	}
	var text strings.Builder
	text.WriteString("package owned\n\n")
	for i := 0; i < 20; i++ {
		fmt.Fprintf(&text, "func Owned%02d() {}\n", i)
	}
	if e := os.WriteFile(filepath.Join(r.Configuration.SourceRoot, "twenty.go"), []byte(text.String()), 0600); e != nil {
		t.Fatal(e)
	}
	return r, path, base
}

func TestHostApplicationCompatibilityOneFileTwenty(t *testing.T) {
	r, path, base := compatibilityOneFile(t)
	r = compatibilityEnvelope(t, r, 4)
	// Explicit native capture packing; twenty targets still require TWO ten-item
	// semantic batches and four physical exchanges from this one file.
	r.Template.Policy.Planning.MaxBatchTargets = 20
	r.Template.ComputationUnitBytes = 1 << 20 // explicit new single-batch TEST opt-in
	for n, arg := range r.Template.CensusArguments {
		if arg == "--batch-targets" {
			r.Template.CensusArguments[n+1] = "20"
		}
	}
	// Explicit TEST-only calibration, below the unchanged 128 GiB parent.
	r.Template.ApplicationAllowance.LogicalBytes = 96 << 30
	r.Template.ApplicationAllowance.ReadHashBytes = 96 << 30
	r.Template.Configuration.Exchange = r.Template.ApplicationAllowance
	r.MaxUnitsPerRun = 1
	writeApplicationFixture(t, r, path)
	var readErr error
	r, readErr = readHostApplication(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	tool := buildApplicationTool(t, base)
	report, e := controlledApplicationRun(t, tool, path, "TEST_COUNT_PHYSICAL=1")
	if e != nil || report.Status != "PAUSED_UNIT_SLICE" || report.Denominator != "UNKNOWN" || report.Units != 1 || report.Targets != 20 || report.Calls != 4 || report.RequestBytes <= 0 || report.ResponseBytes <= 0 || report.Reserved.Calls != 4 || report.Reserved.RequestBytes > 1<<20 || report.Reserved.ResponseBytes > 64<<10 || report.Reserved.WireBytes > 8<<20 || report.Reserved.OutputBytes > 4<<20 {
		t.Fatalf("ASSERT_ONE_FILE_NATIVE_TWENTY_REVIEWED %+v err=%v", report, e)
	}
	a, openErr := product.OpenApplication(r.Configuration, time.Now())
	if openErr != nil {
		t.Fatal(openErr)
	}
	defer a.Close()
	cp, verifyErr := a.VerifyInventory(report.InventoryHead)
	if verifyErr != nil {
		t.Fatal(verifyErr)
	}
	var page product.ApplicationInventoryPage
	if a.GetRecord(cp.PageSHA256, &page) != nil || len(page.Rows) != 1 || page.Rows[0].Requested != 20 || page.Rows[0].Reviewed != 20 {
		t.Fatalf("ASSERT_EXACT_TWENTY_REVIEWED_DISPOSITIONS %+v", page)
	}
	t.Logf("ASSERT_ONE_FILE_NATIVE_TWENTY_REVIEWED A-pass calls=%d request=%d response=%d reserved=%+v observed=%+v", report.Calls, report.RequestBytes, report.ResponseBytes, report.Reserved, report.KnownOwnedWork)
}

func TestHostApplicationCompatibilityPolicyBoundsAndBinding(t *testing.T) {
	r, path, _ := hostApplicationCLIFixture(t)
	oldHash := hostApplicationUnitPolicyDigest(r)
	for _, calls := range []int{-1, 0, 1, 17, int(^uint(0) >> 1)} {
		bad := compatibilityEnvelope(t, r, calls)
		if _, e := applicationChildAllowance(bad, bad.Template); e == nil {
			t.Fatalf("ASSERT_UNIT_POLICY_MAX_OVERFLOW calls=%d", calls)
		}
	}
	changed := compatibilityEnvelope(t, r, 4)
	if hostApplicationUnitPolicyDigest(changed) == oldHash {
		t.Fatal("ASSERT_NEW_UNIT_POLICY_CANNOT_REUSE_HASH")
	}
	raw, _ := json.Marshal(changed)
	if os.WriteFile(path, raw, 0600) != nil {
		t.Fatal("write")
	}
	if _, e := readHostApplication(path); e == nil {
		t.Fatal("ASSERT_STALE_POLICY_REJECTED_AT_LOADER")
	}
	if _, e := os.Stat(r.Configuration.Store); !os.IsNotExist(e) {
		t.Fatal("ASSERT_NO_PARENT_LEDGER_ON_STALE_POLICY")
	}
	t.Log("ASSERT_UNIT_POLICY_BOUNDS_AND_BINDING A-pass")
}

func TestHostApplicationCompatibilityLegacyPause(t *testing.T) {
	r, path, base := compatibilityOneFile(t)
	r.MaxUnitsPerRun = 1
	writeApplicationFixture(t, r, path)
	tool := buildApplicationTool(t, base)
	report, e := controlledApplicationRun(t, tool, path)
	if e != nil || report.Status != "PAUSED_UNIT_SEMANTIC_POLICY" || report.Calls != 0 || report.Units != 0 {
		t.Fatalf("ASSERT_LEGACY_TWENTY_PREFLIGHT_PAUSE %+v %v", report, e)
	}
	claims, _ := filepath.Glob(filepath.Join(r.Configuration.Store, "units", "*", "original", "claim", "*"))
	if len(claims) != 0 {
		t.Fatal("ASSERT_LEGACY_PAUSE_BEFORE_CHILD_LEDGER", claims)
	}
	t.Log("ASSERT_LEGACY_TWENTY_PREFLIGHT_PAUSE A-pass")
}
