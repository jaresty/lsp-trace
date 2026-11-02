//go:build darwin || linux

package privatehostproductv3

import (
	"bytes"
	"testing"
	"time"
)

func applicationGapFixture(c ApplicationConfig, label string) ApplicationInventoryPage {
	d := hash([]byte(label))
	return ApplicationInventoryPage{Schema: "private-application-inventory-page.v1", ApplicationSHA256: ApplicationConfigDigest(c), Gaps: []ApplicationSourceGap{{UnitID: d, InputSHA256: d, RevisionSHA256: hash([]byte(c.RevisionAssertion)), Path: "observed.go", Reason: "CONTROLLED_SOURCE_LIMIT"}}}
}
func TestApplicationInterruptedHigherQuartetNotSelected(t *testing.T) {
	c, now := applicationFixture(t)
	a := applicationCreate(t, c, now)
	first, e := a.PublishInventory(applicationGapFixture(c, "first"), struct{ Next string }{"first"})
	if e != nil {
		t.Fatal(e)
	}
	cp, e := a.VerifyInventory(first)
	if e != nil {
		t.Fatal(e)
	}
	before := map[string][]byte{}
	j := a.implementation
	for _, p := range []string{cp.MachinePaths[0], cp.MachinePaths[1], cp.HumanPaths[0], cp.HumanPaths[1]} {
		b, e := j.read(p, ApplicationRecordLimit)
		if e != nil {
			t.Fatal(e)
		}
		before[p] = b
	}
	higher := applicationGapFixture(c, "higher")
	higher.Previous = first
	higher.FeatureIdentity = "UNRESOLVED"
	higher.SourceCompleteness = "UNKNOWN"
	higher.AbsenceInference = "PROHIBITED"
	higher.NavigationStatus = "UNASSIGNED_NAVIGATION_BACKLOG_NOT_COMPUTED"
	raw, e := applicationMarshal(higher, ApplicationRecordLimit)
	if e != nil {
		t.Fatal(e)
	}
	id := hash(raw)
	if e = j.putObject(applicationObject{applicationObjectName("human-1", id), []byte("interrupted higher publication")}); e != nil {
		t.Fatal(e)
	}
	if _, e = a.PublishInventory(higher, struct{ Next string }{"higher"}); e == nil {
		t.Fatal("ASSERT_APPLICATION_INTERRUPTED_QUARTET_REFUSED")
	}
	if a.Head().LastCompletePublicationSHA256 != first {
		t.Fatal("ASSERT_APPLICATION_INCOMPLETE_HIGHER_SELECTOR_NOT_ADOPTED")
	}
	for p, old := range before {
		b, e := j.read(p, ApplicationRecordLimit)
		if e != nil || !bytes.Equal(old, b) {
			t.Fatal("ASSERT_APPLICATION_PUBLICATION_FAILURE_PRIOR_BYTES")
		}
	}
	a.Close()
	a, e = OpenApplication(c, time.Now())
	if e != nil {
		t.Fatal(e)
	}
	defer a.Close()
	if a.Head().LastCompletePublicationSHA256 != first {
		t.Fatal("ASSERT_APPLICATION_RESTART_PRIOR_COMPLETE_SELECTOR")
	}
}
func TestApplicationBoundedEncodingBeforeMarshal(t *testing.T) {
	c, now := applicationFixture(t)
	a := applicationCreate(t, c, now)
	if _, e := a.PutRecord(struct{ Data string }{string(bytes.Repeat([]byte{'x'}, ApplicationRecordLimit))}); e == nil {
		t.Fatal("ASSERT_APPLICATION_RECORD_FIELD_GRAPH_PRE_ADMISSION")
	}
	if _, e := a.PutRecord(map[string]string{"unknown": "schema"}); e == nil {
		t.Fatal("ASSERT_APPLICATION_NO_UNBOUNDED_MAP_RECORDS")
	}
	page, e := a.ReserveOperation(hash([]byte("bounded directory page")), "directory_page", ApplicationBounds{Records: 128, ReadHashBytes: 1 << 20, ParserBytes: 1 << 20}, now)
	if e != nil {
		t.Fatal(e)
	}
	for n := 0; n < 32; n++ {
		if _, e = a.HasIndexInPage(hash([]byte{byte(n)}), page); e != nil {
			t.Fatal(e)
		}
	}
	if _, e = a.HasIndexInPage(hash([]byte("overflow")), page); e == nil {
		t.Fatal("ASSERT_APPLICATION_PREPAID_PAGE_LOOKUPS_CANNOT_REPLENISH")
	}
}
