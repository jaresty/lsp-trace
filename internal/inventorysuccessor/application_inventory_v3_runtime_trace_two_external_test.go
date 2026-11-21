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

const traceTwoPhase = "runtime-trace-two-06"
const traceTwoBase = "/Users/schwa/.local/state/lsp-trace-evidence/seven-configuration-campaign-v3-20261112-01/runtime-trace-two-716d-06"
const traceTwoExecution = traceTwoBase + "/execution"
const traceTwoLedger = traceTwoExecution + "/preparations.jsonl"
const traceTwoOldLedger = "/Users/schwa/.local/state/lsp-trace-evidence/seven-configuration-campaign-v3-20261112-01/writer-716d-01/preparations.jsonl"
const traceTwoOldSHA = "a21c1d1a83c3caddfed8c144a10ce9854c87bb99f0fda15458ecd436b1164431"
const traceTwoCap int64 = 8 << 20

var traceTwoCases = [...]string{"provider-C-empty", "provider-mixed-CR-C-empty"}
var errTraceTwoCap = errors.New("TRACE_TWO_CAP")
var errTraceTwoIO = errors.New("TRACE_TWO_IO")
var errTraceTwoSlot = errors.New("TRACE_TWO_SLOT_REFUSED")

func traceTwoCase(id string) bool {
	for _, c := range traceTwoCases {
		if c == id {
			return true
		}
	}
	return false
}
func traceTwoRun(id string) string { return "^TestConfigurationV3RuntimeTraceTwo$/^" + id + "$" }
func traceTwoSelection(id, run, ledger, phase string, args []string) bool {
	if !traceTwoCase(id) || run != traceTwoRun(id) || ledger != traceTwoLedger || phase != traceTwoPhase {
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
type traceTwoWriter struct {
	mu           sync.Mutex
	dst          io.Writer
	limit, n     int64
	failed, over bool
}

func (w *traceTwoWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.failed {
		return 0, errTraceTwoCap
	}
	if w.limit < 0 || w.n < 0 || w.n > w.limit {
		w.failed = true
		return 0, errTraceTwoIO
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
		return 0, errTraceTwoIO
	}
	w.n += int64(n)
	if e != nil || n != len(p) {
		w.failed = true
		return n, errTraceTwoIO
	}
	if overflow {
		w.failed = true
		return n, errTraceTwoCap
	}
	return n, nil
}
func (w *traceTwoWriter) state() (int64, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.n, w.failed
}
func (w *traceTwoWriter) overflowed() bool { w.mu.Lock(); defer w.mu.Unlock(); return w.over }
func traceTwoStop(dir string) {
	f, e := os.OpenFile(filepath.Join(dir, "STOP"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e == nil {
		_, _ = f.WriteString("TRACE_TWO_STOP\n")
		_ = f.Close()
	}
}
func traceTwoClaim(dir, id string) error {
	if !traceTwoCase(id) {
		return errTraceTwoSlot
	}
	info, e := os.Lstat(dir)
	if e != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0700 {
		return errTraceTwoSlot
	}
	if _, e = os.Lstat(filepath.Join(dir, "STOP")); !os.IsNotExist(e) {
		return errTraceTwoSlot
	}
	entries, e := os.ReadDir(dir)
	if e != nil {
		return errTraceTwoSlot
	}
	used := 0
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".used") {
			if !traceTwoCase(strings.TrimSuffix(entry.Name(), ".used")) {
				return errTraceTwoSlot
			}
			used++
		}
	}
	if used >= len(traceTwoCases) {
		return errTraceTwoSlot
	}
	f, e := os.OpenFile(filepath.Join(dir, id+".used"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return errTraceTwoSlot
	}
	_, e = f.WriteString("ONE_USE\n")
	ce := f.Close()
	if e != nil || ce != nil {
		traceTwoStop(dir)
		return errTraceTwoSlot
	}
	return nil
}
func traceTwoRead(path string, limit int64) ([]byte, error) {
	info, e := os.Lstat(path)
	if e != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 || info.Size() > limit {
		return nil, errTraceTwoIO
	}
	f, e := os.Open(path)
	if e != nil {
		return nil, errTraceTwoIO
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, limit+1))
	if e != nil || int64(len(b)) > limit {
		return nil, errTraceTwoIO
	}
	return b, nil
}
func traceTwoLedgerValid(raw []byte, next string) bool {
	if !traceTwoCase(next) {
		return false
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(raw) == 0 {
		return true
	}
	if len(lines) >= 2 {
		return false
	}
	seen := map[string]bool{}
	for _, line := range lines {
		var m map[string]string
		if json.Unmarshal([]byte(line), &m) != nil || len(m) != 3 || m["run"] != traceTwoPhase || m["event"] != "CONTROLLED_PREPARATION_ATTEMPT" || !traceTwoCase(m["case"]) || seen[m["case"]] || m["case"] == next {
			return false
		}
		seen[m["case"]] = true
	}
	return true
}
func traceTwoSHA(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

type traceTwoRelease struct {
	Phase        string   `json:"phase"`
	PreflightSHA string   `json:"preflight_sha256"`
	AllowNative  bool     `json:"allow_native"`
	Cases        []string `json:"cases"`
	MaxAttempts  int      `json:"max_attempts"`
	TraceCap     int64    `json:"trace_cap"`
	TraceTotal   int64    `json:"trace_total"`
	Expires      int64    `json:"expires_unix"`
}

func traceTwoReleaseValid(q traceTwoRelease, pre []byte, now int64) bool {
	if !traceTwoSourceFrameValid(pre) {
		return false
	}
	if q.Phase != traceTwoPhase || !q.AllowNative || q.PreflightSHA != traceTwoSHA(pre) || q.MaxAttempts != 2 || q.TraceCap != traceTwoCap || q.TraceTotal != 2*traceTwoCap || q.Expires <= now || q.Expires > now+1800 || len(q.Cases) != 2 {
		return false
	}
	for i, c := range traceTwoCases {
		if q.Cases[i] != c {
			return false
		}
	}
	return true
}

// The accepted clone postimage is mandatory, even when a release hashes a
// syntactically valid but incomplete preflight. Actual source hashes are also
// checked by the separately bound parent controller before every launch.
func traceTwoSourceFrameValid(pre []byte) bool {
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

func traceTwoReleased() bool {
	raw, e := traceTwoRead(traceTwoBase+"/parent-release.json", 16<<10)
	if e != nil {
		return false
	}
	var q traceTwoRelease
	d := json.NewDecoder(strings.NewReader(string(raw)))
	d.DisallowUnknownFields()
	if d.Decode(&q) != nil {
		return false
	}
	var more any
	if d.Decode(&more) != io.EOF {
		return false
	}
	pre, e := traceTwoRead(traceTwoBase+"/PREFLIGHT.json", 128<<10)
	return e == nil && traceTwoReleaseValid(q, pre, time.Now().Unix())
}

// The only lifecycle owner added here is this opt-in TEST controller. It creates
// no task/context and calls the original test unchanged. Native meaning may be
// BLOCKED even if the Go test passes; only parent-side records adjudicate it.
func TestConfigurationV3RuntimeTraceTwo(t *testing.T) {
	if os.Getenv("LSP_TRACE_TWO_ENABLE") != "PARENT_RELEASED_PHASE06" {
		t.Skip("TRACE_TWO_OFF")
	}
	id := os.Getenv("LSP_TRACE_TWO_CASE")
	if t.Name() != "TestConfigurationV3RuntimeTraceTwo" || !traceTwoSelection(id, flag.Lookup("test.run").Value.String(), os.Getenv("CONFIGURATION_V3_COUNT_FILE"), os.Getenv("CONFIGURATION_V3_RUN"), os.Args[1:]) || flag.Lookup("test.trace").Value.String() != "" || !traceTwoReleased() {
		t.Fatal("TRACE_TWO_PREFLIGHT_REFUSED")
	}
	old, e := traceTwoRead(traceTwoOldLedger, 128<<10)
	if e != nil || traceTwoSHA(old) != traceTwoOldSHA {
		t.Fatal("TRACE_TWO_OLD_COUNTER_REFUSED")
	}
	ledger, e := traceTwoRead(traceTwoLedger, 32<<10)
	if e != nil || !traceTwoLedgerValid(ledger, id) {
		t.Fatal("TRACE_TWO_LEDGER_REFUSED")
	}
	if trace.IsEnabled() {
		t.Fatal("TRACE_TWO_ALREADY_TRACING")
	}
	if e = traceTwoClaim(traceTwoExecution, id); e != nil {
		t.Fatal("TRACE_TWO_SLOT_REFUSED")
	}
	f, e := os.OpenFile(filepath.Join(traceTwoExecution, id+".trace"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		traceTwoStop(traceTwoExecution)
		t.Fatal("TRACE_TWO_FILE_REFUSED")
	}
	writer := &traceTwoWriter{dst: f, limit: traceTwoCap}
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
			traceTwoStop(traceTwoExecution)
			t.Error("TRACE_TWO_UNUSABLE")
		}
		marker := struct {
			Case       string `json:"case"`
			Code       string `json:"code"`
			Bytes      int64  `json:"bytes"`
			Overflow   bool   `json:"overflow"`
			TestPassed bool   `json:"test_passed"`
		}{id, code, n, writer.overflowed(), !t.Failed()}
		raw, _ := json.Marshal(marker)
		out, err := os.OpenFile(filepath.Join(traceTwoExecution, id+".complete.json"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err == nil {
			_, err = out.Write(append(raw, '\n'))
			ce := out.Close()
			if ce != nil {
				err = ce
			}
		}
		if err != nil {
			traceTwoStop(traceTwoExecution)
			t.Error("TRACE_TWO_COMPLETION_REFUSED")
		}
	})
	if trace.Start(writer) != nil {
		t.Fatal("TRACE_TWO_START_REFUSED")
	}
	started = true
	TestConfigurationV3ProviderStates(t)
	returned = true
}
