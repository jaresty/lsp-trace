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

type traceDuplicateShort struct{}

func (traceDuplicateShort) Write([]byte) (int, error) { return 0, io.ErrShortWrite }

// No old test/fixture, provider, reserve, trace.Start or trace.Stop is invoked.
func TestTraceDuplicatePure(t *testing.T) {
	if trace.IsEnabled() {
		t.Fatal("ASSERT_DUPLICATE_PURE_DISABLED")
	}
	t.Run("DefaultOff", func(t *testing.T) {
		t.Setenv("LSP_TRACE_DUPLICATE_ENABLE", "")
		if !t.Run("Wrapper", TestConfigurationV3RuntimeTraceDuplicate) {
			t.Fatal("ASSERT_DUPLICATE_DEFAULT_OFF refused")
		}
		if trace.IsEnabled() {
			t.Fatal("ASSERT_DUPLICATE_DEFAULT_OFF tracing")
		}
		t.Log("ASSERT_DUPLICATE_DEFAULT_OFF PASS")
	})
	t.Run("Allowance", func(t *testing.T) {
		if len(traceDuplicateCases) != 1 || strings.Join(traceDuplicateCases[:], ",") != "provider-C-composed-duplicates" || traceDuplicatePhase != "runtime-trace-duplicate-13" || traceDuplicateBase != "/Users/schwa/.local/state/lsp-trace-evidence/seven-configuration-campaign-v3-20261112-01/runtime-trace-duplicate-28f7-13" {
			t.Fatal("ASSERT_DUPLICATE_ALLOWANCE exact1")
		}
		t.Log("ASSERT_DUPLICATE_ALLOWANCE PASS")
	})
	t.Run("SourceFrame", func(t *testing.T) {
		pre := traceDuplicatePurePreflight()
		if !traceDuplicateSourceFrameValid(pre) {
			t.Fatal("ASSERT_DUPLICATE_SOURCE_FRAME valid")
		}
		for _, bad := range [][]byte{nil, []byte(`{}`), []byte(`{"source_frame":[]}`), []byte(strings.Replace(string(pre), "5293bca9", "00000000", 1)), []byte(strings.Replace(string(pre), "dad4edd1", "00000000", 1)), []byte(strings.Replace(string(pre), "typed_boundary.go", "old_boundary.go", 1))} {
			if traceDuplicateSourceFrameValid(bad) {
				t.Fatal("ASSERT_DUPLICATE_SOURCE_FRAME missing-or-stale")
			}
		}
		var frame struct {
			SourceFrame []struct {
				Path string `json:"path"`
				SHA  string `json:"sha256"`
			} `json:"source_frame"`
		}
		if json.Unmarshal(pre, &frame) != nil || len(frame.SourceFrame) != 6 {
			t.Fatal("ASSERT_DUPLICATE_SOURCE_FRAME fixture")
		}
		for _, binding := range frame.SourceFrame {
			for _, bad := range []string{strings.Replace(string(pre), binding.SHA, strings.Repeat("0", 64), 1), strings.Replace(string(pre), binding.Path, "missing/"+binding.Path, 1)} {
				if traceDuplicateSourceFrameValid([]byte(bad)) {
					t.Fatal("ASSERT_DUPLICATE_SOURCE_FRAME mandatory05and09", binding.Path)
				}
			}
		}
		t.Log("ASSERT_DUPLICATE_SOURCE_FRAME PASS")
	})
	t.Run("Selection", func(t *testing.T) {
		for _, id := range traceDuplicateCases {
			run := traceDuplicateRun(id)
			args := []string{"-test.run=" + run, "-test.count=1", "-test.timeout=30s", "-test.v=true"}
			if !traceDuplicateSelection(id, run, traceDuplicateLedger, traceDuplicatePhase, args) {
				t.Fatal("ASSERT_DUPLICATE_SELECTION valid")
			}
			for _, bad := range []string{traceDuplicateOldLedger, traceTwoLedger, "", traceDuplicateLedger + "/../preparations.jsonl"} {
				if traceDuplicateSelection(id, run, bad, traceDuplicatePhase, args) {
					t.Fatal("ASSERT_DUPLICATE_SELECTION oldpath")
				}
			}
			if traceDuplicateSelection(id, run+"|.*", traceDuplicateLedger, traceDuplicatePhase, args) || traceDuplicateSelection(id, run, traceDuplicateLedger, "old-phase", args) || traceDuplicateSelection(id, run, traceDuplicateLedger, traceDuplicatePhase, append(args, "-test.trace=x")) {
				t.Fatal("ASSERT_DUPLICATE_SELECTION exact")
			}
			parts := strings.Split(run, "/")
			if len(parts) != 2 || !regexp.MustCompile(parts[0]).MatchString("TestConfigurationV3RuntimeTraceDuplicate") {
				t.Fatal("ASSERT_DUPLICATE_SELECTION parent")
			}
			matches := 0
			for _, other := range append(traceDuplicateCases[:], "provider-R-empty", "provider-C-unavailable", "provider-C-empty-extra") {
				if regexp.MustCompile(parts[1]).MatchString(other) {
					matches++
					if other != id {
						t.Fatal("ASSERT_DUPLICATE_SELECTION sibling")
					}
				}
			}
			if matches != 1 {
				t.Fatal("ASSERT_DUPLICATE_SELECTION count")
			}
		}
		for _, bad := range []string{"", "provider-R-empty", "provider-C-empty", "provider-mixed-CR-C-unavailable", "provider-mixed-RD-R-unavailable", "provider-C-empty-extra", "../provider-C-empty", "provider-C-empty|.*"} {
			if traceDuplicateCase(bad) {
				t.Fatal("ASSERT_DUPLICATE_SELECTION whitelist")
			}
		}
		raw, _ := json.Marshal(map[string]string{"run": traceDuplicatePhase, "case": traceDuplicateCases[0], "event": "CONTROLLED_PREPARATION_ATTEMPT"})
		if !traceDuplicateLedgerValid(nil, traceDuplicateCases[0]) || traceDuplicateLedgerValid(append(raw, '\n'), traceDuplicateCases[0]) || traceDuplicateLedgerValid(raw, traceDuplicateCases[0]) || traceDuplicateLedgerValid(append(append(raw, '\n'), raw...), traceDuplicateCases[0]) || traceDuplicateLedgerValid([]byte("bad"), traceDuplicateCases[0]) || traceDuplicateLedgerValid([]byte(" "), traceDuplicateCases[0]) {
			t.Fatal("ASSERT_DUPLICATE_SELECTION ledger")
		}
		t.Log("ASSERT_DUPLICATE_SELECTION PASS")
	})
	t.Run("Cap", func(t *testing.T) {
		var b bytes.Buffer
		w := &traceDuplicateWriter{dst: &b, limit: 3}
		if n, e := w.Write([]byte("abc")); n != 3 || e != nil {
			t.Fatal("ASSERT_DUPLICATE_CAP equality")
		}
		if n, e := w.Write([]byte("x")); n != 0 || e != errTraceDuplicateCap {
			t.Fatal("ASSERT_DUPLICATE_CAP overflow")
		}
		if n, e := w.Write([]byte("x")); n != 0 || e == nil {
			t.Fatal("ASSERT_DUPLICATE_CAP sticky")
		}
		if b.String() != "abc" || !w.overflowed() {
			t.Fatal("ASSERT_DUPLICATE_CAP persistence")
		}
		b.Reset()
		w = &traceDuplicateWriter{dst: &b, limit: 3}
		if n, e := w.Write([]byte("abcde")); n != 3 || e != errTraceDuplicateCap || b.Len() != 3 {
			t.Fatal("ASSERT_DUPLICATE_CAP partial")
		}
		w = &traceDuplicateWriter{dst: traceDuplicateShort{}, limit: 3}
		if _, e := w.Write([]byte("x")); e == nil {
			t.Fatal("ASSERT_DUPLICATE_CAP short")
		}
		if _, bad := w.state(); !bad || w.overflowed() {
			t.Fatal("ASSERT_DUPLICATE_CAP failure")
		}
		w = &traceDuplicateWriter{dst: &b, limit: -1}
		if _, e := w.Write(nil); e != errTraceDuplicateIO {
			t.Fatal("ASSERT_DUPLICATE_CAP invalid")
		}
		if traceDuplicateCap != 8<<20 || len(traceDuplicateCases)*int(traceDuplicateCap) != 8<<20 {
			t.Fatal("ASSERT_DUPLICATE_CAP aggregate")
		}
		t.Log("ASSERT_DUPLICATE_CAP PASS")
	})
	t.Run("Slots", func(t *testing.T) {
		dir := t.TempDir()
		if os.Chmod(dir, 0700) != nil {
			t.Fatal("ASSERT_DUPLICATE_SLOTS setup")
		}
		for _, id := range traceDuplicateCases {
			if traceDuplicateClaim(dir, id) != nil {
				t.Fatal("ASSERT_DUPLICATE_SLOTS fresh")
			}
			if traceDuplicateClaim(dir, id) == nil {
				t.Fatal("ASSERT_DUPLICATE_SLOTS duplicate")
			}
		}
		if traceDuplicateClaim(dir, "provider-R-empty") == nil {
			t.Fatal("ASSERT_DUPLICATE_SLOTS third")
		}
		other := t.TempDir()
		_ = os.Chmod(other, 0700)
		traceDuplicateStop(other)
		if traceDuplicateClaim(other, traceDuplicateCases[0]) == nil {
			t.Fatal("ASSERT_DUPLICATE_SLOTS stopped")
		}
		other = t.TempDir()
		_ = os.Chmod(other, 0700)
		if os.WriteFile(filepath.Join(other, "unknown.used"), nil, 0600) != nil {
			t.Fatal("ASSERT_DUPLICATE_SLOTS setup")
		}
		if traceDuplicateClaim(other, traceDuplicateCases[0]) == nil {
			t.Fatal("ASSERT_DUPLICATE_SLOTS foreign")
		}
		t.Log("ASSERT_DUPLICATE_SLOTS PASS")
	})
	t.Run("Release", func(t *testing.T) {
		pre := traceDuplicatePurePreflight()
		q := traceDuplicateRelease{Phase: traceDuplicatePhase, PreflightSHA: traceDuplicateSHA(pre), AllowNative: true, Cases: append([]string(nil), traceDuplicateCases[:]...), MaxAttempts: 1, TraceCap: traceDuplicateCap, TraceTotal: traceDuplicateCap, Expires: 1100}
		if !traceDuplicateReleaseValid(q, pre, 1000) {
			t.Fatal("ASSERT_DUPLICATE_RELEASE valid")
		}
		for _, bad := range [][]string{nil, {"provider-C-composed-duplicates", "provider-C-empty"}, {"provider-C-empty"}} {
			q.Cases = bad
			if traceDuplicateReleaseValid(q, pre, 1000) {
				t.Fatal("ASSERT_DUPLICATE_RELEASE exact-case")
			}
		}
		q.Cases = append([]string(nil), traceDuplicateCases[:]...)
		q.MaxAttempts = 2
		if traceDuplicateReleaseValid(q, pre, 1000) {
			t.Fatal("ASSERT_DUPLICATE_RELEASE ceiling")
		}
		q.MaxAttempts = 1
		q.TraceTotal = 40 << 20
		if traceDuplicateReleaseValid(q, pre, 1000) {
			t.Fatal("ASSERT_DUPLICATE_RELEASE aggregate")
		}
		q.TraceTotal = 8 << 20
		q.Phase = "runtime-trace-five-04"
		if traceDuplicateReleaseValid(q, pre, 1000) {
			t.Fatal("ASSERT_DUPLICATE_RELEASE oldphase")
		}
		q.Phase = traceDuplicatePhase
		q.AllowNative = false
		if traceDuplicateReleaseValid(q, pre, 1000) {
			t.Fatal("ASSERT_DUPLICATE_RELEASE missing")
		}
		q.AllowNative = true
		q.Expires = 1000
		if traceDuplicateReleaseValid(q, pre, 1000) {
			t.Fatal("ASSERT_DUPLICATE_RELEASE expired")
		}
		q.Expires = 2801
		if traceDuplicateReleaseValid(q, pre, 1000) {
			t.Fatal("ASSERT_DUPLICATE_RELEASE retention")
		}
		q.Expires = 1100
		q.TraceCap++
		if traceDuplicateReleaseValid(q, pre, 1000) {
			t.Fatal("ASSERT_DUPLICATE_RELEASE cap")
		}
		q.TraceCap--
		if traceDuplicateReleaseValid(q, []byte("different"), 1000) {
			t.Fatal("ASSERT_DUPLICATE_RELEASE source")
		}
		t.Log("ASSERT_DUPLICATE_RELEASE PASS")
	})
	if trace.IsEnabled() {
		t.Fatal("ASSERT_DUPLICATE_PURE_DISABLED")
	}
}

func traceDuplicatePurePreflight() []byte {
	return []byte(`{"source_frame":[{"path":"internal/programcadmission/typed_boundary.go","sha256":"5293bca9a78339c7a371cd34970c7da26dfe0088ac5260c94cb2b2861b41a4bb"},{"path":"internal/programcadmission/empty_admission_clone_regression_test.go","sha256":"dad4edd1d508beb6fb533c313ab9268270ffa53f68b50e2313b85aa67a9b7f23"},{"path":"internal/referencegrouping/input.go","sha256":"36f784ae1465eb2477799c610fa6e95226e5f5b40fe2d596984a9c19e912d316"},{"path":"internal/referencegrouping/empty_calls_predecessors_pure_test.go","sha256":"961d30c5afcdebd4885a7f076f057a8622795682e19e329c83d50069baa43478"},{"path":"internal/referencegrouping/empty_calls_native_codec_pure_test.go","sha256":"f43aaa6f6381e7d4155251577b4c6ddd9b4378c7454099cb69adeb028477912e"},{"path":"internal/inventorysuccessor/duplicate_calls_assembly_pure_test.go","sha256":"072eebe3e282f24236c121e15eb7fbd9aa85707a468a85dbcfeecd7c9beb094a"}]}`)
}
