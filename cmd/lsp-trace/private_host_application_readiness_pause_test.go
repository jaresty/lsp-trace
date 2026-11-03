//go:build darwin

package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"
)

func TestHostApplicationReadinessInsufficientBeforeProvider(t *testing.T) {
	r, path, _ := readinessLiteralFixture(t)
	r = readinessUnitTime(t, r, 180000)
	// Deliberately insufficient AFTER charged exact discovery and source freeze;
	// no provider/helper process is needed or allowed to rescue this reservation.
	r.Configuration.Grant.Ceiling.ActiveNS = uint64(200 * time.Second)
	writeApplicationFixture(t, r, path)
	var out, diag bytes.Buffer
	code := runPrivateHostApplication([]string{"run", path}, bytes.NewReader(nil), &out, &diag)
	var report applicationTerminal
	if code != 0 || json.Unmarshal(out.Bytes(), &report) != nil || report.Status != "PAUSED_BUDGET" || report.Selected != 1 || report.Calls != 0 || report.Reserved.Calls != 0 || report.Reserved.ActiveNS != uint64(27032*time.Millisecond) {
		t.Fatalf("ASSERT_UNIT180_PUBLIC_PAUSE_BEFORE_PROVIDER A-fail code=%d report=%+v diag=%s", code, report, diag.String())
	}
	for _, suffix := range []string{"application-original-claim.json", "original", "auxiliary/ledger", "cohort-mailbox"} {
		paths, e := filepath.Glob(filepath.Join(r.Configuration.Store, "units", "*", suffix))
		if e != nil || len(paths) != 0 {
			t.Fatal("ASSERT_UNIT180_PUBLIC_PAUSE_NO_CHILD", paths, e)
		}
	}
	t.Logf("ASSERT_UNIT180_PUBLIC_PAUSE_BEFORE_PROVIDER A-pass active=%d calls=%d source-before-provider=1s", report.Reserved.ActiveNS, report.Calls)
}
