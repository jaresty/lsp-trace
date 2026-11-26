//go:build darwin || linux

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

type traceRepairedShort struct{}

func (traceRepairedShort) Write([]byte) (int, error) { return 0, io.ErrShortWrite }

// No old test/fixture, provider, reserve, trace.Start or trace.Stop is invoked.
func TestTraceRepairedPure(t *testing.T) {
	if trace.IsEnabled() {
		t.Fatal("ASSERT_REPAIRED_PURE_DISABLED")
	}
	t.Run("DefaultOff", func(t *testing.T) {
		t.Setenv("LSP_TRACE_REPAIRED_ENABLE", "")
		if !t.Run("Wrapper", TestConfigurationV3RuntimeTraceRepaired) {
			t.Fatal("ASSERT_REPAIRED_DEFAULT_OFF refused")
		}
		if trace.IsEnabled() {
			t.Fatal("ASSERT_REPAIRED_DEFAULT_OFF tracing")
		}
		t.Log("ASSERT_REPAIRED_DEFAULT_OFF PASS")
	})
	t.Run("Allowance", func(t *testing.T) {
		if len(traceRepairedCases) != 2 || strings.Join(traceRepairedCases[:], ",") != "provider-C-empty,provider-mixed-CR-C-empty" {
			t.Fatal("ASSERT_REPAIRED_ALLOWANCE exact2")
		}
		t.Log("ASSERT_REPAIRED_ALLOWANCE PASS")
	})
	t.Run("SourceFrame", func(t *testing.T) {
		pre := traceRepairedPurePreflight()
		if !traceRepairedSourceFrameValid(pre) {
			t.Fatal("ASSERT_REPAIRED_SOURCE_FRAME valid")
		}
		for _, bad := range [][]byte{nil, []byte(`{}`), []byte(`{"source_frame":[]}`), []byte(strings.Replace(string(pre), "5293bca9", "00000000", 1)), []byte(strings.Replace(string(pre), "dad4edd1", "00000000", 1)), []byte(strings.Replace(string(pre), "typed_boundary.go", "old_boundary.go", 1))} {
			if traceRepairedSourceFrameValid(bad) {
				t.Fatal("ASSERT_REPAIRED_SOURCE_FRAME missing-or-stale")
			}
		}
		var frame struct {
			SourceFrame []struct {
				Path string `json:"path"`
				SHA  string `json:"sha256"`
			} `json:"source_frame"`
		}
		if json.Unmarshal(pre, &frame) != nil || len(frame.SourceFrame) != 5 {
			t.Fatal("ASSERT_REPAIRED_SOURCE_FRAME fixture")
		}
		for _, binding := range frame.SourceFrame {
			for _, bad := range []string{strings.Replace(string(pre), binding.SHA, strings.Repeat("0", 64), 1), strings.Replace(string(pre), binding.Path, "missing/"+binding.Path, 1)} {
				if traceRepairedSourceFrameValid([]byte(bad)) {
					t.Fatal("ASSERT_REPAIRED_SOURCE_FRAME mandatory05and09", binding.Path)
				}
			}
		}
		t.Log("ASSERT_REPAIRED_SOURCE_FRAME PASS")
	})
	t.Run("Selection", func(t *testing.T) {
		for _, id := range traceRepairedCases {
			run := traceRepairedRun(id)
			args := []string{"-test.run=" + run, "-test.count=1", "-test.timeout=30s", "-test.v=true"}
			if !traceRepairedSelection(id, run, traceRepairedLedger, traceRepairedPhase, args) {
				t.Fatal("ASSERT_REPAIRED_SELECTION valid")
			}
			for _, bad := range []string{traceRepairedOldLedger, traceTwoLedger, "", traceRepairedLedger + "/../preparations.jsonl"} {
				if traceRepairedSelection(id, run, bad, traceRepairedPhase, args) {
					t.Fatal("ASSERT_REPAIRED_SELECTION oldpath")
				}
			}
			if traceRepairedSelection(id, run+"|.*", traceRepairedLedger, traceRepairedPhase, args) || traceRepairedSelection(id, run, traceRepairedLedger, "old-phase", args) || traceRepairedSelection(id, run, traceRepairedLedger, traceRepairedPhase, append(args, "-test.trace=x")) {
				t.Fatal("ASSERT_REPAIRED_SELECTION exact")
			}
			parts := strings.Split(run, "/")
			if len(parts) != 2 || !regexp.MustCompile(parts[0]).MatchString("TestConfigurationV3RuntimeTraceRepaired") {
				t.Fatal("ASSERT_REPAIRED_SELECTION parent")
			}
			matches := 0
			for _, other := range append(traceRepairedCases[:], "provider-R-empty", "provider-C-unavailable", "provider-C-empty-extra") {
				if regexp.MustCompile(parts[1]).MatchString(other) {
					matches++
					if other != id {
						t.Fatal("ASSERT_REPAIRED_SELECTION sibling")
					}
				}
			}
			if matches != 1 {
				t.Fatal("ASSERT_REPAIRED_SELECTION count")
			}
		}
		for _, bad := range []string{"", "provider-R-empty", "provider-C-composed-duplicates", "provider-mixed-CR-C-unavailable", "provider-mixed-RD-R-unavailable", "provider-C-empty-extra", "../provider-C-empty", "provider-C-empty|.*"} {
			if traceRepairedCase(bad) {
				t.Fatal("ASSERT_REPAIRED_SELECTION whitelist")
			}
		}
		raw, _ := json.Marshal(map[string]string{"run": traceRepairedPhase, "case": traceRepairedCases[0], "event": "CONTROLLED_PREPARATION_ATTEMPT"})
		if !traceRepairedLedgerValid(nil, traceRepairedCases[0]) || !traceRepairedLedgerValid(append(raw, '\n'), traceRepairedCases[1]) || traceRepairedLedgerValid(raw, traceRepairedCases[0]) || traceRepairedLedgerValid(append(append(raw, '\n'), raw...), traceRepairedCases[1]) || traceRepairedLedgerValid([]byte("bad"), traceRepairedCases[0]) {
			t.Fatal("ASSERT_REPAIRED_SELECTION ledger")
		}
		t.Log("ASSERT_REPAIRED_SELECTION PASS")
	})
	t.Run("Cap", func(t *testing.T) {
		var b bytes.Buffer
		w := &traceRepairedWriter{dst: &b, limit: 3}
		if n, e := w.Write([]byte("abc")); n != 3 || e != nil {
			t.Fatal("ASSERT_REPAIRED_CAP equality")
		}
		if n, e := w.Write([]byte("x")); n != 0 || e != errTraceRepairedCap {
			t.Fatal("ASSERT_REPAIRED_CAP overflow")
		}
		if n, e := w.Write([]byte("x")); n != 0 || e == nil {
			t.Fatal("ASSERT_REPAIRED_CAP sticky")
		}
		if b.String() != "abc" || !w.overflowed() {
			t.Fatal("ASSERT_REPAIRED_CAP persistence")
		}
		b.Reset()
		w = &traceRepairedWriter{dst: &b, limit: 3}
		if n, e := w.Write([]byte("abcde")); n != 3 || e != errTraceRepairedCap || b.Len() != 3 {
			t.Fatal("ASSERT_REPAIRED_CAP partial")
		}
		w = &traceRepairedWriter{dst: traceRepairedShort{}, limit: 3}
		if _, e := w.Write([]byte("x")); e == nil {
			t.Fatal("ASSERT_REPAIRED_CAP short")
		}
		if _, bad := w.state(); !bad || w.overflowed() {
			t.Fatal("ASSERT_REPAIRED_CAP failure")
		}
		w = &traceRepairedWriter{dst: &b, limit: -1}
		if _, e := w.Write(nil); e != errTraceRepairedIO {
			t.Fatal("ASSERT_REPAIRED_CAP invalid")
		}
		if traceRepairedCap != 8<<20 || len(traceRepairedCases)*int(traceRepairedCap) != 16<<20 {
			t.Fatal("ASSERT_REPAIRED_CAP aggregate")
		}
		t.Log("ASSERT_REPAIRED_CAP PASS")
	})
	t.Run("Slots", func(t *testing.T) {
		dir := t.TempDir()
		if os.Chmod(dir, 0700) != nil {
			t.Fatal("ASSERT_REPAIRED_SLOTS setup")
		}
		for _, id := range traceRepairedCases {
			if traceRepairedClaim(dir, id) != nil {
				t.Fatal("ASSERT_REPAIRED_SLOTS fresh")
			}
			if traceRepairedClaim(dir, id) == nil {
				t.Fatal("ASSERT_REPAIRED_SLOTS duplicate")
			}
		}
		if traceRepairedClaim(dir, "provider-R-empty") == nil {
			t.Fatal("ASSERT_REPAIRED_SLOTS third")
		}
		other := t.TempDir()
		_ = os.Chmod(other, 0700)
		traceRepairedStop(other)
		if traceRepairedClaim(other, traceRepairedCases[0]) == nil {
			t.Fatal("ASSERT_REPAIRED_SLOTS stopped")
		}
		other = t.TempDir()
		_ = os.Chmod(other, 0700)
		if os.WriteFile(filepath.Join(other, "unknown.used"), nil, 0600) != nil {
			t.Fatal("ASSERT_REPAIRED_SLOTS setup")
		}
		if traceRepairedClaim(other, traceRepairedCases[0]) == nil {
			t.Fatal("ASSERT_REPAIRED_SLOTS foreign")
		}
		t.Log("ASSERT_REPAIRED_SLOTS PASS")
	})
	t.Run("Release", func(t *testing.T) {
		pre := traceRepairedPurePreflight()
		q := traceRepairedRelease{Phase: traceRepairedPhase, PreflightSHA: traceRepairedSHA(pre), AllowNative: true, Cases: append([]string(nil), traceRepairedCases[:]...), MaxAttempts: 2, TraceCap: traceRepairedCap, TraceTotal: 2 * traceRepairedCap, Expires: 1100}
		if !traceRepairedReleaseValid(q, pre, 1000) {
			t.Fatal("ASSERT_REPAIRED_RELEASE valid")
		}
		q.MaxAttempts = 3
		if traceRepairedReleaseValid(q, pre, 1000) {
			t.Fatal("ASSERT_REPAIRED_RELEASE ceiling")
		}
		q.MaxAttempts = 2
		q.TraceTotal = 40 << 20
		if traceRepairedReleaseValid(q, pre, 1000) {
			t.Fatal("ASSERT_REPAIRED_RELEASE aggregate")
		}
		q.TraceTotal = 16 << 20
		q.Phase = "runtime-trace-five-04"
		if traceRepairedReleaseValid(q, pre, 1000) {
			t.Fatal("ASSERT_REPAIRED_RELEASE oldphase")
		}
		q.Phase = traceRepairedPhase
		q.AllowNative = false
		if traceRepairedReleaseValid(q, pre, 1000) {
			t.Fatal("ASSERT_REPAIRED_RELEASE missing")
		}
		q.AllowNative = true
		q.Expires = 1000
		if traceRepairedReleaseValid(q, pre, 1000) {
			t.Fatal("ASSERT_REPAIRED_RELEASE expired")
		}
		q.Expires = 2801
		if traceRepairedReleaseValid(q, pre, 1000) {
			t.Fatal("ASSERT_REPAIRED_RELEASE retention")
		}
		q.Expires = 1100
		q.TraceCap++
		if traceRepairedReleaseValid(q, pre, 1000) {
			t.Fatal("ASSERT_REPAIRED_RELEASE cap")
		}
		q.TraceCap--
		if traceRepairedReleaseValid(q, []byte("different"), 1000) {
			t.Fatal("ASSERT_REPAIRED_RELEASE source")
		}
		t.Log("ASSERT_REPAIRED_RELEASE PASS")
	})
	if trace.IsEnabled() {
		t.Fatal("ASSERT_REPAIRED_PURE_DISABLED")
	}
}

func traceRepairedPurePreflight() []byte {
	return []byte(`{"source_frame":[{"path":"internal/programcadmission/typed_boundary.go","sha256":"5293bca9a78339c7a371cd34970c7da26dfe0088ac5260c94cb2b2861b41a4bb"},{"path":"internal/programcadmission/empty_admission_clone_regression_test.go","sha256":"dad4edd1d508beb6fb533c313ab9268270ffa53f68b50e2313b85aa67a9b7f23"},{"path":"internal/referencegrouping/input.go","sha256":"36f784ae1465eb2477799c610fa6e95226e5f5b40fe2d596984a9c19e912d316"},{"path":"internal/referencegrouping/empty_calls_predecessors_pure_test.go","sha256":"961d30c5afcdebd4885a7f076f057a8622795682e19e329c83d50069baa43478"},{"path":"internal/referencegrouping/empty_calls_native_codec_pure_test.go","sha256":"f43aaa6f6381e7d4155251577b4c6ddd9b4378c7454099cb69adeb028477912e"}]}`)
}
