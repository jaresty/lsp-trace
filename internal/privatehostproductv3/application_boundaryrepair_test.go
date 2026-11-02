//go:build darwin || linux

package privatehostproductv3

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestApplicationBoundaryClockReversal(t *testing.T) {
	c, now := applicationFixture(t)
	a := applicationCreate(t, c, now)
	u := applicationUnit(c, "reversed-clock")
	// Future caller time is inconsistent with the real admission sample and must
	// refuse before any usable record exists, not merely until clocks catch up.
	if e := a.AdmitUnit(u, applicationChild(0), now.Add(30*time.Second)); e == nil {
		t.Fatal("ASSERT_BOUNDARY_INCONSISTENT_ADMISSION_REFUSED")
	}
	if a.implementation.exists("unit", u.UnitID) {
		t.Fatal("ASSERT_BOUNDARY_REFUSED_ADMISSION_NO_RECORD")
	}
	a.Close()
	if d, e := ReadApplicationUnitDeadline(c.Store, u.ApplicationSHA256, u.UnitID); e == nil && d.After(time.Now()) {
		t.Fatal("ASSERT_BOUNDARY_CLOCK_REVERSAL_NO_REOPEN")
	}
}

func TestApplicationBoundaryBeforeFirstBind(t *testing.T) {
	c, now := applicationFixture(t)
	a := applicationCreate(t, c, now)
	u := applicationUnit(c, "unapproved-before-bind")
	u.RevisionSHA256 = hash([]byte(c.RevisionAssertion))
	u.ScopeSHA256 = hash([]byte("scope"))
	if e := a.AdmitUnit(u, applicationChild(2), now); e != nil {
		t.Fatal(e)
	}
	root := filepath.Join(c.Store, "units", u.UnitID[7:])
	if e := os.MkdirAll(root, 0700); e != nil {
		t.Fatal(e)
	}
	source := testConfig(t)
	source.Store = filepath.Join(root, "original")
	source.ScopeSHA256 = u.ScopeSHA256
	source.MaxCalls = 2
	source.Application = "unapproved identity"
	source.ProducerConfigSHA256 = hash([]byte("unapproved producer"))
	source.ApprovalSHA256 = ConfigDigest(source)
	cc, _ := cohortInputs(t)
	cc.Store = filepath.Join(root, "cohort")
	cc.SourceConfigSHA256 = ConfigDigest(source)
	if e := a.BindUnitResultConfiguration(u, source, cc); e == nil {
		t.Fatal("ASSERT_BOUNDARY_FIRST_BIND_CANNOT_AUTHORIZE_SOURCE")
	}
}

func TestApplicationBoundaryIndexedQuartet(t *testing.T) {
	a, c, _, _, _, ref := applicationReferenceFixture(t)
	page := ApplicationInventoryPage{Schema: "private-application-inventory-page.v1", ApplicationSHA256: ApplicationConfigDigest(c), Rows: []UnitResultRef{ref}}
	id, e := a.PutRecord(page)
	if e != nil {
		t.Fatal(e)
	}
	name := applicationObjectName("data", id)
	cp := ApplicationCheckpoint{Schema: "private-application-checkpoint.v1", ApplicationSHA256: page.ApplicationSHA256, PageSHA256: id, MachinePaths: [2]string{name, name}, HumanPaths: [2]string{name, name}, Digests: [4]string{id, id, id, id}}
	selected, e := a.PutRecord(cp)
	if e != nil {
		t.Fatal(e)
	}
	a.Close()
	a, e = OpenApplication(c, time.Now())
	if e != nil {
		t.Fatal(e)
	}
	defer a.Close()
	if _, e = a.VerifyInventory(selected); e == nil {
		t.Fatal("ASSERT_BOUNDARY_INDEXED_ACTUAL_QUARTET")
	}
}

func TestApplicationBoundaryFrozenAdmission(t *testing.T) {
	for _, field := range []string{"valid", "Application", "ProducerConfigSHA256", "HumanReceiptSHA256", "Seeds", "ScopeSHA256", "Store", "CohortGrant"} {
		t.Run(field, func(t *testing.T) {
			c, now := applicationFixture(t)
			a := applicationCreate(t, c, now)
			u := applicationUnit(c, "frozen")
			u.RevisionSHA256 = hash([]byte(c.RevisionAssertion))
			u.ScopeSHA256 = hash([]byte("scope"))
			root := filepath.Join(c.Store, "units", u.UnitID[7:])
			source := testConfig(t)
			source.Store = filepath.Join(root, "original")
			source.ScopeSHA256 = u.ScopeSHA256
			source.MaxCalls = 2
			cc, _ := cohortInputs(t)
			cc.Store = filepath.Join(root, "cohort")
			cc.SourceConfigSHA256 = ConfigDigest(source)
			approved, ac := source, cc
			approved.Requested = 0
			approved.ApprovalSHA256 = ""
			ac.Requested = 0
			if e := a.AdmitUnitWithDerivation(u, applicationChild(2), approved, ac, now); e != nil {
				t.Fatal(e)
			}
			switch field {
			case "Application":
				source.Application = "unapproved"
			case "ProducerConfigSHA256":
				source.ProducerConfigSHA256 = hash([]byte("unapproved"))
			case "HumanReceiptSHA256":
				source.HumanReceiptSHA256 = hash([]byte("unapproved"))
			case "Seeds":
				source.Seeds = []uint64{89, 90}
			case "ScopeSHA256":
				source.ScopeSHA256 = hash([]byte("unapproved"))
				cc.ScopeSHA256 = source.ScopeSHA256
			case "Store":
				source.Store = filepath.Join(root, "other")
			case "CohortGrant":
				cc.Grant = hash([]byte("unapproved"))
			}
			source.ApprovalSHA256 = ConfigDigest(source)
			cc.SourceConfigSHA256 = ConfigDigest(source)
			e := a.BindUnitResultConfiguration(u, source, cc)
			if field == "valid" {
				if e != nil {
					t.Fatal("ASSERT_BOUNDARY_NATIVE_REQUESTED_ALLOWED", e)
				}
			} else {
				if e == nil {
					t.Fatal("ASSERT_BOUNDARY_ADMISSION_FROZEN_BEFORE_FIRST_BIND")
				}
				if a.implementation.exists("derivation", u.UnitID) {
					t.Fatal("ASSERT_BOUNDARY_NO_UNAPPROVED_DERIVATION_STATE")
				}
			}
		})
	}
}

func TestApplicationBoundaryClockProcess(t *testing.T) {
	if os.Getenv("LSP_BOUNDARY_CLOCK_CHILD") == "1" {
		var args struct {
			Root, App, ID string
			Clock         applicationClockStamp
		}
		if json.Unmarshal([]byte(os.Getenv("LSP_BOUNDARY_CLOCK_CASE")), &args) != nil {
			t.Fatal("clock input")
		}
		d, e := readApplicationUnitDeadline(args.Root, args.App, args.ID, func() (applicationClockStamp, error) { return args.Clock, nil })
		if e == nil && d.After(time.Now()) {
			t.Fatal("ASSERT_BOUNDARY_RESTART_ROLLBACK_NO_AUTHORITY")
		}
		return
	}
	c, now := applicationFixture(t)
	a := applicationCreate(t, c, now)
	u := applicationUnit(c, "expired-kernel-window")
	if e := a.AdmitUnit(u, applicationChild(0), now); e != nil {
		t.Fatal(e)
	}
	var unit applicationUnitLedger
	if e := a.implementation.object("unit", u.UnitID, &unit); e != nil {
		t.Fatal(e)
	}
	a.Close()
	// Same actual OS epoch/domain, injected readings only: elapsed two seconds,
	// wall reversed thirty seconds. No clock mutation and no test-only admission.
	stamp := unit.Clock
	stamp.MonotonicNS += int64(2 * time.Second)
	stamp.WallNS -= int64(30 * time.Second)
	raw, _ := json.Marshal(struct {
		Root, App, ID string
		Clock         applicationClockStamp
	}{c.Store, u.ApplicationSHA256, u.UnitID, stamp})
	exe, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	cmd := exec.Command(exe, "-test.run=^TestApplicationBoundaryClockProcess$")
	cmd.Env = append(os.Environ(), "LSP_BOUNDARY_CLOCK_CHILD=1", "LSP_BOUNDARY_CLOCK_CASE="+string(raw))
	if b, e := cmd.CombinedOutput(); e != nil {
		t.Fatalf("ASSERT_BOUNDARY_CROSS_PROCESS_CLOCK %s %v", b, e)
	}
	for _, kind := range []string{"valid", "expired", "small-rollback", "reboot"} {
		t.Run(kind, func(t *testing.T) {
			s := unit.Clock
			s.MonotonicNS += int64(100 * time.Millisecond)
			s.WallNS += int64(100 * time.Millisecond)
			switch kind {
			case "expired":
				s.MonotonicNS += int64(2 * time.Second)
				s.WallNS += int64(2 * time.Second)
			case "small-rollback":
				s.WallNS -= int64(100 * time.Millisecond)
			case "reboot":
				s.Epoch = "other-boot"
			}
			left, e := applicationIntervalRemaining(unit, s)
			if kind == "reboot" {
				if e == nil {
					t.Fatal("ASSERT_BOUNDARY_UNCERTAIN_EPOCH_REFUSED")
				}
				return
			}
			if e != nil {
				t.Fatal(e)
			}
			if kind == "expired" && left != 0 {
				t.Fatal("ASSERT_BOUNDARY_KERNEL_WINDOW_RETIRED")
			}
			if left > 900*time.Millisecond {
				t.Fatal("ASSERT_BOUNDARY_ROLLBACK_NEVER_MINTS_TIME")
			}
		})
	}
}

func TestApplicationBoundarySelectedTruth(t *testing.T) {
	for _, kind := range []string{"valid", "Accepted", "Authority", "FeatureIdentity", "SourceCompleteness", "AbsenceInference", "NavigationStatus", "Previous", "wrong-human", "hardlink", "symlink", "outside"} {
		t.Run(kind, func(t *testing.T) {
			a, c, _, _, _, ref := applicationReferenceFixture(t)
			page := ApplicationInventoryPage{Schema: "private-application-inventory-page.v1", ApplicationSHA256: ApplicationConfigDigest(c), Rows: []UnitResultRef{ref}, FeatureIdentity: "UNRESOLVED", SourceCompleteness: "UNKNOWN", AbsenceInference: "PROHIBITED", NavigationStatus: "UNASSIGNED_NAVIGATION_BACKLOG_NOT_COMPUTED"}
			switch kind {
			case "Accepted":
				page.Accepted = true
			case "Authority":
				page.Authority = 1
			case "FeatureIdentity":
				page.FeatureIdentity = "RESOLVED"
			case "SourceCompleteness":
				page.SourceCompleteness = "COMPLETE"
			case "AbsenceInference":
				page.AbsenceInference = "ALLOWED"
			case "NavigationStatus":
				page.NavigationStatus = "COMPLETE"
			}
			raw := mustJSON(page)
			id := hash(raw)
			human := applicationInventoryHuman(raw)
			cp := ApplicationCheckpoint{Schema: "private-application-checkpoint.v1", ApplicationSHA256: page.ApplicationSHA256, PageSHA256: id}
			for n := 0; n < 2; n++ {
				cp.MachinePaths[n] = applicationObjectName(fmt.Sprintf("inventory-%d", n), id)
				cp.HumanPaths[n] = applicationObjectName(fmt.Sprintf("human-%d", n), id)
				cp.Digests[n] = id
				cp.Digests[n+2] = hash(human)
				if e := a.implementation.putObject(applicationObject{cp.MachinePaths[n], raw}); e != nil {
					t.Fatal(e)
				}
				if e := a.implementation.putObject(applicationObject{cp.HumanPaths[n], human}); e != nil {
					t.Fatal(e)
				}
			}
			switch kind {
			case "Previous":
				cp.Previous = hash([]byte("unrelated"))
			case "wrong-human":
				if e := os.WriteFile(filepath.Join(c.Store, cp.HumanPaths[1]), []byte("wrong"), 0600); e != nil {
					t.Fatal(e)
				}
				cp.Digests[3] = hash([]byte("wrong"))
			case "hardlink", "symlink":
				dst := filepath.Join(c.Store, cp.HumanPaths[1])
				src := filepath.Join(c.Store, cp.HumanPaths[0])
				if e := os.Remove(dst); e != nil {
					t.Fatal(e)
				}
				var e error
				if kind == "hardlink" {
					e = os.Link(src, dst)
				} else {
					e = os.Symlink(src, dst)
				}
				if e != nil {
					t.Fatal(e)
				}
			case "outside":
				cp.HumanPaths[1] = "../outside"
			}
			selected, e := a.PutRecord(cp)
			if e != nil {
				t.Fatal(e)
			}
			_, e = a.VerifyInventory(selected)
			if kind == "valid" {
				if e != nil {
					t.Fatal("ASSERT_BOUNDARY_VALID_SELECTED_QUARTET", e)
				}
				return
			}
			if e == nil {
				t.Fatal("ASSERT_BOUNDARY_SELECTED_TRUTH_AND_TOPOLOGY")
			}
			cursor, e := a.PutRecord(struct{ Next string }{"invalid selector"})
			if e != nil {
				t.Fatal(e)
			}
			if a.CommitCursor(cursor, selected) == nil {
				t.Fatal("ASSERT_BOUNDARY_INVALID_CHECKPOINT_NO_SELECTOR")
			}
			if a.Head().LastCompletePublicationSHA256 != "" {
				t.Fatal("ASSERT_BOUNDARY_SELECTOR_UNCHANGED")
			}
		})
	}
}
