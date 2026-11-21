package inventorysuccessor_test

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime/trace"
	"strings"
	"testing"
)

type traceTwoShort struct{}

func (traceTwoShort) Write([]byte) (int, error) { return 0, io.ErrShortWrite }

// No old test/fixture, provider, reserve, trace.Start or trace.Stop is invoked.
func TestTraceTwoPure(t *testing.T) {
	if trace.IsEnabled() {
		t.Fatal("ASSERT_TWO_PURE_DISABLED")
	}
	t.Run("Allowance", func(t *testing.T) {
		if len(traceTwoCases) != 2 || strings.Join(traceTwoCases[:], ",") != "provider-C-empty,provider-mixed-CR-C-empty" {
			t.Fatal("ASSERT_TWO_ALLOWANCE exact2")
		}
		t.Log("ASSERT_TWO_ALLOWANCE PASS")
	})
	t.Run("SourceFrame", func(t *testing.T) {
		pre := traceTwoPurePreflight()
		if !traceTwoSourceFrameValid(pre) {
			t.Fatal("ASSERT_TWO_SOURCE_FRAME valid")
		}
		for _, bad := range [][]byte{nil, []byte(`{}`), []byte(`{"source_frame":[]}`), []byte(strings.Replace(string(pre), "5293bca9", "00000000", 1)), []byte(strings.Replace(string(pre), "dad4edd1", "00000000", 1)), []byte(strings.Replace(string(pre), "typed_boundary.go", "old_boundary.go", 1))} {
			if traceTwoSourceFrameValid(bad) {
				t.Fatal("ASSERT_TWO_SOURCE_FRAME missing-or-stale")
			}
		}
		t.Log("ASSERT_TWO_SOURCE_FRAME PASS")
	})
	t.Run("Selection", func(t *testing.T) {
		for _, id := range traceTwoCases {
			run := traceTwoRun(id)
			args := []string{"-test.run=" + run, "-test.count=1", "-test.timeout=30s", "-test.v=true"}
			if !traceTwoSelection(id, run, traceTwoLedger, traceTwoPhase, args) {
				t.Fatal("ASSERT_TWO_SELECTION valid")
			}
			for _, bad := range []string{traceTwoOldLedger, "", traceTwoLedger + "/../preparations.jsonl"} {
				if traceTwoSelection(id, run, bad, traceTwoPhase, args) {
					t.Fatal("ASSERT_TWO_SELECTION oldpath")
				}
			}
			if traceTwoSelection(id, run+"|.*", traceTwoLedger, traceTwoPhase, args) || traceTwoSelection(id, run, traceTwoLedger, "old-phase", args) || traceTwoSelection(id, run, traceTwoLedger, traceTwoPhase, append(args, "-test.trace=x")) {
				t.Fatal("ASSERT_TWO_SELECTION exact")
			}
			parts := strings.Split(run, "/")
			if len(parts) != 2 || !regexp.MustCompile(parts[0]).MatchString("TestConfigurationV3RuntimeTraceTwo") {
				t.Fatal("ASSERT_TWO_SELECTION parent")
			}
			matches := 0
			for _, other := range append(traceTwoCases[:], "provider-R-empty", "provider-C-unavailable", "provider-C-empty-extra") {
				if regexp.MustCompile(parts[1]).MatchString(other) {
					matches++
					if other != id {
						t.Fatal("ASSERT_TWO_SELECTION sibling")
					}
				}
			}
			if matches != 1 {
				t.Fatal("ASSERT_TWO_SELECTION count")
			}
		}
		for _, bad := range []string{"", "provider-R-empty", "provider-C-composed-duplicates", "provider-mixed-CR-C-unavailable", "provider-mixed-RD-R-unavailable", "provider-C-empty-extra", "../provider-C-empty", "provider-C-empty|.*"} {
			if traceTwoCase(bad) {
				t.Fatal("ASSERT_TWO_SELECTION whitelist")
			}
		}
		raw, _ := json.Marshal(map[string]string{"run": traceTwoPhase, "case": traceTwoCases[0], "event": "CONTROLLED_PREPARATION_ATTEMPT"})
		if !traceTwoLedgerValid(nil, traceTwoCases[0]) || !traceTwoLedgerValid(append(raw, '\n'), traceTwoCases[1]) || traceTwoLedgerValid(raw, traceTwoCases[0]) || traceTwoLedgerValid(append(append(raw, '\n'), raw...), traceTwoCases[1]) || traceTwoLedgerValid([]byte("bad"), traceTwoCases[0]) {
			t.Fatal("ASSERT_TWO_SELECTION ledger")
		}
		t.Log("ASSERT_TWO_SELECTION PASS")
	})
	t.Run("Cap", func(t *testing.T) {
		var b bytes.Buffer
		w := &traceTwoWriter{dst: &b, limit: 3}
		if n, e := w.Write([]byte("abc")); n != 3 || e != nil {
			t.Fatal("ASSERT_TWO_CAP equality")
		}
		if n, e := w.Write([]byte("x")); n != 0 || e != errTraceTwoCap {
			t.Fatal("ASSERT_TWO_CAP overflow")
		}
		if n, e := w.Write([]byte("x")); n != 0 || e == nil {
			t.Fatal("ASSERT_TWO_CAP sticky")
		}
		if b.String() != "abc" || !w.overflowed() {
			t.Fatal("ASSERT_TWO_CAP persistence")
		}
		b.Reset()
		w = &traceTwoWriter{dst: &b, limit: 3}
		if n, e := w.Write([]byte("abcde")); n != 3 || e != errTraceTwoCap || b.Len() != 3 {
			t.Fatal("ASSERT_TWO_CAP partial")
		}
		w = &traceTwoWriter{dst: traceTwoShort{}, limit: 3}
		if _, e := w.Write([]byte("x")); e == nil {
			t.Fatal("ASSERT_TWO_CAP short")
		}
		if _, bad := w.state(); !bad || w.overflowed() {
			t.Fatal("ASSERT_TWO_CAP failure")
		}
		w = &traceTwoWriter{dst: &b, limit: -1}
		if _, e := w.Write(nil); e != errTraceTwoIO {
			t.Fatal("ASSERT_TWO_CAP invalid")
		}
		if traceTwoCap != 8<<20 || len(traceTwoCases)*int(traceTwoCap) != 16<<20 {
			t.Fatal("ASSERT_TWO_CAP aggregate")
		}
		t.Log("ASSERT_TWO_CAP PASS")
	})
	t.Run("Slots", func(t *testing.T) {
		dir := t.TempDir()
		if os.Chmod(dir, 0700) != nil {
			t.Fatal("ASSERT_TWO_SLOTS setup")
		}
		for _, id := range traceTwoCases {
			if traceTwoClaim(dir, id) != nil {
				t.Fatal("ASSERT_TWO_SLOTS fresh")
			}
			if traceTwoClaim(dir, id) == nil {
				t.Fatal("ASSERT_TWO_SLOTS duplicate")
			}
		}
		if traceTwoClaim(dir, "provider-R-empty") == nil {
			t.Fatal("ASSERT_TWO_SLOTS third")
		}
		other := t.TempDir()
		_ = os.Chmod(other, 0700)
		traceTwoStop(other)
		if traceTwoClaim(other, traceTwoCases[0]) == nil {
			t.Fatal("ASSERT_TWO_SLOTS stopped")
		}
		other = t.TempDir()
		_ = os.Chmod(other, 0700)
		if os.WriteFile(filepath.Join(other, "unknown.used"), nil, 0600) != nil {
			t.Fatal("ASSERT_TWO_SLOTS setup")
		}
		if traceTwoClaim(other, traceTwoCases[0]) == nil {
			t.Fatal("ASSERT_TWO_SLOTS foreign")
		}
		t.Log("ASSERT_TWO_SLOTS PASS")
	})
	t.Run("Release", func(t *testing.T) {
		pre := traceTwoPurePreflight()
		q := traceTwoRelease{Phase: traceTwoPhase, PreflightSHA: traceTwoSHA(pre), AllowNative: true, Cases: append([]string(nil), traceTwoCases[:]...), MaxAttempts: 2, TraceCap: traceTwoCap, TraceTotal: 2 * traceTwoCap, Expires: 1100}
		if !traceTwoReleaseValid(q, pre, 1000) {
			t.Fatal("ASSERT_TWO_RELEASE valid")
		}
		q.MaxAttempts = 3
		if traceTwoReleaseValid(q, pre, 1000) {
			t.Fatal("ASSERT_TWO_RELEASE ceiling")
		}
		q.MaxAttempts = 2
		q.TraceTotal = 40 << 20
		if traceTwoReleaseValid(q, pre, 1000) {
			t.Fatal("ASSERT_TWO_RELEASE aggregate")
		}
		q.TraceTotal = 16 << 20
		q.Phase = "runtime-trace-five-04"
		if traceTwoReleaseValid(q, pre, 1000) {
			t.Fatal("ASSERT_TWO_RELEASE oldphase")
		}
		q.Phase = traceTwoPhase
		q.AllowNative = false
		if traceTwoReleaseValid(q, pre, 1000) {
			t.Fatal("ASSERT_TWO_RELEASE missing")
		}
		q.AllowNative = true
		q.Expires = 1000
		if traceTwoReleaseValid(q, pre, 1000) {
			t.Fatal("ASSERT_TWO_RELEASE expired")
		}
		q.Expires = 2801
		if traceTwoReleaseValid(q, pre, 1000) {
			t.Fatal("ASSERT_TWO_RELEASE retention")
		}
		q.Expires = 1100
		q.TraceCap++
		if traceTwoReleaseValid(q, pre, 1000) {
			t.Fatal("ASSERT_TWO_RELEASE cap")
		}
		q.TraceCap--
		if traceTwoReleaseValid(q, []byte("different"), 1000) {
			t.Fatal("ASSERT_TWO_RELEASE source")
		}
		t.Log("ASSERT_TWO_RELEASE PASS")
	})
	if trace.IsEnabled() {
		t.Fatal("ASSERT_TWO_PURE_DISABLED")
	}
}

func traceTwoPurePreflight() []byte {
	return []byte(`{"source_frame":[{"path":"internal/programcadmission/typed_boundary.go","sha256":"5293bca9a78339c7a371cd34970c7da26dfe0088ac5260c94cb2b2861b41a4bb"},{"path":"internal/programcadmission/empty_admission_clone_regression_test.go","sha256":"dad4edd1d508beb6fb533c313ab9268270ffa53f68b50e2313b85aa67a9b7f23"}]}`)
}
