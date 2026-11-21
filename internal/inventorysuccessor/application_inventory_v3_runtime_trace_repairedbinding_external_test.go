package inventorysuccessor_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"runtime/trace"
	"strings"
	"sync"
	"testing"
	"time"
)

const traceRepairedBindingPhase = "runtime-trace-repairedbinding-19"
const traceRepairedBindingBase = "/Users/schwa/.local/state/lsp-trace-evidence/seven-configuration-campaign-v3-20261112-01/runtime-trace-repairedbinding-19"
const traceRepairedBindingExecution = traceRepairedBindingBase + "/execution"
const traceRepairedBindingLedger = traceRepairedBindingExecution + "/preparations.jsonl"
const traceRepairedBindingOldLedger = "/Users/schwa/.local/state/lsp-trace-evidence/seven-configuration-campaign-v3-20261112-01/writer-716d-01/preparations.jsonl"
const traceRepairedBindingOldSHA = "a21c1d1a83c3caddfed8c144a10ce9854c87bb99f0fda15458ecd436b1164431"
const traceRepairedBindingCap int64 = 8 << 20

var traceRepairedBindingCases = [...]string{"provider-C-composed-duplicates"}
var errTraceRepairedBindingCap = errors.New("TRACE_REPAIREDBINDING_CAP")
var errTraceRepairedBindingIO = errors.New("TRACE_REPAIREDBINDING_IO")
var errTraceRepairedBindingSlot = errors.New("TRACE_REPAIREDBINDING_SLOT_REFUSED")

func traceRepairedBindingCase(id string) bool {
	for _, c := range traceRepairedBindingCases {
		if c == id {
			return true
		}
	}
	return false
}
func traceRepairedBindingRun(id string) string {
	return "^TestConfigurationV3RuntimeTraceRepairedBinding$/^" + id + "$"
}
func traceRepairedBindingSelection(id, run, ledger, phase string, args []string) bool {
	if !traceRepairedBindingCase(id) || run != traceRepairedBindingRun(id) || ledger != traceRepairedBindingLedger || phase != traceRepairedBindingPhase {
		return false
	}
	want := map[string]bool{"-test.run=" + run: true, "-test.count=1": true, "-test.timeout=30s": true, "-test.v=true": true}
	if len(args) != len(want) {
		return false
	}
	for _, a := range args {
		if !want[a] {
			return false
		}
		delete(want, a)
	}
	return len(want) == 0
}

// A sticky failing bounded writer: equality is accepted, overflow is rejected,
// at most the remaining prefix persists. Partial/error traces are never usable.
type traceRepairedBindingWriter struct {
	mu           sync.Mutex
	dst          io.Writer
	limit, n     int64
	failed, over bool
}

func (w *traceRepairedBindingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.failed {
		return 0, errTraceRepairedBindingCap
	}
	if w.limit < 0 || w.n < 0 || w.n > w.limit {
		w.failed = true
		return 0, errTraceRepairedBindingIO
	}
	left := w.limit - w.n
	overflow := int64(len(p)) > left
	if overflow {
		w.over = true
		p = p[:left]
	}
	n, e := w.dst.Write(p)
	if n < 0 || n > len(p) {
		w.failed = true
		return 0, errTraceRepairedBindingIO
	}
	w.n += int64(n)
	if e != nil || n != len(p) {
		w.failed = true
		return n, errTraceRepairedBindingIO
	}
	if overflow {
		w.failed = true
		return n, errTraceRepairedBindingCap
	}
	return n, nil
}
func (w *traceRepairedBindingWriter) state() (int64, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.n, w.failed
}
func (w *traceRepairedBindingWriter) overflowed() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.over
}
func traceRepairedBindingStop(dir string) {
	f, e := os.OpenFile(filepath.Join(dir, "STOP"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e == nil {
		_, _ = f.WriteString("TRACE_REPAIREDBINDING_STOP\n")
		_ = f.Close()
	}
}
func traceRepairedBindingClaim(dir, id string) error {
	if !traceRepairedBindingCase(id) {
		return errTraceRepairedBindingSlot
	}
	info, e := os.Lstat(dir)
	if e != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0700 {
		return errTraceRepairedBindingSlot
	}
	if _, e = os.Lstat(filepath.Join(dir, "STOP")); !os.IsNotExist(e) {
		return errTraceRepairedBindingSlot
	}
	entries, e := os.ReadDir(dir)
	if e != nil {
		return errTraceRepairedBindingSlot
	}
	used := 0
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".used") {
			if !traceRepairedBindingCase(strings.TrimSuffix(entry.Name(), ".used")) {
				return errTraceRepairedBindingSlot
			}
			used++
		}
	}
	if used >= len(traceRepairedBindingCases) {
		return errTraceRepairedBindingSlot
	}
	f, e := os.OpenFile(filepath.Join(dir, id+".used"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return errTraceRepairedBindingSlot
	}
	_, e = f.WriteString("ONE_USE\n")
	ce := f.Close()
	if e != nil || ce != nil {
		traceRepairedBindingStop(dir)
		return errTraceRepairedBindingSlot
	}
	return nil
}
func traceRepairedBindingRead(path string, limit int64) ([]byte, error) {
	info, e := os.Lstat(path)
	if e != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 || info.Size() > limit {
		return nil, errTraceRepairedBindingIO
	}
	f, e := os.Open(path)
	if e != nil {
		return nil, errTraceRepairedBindingIO
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, limit+1))
	if e != nil || int64(len(b)) > limit {
		return nil, errTraceRepairedBindingIO
	}
	return b, nil
}
func traceRepairedBindingLedgerValid(raw []byte, next string) bool {
	if !traceRepairedBindingCase(next) {
		return false
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(raw) == 0 {
		return true
	}
	if len(lines) >= 1 {
		return false
	}
	seen := map[string]bool{}
	for _, line := range lines {
		var m map[string]string
		if json.Unmarshal([]byte(line), &m) != nil || len(m) != 3 || m["run"] != traceRepairedBindingPhase || m["event"] != "CONTROLLED_PREPARATION_ATTEMPT" || !traceRepairedBindingCase(m["case"]) || seen[m["case"]] || m["case"] == next {
			return false
		}
		seen[m["case"]] = true
	}
	return true
}
func traceRepairedBindingSHA(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

type traceRepairedBindingRelease struct {
	Phase        string   `json:"phase"`
	PreflightSHA string   `json:"preflight_sha256"`
	AllowNative  bool     `json:"allow_native"`
	Cases        []string `json:"cases"`
	MaxAttempts  int      `json:"max_attempts"`
	TraceCap     int64    `json:"trace_cap"`
	TraceTotal   int64    `json:"trace_total"`
	Expires      int64    `json:"expires_unix"`
}

func traceRepairedBindingReleaseValid(q traceRepairedBindingRelease, pre []byte, now int64) bool {
	if !traceRepairedBindingSourceFrameValid(pre) {
		return false
	}
	if q.Phase != traceRepairedBindingPhase || !q.AllowNative || q.PreflightSHA != traceRepairedBindingSHA(pre) || q.MaxAttempts != 1 || q.TraceCap != traceRepairedBindingCap || q.TraceTotal != traceRepairedBindingCap || q.Expires <= now || q.Expires > now+1800 || len(q.Cases) != 1 {
		return false
	}
	for i, c := range traceRepairedBindingCases {
		if q.Cases[i] != c {
			return false
		}
	}
	return true
}

// The accepted05 clone and09 predecessor-repair postimages are mandatory, even when a release hashes a
// syntactically valid but incomplete preflight. Actual source hashes are also
// checked by the separately bound parent controller before every launch.
func traceRepairedBindingSourceFrameValid(pre []byte) bool {
	var p struct {
		DiagnosticCodes []string `json:"diagnostic_codes"`
		SourceFrame     []struct {
			Path string `json:"path"`
			SHA  string `json:"sha256"`
		} `json:"source_frame"`
	}
	if json.Unmarshal(pre, &p) != nil {
		return false
	}
	for _, required := range traceRepairedBindingDiagnosticCodes {
		found := false
		for _, code := range p.DiagnosticCodes {
			if code == required {
				if found {
					return false
				}
				found = true
			}
		}
		if !found {
			return false
		}
	}
	want := map[string]string{
		"internal/inventorysuccessor/private_managed_input_operation_v2.go":   "a922f7411d2d22e3e26b8b43b808009decbf413a8ddd0ca1fe40ae9c33bbb29d",
		"internal/referencegrouping/input.go":                                 "36f784ae1465eb2477799c610fa6e95226e5f5b40fe2d596984a9c19e912d316",
		"internal/inventorytracediag/trace.go":                                "0c6fd9cfddf9576a758d08ab62494c039b95bcce8acdea6847d36bd876cb3613",
		"internal/programcadmission/typed_boundary.go":                        "5293bca9a78339c7a371cd34970c7da26dfe0088ac5260c94cb2b2861b41a4bb",
		"internal/programcadmission/empty_admission_clone_regression_test.go": "dad4edd1d508beb6fb533c313ab9268270ffa53f68b50e2313b85aa67a9b7f23",
		"internal/referencegrouping/empty_calls_predecessors_pure_test.go":    "961d30c5afcdebd4885a7f076f057a8622795682e19e329c83d50069baa43478",
		"internal/referencegrouping/empty_calls_native_codec_pure_test.go":    "f43aaa6f6381e7d4155251577b4c6ddd9b4378c7454099cb69adeb028477912e",
		"internal/inventorysuccessor/duplicate_calls_assembly_pure_test.go":   "072eebe3e282f24236c121e15eb7fbd9aa85707a468a85dbcfeecd7c9beb094a",
		"internal/inventorytracediag/phase14_test.go":                         "1b4e9d4f8a4b8f7298f609f6fb84c19bb470621388fb4dc99ba21f1d369b83ed",
		"internal/programcadmission/leiden_projection.go":                     "58fe6b796c6541723cf64f7d246b895449bb2974a7e151cbbd6f2553ab921ee1",
		"internal/programcadmission/private_binding_budget.go":                "fc3d091e6ce07b8896abab51067b2fcad15e4374094cbd28f77ad5901589cfd8",
		"internal/programcadmission/private_binding_budget_test.go":           "b7e4dae5d6a31971fe069036b86270336dd68ae7c64e1a24b98519300201c865",
		"internal/inventorysuccessor/sealed_binding_budget_18_pure_test.go":   "18cc95541c0d717874df238bd2b1809871b790f842d190488e29c775076f3a75",
	}
	seen := map[string]bool{}
	for _, entry := range p.SourceFrame {
		if seen[entry.Path] {
			return false
		}
		seen[entry.Path] = true
		if h, ok := want[entry.Path]; ok {
			if h != entry.SHA {
				return false
			}
			delete(want, entry.Path)
		}
	}
	return len(want) == 0
}

func traceRepairedBindingReleased() bool {
	raw, e := traceRepairedBindingRead(traceRepairedBindingBase+"/parent-release.json", 16<<10)
	if e != nil {
		return false
	}
	var q traceRepairedBindingRelease
	d := json.NewDecoder(strings.NewReader(string(raw)))
	d.DisallowUnknownFields()
	if d.Decode(&q) != nil {
		return false
	}
	var more any
	if d.Decode(&more) != io.EOF {
		return false
	}
	pre, e := traceRepairedBindingRead(traceRepairedBindingBase+"/PREFLIGHT.json", 128<<10)
	return e == nil && traceRepairedBindingReleaseValid(q, pre, time.Now().Unix())
}

// The only lifecycle owner added here is this opt-in TEST controller. It creates
// no task/context and calls the original test unchanged. Native meaning may be
// BLOCKED even if the Go test passes; only parent-side records adjudicate it.
func TestConfigurationV3RuntimeTraceRepairedBinding(t *testing.T) {
	if os.Getenv("LSP_TRACE_REPAIREDBINDING_ENABLE") != "PARENT_RELEASED_PHASE19" {
		t.Skip("TRACE_REPAIREDBINDING_OFF")
	}
	id := os.Getenv("LSP_TRACE_REPAIREDBINDING_CASE")
	if t.Name() != "TestConfigurationV3RuntimeTraceRepairedBinding" || !traceRepairedBindingSelection(id, flag.Lookup("test.run").Value.String(), os.Getenv("CONFIGURATION_V3_COUNT_FILE"), os.Getenv("CONFIGURATION_V3_RUN"), os.Args[1:]) || flag.Lookup("test.trace").Value.String() != "" || !traceRepairedBindingReleased() {
		t.Fatal("TRACE_REPAIREDBINDING_PREFLIGHT_REFUSED")
	}
	old, e := traceRepairedBindingRead(traceRepairedBindingOldLedger, 128<<10)
	if e != nil || traceRepairedBindingSHA(old) != traceRepairedBindingOldSHA {
		t.Fatal("TRACE_REPAIREDBINDING_OLD_COUNTER_REFUSED")
	}
	ledger, e := traceRepairedBindingRead(traceRepairedBindingLedger, 32<<10)
	if e != nil || !traceRepairedBindingLedgerValid(ledger, id) {
		t.Fatal("TRACE_REPAIREDBINDING_LEDGER_REFUSED")
	}
	if trace.IsEnabled() {
		t.Fatal("TRACE_REPAIREDBINDING_ALREADY_TRACING")
	}
	if e = traceRepairedBindingClaim(traceRepairedBindingExecution, id); e != nil {
		t.Fatal("TRACE_REPAIREDBINDING_SLOT_REFUSED")
	}
	f, e := os.OpenFile(filepath.Join(traceRepairedBindingExecution, id+".trace"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		traceRepairedBindingStop(traceRepairedBindingExecution)
		t.Fatal("TRACE_REPAIREDBINDING_FILE_REFUSED")
	}
	writer := &traceRepairedBindingWriter{dst: f, limit: traceRepairedBindingCap}
	started := false
	returned := false
	t.Cleanup(func() {
		if started {
			trace.Stop()
		}
		closeErr := f.Close()
		n, bad := writer.state()
		ok := started && returned && !bad && closeErr == nil && !t.Failed()
		code := "TRACE_UNUSABLE"
		if ok {
			code = "TRACE_COMPLETE"
		} else {
			traceRepairedBindingStop(traceRepairedBindingExecution)
			t.Error("TRACE_REPAIREDBINDING_UNUSABLE")
		}
		marker := struct {
			Case       string `json:"case"`
			Code       string `json:"code"`
			Bytes      int64  `json:"bytes"`
			Overflow   bool   `json:"overflow"`
			TestPassed bool   `json:"test_passed"`
		}{id, code, n, writer.overflowed(), !t.Failed()}
		raw, _ := json.Marshal(marker)
		out, err := os.OpenFile(filepath.Join(traceRepairedBindingExecution, id+".complete.json"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err == nil {
			_, err = out.Write(append(raw, '\n'))
			ce := out.Close()
			if ce != nil {
				err = ce
			}
		}
		if err != nil {
			traceRepairedBindingStop(traceRepairedBindingExecution)
			t.Error("TRACE_REPAIREDBINDING_COMPLETION_REFUSED")
		}
	})
	if trace.Start(writer) != nil {
		t.Fatal("TRACE_REPAIREDBINDING_START_REFUSED")
	}
	started = true
	TestConfigurationV3ProviderStates(t)
	returned = true
}

// Exact registry-derived required codes; preparation preimage is intentionally incomplete.
var traceRepairedBindingDiagnosticCodes = []string{"COMPOSITE_BINDING_BOUND_REFUSED", "COMPOSITE_BINDING_JSON_REFUSED", "NATIVE_PREDECESSOR_COUNT_REFUSED", "NATIVE_PREDECESSOR_BYTES_REFUSED", "NATIVE_AGGREGATE_BOUND_REFUSED"}
