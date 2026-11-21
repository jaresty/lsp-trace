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

type traceFiveShort struct{}

func (traceFiveShort) Write([]byte) (int, error) { return 0, io.ErrShortWrite }

// No old test/fixture, provider, reserve, trace.Start or trace.Stop is invoked.
func TestTraceFivePure(t *testing.T) {
	if trace.IsEnabled() {
		t.Fatal("ASSERT_FIVE_PURE_DISABLED")
	}
	t.Run("Selection", func(t *testing.T) {
		for _, id := range traceFiveCases {
			run := traceFiveRun(id)
			args := []string{"-test.run=" + run, "-test.count=1", "-test.timeout=30s", "-test.v=true"}
			if !traceFiveSelection(id, run, traceFiveLedger, traceFivePhase, args) {
				t.Fatal("ASSERT_FIVE_SELECTION valid")
			}
			for _, bad := range []string{traceFiveOldLedger, "", traceFiveLedger + "/../preparations.jsonl"} {
				if traceFiveSelection(id, run, bad, traceFivePhase, args) {
					t.Fatal("ASSERT_FIVE_SELECTION oldpath")
				}
			}
			if traceFiveSelection(id, run+"|.*", traceFiveLedger, traceFivePhase, args) || traceFiveSelection(id, run, traceFiveLedger, "old-phase", args) || traceFiveSelection(id, run, traceFiveLedger, traceFivePhase, append(args, "-test.trace=x")) {
				t.Fatal("ASSERT_FIVE_SELECTION exact")
			}
			parts := strings.Split(run, "/")
			if len(parts) != 2 || !regexp.MustCompile(parts[0]).MatchString("TestConfigurationV3RuntimeTraceFive") {
				t.Fatal("ASSERT_FIVE_SELECTION parent")
			}
			matches := 0
			for _, other := range append(traceFiveCases[:], "provider-R-empty", "provider-C-unavailable", "provider-C-empty-extra") {
				if regexp.MustCompile(parts[1]).MatchString(other) {
					matches++
					if other != id {
						t.Fatal("ASSERT_FIVE_SELECTION sibling")
					}
				}
			}
			if matches != 1 {
				t.Fatal("ASSERT_FIVE_SELECTION count")
			}
		}
		for _, bad := range []string{"", "provider-R-empty", "provider-C-empty-extra", "../provider-C-empty", "provider-C-empty|.*"} {
			if traceFiveCase(bad) {
				t.Fatal("ASSERT_FIVE_SELECTION whitelist")
			}
		}
		raw, _ := json.Marshal(map[string]string{"run": traceFivePhase, "case": traceFiveCases[0], "event": "CONTROLLED_PREPARATION_ATTEMPT"})
		if !traceFiveLedgerValid(nil, traceFiveCases[0]) || !traceFiveLedgerValid(append(raw, '\n'), traceFiveCases[1]) || traceFiveLedgerValid(raw, traceFiveCases[0]) || traceFiveLedgerValid(append(append(raw, '\n'), raw...), traceFiveCases[1]) || traceFiveLedgerValid([]byte("bad"), traceFiveCases[0]) {
			t.Fatal("ASSERT_FIVE_SELECTION ledger")
		}
		t.Log("ASSERT_FIVE_SELECTION PASS")
	})
	t.Run("Cap", func(t *testing.T) {
		var b bytes.Buffer
		w := &traceFiveWriter{dst: &b, limit: 3}
		if n, e := w.Write([]byte("abc")); n != 3 || e != nil {
			t.Fatal("ASSERT_FIVE_CAP equality")
		}
		if n, e := w.Write([]byte("x")); n != 0 || e != errTraceFiveCap {
			t.Fatal("ASSERT_FIVE_CAP overflow")
		}
		if n, e := w.Write([]byte("x")); n != 0 || e == nil {
			t.Fatal("ASSERT_FIVE_CAP sticky")
		}
		if b.String() != "abc" || !w.overflowed() {
			t.Fatal("ASSERT_FIVE_CAP persistence")
		}
		b.Reset()
		w = &traceFiveWriter{dst: &b, limit: 3}
		if n, e := w.Write([]byte("abcde")); n != 3 || e != errTraceFiveCap || b.Len() != 3 {
			t.Fatal("ASSERT_FIVE_CAP partial")
		}
		w = &traceFiveWriter{dst: traceFiveShort{}, limit: 3}
		if _, e := w.Write([]byte("x")); e == nil {
			t.Fatal("ASSERT_FIVE_CAP short")
		}
		if _, bad := w.state(); !bad || w.overflowed() {
			t.Fatal("ASSERT_FIVE_CAP failure")
		}
		w = &traceFiveWriter{dst: &b, limit: -1}
		if _, e := w.Write(nil); e != errTraceFiveIO {
			t.Fatal("ASSERT_FIVE_CAP invalid")
		}
		if traceFiveCap != 8<<20 || len(traceFiveCases)*int(traceFiveCap) != 40<<20 {
			t.Fatal("ASSERT_FIVE_CAP aggregate")
		}
		t.Log("ASSERT_FIVE_CAP PASS")
	})
	t.Run("Slots", func(t *testing.T) {
		dir := t.TempDir()
		if os.Chmod(dir, 0700) != nil {
			t.Fatal("ASSERT_FIVE_SLOTS setup")
		}
		for _, id := range traceFiveCases {
			if traceFiveClaim(dir, id) != nil {
				t.Fatal("ASSERT_FIVE_SLOTS fresh")
			}
			if traceFiveClaim(dir, id) == nil {
				t.Fatal("ASSERT_FIVE_SLOTS duplicate")
			}
		}
		if traceFiveClaim(dir, "provider-R-empty") == nil {
			t.Fatal("ASSERT_FIVE_SLOTS sixth")
		}
		other := t.TempDir()
		_ = os.Chmod(other, 0700)
		traceFiveStop(other)
		if traceFiveClaim(other, traceFiveCases[0]) == nil {
			t.Fatal("ASSERT_FIVE_SLOTS stopped")
		}
		other = t.TempDir()
		_ = os.Chmod(other, 0700)
		if os.WriteFile(filepath.Join(other, "unknown.used"), nil, 0600) != nil {
			t.Fatal("ASSERT_FIVE_SLOTS setup")
		}
		if traceFiveClaim(other, traceFiveCases[0]) == nil {
			t.Fatal("ASSERT_FIVE_SLOTS foreign")
		}
		t.Log("ASSERT_FIVE_SLOTS PASS")
	})
	t.Run("Release", func(t *testing.T) {
		pre := []byte("frozen-preflight")
		q := traceFiveRelease{Phase: traceFivePhase, PreflightSHA: traceFiveSHA(pre), AllowNative: true, Cases: append([]string(nil), traceFiveCases[:]...), MaxAttempts: 5, TraceCap: traceFiveCap, TraceTotal: 5 * traceFiveCap, Expires: 1100}
		if !traceFiveReleaseValid(q, pre, 1000) {
			t.Fatal("ASSERT_FIVE_RELEASE valid")
		}
		q.AllowNative = false
		if traceFiveReleaseValid(q, pre, 1000) {
			t.Fatal("ASSERT_FIVE_RELEASE missing")
		}
		q.AllowNative = true
		q.Expires = 1000
		if traceFiveReleaseValid(q, pre, 1000) {
			t.Fatal("ASSERT_FIVE_RELEASE expired")
		}
		q.Expires = 2801
		if traceFiveReleaseValid(q, pre, 1000) {
			t.Fatal("ASSERT_FIVE_RELEASE retention")
		}
		q.Expires = 1100
		q.TraceCap++
		if traceFiveReleaseValid(q, pre, 1000) {
			t.Fatal("ASSERT_FIVE_RELEASE cap")
		}
		q.TraceCap--
		if traceFiveReleaseValid(q, []byte("different"), 1000) {
			t.Fatal("ASSERT_FIVE_RELEASE source")
		}
		t.Log("ASSERT_FIVE_RELEASE PASS")
	})
	if trace.IsEnabled() {
		t.Fatal("ASSERT_FIVE_PURE_DISABLED")
	}
}
