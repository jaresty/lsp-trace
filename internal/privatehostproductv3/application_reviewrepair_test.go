//go:build darwin || linux

package privatehostproductv3

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func applicationReferenceFixture(t *testing.T) (*ApplicationJournal, ApplicationConfig, UnitBinding, CohortConfig, CohortReport, UnitResultRef) {
	t.Helper()
	c, now := applicationFixture(t)
	a := applicationCreate(t, c, now)
	u := applicationUnit(c, "reference-unit")
	u.RevisionSHA256 = hash([]byte(c.RevisionAssertion))
	u.ScopeSHA256 = hash([]byte("scope"))
	root := filepath.Join(c.Store, "units", u.UnitID[7:])
	if e := os.MkdirAll(root, 0700); e != nil {
		t.Fatal(e)
	}
	source := testConfig(t)
	source.Store = filepath.Join(root, "original")
	source.MaxCalls = 2
	source.ScopeSHA256 = u.ScopeSHA256
	source.ApprovalSHA256 = ConfigDigest(source)
	cc, items := cohortInputs(t)
	cc.Store = filepath.Join(root, "cohort")
	cc.SourceConfigSHA256 = ConfigDigest(source)
	for n := range items {
		items[n].ConfigSHA256 = cc.SourceConfigSHA256
	}
	approvedSource, approvedCohort := source, cc
	approvedSource.Requested, approvedSource.ApprovalSHA256 = 0, ""
	approvedCohort.Requested = 0
	if e := a.AdmitUnitWithDerivation(u, applicationChild(2), approvedSource, approvedCohort, now); e != nil {
		t.Fatal(e)
	}
	r, e := RunCohort(context.Background(), cc, items, &cohortFixture{})
	if e != nil {
		t.Fatal(e)
	}
	if e = a.BindUnitResultConfiguration(u, source, cc); e != nil {
		t.Fatal(e)
	}
	ref, e := BuildUnitResultRef(ApplicationConfigDigest(c), u, cc, r)
	if e != nil {
		t.Fatal(e)
	}
	return a, c, u, cc, r, ref
}
func TestApplicationReviewRepairQuartet(t *testing.T) {
	a, c, _, _, _, ref := applicationReferenceFixture(t)
	if VerifyUnitResultRef(ref) != nil {
		t.Fatal("valid reference")
	}
	prior, e := a.PublishInventory(applicationGapFixture(c, "prior"), struct{ Next string }{"prior"})
	if e != nil {
		t.Fatal(e)
	}
	ref.MachinePaths = [2]string{ref.MachinePaths[0], ref.MachinePaths[0]}
	ref.HumanPaths = ref.MachinePaths
	for n := range ref.Digests {
		ref.Digests[n] = ref.Digests[0]
	}
	if VerifyUnitResultRef(ref) == nil {
		t.Error("ASSERT_APPLICATION_SELECTED_QUARTET_IS_VERIFIED_QUARTET")
	}
	if _, e = a.PublishInventory(ApplicationInventoryPage{Schema: "private-application-inventory-page.v1", ApplicationSHA256: ref.ApplicationSHA256, Rows: []UnitResultRef{ref}}, struct{ Next string }{"bad"}); e == nil {
		t.Error("ASSERT_APPLICATION_ALIAS_PUBLICATION_REFUSED")
	}
	if a.Head().LastCompletePublicationSHA256 != prior {
		t.Error("ASSERT_APPLICATION_REFUSAL_PRIOR_SELECTOR_UNCHANGED")
	}
}
func TestApplicationReviewRepairValidAndFilesystem(t *testing.T) {
	for _, kind := range []string{"valid", "mismatch", "hardlink", "symlink", "outside", "wrong-human"} {
		t.Run(kind, func(t *testing.T) {
			a, c, u, cc, r, ref := applicationReferenceFixture(t)
			if kind == "valid" {
				id, e := a.PublishInventory(ApplicationInventoryPage{Schema: "private-application-inventory-page.v1", ApplicationSHA256: ApplicationConfigDigest(c), Rows: []UnitResultRef{ref}}, struct{ Next string }{"valid"})
				if e != nil {
					t.Fatal("ASSERT_APPLICATION_VALID_ADMITTED_PUBLICATION", e)
				}
				a.Close()
				again, e := OpenApplication(c, time.Now())
				if e != nil {
					t.Fatal(e)
				}
				defer again.Close()
				if _, e = again.VerifyInventory(id); e != nil {
					t.Fatal("ASSERT_APPLICATION_VALID_RESTART_VERIFICATION", e)
				}
				return
			}
			switch kind {
			case "mismatch":
				ref.HumanPaths[1] = ref.HumanPaths[0]
			case "outside":
				ref.HumanPaths[1] = "../outside.md"
			case "hardlink", "symlink":
				dst := filepath.Join(ref.Store, ref.HumanPaths[1])
				src := filepath.Join(ref.Store, ref.HumanPaths[0])
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
			case "wrong-human":
				b, e := os.ReadFile(filepath.Join(ref.Store, ref.MachinePaths[0]))
				if e != nil {
					t.Fatal(e)
				}
				if e = os.WriteFile(filepath.Join(ref.Store, ref.HumanPaths[1]), b, 0600); e != nil {
					t.Fatal(e)
				}
				ref.Digests[3] = hash(b)
			}
			if VerifyUnitResultRef(ref) == nil {
				t.Fatal("ASSERT_APPLICATION_ACTUAL_SELECTED_FILES_REFUSED")
			}
			if _, e := a.PublishInventory(ApplicationInventoryPage{Schema: "private-application-inventory-page.v1", ApplicationSHA256: ApplicationConfigDigest(c), Rows: []UnitResultRef{ref}}, struct{ Next string }{"bad"}); e == nil {
				t.Fatal("ASSERT_APPLICATION_FILES_BEFORE_SELECTOR")
			}
			if a.Head().LastCompletePublicationSHA256 != "" {
				t.Fatal("ASSERT_APPLICATION_INVALID_FILES_NO_SELECTOR")
			}
			_ = u
			_ = cc
			_ = r
		})
	}
}

func TestApplicationReviewRepairIndexedJoinAfterRestart(t *testing.T) {
	a, c, _, _, _, ref := applicationReferenceFixture(t)
	ref.InputSHA256 = hash([]byte("wrong indexed full input digest"))
	page := ApplicationInventoryPage{Schema: "private-application-inventory-page.v1", ApplicationSHA256: ApplicationConfigDigest(c), Rows: []UnitResultRef{ref}}
	// Deliberately install an untrusted index object without PublishInventory.
	// Read-side verification must not infer admission from its selected position.
	raw, e := applicationMarshal(page, ApplicationRecordLimit)
	if e != nil {
		t.Fatal(e)
	}
	id, e := a.PutRecord(page)
	if e != nil {
		t.Fatal(e)
	}
	name := applicationObjectName("data", id)
	cp := ApplicationCheckpoint{Schema: "private-application-checkpoint.v1", ApplicationSHA256: ApplicationConfigDigest(c), PageSHA256: id, MachinePaths: [2]string{name, name}, HumanPaths: [2]string{name, name}, Digests: [4]string{hash(raw), hash(raw), hash(raw), hash(raw)}}
	selected, e := a.PutRecord(cp)
	if e != nil {
		t.Fatal(e)
	}
	a.Close()
	again, e := OpenApplication(c, time.Now())
	if e != nil {
		t.Fatal(e)
	}
	defer again.Close()
	if _, e = again.VerifyInventory(selected); e == nil {
		t.Fatal("ASSERT_APPLICATION_INDEX_READ_CANNOT_BYPASS_ADMITTED_JOIN")
	}
}

func TestApplicationReviewRepairJoins(t *testing.T) {
	for _, field := range []string{"UnitID", "InputSHA256", "RevisionSHA256", "SourceConfigSHA256", "combined"} {
		t.Run(field, func(t *testing.T) {
			a, c, u, cc, r, ref := applicationReferenceFixture(t)
			wrong := hash([]byte("unrelated full digest"))
			switch field {
			case "UnitID":
				ref.UnitID = wrong
			case "InputSHA256":
				ref.InputSHA256 = wrong
			case "RevisionSHA256":
				ref.RevisionSHA256 = wrong
			case "SourceConfigSHA256":
				ref.SourceConfigSHA256 = wrong
			case "combined":
				ref.UnitID = wrong
				ref.InputSHA256 = wrong
				ref.RevisionSHA256 = wrong
				ref.SourceConfigSHA256 = wrong
			}
			if VerifyUnitResultRef(ref) == nil {
				t.Error("ASSERT_APPLICATION_FULL_JOIN_PARENT_ADMISSION")
			}
			if _, e := a.PublishInventory(ApplicationInventoryPage{Schema: "private-application-inventory-page.v1", ApplicationSHA256: ApplicationConfigDigest(c), Rows: []UnitResultRef{ref}}, struct{ Next string }{"bad"}); e == nil {
				t.Error("ASSERT_APPLICATION_WRONG_JOIN_PUBLICATION_REFUSED")
			}
			originalUnit := u
			u.InputSHA256 = wrong
			if _, e := BuildUnitResultRef(ApplicationConfigDigest(c), u, cc, r); e == nil {
				t.Error("ASSERT_APPLICATION_BUILD_WRONG_UNIT_REFUSED")
			}
			cc.SourceConfigSHA256 = wrong
			if _, e := BuildUnitResultRef(ApplicationConfigDigest(c), originalUnit, cc, r); e == nil {
				t.Error("ASSERT_APPLICATION_BUILD_WRONG_CONFIGURATION_REFUSED")
			}
			a.Close()
			again, e := OpenApplication(c, time.Now())
			if e != nil {
				t.Fatal(e)
			}
			defer again.Close()
			if _, e = again.PublishInventory(ApplicationInventoryPage{Schema: "private-application-inventory-page.v1", ApplicationSHA256: ApplicationConfigDigest(c), Rows: []UnitResultRef{ref}}, struct{ Next string }{"restart"}); e == nil {
				t.Error("ASSERT_APPLICATION_RESTART_JOIN_REFUSED")
			}
		})
	}
}
