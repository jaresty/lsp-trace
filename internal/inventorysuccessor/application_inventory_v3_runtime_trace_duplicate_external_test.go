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

const traceDuplicatePhase = "runtime-trace-duplicate-13"
const traceDuplicateBase = "/Users/schwa/.local/state/lsp-trace-evidence/seven-configuration-campaign-v3-20261112-01/runtime-trace-duplicate-28f7-13"
const traceDuplicateExecution = traceDuplicateBase + "/execution"
const traceDuplicateLedger = traceDuplicateExecution + "/preparations.jsonl"
const traceDuplicateOldLedger = "/Users/schwa/.local/state/lsp-trace-evidence/seven-configuration-campaign-v3-20261112-01/writer-716d-01/preparations.jsonl"
const traceDuplicateOldSHA = "a21c1d1a83c3caddfed8c144a10ce9854c87bb99f0fda15458ecd436b1164431"
const traceDuplicateCap int64 = 8 << 20

var traceDuplicateCases = [...]string{"provider-C-composed-duplicates"}
var errTraceDuplicateCap = errors.New("TRACE_DUPLICATE_CAP")
var errTraceDuplicateIO = errors.New("TRACE_DUPLICATE_IO")
var errTraceDuplicateSlot = errors.New("TRACE_DUPLICATE_SLOT_REFUSED")

func traceDuplicateCase(id string) bool {
	for _, c := range traceDuplicateCases {
		if c == id {
			return true
		}
	}
	return false
}
func traceDuplicateRun(id string) string {
	return "^TestConfigurationV3RuntimeTraceDuplicate$/^" + id + "$"
}
func traceDuplicateSelection(id, run, ledger, phase string, args []string) bool {
	if !traceDuplicateCase(id) || run != traceDuplicateRun(id) || ledger != traceDuplicateLedger || phase != traceDuplicatePhase {
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
type traceDuplicateWriter struct {
	mu           sync.Mutex
	dst          io.Writer
	limit, n     int64
	failed, over bool
}

func (w *traceDuplicateWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.failed {
		return 0, errTraceDuplicateCap
	}
	if w.limit < 0 || w.n < 0 || w.n > w.limit {
		w.failed = true
		return 0, errTraceDuplicateIO
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
		return 0, errTraceDuplicateIO
	}
	w.n += int64(n)
	if e != nil || n != len(p) {
		w.failed = true
		return n, errTraceDuplicateIO
	}
	if overflow {
		w.failed = true
		return n, errTraceDuplicateCap
	}
	return n, nil
}
func (w *traceDuplicateWriter) state() (int64, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.n, w.failed
}
func (w *traceDuplicateWriter) overflowed() bool { w.mu.Lock(); defer w.mu.Unlock(); return w.over }
func traceDuplicateStop(dir string) {
	f, e := os.OpenFile(filepath.Join(dir, "STOP"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e == nil {
		_, _ = f.WriteString("TRACE_DUPLICATE_STOP\n")
		_ = f.Close()
	}
}
func traceDuplicateClaim(dir, id string) error {
	if !traceDuplicateCase(id) {
		return errTraceDuplicateSlot
	}
	info, e := os.Lstat(dir)
	if e != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0700 {
		return errTraceDuplicateSlot
	}
	if _, e = os.Lstat(filepath.Join(dir, "STOP")); !os.IsNotExist(e) {
		return errTraceDuplicateSlot
	}
	entries, e := os.ReadDir(dir)
	if e != nil {
		return errTraceDuplicateSlot
	}
	used := 0
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".used") {
			if !traceDuplicateCase(strings.TrimSuffix(entry.Name(), ".used")) {
				return errTraceDuplicateSlot
			}
			used++
		}
	}
	if used >= len(traceDuplicateCases) {
		return errTraceDuplicateSlot
	}
	f, e := os.OpenFile(filepath.Join(dir, id+".used"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return errTraceDuplicateSlot
	}
	_, e = f.WriteString("ONE_USE\n")
	ce := f.Close()
	if e != nil || ce != nil {
		traceDuplicateStop(dir)
		return errTraceDuplicateSlot
	}
	return nil
}
func traceDuplicateRead(path string, limit int64) ([]byte, error) {
	info, e := os.Lstat(path)
	if e != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 || info.Size() > limit {
		return nil, errTraceDuplicateIO
	}
	f, e := os.Open(path)
	if e != nil {
		return nil, errTraceDuplicateIO
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, limit+1))
	if e != nil || int64(len(b)) > limit {
		return nil, errTraceDuplicateIO
	}
	return b, nil
}
func traceDuplicateLedgerValid(raw []byte, next string) bool {
	if !traceDuplicateCase(next) {
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
		if json.Unmarshal([]byte(line), &m) != nil || len(m) != 3 || m["run"] != traceDuplicatePhase || m["event"] != "CONTROLLED_PREPARATION_ATTEMPT" || !traceDuplicateCase(m["case"]) || seen[m["case"]] || m["case"] == next {
			return false
		}
		seen[m["case"]] = true
	}
	return true
}
func traceDuplicateSHA(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

type traceDuplicateRelease struct {
	Phase        string   `json:"phase"`
	PreflightSHA string   `json:"preflight_sha256"`
	AllowNative  bool     `json:"allow_native"`
	Cases        []string `json:"cases"`
	MaxAttempts  int      `json:"max_attempts"`
	TraceCap     int64    `json:"trace_cap"`
	TraceTotal   int64    `json:"trace_total"`
	Expires      int64    `json:"expires_unix"`
}

func traceDuplicateReleaseValid(q traceDuplicateRelease, pre []byte, now int64) bool {
	if !traceDuplicateSourceFrameValid(pre) {
		return false
	}
	if q.Phase != traceDuplicatePhase || !q.AllowNative || q.PreflightSHA != traceDuplicateSHA(pre) || q.MaxAttempts != 1 || q.TraceCap != traceDuplicateCap || q.TraceTotal != traceDuplicateCap || q.Expires <= now || q.Expires > now+1800 || len(q.Cases) != 1 {
		return false
	}
	for i, c := range traceDuplicateCases {
		if q.Cases[i] != c {
			return false
		}
	}
	return true
}

// The accepted05 clone and09 predecessor-repair postimages are mandatory, even when a release hashes a
// syntactically valid but incomplete preflight. Actual source hashes are also
// checked by the separately bound parent controller before every launch.
func traceDuplicateSourceFrameValid(pre []byte) bool {
	var p struct {
		SourceFrame []struct {
			Path string `json:"path"`
			SHA  string `json:"sha256"`
		} `json:"source_frame"`
	}
	if json.Unmarshal(pre, &p) != nil {
		return false
	}
	want := map[string]string{
		"internal/programcadmission/typed_boundary.go":                        "5293bca9a78339c7a371cd34970c7da26dfe0088ac5260c94cb2b2861b41a4bb",
		"internal/programcadmission/empty_admission_clone_regression_test.go": "dad4edd1d508beb6fb533c313ab9268270ffa53f68b50e2313b85aa67a9b7f23",
		"internal/referencegrouping/input.go":                                 "36f784ae1465eb2477799c610fa6e95226e5f5b40fe2d596984a9c19e912d316",
		"internal/referencegrouping/empty_calls_predecessors_pure_test.go":    "961d30c5afcdebd4885a7f076f057a8622795682e19e329c83d50069baa43478",
		"internal/referencegrouping/empty_calls_native_codec_pure_test.go":    "f43aaa6f6381e7d4155251577b4c6ddd9b4378c7454099cb69adeb028477912e",
		"internal/inventorysuccessor/duplicate_calls_assembly_pure_test.go":   "072eebe3e282f24236c121e15eb7fbd9aa85707a468a85dbcfeecd7c9beb094a",
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

func traceDuplicateReleased() bool {
	raw, e := traceDuplicateRead(traceDuplicateBase+"/parent-release.json", 16<<10)
	if e != nil {
		return false
	}
	var q traceDuplicateRelease
	d := json.NewDecoder(strings.NewReader(string(raw)))
	d.DisallowUnknownFields()
	if d.Decode(&q) != nil {
		return false
	}
	var more any
	if d.Decode(&more) != io.EOF {
		return false
	}
	pre, e := traceDuplicateRead(traceDuplicateBase+"/PREFLIGHT.json", 128<<10)
	return e == nil && traceDuplicateReleaseValid(q, pre, time.Now().Unix())
}

// The only lifecycle owner added here is this opt-in TEST controller. It creates
// no task/context and calls the original test unchanged. Native meaning may be
// BLOCKED even if the Go test passes; only parent-side records adjudicate it.
func TestConfigurationV3RuntimeTraceDuplicate(t *testing.T) {
	if os.Getenv("LSP_TRACE_DUPLICATE_ENABLE") != "PARENT_RELEASED_PHASE13" {
		t.Skip("TRACE_DUPLICATE_OFF")
	}
	id := os.Getenv("LSP_TRACE_DUPLICATE_CASE")
	if t.Name() != "TestConfigurationV3RuntimeTraceDuplicate" || !traceDuplicateSelection(id, flag.Lookup("test.run").Value.String(), os.Getenv("CONFIGURATION_V3_COUNT_FILE"), os.Getenv("CONFIGURATION_V3_RUN"), os.Args[1:]) || flag.Lookup("test.trace").Value.String() != "" || !traceDuplicateReleased() {
		t.Fatal("TRACE_DUPLICATE_PREFLIGHT_REFUSED")
	}
	old, e := traceDuplicateRead(traceDuplicateOldLedger, 128<<10)
	if e != nil || traceDuplicateSHA(old) != traceDuplicateOldSHA {
		t.Fatal("TRACE_DUPLICATE_OLD_COUNTER_REFUSED")
	}
	ledger, e := traceDuplicateRead(traceDuplicateLedger, 32<<10)
	if e != nil || !traceDuplicateLedgerValid(ledger, id) {
		t.Fatal("TRACE_DUPLICATE_LEDGER_REFUSED")
	}
	if trace.IsEnabled() {
		t.Fatal("TRACE_DUPLICATE_ALREADY_TRACING")
	}
	if e = traceDuplicateClaim(traceDuplicateExecution, id); e != nil {
		t.Fatal("TRACE_DUPLICATE_SLOT_REFUSED")
	}
	f, e := os.OpenFile(filepath.Join(traceDuplicateExecution, id+".trace"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		traceDuplicateStop(traceDuplicateExecution)
		t.Fatal("TRACE_DUPLICATE_FILE_REFUSED")
	}
	writer := &traceDuplicateWriter{dst: f, limit: traceDuplicateCap}
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
			traceDuplicateStop(traceDuplicateExecution)
			t.Error("TRACE_DUPLICATE_UNUSABLE")
		}
		marker := struct {
			Case       string `json:"case"`
			Code       string `json:"code"`
			Bytes      int64  `json:"bytes"`
			Overflow   bool   `json:"overflow"`
			TestPassed bool   `json:"test_passed"`
		}{id, code, n, writer.overflowed(), !t.Failed()}
		raw, _ := json.Marshal(marker)
		out, err := os.OpenFile(filepath.Join(traceDuplicateExecution, id+".complete.json"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err == nil {
			_, err = out.Write(append(raw, '\n'))
			ce := out.Close()
			if ce != nil {
				err = ce
			}
		}
		if err != nil {
			traceDuplicateStop(traceDuplicateExecution)
			t.Error("TRACE_DUPLICATE_COMPLETION_REFUSED")
		}
	})
	if trace.Start(writer) != nil {
		t.Fatal("TRACE_DUPLICATE_START_REFUSED")
	}
	started = true
	TestConfigurationV3ProviderStates(t)
	returned = true
}
