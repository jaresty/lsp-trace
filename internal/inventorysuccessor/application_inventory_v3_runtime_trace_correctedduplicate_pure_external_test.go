//go:build darwin || linux

package inventorysuccessor_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"lsp-trace/internal/programcadmission"
	"os"
	"path/filepath"
	"regexp"
	"runtime/trace"
	"strings"
	"testing"
)

type traceCorrectedDuplicateShort struct{}

func (traceCorrectedDuplicateShort) Write([]byte) (int, error) { return 0, io.ErrShortWrite }

// No old test/fixture, provider, reserve, trace.Start or trace.Stop is invoked.
func TestTraceCorrectedDuplicatePure(t *testing.T) {
	if trace.IsEnabled() {
		t.Fatal("ASSERT_CORRECTEDDUPLICATE_PURE_DISABLED")
	}
	t.Run("DefaultOff", func(t *testing.T) {
		t.Setenv("LSP_TRACE_CORRECTEDDUPLICATE_ENABLE", "")
		if !t.Run("Wrapper", TestConfigurationV3RuntimeTraceCorrectedDuplicate) {
			t.Fatal("ASSERT_CORRECTEDDUPLICATE_DEFAULT_OFF refused")
		}
		if trace.IsEnabled() {
			t.Fatal("ASSERT_CORRECTEDDUPLICATE_DEFAULT_OFF tracing")
		}
		t.Log("ASSERT_CORRECTEDDUPLICATE_DEFAULT_OFF PASS")
	})
	t.Run("Allowance", func(t *testing.T) {
		if len(traceCorrectedDuplicateCases) != 1 || strings.Join(traceCorrectedDuplicateCases[:], ",") != "provider-C-composed-duplicates" || traceCorrectedDuplicatePhase != "runtime-trace-correctedduplicate-21" || traceCorrectedDuplicateBase != "/Users/schwa/.local/state/lsp-trace-evidence/seven-configuration-campaign-v3-20261112-01/runtime-trace-correctedduplicate-21" {
			t.Fatal("ASSERT_CORRECTEDDUPLICATE_ALLOWANCE exact1")
		}
		t.Log("ASSERT_CORRECTEDDUPLICATE_ALLOWANCE PASS")
	})
	t.Run("SourceFrame", func(t *testing.T) {
		pre := traceCorrectedDuplicatePurePreflight()
		if !traceCorrectedDuplicateSourceFrameValid(pre) {
			t.Fatal("ASSERT_CORRECTEDDUPLICATE_SOURCE_FRAME valid")
		}
		for _, bad := range [][]byte{nil, []byte(`{}`), []byte(`{"source_frame":[]}`), []byte(strings.Replace(string(pre), "5293bca9", "00000000", 1)), []byte(strings.Replace(string(pre), "dad4edd1", "00000000", 1)), []byte(strings.Replace(string(pre), "typed_boundary.go", "old_boundary.go", 1))} {
			if traceCorrectedDuplicateSourceFrameValid(bad) {
				t.Fatal("ASSERT_CORRECTEDDUPLICATE_SOURCE_FRAME missing-or-stale")
			}
		}
		var frame struct {
			SourceFrame []struct {
				Path string `json:"path"`
				SHA  string `json:"sha256"`
			} `json:"source_frame"`
		}
		if json.Unmarshal(pre, &frame) != nil || len(frame.SourceFrame) != 16 {
			t.Fatal("ASSERT_CORRECTEDDUPLICATE_SOURCE_FRAME fixture")
		}
		for _, binding := range frame.SourceFrame {
			for _, bad := range []string{strings.Replace(string(pre), binding.SHA, strings.Repeat("0", 64), 1), strings.Replace(string(pre), binding.Path, "missing/"+binding.Path, 1)} {
				if traceCorrectedDuplicateSourceFrameValid([]byte(bad)) {
					t.Fatal("ASSERT_CORRECTEDDUPLICATE_SOURCE_FRAME mandatory05and09", binding.Path)
				}
			}
		}
		t.Log("ASSERT_CORRECTEDDUPLICATE_SOURCE_FRAME PASS")
	})
	t.Run("Selection", func(t *testing.T) {
		for _, id := range traceCorrectedDuplicateCases {
			run := traceCorrectedDuplicateRun(id)
			args := []string{"-test.run=" + run, "-test.count=1", "-test.timeout=30s", "-test.v=true"}
			if !traceCorrectedDuplicateSelection(id, run, traceCorrectedDuplicateLedger, traceCorrectedDuplicatePhase, args) {
				t.Fatal("ASSERT_CORRECTEDDUPLICATE_SELECTION valid")
			}
			for _, bad := range []string{traceCorrectedDuplicateOldLedger, traceTwoLedger, "", traceCorrectedDuplicateLedger + "/../preparations.jsonl"} {
				if traceCorrectedDuplicateSelection(id, run, bad, traceCorrectedDuplicatePhase, args) {
					t.Fatal("ASSERT_CORRECTEDDUPLICATE_SELECTION oldpath")
				}
			}
			if traceCorrectedDuplicateSelection(id, run+"|.*", traceCorrectedDuplicateLedger, traceCorrectedDuplicatePhase, args) || traceCorrectedDuplicateSelection(id, run, traceCorrectedDuplicateLedger, "old-phase", args) || traceCorrectedDuplicateSelection(id, run, traceCorrectedDuplicateLedger, traceCorrectedDuplicatePhase, append(args, "-test.trace=x")) {
				t.Fatal("ASSERT_CORRECTEDDUPLICATE_SELECTION exact")
			}
			parts := strings.Split(run, "/")
			if len(parts) != 2 || !regexp.MustCompile(parts[0]).MatchString("TestConfigurationV3RuntimeTraceCorrectedDuplicate") {
				t.Fatal("ASSERT_CORRECTEDDUPLICATE_SELECTION parent")
			}
			matches := 0
			for _, other := range append(traceCorrectedDuplicateCases[:], "provider-R-empty", "provider-C-unavailable", "provider-C-empty-extra") {
				if regexp.MustCompile(parts[1]).MatchString(other) {
					matches++
					if other != id {
						t.Fatal("ASSERT_CORRECTEDDUPLICATE_SELECTION sibling")
					}
				}
			}
			if matches != 1 {
				t.Fatal("ASSERT_CORRECTEDDUPLICATE_SELECTION count")
			}
		}
		for _, bad := range []string{"", "provider-R-empty", "provider-C-empty", "provider-mixed-CR-C-unavailable", "provider-mixed-RD-R-unavailable", "provider-C-empty-extra", "../provider-C-empty", "provider-C-empty|.*"} {
			if traceCorrectedDuplicateCase(bad) {
				t.Fatal("ASSERT_CORRECTEDDUPLICATE_SELECTION whitelist")
			}
		}
		raw, _ := json.Marshal(map[string]string{"run": traceCorrectedDuplicatePhase, "case": traceCorrectedDuplicateCases[0], "event": "CONTROLLED_PREPARATION_ATTEMPT"})
		if !traceCorrectedDuplicateLedgerValid(nil, traceCorrectedDuplicateCases[0]) || traceCorrectedDuplicateLedgerValid(append(raw, '\n'), traceCorrectedDuplicateCases[0]) || traceCorrectedDuplicateLedgerValid(raw, traceCorrectedDuplicateCases[0]) || traceCorrectedDuplicateLedgerValid(append(append(raw, '\n'), raw...), traceCorrectedDuplicateCases[0]) || traceCorrectedDuplicateLedgerValid([]byte("bad"), traceCorrectedDuplicateCases[0]) || traceCorrectedDuplicateLedgerValid([]byte(" "), traceCorrectedDuplicateCases[0]) {
			t.Fatal("ASSERT_CORRECTEDDUPLICATE_SELECTION ledger")
		}
		t.Log("ASSERT_CORRECTEDDUPLICATE_SELECTION PASS")
	})
	t.Run("Cap", func(t *testing.T) {
		var b bytes.Buffer
		w := &traceCorrectedDuplicateWriter{dst: &b, limit: 3}
		if n, e := w.Write([]byte("abc")); n != 3 || e != nil {
			t.Fatal("ASSERT_CORRECTEDDUPLICATE_CAP equality")
		}
		if n, e := w.Write([]byte("x")); n != 0 || e != errTraceCorrectedDuplicateCap {
			t.Fatal("ASSERT_CORRECTEDDUPLICATE_CAP overflow")
		}
		if n, e := w.Write([]byte("x")); n != 0 || e == nil {
			t.Fatal("ASSERT_CORRECTEDDUPLICATE_CAP sticky")
		}
		if b.String() != "abc" || !w.overflowed() {
			t.Fatal("ASSERT_CORRECTEDDUPLICATE_CAP persistence")
		}
		b.Reset()
		w = &traceCorrectedDuplicateWriter{dst: &b, limit: 3}
		if n, e := w.Write([]byte("abcde")); n != 3 || e != errTraceCorrectedDuplicateCap || b.Len() != 3 {
			t.Fatal("ASSERT_CORRECTEDDUPLICATE_CAP partial")
		}
		w = &traceCorrectedDuplicateWriter{dst: traceCorrectedDuplicateShort{}, limit: 3}
		if _, e := w.Write([]byte("x")); e == nil {
			t.Fatal("ASSERT_CORRECTEDDUPLICATE_CAP short")
		}
		if _, bad := w.state(); !bad || w.overflowed() {
			t.Fatal("ASSERT_CORRECTEDDUPLICATE_CAP failure")
		}
		w = &traceCorrectedDuplicateWriter{dst: &b, limit: -1}
		if _, e := w.Write(nil); e != errTraceCorrectedDuplicateIO {
			t.Fatal("ASSERT_CORRECTEDDUPLICATE_CAP invalid")
		}
		if traceCorrectedDuplicateCap != 8<<20 || len(traceCorrectedDuplicateCases)*int(traceCorrectedDuplicateCap) != 8<<20 {
			t.Fatal("ASSERT_CORRECTEDDUPLICATE_CAP aggregate")
		}
		t.Log("ASSERT_CORRECTEDDUPLICATE_CAP PASS")
	})
	t.Run("Slots", func(t *testing.T) {
		dir := t.TempDir()
		if os.Chmod(dir, 0700) != nil {
			t.Fatal("ASSERT_CORRECTEDDUPLICATE_SLOTS setup")
		}
		for _, id := range traceCorrectedDuplicateCases {
			if traceCorrectedDuplicateClaim(dir, id) != nil {
				t.Fatal("ASSERT_CORRECTEDDUPLICATE_SLOTS fresh")
			}
			if traceCorrectedDuplicateClaim(dir, id) == nil {
				t.Fatal("ASSERT_CORRECTEDDUPLICATE_SLOTS duplicate")
			}
		}
		if traceCorrectedDuplicateClaim(dir, "provider-R-empty") == nil {
			t.Fatal("ASSERT_CORRECTEDDUPLICATE_SLOTS third")
		}
		other := t.TempDir()
		_ = os.Chmod(other, 0700)
		traceCorrectedDuplicateStop(other)
		if traceCorrectedDuplicateClaim(other, traceCorrectedDuplicateCases[0]) == nil {
			t.Fatal("ASSERT_CORRECTEDDUPLICATE_SLOTS stopped")
		}
		other = t.TempDir()
		_ = os.Chmod(other, 0700)
		if os.WriteFile(filepath.Join(other, "unknown.used"), nil, 0600) != nil {
			t.Fatal("ASSERT_CORRECTEDDUPLICATE_SLOTS setup")
		}
		if traceCorrectedDuplicateClaim(other, traceCorrectedDuplicateCases[0]) == nil {
			t.Fatal("ASSERT_CORRECTEDDUPLICATE_SLOTS foreign")
		}
		t.Log("ASSERT_CORRECTEDDUPLICATE_SLOTS PASS")
	})
	t.Run("Release", func(t *testing.T) {
		pre := traceCorrectedDuplicatePurePreflight()
		q := traceCorrectedDuplicateRelease{Phase: traceCorrectedDuplicatePhase, PreflightSHA: traceCorrectedDuplicateSHA(pre), AllowNative: true, Cases: append([]string(nil), traceCorrectedDuplicateCases[:]...), MaxAttempts: 1, TraceCap: traceCorrectedDuplicateCap, TraceTotal: traceCorrectedDuplicateCap, Expires: 1100}
		if !traceCorrectedDuplicateReleaseValid(q, pre, 1000) {
			t.Fatal("ASSERT_CORRECTEDDUPLICATE_RELEASE valid")
		}
		for _, bad := range [][]string{nil, {"provider-C-composed-duplicates", "provider-C-empty"}, {"provider-C-empty"}} {
			q.Cases = bad
			if traceCorrectedDuplicateReleaseValid(q, pre, 1000) {
				t.Fatal("ASSERT_CORRECTEDDUPLICATE_RELEASE exact-case")
			}
		}
		q.Cases = append([]string(nil), traceCorrectedDuplicateCases[:]...)
		q.MaxAttempts = 2
		if traceCorrectedDuplicateReleaseValid(q, pre, 1000) {
			t.Fatal("ASSERT_CORRECTEDDUPLICATE_RELEASE ceiling")
		}
		q.MaxAttempts = 1
		q.TraceTotal = 40 << 20
		if traceCorrectedDuplicateReleaseValid(q, pre, 1000) {
			t.Fatal("ASSERT_CORRECTEDDUPLICATE_RELEASE aggregate")
		}
		q.TraceTotal = 8 << 20
		q.Phase = "runtime-trace-five-04"
		if traceCorrectedDuplicateReleaseValid(q, pre, 1000) {
			t.Fatal("ASSERT_CORRECTEDDUPLICATE_RELEASE oldphase")
		}
		q.Phase = traceCorrectedDuplicatePhase
		q.AllowNative = false
		if traceCorrectedDuplicateReleaseValid(q, pre, 1000) {
			t.Fatal("ASSERT_CORRECTEDDUPLICATE_RELEASE missing")
		}
		q.AllowNative = true
		q.Expires = 1000
		if traceCorrectedDuplicateReleaseValid(q, pre, 1000) {
			t.Fatal("ASSERT_CORRECTEDDUPLICATE_RELEASE expired")
		}
		q.Expires = 2801
		if traceCorrectedDuplicateReleaseValid(q, pre, 1000) {
			t.Fatal("ASSERT_CORRECTEDDUPLICATE_RELEASE retention")
		}
		q.Expires = 1100
		q.TraceCap++
		if traceCorrectedDuplicateReleaseValid(q, pre, 1000) {
			t.Fatal("ASSERT_CORRECTEDDUPLICATE_RELEASE cap")
		}
		q.TraceCap--
		if traceCorrectedDuplicateReleaseValid(q, []byte("different"), 1000) {
			t.Fatal("ASSERT_CORRECTEDDUPLICATE_RELEASE source")
		}
		t.Log("ASSERT_CORRECTEDDUPLICATE_RELEASE PASS")
	})
	if trace.IsEnabled() {
		t.Fatal("ASSERT_CORRECTEDDUPLICATE_PURE_DISABLED")
	}
}

func traceCorrectedDuplicatePurePreflight() []byte {
	return []byte(`{"source_frame":[{"path":"internal/inventorysuccessor/private_managed_input_operation_v2.go","sha256":"a922f7411d2d22e3e26b8b43b808009decbf413a8ddd0ca1fe40ae9c33bbb29d"},{"path":"internal/referencegrouping/input.go","sha256":"36f784ae1465eb2477799c610fa6e95226e5f5b40fe2d596984a9c19e912d316"},{"path":"internal/inventorytracediag/trace.go","sha256":"0c6fd9cfddf9576a758d08ab62494c039b95bcce8acdea6847d36bd876cb3613"},{"path":"internal/programcadmission/typed_boundary.go","sha256":"5293bca9a78339c7a371cd34970c7da26dfe0088ac5260c94cb2b2861b41a4bb"},{"path":"internal/programcadmission/empty_admission_clone_regression_test.go","sha256":"dad4edd1d508beb6fb533c313ab9268270ffa53f68b50e2313b85aa67a9b7f23"},{"path":"internal/referencegrouping/empty_calls_predecessors_pure_test.go","sha256":"961d30c5afcdebd4885a7f076f057a8622795682e19e329c83d50069baa43478"},{"path":"internal/referencegrouping/empty_calls_native_codec_pure_test.go","sha256":"f43aaa6f6381e7d4155251577b4c6ddd9b4378c7454099cb69adeb028477912e"},{"path":"internal/inventorysuccessor/duplicate_calls_assembly_pure_test.go","sha256":"072eebe3e282f24236c121e15eb7fbd9aa85707a468a85dbcfeecd7c9beb094a"},{"path":"internal/inventorytracediag/phase14_test.go","sha256":"1b4e9d4f8a4b8f7298f609f6fb84c19bb470621388fb4dc99ba21f1d369b83ed"},{"path":"internal/programcadmission/leiden_projection.go","sha256":"58fe6b796c6541723cf64f7d246b895449bb2974a7e151cbbd6f2553ab921ee1"},{"path":"internal/programcadmission/private_binding_budget.go","sha256":"fc3d091e6ce07b8896abab51067b2fcad15e4374094cbd28f77ad5901589cfd8"},{"path":"internal/programcadmission/private_binding_budget_test.go","sha256":"b7e4dae5d6a31971fe069036b86270336dd68ae7c64e1a24b98519300201c865"},{"path":"internal/inventorysuccessor/sealed_binding_budget_18_pure_test.go","sha256":"18cc95541c0d717874df238bd2b1809871b790f842d190488e29c775076f3a75"},{"path":"internal/inventorysuccessor/application_inventory_v3_configuration_external_test.go","sha256":"76bf3fc9142309665287702d46ca015c79d4266ec543fd2be0ad7982f44ce4e3"},{"path":"internal/inventorysuccessor/application_inventory_v3_configuration_responder_external_test.go","sha256":"dad5a60f35373681d97d693d69f91b230089a02b6123b5aaf3dead880e2ef086"},{"path":"internal/inventorysuccessor/application_inventory_v3_duplicate_literal_pure_external_test.go","sha256":"e136357f229ab23d00e74d290138584bd16fe324f28447b90e10c5e1548882fe"}],"diagnostic_codes":["COMPOSITE_BINDING_BOUND_REFUSED","COMPOSITE_BINDING_JSON_REFUSED","NATIVE_PREDECESSOR_COUNT_REFUSED","NATIVE_PREDECESSOR_BYTES_REFUSED","NATIVE_AGGREGATE_BOUND_REFUSED"]}`)
}

// Source binding is pure; it does not claim native DTO raw multiplicity.
func TestTraceCorrectedDuplicateCurrent20(t *testing.T) {
	t.Run("Canonical", func(t *testing.T) {
		t.Log("PROCEDURE ASSERT_PHASE21_CANONICAL: helper accepts exactly two unique nonempty CALLS; rejects four, duplicate and empty")
		typed := programcadmission.TypedInput{Occurrences: []programcadmission.TypedOccurrence{{Kind: programcadmission.TypedCalls, Identity: "one"}, {Kind: programcadmission.TypedCalls, Identity: "two"}}}
		if err := configurationDuplicateCanonicalV3(typed); err != nil {
			t.Fatal("ASSERT_PHASE21_CANONICAL FAIL valid", err)
		}
		for _, kind := range []string{"four", "duplicate", "empty"} {
			bad := typed
			bad.Occurrences = append([]programcadmission.TypedOccurrence(nil), typed.Occurrences...)
			switch kind {
			case "four":
				a, b := bad.Occurrences[0], bad.Occurrences[1]
				a.Identity = "three"
				b.Identity = "four"
				bad.Occurrences = append(bad.Occurrences, a, b)
			case "duplicate":
				bad.Occurrences[1].Identity = "one"
			case "empty":
				bad.Occurrences[0].Identity = ""
			}
			if configurationDuplicateCanonicalV3(bad) == nil {
				t.Fatal("ASSERT_PHASE21_CANONICAL FAIL", kind)
			}
		}
		t.Log("ASSERT_PHASE21_CANONICAL PASS")
	})
	t.Run("CurrentSources", func(t *testing.T) {
		t.Log("PROCEDURE ASSERT_PHASE21_CURRENT_SOURCES: current20 and owner-budget source hashes must match mandatory pins")
		var p struct {
			SourceFrame []struct {
				Path string `json:"path"`
				SHA  string `json:"sha256"`
			} `json:"source_frame"`
		}
		if json.Unmarshal(traceCorrectedDuplicatePurePreflight(), &p) != nil {
			t.Fatal("fixture")
		}
		for _, e := range p.SourceFrame {
			raw, err := os.ReadFile(filepath.Join("../..", e.Path))
			if err != nil || fmt.Sprintf("%x", sha256.Sum256(raw)) != e.SHA {
				t.Fatal("ASSERT_PHASE21_CURRENT_SOURCES FAIL", e.Path)
			}
		}
		raw, err := os.ReadFile("application_inventory_v3_configuration_external_test.go")
		if err != nil || !strings.Contains(string(raw), "configurationDuplicateCanonicalV3(input.TypedInput)") {
			t.Fatal("ASSERT_PHASE21_CURRENT_SOURCES FAIL canonical seam")
		}
		t.Log("ASSERT_PHASE21_CURRENT_SOURCES PASS")
	})
	t.Run("Release600", func(t *testing.T) {
		t.Log("PROCEDURE ASSERT_PHASE21_RELEASE600: equality600 accepts and601 refuses")
		pre := traceCorrectedDuplicatePurePreflight()
		q := traceCorrectedDuplicateRelease{Phase: traceCorrectedDuplicatePhase, PreflightSHA: traceCorrectedDuplicateSHA(pre), AllowNative: true, Cases: []string{"provider-C-composed-duplicates"}, MaxAttempts: 1, TraceCap: 8388608, TraceTotal: 8388608, Expires: 1600}
		if !traceCorrectedDuplicateReleaseValid(q, pre, 1000) {
			t.Fatal("ASSERT_PHASE21_RELEASE600 FAIL equality")
		}
		q.Expires = 1601
		if traceCorrectedDuplicateReleaseValid(q, pre, 1000) {
			t.Fatal("ASSERT_PHASE21_RELEASE600 FAIL601")
		}
		t.Log("ASSERT_PHASE21_RELEASE600 PASS")
	})
}
