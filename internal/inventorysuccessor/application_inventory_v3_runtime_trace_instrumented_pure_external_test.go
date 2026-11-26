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

type traceInstrumentedShort struct{}

func (traceInstrumentedShort) Write([]byte) (int, error) { return 0, io.ErrShortWrite }

// No old test/fixture, provider, reserve, trace.Start or trace.Stop is invoked.
func TestTraceInstrumentedPure(t *testing.T) {
	if trace.IsEnabled() {
		t.Fatal("ASSERT_DUPLICATE_PURE_DISABLED")
	}
	t.Run("DefaultOff", func(t *testing.T) {
		t.Setenv("LSP_TRACE_INSTRUMENTED_ENABLE", "")
		if !t.Run("Wrapper", TestConfigurationV3RuntimeTraceInstrumented) {
			t.Fatal("ASSERT_DUPLICATE_DEFAULT_OFF refused")
		}
		if trace.IsEnabled() {
			t.Fatal("ASSERT_DUPLICATE_DEFAULT_OFF tracing")
		}
		t.Log("ASSERT_DUPLICATE_DEFAULT_OFF PASS")
	})
	t.Run("Allowance", func(t *testing.T) {
		if len(traceInstrumentedCases) != 1 || strings.Join(traceInstrumentedCases[:], ",") != "provider-C-composed-duplicates" || traceInstrumentedPhase != "runtime-trace-instrumented-15" || traceInstrumentedBase != "/Users/schwa/.local/state/lsp-trace-evidence/seven-configuration-campaign-v3-20261112-01/runtime-trace-instrumented-15" {
			t.Fatal("ASSERT_DUPLICATE_ALLOWANCE exact1")
		}
		t.Log("ASSERT_DUPLICATE_ALLOWANCE PASS")
	})
	t.Run("SourceFrame", func(t *testing.T) {
		pre := traceInstrumentedPurePreflight()
		if !traceInstrumentedSourceFrameValid(pre) {
			t.Fatal("ASSERT_DUPLICATE_SOURCE_FRAME valid")
		}
		for _, bad := range [][]byte{nil, []byte(`{}`), []byte(`{"source_frame":[]}`), []byte(strings.Replace(string(pre), "5293bca9", "00000000", 1)), []byte(strings.Replace(string(pre), "dad4edd1", "00000000", 1)), []byte(strings.Replace(string(pre), "typed_boundary.go", "old_boundary.go", 1))} {
			if traceInstrumentedSourceFrameValid(bad) {
				t.Fatal("ASSERT_DUPLICATE_SOURCE_FRAME missing-or-stale")
			}
		}
		var frame struct {
			SourceFrame []struct {
				Path string `json:"path"`
				SHA  string `json:"sha256"`
			} `json:"source_frame"`
		}
		if json.Unmarshal(pre, &frame) != nil || len(frame.SourceFrame) != 9 {
			t.Fatal("ASSERT_DUPLICATE_SOURCE_FRAME fixture")
		}
		for _, binding := range frame.SourceFrame {
			for _, bad := range []string{strings.Replace(string(pre), binding.SHA, strings.Repeat("0", 64), 1), strings.Replace(string(pre), binding.Path, "missing/"+binding.Path, 1)} {
				if traceInstrumentedSourceFrameValid([]byte(bad)) {
					t.Fatal("ASSERT_DUPLICATE_SOURCE_FRAME mandatory05and09", binding.Path)
				}
			}
		}
		t.Log("ASSERT_DUPLICATE_SOURCE_FRAME PASS")
	})
	t.Run("Selection", func(t *testing.T) {
		for _, id := range traceInstrumentedCases {
			run := traceInstrumentedRun(id)
			args := []string{"-test.run=" + run, "-test.count=1", "-test.timeout=30s", "-test.v=true"}
			if !traceInstrumentedSelection(id, run, traceInstrumentedLedger, traceInstrumentedPhase, args) {
				t.Fatal("ASSERT_DUPLICATE_SELECTION valid")
			}
			for _, bad := range []string{traceInstrumentedOldLedger, traceTwoLedger, "", traceInstrumentedLedger + "/../preparations.jsonl"} {
				if traceInstrumentedSelection(id, run, bad, traceInstrumentedPhase, args) {
					t.Fatal("ASSERT_DUPLICATE_SELECTION oldpath")
				}
			}
			if traceInstrumentedSelection(id, run+"|.*", traceInstrumentedLedger, traceInstrumentedPhase, args) || traceInstrumentedSelection(id, run, traceInstrumentedLedger, "old-phase", args) || traceInstrumentedSelection(id, run, traceInstrumentedLedger, traceInstrumentedPhase, append(args, "-test.trace=x")) {
				t.Fatal("ASSERT_DUPLICATE_SELECTION exact")
			}
			parts := strings.Split(run, "/")
			if len(parts) != 2 || !regexp.MustCompile(parts[0]).MatchString("TestConfigurationV3RuntimeTraceInstrumented") {
				t.Fatal("ASSERT_DUPLICATE_SELECTION parent")
			}
			matches := 0
			for _, other := range append(traceInstrumentedCases[:], "provider-R-empty", "provider-C-unavailable", "provider-C-empty-extra") {
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
			if traceInstrumentedCase(bad) {
				t.Fatal("ASSERT_DUPLICATE_SELECTION whitelist")
			}
		}
		raw, _ := json.Marshal(map[string]string{"run": traceInstrumentedPhase, "case": traceInstrumentedCases[0], "event": "CONTROLLED_PREPARATION_ATTEMPT"})
		if !traceInstrumentedLedgerValid(nil, traceInstrumentedCases[0]) || traceInstrumentedLedgerValid(append(raw, '\n'), traceInstrumentedCases[0]) || traceInstrumentedLedgerValid(raw, traceInstrumentedCases[0]) || traceInstrumentedLedgerValid(append(append(raw, '\n'), raw...), traceInstrumentedCases[0]) || traceInstrumentedLedgerValid([]byte("bad"), traceInstrumentedCases[0]) || traceInstrumentedLedgerValid([]byte(" "), traceInstrumentedCases[0]) {
			t.Fatal("ASSERT_DUPLICATE_SELECTION ledger")
		}
		t.Log("ASSERT_DUPLICATE_SELECTION PASS")
	})
	t.Run("Cap", func(t *testing.T) {
		var b bytes.Buffer
		w := &traceInstrumentedWriter{dst: &b, limit: 3}
		if n, e := w.Write([]byte("abc")); n != 3 || e != nil {
			t.Fatal("ASSERT_DUPLICATE_CAP equality")
		}
		if n, e := w.Write([]byte("x")); n != 0 || e != errTraceInstrumentedCap {
			t.Fatal("ASSERT_DUPLICATE_CAP overflow")
		}
		if n, e := w.Write([]byte("x")); n != 0 || e == nil {
			t.Fatal("ASSERT_DUPLICATE_CAP sticky")
		}
		if b.String() != "abc" || !w.overflowed() {
			t.Fatal("ASSERT_DUPLICATE_CAP persistence")
		}
		b.Reset()
		w = &traceInstrumentedWriter{dst: &b, limit: 3}
		if n, e := w.Write([]byte("abcde")); n != 3 || e != errTraceInstrumentedCap || b.Len() != 3 {
			t.Fatal("ASSERT_DUPLICATE_CAP partial")
		}
		w = &traceInstrumentedWriter{dst: traceInstrumentedShort{}, limit: 3}
		if _, e := w.Write([]byte("x")); e == nil {
			t.Fatal("ASSERT_DUPLICATE_CAP short")
		}
		if _, bad := w.state(); !bad || w.overflowed() {
			t.Fatal("ASSERT_DUPLICATE_CAP failure")
		}
		w = &traceInstrumentedWriter{dst: &b, limit: -1}
		if _, e := w.Write(nil); e != errTraceInstrumentedIO {
			t.Fatal("ASSERT_DUPLICATE_CAP invalid")
		}
		if traceInstrumentedCap != 8<<20 || len(traceInstrumentedCases)*int(traceInstrumentedCap) != 8<<20 {
			t.Fatal("ASSERT_DUPLICATE_CAP aggregate")
		}
		t.Log("ASSERT_DUPLICATE_CAP PASS")
	})
	t.Run("Slots", func(t *testing.T) {
		dir := t.TempDir()
		if os.Chmod(dir, 0700) != nil {
			t.Fatal("ASSERT_DUPLICATE_SLOTS setup")
		}
		for _, id := range traceInstrumentedCases {
			if traceInstrumentedClaim(dir, id) != nil {
				t.Fatal("ASSERT_DUPLICATE_SLOTS fresh")
			}
			if traceInstrumentedClaim(dir, id) == nil {
				t.Fatal("ASSERT_DUPLICATE_SLOTS duplicate")
			}
		}
		if traceInstrumentedClaim(dir, "provider-R-empty") == nil {
			t.Fatal("ASSERT_DUPLICATE_SLOTS third")
		}
		other := t.TempDir()
		_ = os.Chmod(other, 0700)
		traceInstrumentedStop(other)
		if traceInstrumentedClaim(other, traceInstrumentedCases[0]) == nil {
			t.Fatal("ASSERT_DUPLICATE_SLOTS stopped")
		}
		other = t.TempDir()
		_ = os.Chmod(other, 0700)
		if os.WriteFile(filepath.Join(other, "unknown.used"), nil, 0600) != nil {
			t.Fatal("ASSERT_DUPLICATE_SLOTS setup")
		}
		if traceInstrumentedClaim(other, traceInstrumentedCases[0]) == nil {
			t.Fatal("ASSERT_DUPLICATE_SLOTS foreign")
		}
		t.Log("ASSERT_DUPLICATE_SLOTS PASS")
	})
	t.Run("Release", func(t *testing.T) {
		pre := traceInstrumentedPurePreflight()
		q := traceInstrumentedRelease{Phase: traceInstrumentedPhase, PreflightSHA: traceInstrumentedSHA(pre), AllowNative: true, Cases: append([]string(nil), traceInstrumentedCases[:]...), MaxAttempts: 1, TraceCap: traceInstrumentedCap, TraceTotal: traceInstrumentedCap, Expires: 1100}
		if !traceInstrumentedReleaseValid(q, pre, 1000) {
			t.Fatal("ASSERT_DUPLICATE_RELEASE valid")
		}
		for _, bad := range [][]string{nil, {"provider-C-composed-duplicates", "provider-C-empty"}, {"provider-C-empty"}} {
			q.Cases = bad
			if traceInstrumentedReleaseValid(q, pre, 1000) {
				t.Fatal("ASSERT_DUPLICATE_RELEASE exact-case")
			}
		}
		q.Cases = append([]string(nil), traceInstrumentedCases[:]...)
		q.MaxAttempts = 2
		if traceInstrumentedReleaseValid(q, pre, 1000) {
			t.Fatal("ASSERT_DUPLICATE_RELEASE ceiling")
		}
		q.MaxAttempts = 1
		q.TraceTotal = 40 << 20
		if traceInstrumentedReleaseValid(q, pre, 1000) {
			t.Fatal("ASSERT_DUPLICATE_RELEASE aggregate")
		}
		q.TraceTotal = 8 << 20
		q.Phase = "runtime-trace-five-04"
		if traceInstrumentedReleaseValid(q, pre, 1000) {
			t.Fatal("ASSERT_DUPLICATE_RELEASE oldphase")
		}
		q.Phase = traceInstrumentedPhase
		q.AllowNative = false
		if traceInstrumentedReleaseValid(q, pre, 1000) {
			t.Fatal("ASSERT_DUPLICATE_RELEASE missing")
		}
		q.AllowNative = true
		q.Expires = 1000
		if traceInstrumentedReleaseValid(q, pre, 1000) {
			t.Fatal("ASSERT_DUPLICATE_RELEASE expired")
		}
		q.Expires = 2801
		if traceInstrumentedReleaseValid(q, pre, 1000) {
			t.Fatal("ASSERT_DUPLICATE_RELEASE retention")
		}
		q.Expires = 1100
		q.TraceCap++
		if traceInstrumentedReleaseValid(q, pre, 1000) {
			t.Fatal("ASSERT_DUPLICATE_RELEASE cap")
		}
		q.TraceCap--
		if traceInstrumentedReleaseValid(q, []byte("different"), 1000) {
			t.Fatal("ASSERT_DUPLICATE_RELEASE source")
		}
		t.Log("ASSERT_DUPLICATE_RELEASE PASS")
	})
	if trace.IsEnabled() {
		t.Fatal("ASSERT_DUPLICATE_PURE_DISABLED")
	}
}

func traceInstrumentedPurePreflight() []byte {
	return []byte(`{"source_frame":[{"path":"internal/programcadmission/typed_boundary.go","sha256":"5293bca9a78339c7a371cd34970c7da26dfe0088ac5260c94cb2b2861b41a4bb"},{"path":"internal/programcadmission/empty_admission_clone_regression_test.go","sha256":"dad4edd1d508beb6fb533c313ab9268270ffa53f68b50e2313b85aa67a9b7f23"},{"path":"internal/referencegrouping/input.go","sha256":"36f784ae1465eb2477799c610fa6e95226e5f5b40fe2d596984a9c19e912d316"},{"path":"internal/referencegrouping/empty_calls_predecessors_pure_test.go","sha256":"961d30c5afcdebd4885a7f076f057a8622795682e19e329c83d50069baa43478"},{"path":"internal/referencegrouping/empty_calls_native_codec_pure_test.go","sha256":"f43aaa6f6381e7d4155251577b4c6ddd9b4378c7454099cb69adeb028477912e"},{"path":"internal/inventorysuccessor/duplicate_calls_assembly_pure_test.go","sha256":"072eebe3e282f24236c121e15eb7fbd9aa85707a468a85dbcfeecd7c9beb094a"},{"path":"internal/inventorysuccessor/private_managed_input_operation_v2.go","sha256":"f146c1e45b74522efad317ad483e6fd0c75841303671806b3c65ae265c3271d0"},{"path":"internal/inventorytracediag/trace.go","sha256":"0c6fd9cfddf9576a758d08ab62494c039b95bcce8acdea6847d36bd876cb3613"},{"path":"internal/inventorytracediag/phase14_test.go","sha256":"1b4e9d4f8a4b8f7298f609f6fb84c19bb470621388fb4dc99ba21f1d369b83ed"}],"diagnostic_codes":["COMPOSITE_BINDING_BOUND_REFUSED","COMPOSITE_BINDING_JSON_REFUSED","NATIVE_PREDECESSOR_COUNT_REFUSED","NATIVE_PREDECESSOR_BYTES_REFUSED","NATIVE_AGGREGATE_BOUND_REFUSED"]}`)
}

func TestTraceInstrumentedPhase15Guards(t *testing.T) {
	if trace.IsEnabled() {
		t.Fatal("ASSERT_INSTRUMENTED_DISABLED")
	}
	t.Run("COMPOSITE_BINDING_BOUND_REFUSED", func(t *testing.T) {
		found := false
		for _, c := range traceInstrumentedDiagnosticCodes {
			if c == "COMPOSITE_BINDING_BOUND_REFUSED" {
				found = true
			}
		}
		if !found {
			t.Fatal("ASSERT_INSTRUMENTED_CODE_COMPOSITE_BINDING_BOUND_REFUSED missing")
		}
		t.Log("ASSERT_INSTRUMENTED_CODE_COMPOSITE_BINDING_BOUND_REFUSED PASS")
	})
	t.Run("COMPOSITE_BINDING_JSON_REFUSED", func(t *testing.T) {
		found := false
		for _, c := range traceInstrumentedDiagnosticCodes {
			if c == "COMPOSITE_BINDING_JSON_REFUSED" {
				found = true
			}
		}
		if !found {
			t.Fatal("ASSERT_INSTRUMENTED_CODE_COMPOSITE_BINDING_JSON_REFUSED missing")
		}
		t.Log("ASSERT_INSTRUMENTED_CODE_COMPOSITE_BINDING_JSON_REFUSED PASS")
	})
	t.Run("NATIVE_PREDECESSOR_COUNT_REFUSED", func(t *testing.T) {
		found := false
		for _, c := range traceInstrumentedDiagnosticCodes {
			if c == "NATIVE_PREDECESSOR_COUNT_REFUSED" {
				found = true
			}
		}
		if !found {
			t.Fatal("ASSERT_INSTRUMENTED_CODE_NATIVE_PREDECESSOR_COUNT_REFUSED missing")
		}
		t.Log("ASSERT_INSTRUMENTED_CODE_NATIVE_PREDECESSOR_COUNT_REFUSED PASS")
	})
	t.Run("NATIVE_PREDECESSOR_BYTES_REFUSED", func(t *testing.T) {
		found := false
		for _, c := range traceInstrumentedDiagnosticCodes {
			if c == "NATIVE_PREDECESSOR_BYTES_REFUSED" {
				found = true
			}
		}
		if !found {
			t.Fatal("ASSERT_INSTRUMENTED_CODE_NATIVE_PREDECESSOR_BYTES_REFUSED missing")
		}
		t.Log("ASSERT_INSTRUMENTED_CODE_NATIVE_PREDECESSOR_BYTES_REFUSED PASS")
	})
	t.Run("NATIVE_AGGREGATE_BOUND_REFUSED", func(t *testing.T) {
		found := false
		for _, c := range traceInstrumentedDiagnosticCodes {
			if c == "NATIVE_AGGREGATE_BOUND_REFUSED" {
				found = true
			}
		}
		if !found {
			t.Fatal("ASSERT_INSTRUMENTED_CODE_NATIVE_AGGREGATE_BOUND_REFUSED missing")
		}
		t.Log("ASSERT_INSTRUMENTED_CODE_NATIVE_AGGREGATE_BOUND_REFUSED PASS")
	})
	var fixture map[string]json.RawMessage
	if json.Unmarshal(traceInstrumentedPurePreflight(), &fixture) != nil {
		t.Fatal("fixture")
	}
	var frame []map[string]string
	_ = json.Unmarshal(fixture["source_frame"], &frame)
	t.Run("Pin_private_managed_input_operation_v2_go", func(t *testing.T) {
		for _, mutation := range []string{"missing", "stale"} {
			var changed []map[string]string
			for _, entry := range frame {
				e := map[string]string{"path": entry["path"], "sha256": entry["sha256"]}
				if e["path"] == "internal/inventorysuccessor/private_managed_input_operation_v2.go" {
					if mutation == "missing" {
						continue
					}
					e["sha256"] = strings.Repeat("0", 64)
				}
				changed = append(changed, e)
			}
			raw, _ := json.Marshal(map[string]any{"source_frame": changed, "diagnostic_codes": traceInstrumentedDiagnosticCodes})
			if traceInstrumentedSourceFrameValid(raw) {
				t.Fatal("ASSERT_INSTRUMENTED_PIN_private_managed_input_operation_v2_go " + mutation + " accepted")
			}
			t.Log("ASSERT_INSTRUMENTED_PIN_private_managed_input_operation_v2_go " + mutation + " REJECTED")
		}
	})
	t.Run("Pin_trace_go", func(t *testing.T) {
		for _, mutation := range []string{"missing", "stale"} {
			var changed []map[string]string
			for _, entry := range frame {
				e := map[string]string{"path": entry["path"], "sha256": entry["sha256"]}
				if e["path"] == "internal/inventorytracediag/trace.go" {
					if mutation == "missing" {
						continue
					}
					e["sha256"] = strings.Repeat("0", 64)
				}
				changed = append(changed, e)
			}
			raw, _ := json.Marshal(map[string]any{"source_frame": changed, "diagnostic_codes": traceInstrumentedDiagnosticCodes})
			if traceInstrumentedSourceFrameValid(raw) {
				t.Fatal("ASSERT_INSTRUMENTED_PIN_trace_go " + mutation + " accepted")
			}
			t.Log("ASSERT_INSTRUMENTED_PIN_trace_go " + mutation + " REJECTED")
		}
	})
	t.Run("Pin_phase14_test_go", func(t *testing.T) {
		for _, mutation := range []string{"missing", "stale"} {
			var changed []map[string]string
			for _, entry := range frame {
				e := map[string]string{"path": entry["path"], "sha256": entry["sha256"]}
				if e["path"] == "internal/inventorytracediag/phase14_test.go" {
					if mutation == "missing" {
						continue
					}
					e["sha256"] = strings.Repeat("0", 64)
				}
				changed = append(changed, e)
			}
			raw, _ := json.Marshal(map[string]any{"source_frame": changed, "diagnostic_codes": traceInstrumentedDiagnosticCodes})
			if traceInstrumentedSourceFrameValid(raw) {
				t.Fatal("ASSERT_INSTRUMENTED_PIN_phase14_test_go " + mutation + " accepted")
			}
			t.Log("ASSERT_INSTRUMENTED_PIN_phase14_test_go " + mutation + " REJECTED")
		}
	})
}
