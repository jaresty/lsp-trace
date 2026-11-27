//go:build darwin || linux

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

const traceCorrectedDuplicatePhase = "runtime-trace-correctedduplicate-21"
const traceCorrectedDuplicateBase = "/Users/schwa/.local/state/lsp-trace-evidence/seven-configuration-campaign-v3-20261112-01/runtime-trace-correctedduplicate-21"
const traceCorrectedDuplicateExecution = traceCorrectedDuplicateBase + "/execution"
const traceCorrectedDuplicateLedger = traceCorrectedDuplicateExecution + "/preparations.jsonl"
const traceCorrectedDuplicateOldLedger = "/Users/schwa/.local/state/lsp-trace-evidence/seven-configuration-campaign-v3-20261112-01/writer-716d-01/preparations.jsonl"
const traceCorrectedDuplicateOldSHA = "a21c1d1a83c3caddfed8c144a10ce9854c87bb99f0fda15458ecd436b1164431"
const traceCorrectedDuplicateCap int64 = 8 << 20

var traceCorrectedDuplicateCases = [...]string{"provider-C-composed-duplicates"}
var errTraceCorrectedDuplicateCap = errors.New("TRACE_CORRECTEDDUPLICATE_CAP")
var errTraceCorrectedDuplicateIO = errors.New("TRACE_CORRECTEDDUPLICATE_IO")
var errTraceCorrectedDuplicateSlot = errors.New("TRACE_CORRECTEDDUPLICATE_SLOT_REFUSED")

func traceCorrectedDuplicateCase(id string) bool {
	for _, c := range traceCorrectedDuplicateCases {
		if c == id {
			return true
		}
	}
	return false
}
func traceCorrectedDuplicateRun(id string) string {
	return "^TestConfigurationV3RuntimeTraceCorrectedDuplicate$/^" + id + "$"
}
func traceCorrectedDuplicateSelection(id, run, ledger, phase string, args []string) bool {
	if !traceCorrectedDuplicateCase(id) || run != traceCorrectedDuplicateRun(id) || ledger != traceCorrectedDuplicateLedger || phase != traceCorrectedDuplicatePhase {
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
type traceCorrectedDuplicateWriter struct {
	mu           sync.Mutex
	dst          io.Writer
	limit, n     int64
	failed, over bool
}

func (w *traceCorrectedDuplicateWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.failed {
		return 0, errTraceCorrectedDuplicateCap
	}
	if w.limit < 0 || w.n < 0 || w.n > w.limit {
		w.failed = true
		return 0, errTraceCorrectedDuplicateIO
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
		return 0, errTraceCorrectedDuplicateIO
	}
	w.n += int64(n)
	if e != nil || n != len(p) {
		w.failed = true
		return n, errTraceCorrectedDuplicateIO
	}
	if overflow {
		w.failed = true
		return n, errTraceCorrectedDuplicateCap
	}
	return n, nil
}
func (w *traceCorrectedDuplicateWriter) state() (int64, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.n, w.failed
}
func (w *traceCorrectedDuplicateWriter) overflowed() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.over
}
func traceCorrectedDuplicateStop(dir string) {
	f, e := os.OpenFile(filepath.Join(dir, "STOP"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e == nil {
		_, _ = f.WriteString("TRACE_CORRECTEDDUPLICATE_STOP\n")
		_ = f.Close()
	}
}
func traceCorrectedDuplicateClaim(dir, id string) error {
	if !traceCorrectedDuplicateCase(id) {
		return errTraceCorrectedDuplicateSlot
	}
	info, e := os.Lstat(dir)
	if e != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0700 {
		return errTraceCorrectedDuplicateSlot
	}
	if _, e = os.Lstat(filepath.Join(dir, "STOP")); !os.IsNotExist(e) {
		return errTraceCorrectedDuplicateSlot
	}
	entries, e := os.ReadDir(dir)
	if e != nil {
		return errTraceCorrectedDuplicateSlot
	}
	used := 0
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".used") {
			if !traceCorrectedDuplicateCase(strings.TrimSuffix(entry.Name(), ".used")) {
				return errTraceCorrectedDuplicateSlot
			}
			used++
		}
	}
	if used >= len(traceCorrectedDuplicateCases) {
		return errTraceCorrectedDuplicateSlot
	}
	f, e := os.OpenFile(filepath.Join(dir, id+".used"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return errTraceCorrectedDuplicateSlot
	}
	_, e = f.WriteString("ONE_USE\n")
	ce := f.Close()
	if e != nil || ce != nil {
		traceCorrectedDuplicateStop(dir)
		return errTraceCorrectedDuplicateSlot
	}
	return nil
}
func traceCorrectedDuplicateRead(path string, limit int64) ([]byte, error) {
	info, e := os.Lstat(path)
	if e != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 || info.Size() > limit {
		return nil, errTraceCorrectedDuplicateIO
	}
	f, e := os.Open(path)
	if e != nil {
		return nil, errTraceCorrectedDuplicateIO
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, limit+1))
	if e != nil || int64(len(b)) > limit {
		return nil, errTraceCorrectedDuplicateIO
	}
	return b, nil
}
func traceCorrectedDuplicateLedgerValid(raw []byte, next string) bool {
	if !traceCorrectedDuplicateCase(next) {
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
		if json.Unmarshal([]byte(line), &m) != nil || len(m) != 3 || m["run"] != traceCorrectedDuplicatePhase || m["event"] != "CONTROLLED_PREPARATION_ATTEMPT" || !traceCorrectedDuplicateCase(m["case"]) || seen[m["case"]] || m["case"] == next {
			return false
		}
		seen[m["case"]] = true
	}
	return true
}
func traceCorrectedDuplicateSHA(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

type traceCorrectedDuplicateRelease struct {
	Phase        string   `json:"phase"`
	PreflightSHA string   `json:"preflight_sha256"`
	AllowNative  bool     `json:"allow_native"`
	Cases        []string `json:"cases"`
	MaxAttempts  int      `json:"max_attempts"`
	TraceCap     int64    `json:"trace_cap"`
	TraceTotal   int64    `json:"trace_total"`
	Expires      int64    `json:"expires_unix"`
}

func traceCorrectedDuplicateReleaseValid(q traceCorrectedDuplicateRelease, pre []byte, now int64) bool {
	if !traceCorrectedDuplicateSourceFrameValid(pre) {
		return false
	}
	if q.Phase != traceCorrectedDuplicatePhase || !q.AllowNative || q.PreflightSHA != traceCorrectedDuplicateSHA(pre) || q.MaxAttempts != 1 || q.TraceCap != traceCorrectedDuplicateCap || q.TraceTotal != traceCorrectedDuplicateCap || q.Expires <= now || q.Expires > now+600 || len(q.Cases) != 1 {
		return false
	}
	for i, c := range traceCorrectedDuplicateCases {
		if q.Cases[i] != c {
			return false
		}
	}
	return true
}

// The accepted05 clone and09 predecessor-repair postimages are mandatory, even when a release hashes a
// syntactically valid but incomplete preflight. Actual source hashes are also
// checked by the separately bound parent controller before every launch.
func traceCorrectedDuplicateSourceFrameValid(pre []byte) bool {
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
	for _, required := range traceCorrectedDuplicateDiagnosticCodes {
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
	// Current accepted frame, not renewed historical release authority. The phase14
	// baseline supersedes the unavailable old pin; it does not recover old history
	// or assert old/new byte equivalence.
	want := map[string]string{
		"internal/inventorysuccessor/private_managed_input_operation_v2.go":                             "650f79f91b5ccef3194937cdb57aa4ffdceea84027c85ca3b9b1309b5c0f7eba",
		"internal/referencegrouping/input.go":                                                           "36f784ae1465eb2477799c610fa6e95226e5f5b40fe2d596984a9c19e912d316",
		"internal/inventorytracediag/trace.go":                                                          "0c6fd9cfddf9576a758d08ab62494c039b95bcce8acdea6847d36bd876cb3613",
		"internal/programcadmission/typed_boundary.go":                                                  "5293bca9a78339c7a371cd34970c7da26dfe0088ac5260c94cb2b2861b41a4bb",
		"internal/programcadmission/empty_admission_clone_regression_test.go":                           "dad4edd1d508beb6fb533c313ab9268270ffa53f68b50e2313b85aa67a9b7f23",
		"internal/referencegrouping/empty_calls_predecessors_pure_test.go":                              "961d30c5afcdebd4885a7f076f057a8622795682e19e329c83d50069baa43478",
		"internal/referencegrouping/empty_calls_native_codec_pure_test.go":                              "f43aaa6f6381e7d4155251577b4c6ddd9b4378c7454099cb69adeb028477912e",
		"internal/inventorysuccessor/duplicate_calls_assembly_pure_test.go":                             "072eebe3e282f24236c121e15eb7fbd9aa85707a468a85dbcfeecd7c9beb094a",
		"internal/inventorytracediag/phase14_test.go":                                                   "5cf132be68a897f637a6c3534543fbf58e66c54bfbe464896f8f3e953fd08bfc",
		"internal/programcadmission/leiden_projection.go":                                               "58fe6b796c6541723cf64f7d246b895449bb2974a7e151cbbd6f2553ab921ee1",
		"internal/programcadmission/private_binding_budget.go":                                          "fc3d091e6ce07b8896abab51067b2fcad15e4374094cbd28f77ad5901589cfd8",
		"internal/programcadmission/private_binding_budget_test.go":                                     "b7e4dae5d6a31971fe069036b86270336dd68ae7c64e1a24b98519300201c865",
		"internal/inventorysuccessor/sealed_binding_budget_18_pure_test.go":                             "18cc95541c0d717874df238bd2b1809871b790f842d190488e29c775076f3a75",
		"internal/inventorysuccessor/application_inventory_v3_configuration_external_test.go":           "76bf3fc9142309665287702d46ca015c79d4266ec543fd2be0ad7982f44ce4e3",
		"internal/inventorysuccessor/application_inventory_v3_configuration_responder_external_test.go": "dad5a60f35373681d97d693d69f91b230089a02b6123b5aaf3dead880e2ef086",
		"internal/inventorysuccessor/application_inventory_v3_duplicate_literal_pure_external_test.go":  "e136357f229ab23d00e74d290138584bd16fe324f28447b90e10c5e1548882fe",
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

func traceCorrectedDuplicateReleased() bool {
	raw, e := traceCorrectedDuplicateRead(traceCorrectedDuplicateBase+"/parent-release.json", 16<<10)
	if e != nil {
		return false
	}
	var q traceCorrectedDuplicateRelease
	d := json.NewDecoder(strings.NewReader(string(raw)))
	d.DisallowUnknownFields()
	if d.Decode(&q) != nil {
		return false
	}
	var more any
	if d.Decode(&more) != io.EOF {
		return false
	}
	pre, e := traceCorrectedDuplicateRead(traceCorrectedDuplicateBase+"/PREFLIGHT.json", 128<<10)
	return e == nil && traceCorrectedDuplicateReleaseValid(q, pre, time.Now().Unix())
}

// The only lifecycle owner added here is this opt-in TEST controller. It creates
// no task/context and calls the original test unchanged. Native meaning may be
// BLOCKED even if the Go test passes; only parent-side records adjudicate it.
func TestConfigurationV3RuntimeTraceCorrectedDuplicate(t *testing.T) {
	if os.Getenv("LSP_TRACE_CORRECTEDDUPLICATE_ENABLE") != "PARENT_RELEASED_PHASE21" {
		t.Skip("TRACE_CORRECTEDDUPLICATE_OFF")
	}
	id := os.Getenv("LSP_TRACE_CORRECTEDDUPLICATE_CASE")
	if t.Name() != "TestConfigurationV3RuntimeTraceCorrectedDuplicate" || !traceCorrectedDuplicateSelection(id, flag.Lookup("test.run").Value.String(), os.Getenv("CONFIGURATION_V3_COUNT_FILE"), os.Getenv("CONFIGURATION_V3_RUN"), os.Args[1:]) || flag.Lookup("test.trace").Value.String() != "" || !traceCorrectedDuplicateReleased() {
		t.Fatal("TRACE_CORRECTEDDUPLICATE_PREFLIGHT_REFUSED")
	}
	old, e := traceCorrectedDuplicateRead(traceCorrectedDuplicateOldLedger, 128<<10)
	if e != nil || traceCorrectedDuplicateSHA(old) != traceCorrectedDuplicateOldSHA {
		t.Fatal("TRACE_CORRECTEDDUPLICATE_OLD_COUNTER_REFUSED")
	}
	ledger, e := traceCorrectedDuplicateRead(traceCorrectedDuplicateLedger, 32<<10)
	if e != nil || !traceCorrectedDuplicateLedgerValid(ledger, id) {
		t.Fatal("TRACE_CORRECTEDDUPLICATE_LEDGER_REFUSED")
	}
	if trace.IsEnabled() {
		t.Fatal("TRACE_CORRECTEDDUPLICATE_ALREADY_TRACING")
	}
	if e = traceCorrectedDuplicateClaim(traceCorrectedDuplicateExecution, id); e != nil {
		t.Fatal("TRACE_CORRECTEDDUPLICATE_SLOT_REFUSED")
	}
	f, e := os.OpenFile(filepath.Join(traceCorrectedDuplicateExecution, id+".trace"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		traceCorrectedDuplicateStop(traceCorrectedDuplicateExecution)
		t.Fatal("TRACE_CORRECTEDDUPLICATE_FILE_REFUSED")
	}
	writer := &traceCorrectedDuplicateWriter{dst: f, limit: traceCorrectedDuplicateCap}
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
			traceCorrectedDuplicateStop(traceCorrectedDuplicateExecution)
			t.Error("TRACE_CORRECTEDDUPLICATE_UNUSABLE")
		}
		marker := struct {
			Case       string `json:"case"`
			Code       string `json:"code"`
			Bytes      int64  `json:"bytes"`
			Overflow   bool   `json:"overflow"`
			TestPassed bool   `json:"test_passed"`
		}{id, code, n, writer.overflowed(), !t.Failed()}
		raw, _ := json.Marshal(marker)
		out, err := os.OpenFile(filepath.Join(traceCorrectedDuplicateExecution, id+".complete.json"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err == nil {
			_, err = out.Write(append(raw, '\n'))
			ce := out.Close()
			if ce != nil {
				err = ce
			}
		}
		if err != nil {
			traceCorrectedDuplicateStop(traceCorrectedDuplicateExecution)
			t.Error("TRACE_CORRECTEDDUPLICATE_COMPLETION_REFUSED")
		}
	})
	if trace.Start(writer) != nil {
		t.Fatal("TRACE_CORRECTEDDUPLICATE_START_REFUSED")
	}
	started = true
	TestConfigurationV3ProviderStates(t)
	returned = true
}

// Exact registry-derived required codes; preparation preimage is intentionally incomplete.
var traceCorrectedDuplicateDiagnosticCodes = []string{"COMPOSITE_BINDING_BOUND_REFUSED", "COMPOSITE_BINDING_JSON_REFUSED", "NATIVE_PREDECESSOR_COUNT_REFUSED", "NATIVE_PREDECESSOR_BYTES_REFUSED", "NATIVE_AGGREGATE_BOUND_REFUSED"}
