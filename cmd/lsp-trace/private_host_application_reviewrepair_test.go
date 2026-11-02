//go:build darwin

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	product "lsp-trace/internal/privatehostproductv3"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestHostApplicationReviewRepairOverflow(t *testing.T) {
	for _, field := range []string{"LogicalBytes", "ReadHashBytes", "WriteBytes", "Work"} {
		t.Run(field, func(t *testing.T) {
			r, path, _ := hostApplicationCLIFixture(t)
			child := reflect.ValueOf(&r.Template.ApplicationAllowance).Elem().FieldByName(field)
			bootstrap := reflect.ValueOf(r.Template.BootstrapAllowance).FieldByName(field).Uint()
			child.SetUint(^uint64(0) - bootstrap + 1)
			writeApplicationFixture(t, r, path)
			if _, e := readHostApplication(path); e == nil {
				t.Error("ASSERT_APPLICATION_COMPONENT_OVERFLOW_REFUSED")
			}
			var out, diagnostic bytes.Buffer
			if runPrivateHostApplication([]string{"setup", path}, bytes.NewReader(nil), &out, &diagnostic) == 0 {
				t.Error("ASSERT_APPLICATION_PUBLIC_OVERFLOW_REFUSED_BEFORE_WORK")
			}
			ledgers, _ := filepath.Glob(filepath.Join(r.Configuration.Store, "units", "*", "auxiliary", "ledger"))
			if len(ledgers) != 0 {
				t.Fatal("ASSERT_APPLICATION_OVERFLOW_NO_CHILD_LEDGER")
			}
		})
	}
}

func TestHostApplicationReviewRepairExactAllowance(t *testing.T) {
	r, path, _ := hostApplicationCLIFixture(t)
	for _, field := range []string{"LogicalBytes", "ReadHashBytes", "WriteBytes", "Work"} {
		bootstrap := reflect.ValueOf(r.Template.BootstrapAllowance).FieldByName(field).Uint()
		reflect.ValueOf(&r.Template.ApplicationAllowance).Elem().FieldByName(field).SetUint(^uint64(0) - bootstrap)
	}
	writeApplicationFixture(t, r, path)
	admitted, e := readHostApplication(path)
	if e != nil {
		t.Fatal("ASSERT_APPLICATION_EXACT_UINT_BOUNDARY_ALLOWED", e)
	}
	cost, e := applicationChildAllowance(admitted, admitted.Template)
	if e != nil {
		t.Fatal(e)
	}
	for _, field := range []string{"LogicalBytes", "ReadHashBytes", "WriteBytes", "Work"} {
		if reflect.ValueOf(cost).FieldByName(field).Uint() != ^uint64(0) {
			t.Fatal("ASSERT_APPLICATION_EXACT_PREPAID_COMPONENTS")
		}
	}
}

type applicationShortWriter struct{}

func (applicationShortWriter) Write(p []byte) (int, error) { return len(p) - 1, nil }

func TestHostApplicationReviewRepairSharedWait(t *testing.T) {
	r, path, _ := hostApplicationCLIFixture(t)
	var out, diagnostic bytes.Buffer
	if runPrivateHostApplication([]string{"setup", path}, bytes.NewReader(nil), &out, &diagnostic) != 0 {
		t.Fatal(diagnostic.String())
	}
	var setup struct{ RunID string }
	if json.Unmarshal(out.Bytes(), &setup) != nil {
		t.Fatal("setup")
	}
	// Consume the actual production slot allocator without sleeping: it prepays
	// each future poll before work, so this deterministically simulates 10s spent.
	for n := 0; n < 100; n++ {
		if e := applicationClaimWaitSlice(r.Configuration.Store, setup.RunID, []byte(`{}`)); e != nil {
			t.Fatal(e)
		}
	}
	for n, role := range []string{"interpreter", "reviewer", "interpreter"} {
		out.Reset()
		diagnostic.Reset()
		if runPrivateHostApplication([]string{"wait", path, setup.RunID, role, fmt.Sprint(n), "270000"}, bytes.NewReader(nil), &out, &diagnostic) != 0 {
			t.Fatal(diagnostic.String())
		}
		var event applicationHostEvent
		if json.Unmarshal(out.Bytes(), &event) != nil || event.Event != "PAUSED_BUDGET" {
			t.Fatalf("ASSERT_APPLICATION_WAIT_SHARED_NO_RESET %s", out.String())
		}
	}
	if runPrivateHostApplication([]string{"wait", path, setup.RunID, "reviewer", "4", "270000"}, bytes.NewReader(nil), applicationShortWriter{}, &diagnostic) == 0 {
		t.Fatal("ASSERT_APPLICATION_WAIT_SHORT_WRITE_PROPAGATED")
	}
	// Independent process reopens the same finite pool; it cannot replenish it.
	tool := buildApplicationTool(t, filepath.Dir(path))
	raw, e := exec.Command(tool, "private-host-application", "wait", path, setup.RunID, "reviewer", "5", "270000").CombinedOutput()
	if e != nil || !bytes.Contains(raw, []byte(`"Event":"PAUSED_BUDGET"`)) {
		t.Fatalf("ASSERT_APPLICATION_WAIT_PROCESS_RESTART_NO_RESET %s %v", raw, e)
	}
	a, e := product.OpenApplication(r.Configuration, time.Now())
	if e != nil {
		t.Fatal(e)
	}
	defer a.Close()
	if a.Head().Reserved.ActiveNS > uint64(240*time.Second) {
		t.Fatal("ASSERT_APPLICATION_WAIT_GLOBAL_CEILING")
	}
}

func TestHostApplicationReviewRepairChildWindow(t *testing.T) {
	r, path, _ := hostApplicationCLIFixture(t)
	var out, diagnostic bytes.Buffer
	if runPrivateHostApplication([]string{"setup", path}, bytes.NewReader(nil), &out, &diagnostic) != 0 {
		t.Fatal(diagnostic.String())
	}
	var setup struct{ RunID string }
	if json.Unmarshal(out.Bytes(), &setup) != nil {
		t.Fatal("setup")
	}
	for n := 0; n < 100; n++ {
		if e := applicationClaimWaitSlice(r.Configuration.Store, setup.RunID, []byte(`{}`)); e != nil {
			t.Fatal(e)
		}
	}
	d := r.Configuration.SelectionRuleSHA256
	u := product.UnitBinding{ApplicationSHA256: product.ApplicationConfigDigest(r.Configuration), UnitID: d, InputSHA256: d, RevisionSHA256: d, SourceFreezeSHA256: d, ScopeSHA256: d, ProducerConfigSHA256: r.Configuration.ProducerConfigSHA256, PolicySHA256: r.Configuration.UnitPolicySHA256, RootIdentitySHA256: d}
	a, e := product.OpenApplication(r.Configuration, time.Now())
	if e != nil {
		t.Fatal(e)
	}
	if e = a.AdmitUnit(u, product.ApplicationBounds{ActiveNS: uint64(500 * time.Millisecond)}, time.Now()); e != nil {
		t.Fatal(e)
	}
	deadline, e := product.ReadApplicationUnitDeadline(r.Configuration.Store, u.ApplicationSHA256, u.UnitID)
	if e != nil {
		t.Fatal(e)
	}
	a.Close()
	raw, _ := json.Marshal(struct{ UnitID string }{u.UnitID})
	if e = applicationWriteOnce(applicationWaitUnitPath(r.Configuration.Store, setup.RunID, 1), raw); e != nil {
		t.Fatal(e)
	}
	out.Reset()
	if runPrivateHostApplication([]string{"wait", path, setup.RunID, "interpreter", "0", "80"}, bytes.NewReader(nil), &out, &diagnostic) != 0 || !bytes.Contains(out.Bytes(), []byte(`"Event":"DEADLINE"`)) {
		t.Fatalf("ASSERT_APPLICATION_EXISTING_CHILD_WINDOW_SHARED %s %s", out.String(), diagnostic.String())
	}
	time.Sleep(time.Until(deadline) + 20*time.Millisecond)
	out.Reset()
	if runPrivateHostApplication([]string{"wait", path, setup.RunID, "reviewer", "1", "270000"}, bytes.NewReader(nil), &out, &diagnostic) != 0 || !bytes.Contains(out.Bytes(), []byte(`"Event":"PAUSED_BUDGET"`)) {
		t.Fatalf("ASSERT_APPLICATION_CHILD_WINDOW_NEVER_RENEWED %s %s", out.String(), diagnostic.String())
	}
}

func TestHostApplicationReviewRepairCLIExpiry(t *testing.T) {
	r, path, base := hostApplicationCLIFixture(t)
	tool := buildApplicationTool(t, base)
	r.Configuration.Grant.ExpiresUnixNano = time.Now().Add(1800 * time.Millisecond).UnixNano()
	writeApplicationFixture(t, r, path)
	raw, e := exec.Command(tool, "private-host-application", "setup", path).CombinedOutput()
	if e != nil {
		t.Fatalf("setup %s %v", raw, e)
	}
	var setup struct{ RunID string }
	if json.Unmarshal(raw, &setup) != nil {
		t.Fatal("setup")
	}
	started := time.Now()
	raw, e = exec.Command(tool, "private-host-application", "wait", path, setup.RunID, "interpreter", "0", "5000").CombinedOutput()
	if e != nil || !bytes.Contains(raw, []byte(`"Event":"APPLICATION_EXPIRED"`)) || time.Since(started) > 3*time.Second {
		t.Fatalf("ASSERT_APPLICATION_ACTUAL_CLI_EXPIRY %s %v %s", raw, e, time.Since(started))
	}
}

func TestHostApplicationReviewRepairWaitExpiry(t *testing.T) {
	r, path, _ := hostApplicationCLIFixture(t)
	r.Configuration.Grant.ExpiresUnixNano = time.Now().Add(800 * time.Millisecond).UnixNano()
	writeApplicationFixture(t, r, path)
	var out, diagnostic bytes.Buffer
	if runPrivateHostApplication([]string{"setup", path}, bytes.NewReader(nil), &out, &diagnostic) != 0 {
		t.Fatal(diagnostic.String())
	}
	var setup struct{ RunID string }
	if json.Unmarshal(out.Bytes(), &setup) != nil {
		t.Fatal("setup JSON")
	}
	out.Reset()
	if runPrivateHostApplication([]string{"wait", path, setup.RunID, "interpreter", "0", "1400"}, bytes.NewReader(nil), &out, &diagnostic) != 0 {
		t.Fatal(diagnostic.String())
	}
	var event applicationHostEvent
	if json.Unmarshal(out.Bytes(), &event) != nil || event.Event != "APPLICATION_EXPIRED" {
		t.Fatalf("ASSERT_APPLICATION_WAIT_ABSOLUTE_EXPIRY: %s", out.String())
	}
	if _, e := os.Stat(r.Configuration.Store); e != nil {
		t.Fatal(e)
	}
}
