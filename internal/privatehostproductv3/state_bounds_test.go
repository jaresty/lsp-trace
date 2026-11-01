//go:build darwin || linux

package privatehostproductv3

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestHostProductProgressAndReportBoundBeforePublication(t *testing.T) {
	c := testConfig(t)
	if Initialize(c) != nil {
		t.Fatal("init")
	}
	s, e := openStore(c)
	if e != nil {
		t.Fatal(e)
	}
	defer s.close()
	p := progress{Config: c, Observations: []preparedObservation{{Bodies: []ownBody{{Body: strings.Repeat("<", stateLimit)}}}}}
	if e = s.save(p); !errors.Is(e, ErrPaused) || s.stateSeq != 0 {
		t.Fatal("ASSERT_PROGRESS_BOUND_BEFORE_PUBLICATION", e)
	}
	d, e := s.root.Open("pages")
	if e != nil {
		t.Fatal(e)
	}
	entries, e := d.Readdirnames(-1)
	d.Close()
	if e != nil || len(entries) != 0 {
		t.Fatal("ASSERT_OVERSIZED_PROGRESS_NO_PARTIAL_RECORD")
	}
	r := Report{Status: "FAILED", Candidates: []Candidate{{Explanation: strings.Repeat("<", stateLimit)}}}
	if e = s.deliver(&r); !errors.Is(e, ErrPaused) {
		t.Fatal("ASSERT_REPORT_BOUND_BEFORE_PUBLICATION", e)
	}
	if _, e = s.root.Stat("result/report-00000001.json"); !os.IsNotExist(e) {
		t.Fatal("ASSERT_OVERSIZED_REPORT_NO_PARTIAL_PAIR")
	}
}
