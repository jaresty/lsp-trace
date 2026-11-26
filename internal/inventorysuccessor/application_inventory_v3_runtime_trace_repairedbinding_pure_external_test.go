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

type traceRepairedBindingShort struct{}

func (traceRepairedBindingShort) Write([]byte) (int, error) { return 0, io.ErrShortWrite }

// No old test/fixture, provider, reserve, trace.Start or trace.Stop is invoked.
func TestTraceRepairedBindingPure(t *testing.T) {
	if trace.IsEnabled() {
		t.Fatal("ASSERT_DUPLICATE_PURE_DISABLED")
	}
	t.Run("DefaultOff", func(t *testing.T) {
		t.Setenv("LSP_TRACE_REPAIREDBINDING_ENABLE", "")
		if !t.Run("Wrapper", TestConfigurationV3RuntimeTraceRepairedBinding) {
			t.Fatal("ASSERT_DUPLICATE_DEFAULT_OFF refused")
		}
		if trace.IsEnabled() {
			t.Fatal("ASSERT_DUPLICATE_DEFAULT_OFF tracing")
		}
		t.Log("ASSERT_DUPLICATE_DEFAULT_OFF PASS")
	})
	t.Run("Allowance", func(t *testing.T) {
		if len(traceRepairedBindingCases) != 1 || strings.Join(traceRepairedBindingCases[:], ",") != "provider-C-composed-duplicates" || traceRepairedBindingPhase != "runtime-trace-repairedbinding-19" || traceRepairedBindingBase != "/Users/schwa/.local/state/lsp-trace-evidence/seven-configuration-campaign-v3-20261112-01/runtime-trace-repairedbinding-19" {
			t.Fatal("ASSERT_DUPLICATE_ALLOWANCE exact1")
		}
		t.Log("ASSERT_DUPLICATE_ALLOWANCE PASS")
	})
	t.Run("SourceFrame", func(t *testing.T) {
		pre := traceRepairedBindingPurePreflight()
		if !traceRepairedBindingSourceFrameValid(pre) {
			t.Fatal("ASSERT_DUPLICATE_SOURCE_FRAME valid")
		}
		for _, bad := range [][]byte{nil, []byte(`{}`), []byte(`{"source_frame":[]}`), []byte(strings.Replace(string(pre), "5293bca9", "00000000", 1)), []byte(strings.Replace(string(pre), "dad4edd1", "00000000", 1)), []byte(strings.Replace(string(pre), "typed_boundary.go", "old_boundary.go", 1))} {
			if traceRepairedBindingSourceFrameValid(bad) {
				t.Fatal("ASSERT_DUPLICATE_SOURCE_FRAME missing-or-stale")
			}
		}
		var frame struct {
			SourceFrame []struct {
				Path string `json:"path"`
				SHA  string `json:"sha256"`
			} `json:"source_frame"`
		}
		if json.Unmarshal(pre, &frame) != nil || len(frame.SourceFrame) != 13 {
			t.Fatal("ASSERT_DUPLICATE_SOURCE_FRAME fixture")
		}
		for _, binding := range frame.SourceFrame {
			for _, bad := range []string{strings.Replace(string(pre), binding.SHA, strings.Repeat("0", 64), 1), strings.Replace(string(pre), binding.Path, "missing/"+binding.Path, 1)} {
				if traceRepairedBindingSourceFrameValid([]byte(bad)) {
					t.Fatal("ASSERT_DUPLICATE_SOURCE_FRAME mandatory05and09", binding.Path)
				}
			}
		}
		t.Log("ASSERT_DUPLICATE_SOURCE_FRAME PASS")
	})
	t.Run("Selection", func(t *testing.T) {
		for _, id := range traceRepairedBindingCases {
			run := traceRepairedBindingRun(id)
			args := []string{"-test.run=" + run, "-test.count=1", "-test.timeout=30s", "-test.v=true"}
			if !traceRepairedBindingSelection(id, run, traceRepairedBindingLedger, traceRepairedBindingPhase, args) {
				t.Fatal("ASSERT_DUPLICATE_SELECTION valid")
			}
			for _, bad := range []string{traceRepairedBindingOldLedger, traceTwoLedger, "", traceRepairedBindingLedger + "/../preparations.jsonl"} {
				if traceRepairedBindingSelection(id, run, bad, traceRepairedBindingPhase, args) {
					t.Fatal("ASSERT_DUPLICATE_SELECTION oldpath")
				}
			}
			if traceRepairedBindingSelection(id, run+"|.*", traceRepairedBindingLedger, traceRepairedBindingPhase, args) || traceRepairedBindingSelection(id, run, traceRepairedBindingLedger, "old-phase", args) || traceRepairedBindingSelection(id, run, traceRepairedBindingLedger, traceRepairedBindingPhase, append(args, "-test.trace=x")) {
				t.Fatal("ASSERT_DUPLICATE_SELECTION exact")
			}
			parts := strings.Split(run, "/")
			if len(parts) != 2 || !regexp.MustCompile(parts[0]).MatchString("TestConfigurationV3RuntimeTraceRepairedBinding") {
				t.Fatal("ASSERT_DUPLICATE_SELECTION parent")
			}
			matches := 0
			for _, other := range append(traceRepairedBindingCases[:], "provider-R-empty", "provider-C-unavailable", "provider-C-empty-extra") {
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
			if traceRepairedBindingCase(bad) {
				t.Fatal("ASSERT_DUPLICATE_SELECTION whitelist")
			}
		}
		raw, _ := json.Marshal(map[string]string{"run": traceRepairedBindingPhase, "case": traceRepairedBindingCases[0], "event": "CONTROLLED_PREPARATION_ATTEMPT"})
		if !traceRepairedBindingLedgerValid(nil, traceRepairedBindingCases[0]) || traceRepairedBindingLedgerValid(append(raw, '\n'), traceRepairedBindingCases[0]) || traceRepairedBindingLedgerValid(raw, traceRepairedBindingCases[0]) || traceRepairedBindingLedgerValid(append(append(raw, '\n'), raw...), traceRepairedBindingCases[0]) || traceRepairedBindingLedgerValid([]byte("bad"), traceRepairedBindingCases[0]) || traceRepairedBindingLedgerValid([]byte(" "), traceRepairedBindingCases[0]) {
			t.Fatal("ASSERT_DUPLICATE_SELECTION ledger")
		}
		t.Log("ASSERT_DUPLICATE_SELECTION PASS")
	})
	t.Run("Cap", func(t *testing.T) {
		var b bytes.Buffer
		w := &traceRepairedBindingWriter{dst: &b, limit: 3}
		if n, e := w.Write([]byte("abc")); n != 3 || e != nil {
			t.Fatal("ASSERT_DUPLICATE_CAP equality")
		}
		if n, e := w.Write([]byte("x")); n != 0 || e != errTraceRepairedBindingCap {
			t.Fatal("ASSERT_DUPLICATE_CAP overflow")
		}
		if n, e := w.Write([]byte("x")); n != 0 || e == nil {
			t.Fatal("ASSERT_DUPLICATE_CAP sticky")
		}
		if b.String() != "abc" || !w.overflowed() {
			t.Fatal("ASSERT_DUPLICATE_CAP persistence")
		}
		b.Reset()
		w = &traceRepairedBindingWriter{dst: &b, limit: 3}
		if n, e := w.Write([]byte("abcde")); n != 3 || e != errTraceRepairedBindingCap || b.Len() != 3 {
			t.Fatal("ASSERT_DUPLICATE_CAP partial")
		}
		w = &traceRepairedBindingWriter{dst: traceRepairedBindingShort{}, limit: 3}
		if _, e := w.Write([]byte("x")); e == nil {
			t.Fatal("ASSERT_DUPLICATE_CAP short")
		}
		if _, bad := w.state(); !bad || w.overflowed() {
			t.Fatal("ASSERT_DUPLICATE_CAP failure")
		}
		w = &traceRepairedBindingWriter{dst: &b, limit: -1}
		if _, e := w.Write(nil); e != errTraceRepairedBindingIO {
			t.Fatal("ASSERT_DUPLICATE_CAP invalid")
		}
		if traceRepairedBindingCap != 8<<20 || len(traceRepairedBindingCases)*int(traceRepairedBindingCap) != 8<<20 {
			t.Fatal("ASSERT_DUPLICATE_CAP aggregate")
		}
		t.Log("ASSERT_DUPLICATE_CAP PASS")
	})
	t.Run("Slots", func(t *testing.T) {
		dir := t.TempDir()
		if os.Chmod(dir, 0700) != nil {
			t.Fatal("ASSERT_DUPLICATE_SLOTS setup")
		}
		for _, id := range traceRepairedBindingCases {
			if traceRepairedBindingClaim(dir, id) != nil {
				t.Fatal("ASSERT_DUPLICATE_SLOTS fresh")
			}
			if traceRepairedBindingClaim(dir, id) == nil {
				t.Fatal("ASSERT_DUPLICATE_SLOTS duplicate")
			}
		}
		if traceRepairedBindingClaim(dir, "provider-R-empty") == nil {
			t.Fatal("ASSERT_DUPLICATE_SLOTS third")
		}
		other := t.TempDir()
		_ = os.Chmod(other, 0700)
		traceRepairedBindingStop(other)
		if traceRepairedBindingClaim(other, traceRepairedBindingCases[0]) == nil {
			t.Fatal("ASSERT_DUPLICATE_SLOTS stopped")
		}
		other = t.TempDir()
		_ = os.Chmod(other, 0700)
		if os.WriteFile(filepath.Join(other, "unknown.used"), nil, 0600) != nil {
			t.Fatal("ASSERT_DUPLICATE_SLOTS setup")
		}
		if traceRepairedBindingClaim(other, traceRepairedBindingCases[0]) == nil {
			t.Fatal("ASSERT_DUPLICATE_SLOTS foreign")
		}
		t.Log("ASSERT_DUPLICATE_SLOTS PASS")
	})
	t.Run("Release", func(t *testing.T) {
		pre := traceRepairedBindingPurePreflight()
		q := traceRepairedBindingRelease{Phase: traceRepairedBindingPhase, PreflightSHA: traceRepairedBindingSHA(pre), AllowNative: true, Cases: append([]string(nil), traceRepairedBindingCases[:]...), MaxAttempts: 1, TraceCap: traceRepairedBindingCap, TraceTotal: traceRepairedBindingCap, Expires: 1100}
		if !traceRepairedBindingReleaseValid(q, pre, 1000) {
			t.Fatal("ASSERT_DUPLICATE_RELEASE valid")
		}
		for _, bad := range [][]string{nil, {"provider-C-composed-duplicates", "provider-C-empty"}, {"provider-C-empty"}} {
			q.Cases = bad
			if traceRepairedBindingReleaseValid(q, pre, 1000) {
				t.Fatal("ASSERT_DUPLICATE_RELEASE exact-case")
			}
		}
		q.Cases = append([]string(nil), traceRepairedBindingCases[:]...)
		q.MaxAttempts = 2
		if traceRepairedBindingReleaseValid(q, pre, 1000) {
			t.Fatal("ASSERT_DUPLICATE_RELEASE ceiling")
		}
		q.MaxAttempts = 1
		q.TraceTotal = 40 << 20
		if traceRepairedBindingReleaseValid(q, pre, 1000) {
			t.Fatal("ASSERT_DUPLICATE_RELEASE aggregate")
		}
		q.TraceTotal = 8 << 20
		q.Phase = "runtime-trace-five-04"
		if traceRepairedBindingReleaseValid(q, pre, 1000) {
			t.Fatal("ASSERT_DUPLICATE_RELEASE oldphase")
		}
		q.Phase = traceRepairedBindingPhase
		q.AllowNative = false
		if traceRepairedBindingReleaseValid(q, pre, 1000) {
			t.Fatal("ASSERT_DUPLICATE_RELEASE missing")
		}
		q.AllowNative = true
		q.Expires = 1000
		if traceRepairedBindingReleaseValid(q, pre, 1000) {
			t.Fatal("ASSERT_DUPLICATE_RELEASE expired")
		}
		q.Expires = 2801
		if traceRepairedBindingReleaseValid(q, pre, 1000) {
			t.Fatal("ASSERT_DUPLICATE_RELEASE retention")
		}
		q.Expires = 1100
		q.TraceCap++
		if traceRepairedBindingReleaseValid(q, pre, 1000) {
			t.Fatal("ASSERT_DUPLICATE_RELEASE cap")
		}
		q.TraceCap--
		if traceRepairedBindingReleaseValid(q, []byte("different"), 1000) {
			t.Fatal("ASSERT_DUPLICATE_RELEASE source")
		}
		t.Log("ASSERT_DUPLICATE_RELEASE PASS")
	})
	if trace.IsEnabled() {
		t.Fatal("ASSERT_DUPLICATE_PURE_DISABLED")
	}
}

func traceRepairedBindingPurePreflight() []byte {
	return []byte(`{"source_frame":[{"path":"internal/inventorysuccessor/private_managed_input_operation_v2.go","sha256":"a922f7411d2d22e3e26b8b43b808009decbf413a8ddd0ca1fe40ae9c33bbb29d"},{"path":"internal/referencegrouping/input.go","sha256":"36f784ae1465eb2477799c610fa6e95226e5f5b40fe2d596984a9c19e912d316"},{"path":"internal/inventorytracediag/trace.go","sha256":"0c6fd9cfddf9576a758d08ab62494c039b95bcce8acdea6847d36bd876cb3613"},{"path":"internal/programcadmission/typed_boundary.go","sha256":"5293bca9a78339c7a371cd34970c7da26dfe0088ac5260c94cb2b2861b41a4bb"},{"path":"internal/programcadmission/empty_admission_clone_regression_test.go","sha256":"dad4edd1d508beb6fb533c313ab9268270ffa53f68b50e2313b85aa67a9b7f23"},{"path":"internal/referencegrouping/empty_calls_predecessors_pure_test.go","sha256":"961d30c5afcdebd4885a7f076f057a8622795682e19e329c83d50069baa43478"},{"path":"internal/referencegrouping/empty_calls_native_codec_pure_test.go","sha256":"f43aaa6f6381e7d4155251577b4c6ddd9b4378c7454099cb69adeb028477912e"},{"path":"internal/inventorysuccessor/duplicate_calls_assembly_pure_test.go","sha256":"072eebe3e282f24236c121e15eb7fbd9aa85707a468a85dbcfeecd7c9beb094a"},{"path":"internal/inventorytracediag/phase14_test.go","sha256":"1b4e9d4f8a4b8f7298f609f6fb84c19bb470621388fb4dc99ba21f1d369b83ed"},{"path":"internal/programcadmission/leiden_projection.go","sha256":"58fe6b796c6541723cf64f7d246b895449bb2974a7e151cbbd6f2553ab921ee1"},{"path":"internal/programcadmission/private_binding_budget.go","sha256":"fc3d091e6ce07b8896abab51067b2fcad15e4374094cbd28f77ad5901589cfd8"},{"path":"internal/programcadmission/private_binding_budget_test.go","sha256":"b7e4dae5d6a31971fe069036b86270336dd68ae7c64e1a24b98519300201c865"},{"path":"internal/inventorysuccessor/sealed_binding_budget_18_pure_test.go","sha256":"18cc95541c0d717874df238bd2b1809871b790f842d190488e29c775076f3a75"}],"diagnostic_codes":["COMPOSITE_BINDING_BOUND_REFUSED","COMPOSITE_BINDING_JSON_REFUSED","NATIVE_PREDECESSOR_COUNT_REFUSED","NATIVE_PREDECESSOR_BYTES_REFUSED","NATIVE_AGGREGATE_BOUND_REFUSED"]}`)
}

func TestTraceRepairedBindingPhase19RegistryGuards(t *testing.T) {
	if trace.IsEnabled() {
		t.Fatal("ASSERT_INSTRUMENTED_DISABLED")
	}
	t.Run("COMPOSITE_BINDING_BOUND_REFUSED", func(t *testing.T) {
		found := false
		for _, c := range traceRepairedBindingDiagnosticCodes {
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
		for _, c := range traceRepairedBindingDiagnosticCodes {
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
		for _, c := range traceRepairedBindingDiagnosticCodes {
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
		for _, c := range traceRepairedBindingDiagnosticCodes {
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
		for _, c := range traceRepairedBindingDiagnosticCodes {
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
	if json.Unmarshal(traceRepairedBindingPurePreflight(), &fixture) != nil {
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
			raw, _ := json.Marshal(map[string]any{"source_frame": changed, "diagnostic_codes": traceRepairedBindingDiagnosticCodes})
			if traceRepairedBindingSourceFrameValid(raw) {
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
			raw, _ := json.Marshal(map[string]any{"source_frame": changed, "diagnostic_codes": traceRepairedBindingDiagnosticCodes})
			if traceRepairedBindingSourceFrameValid(raw) {
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
			raw, _ := json.Marshal(map[string]any{"source_frame": changed, "diagnostic_codes": traceRepairedBindingDiagnosticCodes})
			if traceRepairedBindingSourceFrameValid(raw) {
				t.Fatal("ASSERT_INSTRUMENTED_PIN_phase14_test_go " + mutation + " accepted")
			}
			t.Log("ASSERT_INSTRUMENTED_PIN_phase14_test_go " + mutation + " REJECTED")
		}
	})
}

// Pure preparation guards: no provider, native launch, trace lifecycle, or release file.
func TestTraceRepairedBindingPhase19Guards(t *testing.T) {
	if trace.IsEnabled() {
		t.Fatal("ASSERT_PHASE19_DISABLED")
	}
	var fixture map[string]json.RawMessage
	if json.Unmarshal(traceRepairedBindingPurePreflight(), &fixture) != nil {
		t.Fatal("fixture")
	}
	var frame []map[string]string
	if json.Unmarshal(fixture["source_frame"], &frame) != nil {
		t.Fatal("frame")
	}

	t.Run("Pin0", func(t *testing.T) {
		t.Log("PROCEDURE ASSERT_PHASE19_PIN_0 valid fixture; missing and stale exact current pin must refuse")
		if !traceRepairedBindingSourceFrameValid(traceRepairedBindingPurePreflight()) {
			t.Fatal("ASSERT_PHASE19_PIN_0 valid rejected")
		}
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
			raw, _ := json.Marshal(map[string]any{"source_frame": changed, "diagnostic_codes": traceRepairedBindingDiagnosticCodes})
			if traceRepairedBindingSourceFrameValid(raw) {
				t.Error("ASSERT_PHASE19_PIN_0 FAIL " + mutation + " accepted")
			}
		}
		if !t.Failed() {
			t.Log("ASSERT_PHASE19_PIN_0 PASS")
		}
	})

	t.Run("Pin1", func(t *testing.T) {
		t.Log("PROCEDURE ASSERT_PHASE19_PIN_1 valid fixture; missing and stale exact current pin must refuse")
		if !traceRepairedBindingSourceFrameValid(traceRepairedBindingPurePreflight()) {
			t.Fatal("ASSERT_PHASE19_PIN_1 valid rejected")
		}
		for _, mutation := range []string{"missing", "stale"} {
			var changed []map[string]string
			for _, entry := range frame {
				e := map[string]string{"path": entry["path"], "sha256": entry["sha256"]}
				if e["path"] == "internal/programcadmission/leiden_projection.go" {
					if mutation == "missing" {
						continue
					}
					e["sha256"] = strings.Repeat("0", 64)
				}
				changed = append(changed, e)
			}
			raw, _ := json.Marshal(map[string]any{"source_frame": changed, "diagnostic_codes": traceRepairedBindingDiagnosticCodes})
			if traceRepairedBindingSourceFrameValid(raw) {
				t.Error("ASSERT_PHASE19_PIN_1 FAIL " + mutation + " accepted")
			}
		}
		if !t.Failed() {
			t.Log("ASSERT_PHASE19_PIN_1 PASS")
		}
	})

	t.Run("Pin2", func(t *testing.T) {
		t.Log("PROCEDURE ASSERT_PHASE19_PIN_2 valid fixture; missing and stale exact current pin must refuse")
		if !traceRepairedBindingSourceFrameValid(traceRepairedBindingPurePreflight()) {
			t.Fatal("ASSERT_PHASE19_PIN_2 valid rejected")
		}
		for _, mutation := range []string{"missing", "stale"} {
			var changed []map[string]string
			for _, entry := range frame {
				e := map[string]string{"path": entry["path"], "sha256": entry["sha256"]}
				if e["path"] == "internal/programcadmission/private_binding_budget.go" {
					if mutation == "missing" {
						continue
					}
					e["sha256"] = strings.Repeat("0", 64)
				}
				changed = append(changed, e)
			}
			raw, _ := json.Marshal(map[string]any{"source_frame": changed, "diagnostic_codes": traceRepairedBindingDiagnosticCodes})
			if traceRepairedBindingSourceFrameValid(raw) {
				t.Error("ASSERT_PHASE19_PIN_2 FAIL " + mutation + " accepted")
			}
		}
		if !t.Failed() {
			t.Log("ASSERT_PHASE19_PIN_2 PASS")
		}
	})

	t.Run("Pin3", func(t *testing.T) {
		t.Log("PROCEDURE ASSERT_PHASE19_PIN_3 valid fixture; missing and stale exact current pin must refuse")
		if !traceRepairedBindingSourceFrameValid(traceRepairedBindingPurePreflight()) {
			t.Fatal("ASSERT_PHASE19_PIN_3 valid rejected")
		}
		for _, mutation := range []string{"missing", "stale"} {
			var changed []map[string]string
			for _, entry := range frame {
				e := map[string]string{"path": entry["path"], "sha256": entry["sha256"]}
				if e["path"] == "internal/programcadmission/private_binding_budget_test.go" {
					if mutation == "missing" {
						continue
					}
					e["sha256"] = strings.Repeat("0", 64)
				}
				changed = append(changed, e)
			}
			raw, _ := json.Marshal(map[string]any{"source_frame": changed, "diagnostic_codes": traceRepairedBindingDiagnosticCodes})
			if traceRepairedBindingSourceFrameValid(raw) {
				t.Error("ASSERT_PHASE19_PIN_3 FAIL " + mutation + " accepted")
			}
		}
		if !t.Failed() {
			t.Log("ASSERT_PHASE19_PIN_3 PASS")
		}
	})

	t.Run("Pin4", func(t *testing.T) {
		t.Log("PROCEDURE ASSERT_PHASE19_PIN_4 valid fixture; missing and stale exact current pin must refuse")
		if !traceRepairedBindingSourceFrameValid(traceRepairedBindingPurePreflight()) {
			t.Fatal("ASSERT_PHASE19_PIN_4 valid rejected")
		}
		for _, mutation := range []string{"missing", "stale"} {
			var changed []map[string]string
			for _, entry := range frame {
				e := map[string]string{"path": entry["path"], "sha256": entry["sha256"]}
				if e["path"] == "internal/inventorysuccessor/sealed_binding_budget_18_pure_test.go" {
					if mutation == "missing" {
						continue
					}
					e["sha256"] = strings.Repeat("0", 64)
				}
				changed = append(changed, e)
			}
			raw, _ := json.Marshal(map[string]any{"source_frame": changed, "diagnostic_codes": traceRepairedBindingDiagnosticCodes})
			if traceRepairedBindingSourceFrameValid(raw) {
				t.Error("ASSERT_PHASE19_PIN_4 FAIL " + mutation + " accepted")
			}
		}
		if !t.Failed() {
			t.Log("ASSERT_PHASE19_PIN_4 PASS")
		}
	})

	t.Run("Case", func(t *testing.T) {
		t.Log("PROCEDURE ASSERT_PHASE19_CASE literal sole duplicate selection")
		if len(traceRepairedBindingCases) != 1 || traceRepairedBindingCases[0] != "provider-C-composed-duplicates" {
			t.Fatal("ASSERT_PHASE19_CASE FAIL")
		}
		t.Log("ASSERT_PHASE19_CASE PASS")
	})
	t.Run("Cap", func(t *testing.T) {
		t.Log("PROCEDURE ASSERT_PHASE19_CAP literal8MiB")
		if traceRepairedBindingCap != 8388608 {
			t.Fatal("ASSERT_PHASE19_CAP FAIL")
		}
		t.Log("ASSERT_PHASE19_CAP PASS")
	})
	t.Run("Namespace", func(t *testing.T) {
		t.Log("PROCEDURE ASSERT_PHASE19_NAMESPACE exact new phase/base/run")
		if traceRepairedBindingPhase != "runtime-trace-repairedbinding-19" || traceRepairedBindingBase != "/Users/schwa/.local/state/lsp-trace-evidence/seven-configuration-campaign-v3-20261112-01/runtime-trace-repairedbinding-19" || traceRepairedBindingRun("provider-C-composed-duplicates") != "^TestConfigurationV3RuntimeTraceRepairedBinding$/^provider-C-composed-duplicates$" {
			t.Fatal("ASSERT_PHASE19_NAMESPACE FAIL")
		}
		t.Log("ASSERT_PHASE19_NAMESPACE PASS")
	})
	t.Run("Budget", func(t *testing.T) {
		t.Log("PROCEDURE ASSERT_PHASE19_BUDGET valid1; forbid2 and retention1801")
		pre := traceRepairedBindingPurePreflight()
		q := traceRepairedBindingRelease{Phase: traceRepairedBindingPhase, PreflightSHA: traceRepairedBindingSHA(pre), AllowNative: true, Cases: []string{"provider-C-composed-duplicates"}, MaxAttempts: 1, TraceCap: 8388608, TraceTotal: 8388608, Expires: 1100}
		if !traceRepairedBindingReleaseValid(q, pre, 1000) {
			t.Fatal("ASSERT_PHASE19_BUDGET valid rejected")
		}
		q.MaxAttempts = 2
		if traceRepairedBindingReleaseValid(q, pre, 1000) {
			t.Fatal("ASSERT_PHASE19_BUDGET FAIL attempts2 accepted")
		}
		q.MaxAttempts = 1
		q.Expires = 2801
		if traceRepairedBindingReleaseValid(q, pre, 1000) {
			t.Fatal("ASSERT_PHASE19_BUDGET FAIL expiry1801 accepted")
		}
		t.Log("ASSERT_PHASE19_BUDGET PASS")
	})
	t.Run("Release", func(t *testing.T) {
		t.Log("PROCEDURE ASSERT_PHASE19_RELEASE disallow false authorization")
		pre := traceRepairedBindingPurePreflight()
		q := traceRepairedBindingRelease{Phase: traceRepairedBindingPhase, PreflightSHA: traceRepairedBindingSHA(pre), AllowNative: true, Cases: []string{"provider-C-composed-duplicates"}, MaxAttempts: 1, TraceCap: 8388608, TraceTotal: 8388608, Expires: 1100}
		if !traceRepairedBindingReleaseValid(q, pre, 1000) {
			t.Fatal("ASSERT_PHASE19_RELEASE valid rejected")
		}
		q.AllowNative = false
		if traceRepairedBindingReleaseValid(q, pre, 1000) {
			t.Fatal("ASSERT_PHASE19_RELEASE FAIL false accepted")
		}
		t.Log("ASSERT_PHASE19_RELEASE PASS")
	})
}
