//go:build darwin || linux

package privatehostproductv3

import (
	"os"
	"path/filepath"
	"testing"
)

func TestApplicationCompleteSlotRewindCannotResetPrepay(t *testing.T) {
	c, now := applicationFixture(t)
	a := applicationCreate(t, c, now)
	if e := a.AdmitUnit(applicationUnit(c, "original sixteen"), applicationChild(16), now); e != nil {
		t.Fatal(e)
	}
	a.Close()
	if e := os.Truncate(filepath.Join(c.Store, "journal"), applicationSlotBytes); e != nil {
		t.Fatal(e)
	}
	reopened, e := OpenApplication(c, now)
	if e != nil {
		return
	} // Accountable refusal is sufficient; restoring spend is optional.
	defer reopened.Close()
	if reopened.Head().Reserved.Calls < 16 {
		t.Fatal("ASSERT_APPLICATION_COMPLETE_SLOT_REWIND_MUST_NOT_RESET_PREPAY")
	}
}

func TestApplicationMalformedHeaderStillConsumesAttempt(t *testing.T) {
	c, now := applicationFixture(t)
	a := applicationCreate(t, c, now)
	a.Close()
	if e := os.WriteFile(filepath.Join(c.Store, "header.json"), []byte("malformed"), 0600); e != nil {
		t.Fatal(e)
	}
	for n := 0; n < 2; n++ {
		if bad, e := OpenApplication(c, now); e == nil {
			bad.Close()
			t.Fatal("malformed header admitted")
		}
	}
	st, e := os.Stat(filepath.Join(c.Store, "bootstrap"))
	if e != nil {
		t.Fatal(e)
	}
	if st.Size() != 3 {
		t.Fatalf("ASSERT_APPLICATION_MALFORMED_HEADER_ATTEMPTS_DURABLY_CHARGED: attempts=%d", st.Size())
	}
}

func TestApplicationPrepaidReturnSurvivesProcessingPause(t *testing.T) {
	c, now := applicationFixture(t)
	a := applicationCreate(t, c, now)
	u := applicationUnit(c, "retention")
	if e := a.AdmitUnit(u, applicationChild(1), now); e != nil {
		t.Fatal(e)
	}
	r, e := a.ReserveExchange(u.UnitID, 1, hash([]byte("request")), ApplicationBounds{Calls: 1, RequestBytes: 7, ResponseBytes: 1024, WireBytes: 1031}, now)
	if e != nil {
		t.Fatal(e)
	}
	// The caller is entitled to use all still-unreserved parent processing work;
	// receipt retention must draw from the child's already-prepaid envelope.
	remaining := c.Grant.Ceiling.Records - a.Head().Reserved.Records - applicationOverhead().Records
	if _, e = a.ReserveOperation(hash([]byte("remaining processing")), "discovery", ApplicationBounds{Records: remaining}, now); e != nil {
		t.Fatal(e)
	}
	e = a.RecordReturnedBytes(r.EventID, ObservedWork{EventID: r.EventID, RequestBytes: 7, ResponseBytes: 23, ResponseSHA256: hash([]byte("known physical return")), Accounting: "UNKNOWN_HISTORY_KNOWN_RETURNED_BYTES"})
	if e != nil || a.Head().Observed.ResponseBytes != 23 {
		t.Fatalf("ASSERT_APPLICATION_PREPAID_RETURN_CANNOT_BE_LOST_TO_PROCESSING_PAUSE: error=%v known=%d", e, a.Head().Observed.ResponseBytes)
	}
}
