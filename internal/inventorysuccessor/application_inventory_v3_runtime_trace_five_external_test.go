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

const traceFivePhase = "runtime-trace-five-04"
const traceFiveBase = "/Users/schwa/.local/state/lsp-trace-evidence/seven-configuration-campaign-v3-20261112-01/runtime-trace-five-716d-04"
const traceFiveExecution = traceFiveBase + "/execution"
const traceFiveLedger = traceFiveExecution + "/preparations.jsonl"
const traceFiveOldLedger = "/Users/schwa/.local/state/lsp-trace-evidence/seven-configuration-campaign-v3-20261112-01/writer-716d-01/preparations.jsonl"
const traceFiveOldSHA = "a21c1d1a83c3caddfed8c144a10ce9854c87bb99f0fda15458ecd436b1164431"
const traceFiveCap int64 = 8 << 20

var traceFiveCases = [...]string{"provider-C-empty", "provider-mixed-CR-C-empty", "provider-C-composed-duplicates", "provider-mixed-CR-C-unavailable", "provider-mixed-RD-R-unavailable"}
var errTraceFiveCap = errors.New("TRACE_FIVE_CAP")
var errTraceFiveIO = errors.New("TRACE_FIVE_IO")
var errTraceFiveSlot = errors.New("TRACE_FIVE_SLOT_REFUSED")

func traceFiveCase(id string) bool {
	for _, c := range traceFiveCases {
		if c == id {
			return true
		}
	}
	return false
}
func traceFiveRun(id string) string { return "^TestConfigurationV3RuntimeTraceFive$/^" + id + "$" }
func traceFiveSelection(id, run, ledger, phase string, args []string) bool {
	if !traceFiveCase(id) || run != traceFiveRun(id) || ledger != traceFiveLedger || phase != traceFivePhase {
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
type traceFiveWriter struct {
	mu           sync.Mutex
	dst          io.Writer
	limit, n     int64
	failed, over bool
}

func (w *traceFiveWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.failed {
		return 0, errTraceFiveCap
	}
	if w.limit < 0 || w.n < 0 || w.n > w.limit {
		w.failed = true
		return 0, errTraceFiveIO
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
		return 0, errTraceFiveIO
	}
	w.n += int64(n)
	if e != nil || n != len(p) {
		w.failed = true
		return n, errTraceFiveIO
	}
	if overflow {
		w.failed = true
		return n, errTraceFiveCap
	}
	return n, nil
}
func (w *traceFiveWriter) state() (int64, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.n, w.failed
}
func (w *traceFiveWriter) overflowed() bool { w.mu.Lock(); defer w.mu.Unlock(); return w.over }
func traceFiveStop(dir string) {
	f, e := os.OpenFile(filepath.Join(dir, "STOP"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e == nil {
		_, _ = f.WriteString("TRACE_FIVE_STOP\n")
		_ = f.Close()
	}
}
func traceFiveClaim(dir, id string) error {
	if !traceFiveCase(id) {
		return errTraceFiveSlot
	}
	info, e := os.Lstat(dir)
	if e != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0700 {
		return errTraceFiveSlot
	}
	if _, e = os.Lstat(filepath.Join(dir, "STOP")); !os.IsNotExist(e) {
		return errTraceFiveSlot
	}
	entries, e := os.ReadDir(dir)
	if e != nil {
		return errTraceFiveSlot
	}
	used := 0
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".used") {
			if !traceFiveCase(strings.TrimSuffix(entry.Name(), ".used")) {
				return errTraceFiveSlot
			}
			used++
		}
	}
	if used >= len(traceFiveCases) {
		return errTraceFiveSlot
	}
	f, e := os.OpenFile(filepath.Join(dir, id+".used"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return errTraceFiveSlot
	}
	_, e = f.WriteString("ONE_USE\n")
	ce := f.Close()
	if e != nil || ce != nil {
		traceFiveStop(dir)
		return errTraceFiveSlot
	}
	return nil
}
func traceFiveRead(path string, limit int64) ([]byte, error) {
	info, e := os.Lstat(path)
	if e != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 || info.Size() > limit {
		return nil, errTraceFiveIO
	}
	f, e := os.Open(path)
	if e != nil {
		return nil, errTraceFiveIO
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, limit+1))
	if e != nil || int64(len(b)) > limit {
		return nil, errTraceFiveIO
	}
	return b, nil
}
func traceFiveLedgerValid(raw []byte, next string) bool {
	if !traceFiveCase(next) {
		return false
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(raw) == 0 {
		return true
	}
	if len(lines) >= 5 {
		return false
	}
	seen := map[string]bool{}
	for _, line := range lines {
		var m map[string]string
		if json.Unmarshal([]byte(line), &m) != nil || len(m) != 3 || m["run"] != traceFivePhase || m["event"] != "CONTROLLED_PREPARATION_ATTEMPT" || !traceFiveCase(m["case"]) || seen[m["case"]] || m["case"] == next {
			return false
		}
		seen[m["case"]] = true
	}
	return true
}
func traceFiveSHA(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

type traceFiveRelease struct {
	Phase        string   `json:"phase"`
	PreflightSHA string   `json:"preflight_sha256"`
	AllowNative  bool     `json:"allow_native"`
	Cases        []string `json:"cases"`
	MaxAttempts  int      `json:"max_attempts"`
	TraceCap     int64    `json:"trace_cap"`
	TraceTotal   int64    `json:"trace_total"`
	Expires      int64    `json:"expires_unix"`
}

func traceFiveReleaseValid(q traceFiveRelease, pre []byte, now int64) bool {
	if q.Phase != traceFivePhase || !q.AllowNative || q.PreflightSHA != traceFiveSHA(pre) || q.MaxAttempts != 5 || q.TraceCap != traceFiveCap || q.TraceTotal != 5*traceFiveCap || q.Expires <= now || q.Expires > now+1800 || len(q.Cases) != 5 {
		return false
	}
	for i, c := range traceFiveCases {
		if q.Cases[i] != c {
			return false
		}
	}
	return true
}
func traceFiveReleased() bool {
	raw, e := traceFiveRead(traceFiveBase+"/parent-release.json", 16<<10)
	if e != nil {
		return false
	}
	var q traceFiveRelease
	d := json.NewDecoder(strings.NewReader(string(raw)))
	d.DisallowUnknownFields()
	if d.Decode(&q) != nil {
		return false
	}
	var more any
	if d.Decode(&more) != io.EOF {
		return false
	}
	pre, e := traceFiveRead(traceFiveBase+"/PREFLIGHT.json", 128<<10)
	return e == nil && traceFiveReleaseValid(q, pre, time.Now().Unix())
}

// The only lifecycle owner added here is this opt-in TEST controller. It creates
// no task/context and calls the original test unchanged. Native meaning may be
// BLOCKED even if the Go test passes; only parent-side records adjudicate it.
func TestConfigurationV3RuntimeTraceFive(t *testing.T) {
	if os.Getenv("LSP_TRACE_FIVE_ENABLE") != "PARENT_RELEASED_PHASE04" {
		t.Skip("TRACE_FIVE_OFF")
	}
	id := os.Getenv("LSP_TRACE_FIVE_CASE")
	if t.Name() != "TestConfigurationV3RuntimeTraceFive" || !traceFiveSelection(id, flag.Lookup("test.run").Value.String(), os.Getenv("CONFIGURATION_V3_COUNT_FILE"), os.Getenv("CONFIGURATION_V3_RUN"), os.Args[1:]) || flag.Lookup("test.trace").Value.String() != "" || !traceFiveReleased() {
		t.Fatal("TRACE_FIVE_PREFLIGHT_REFUSED")
	}
	old, e := traceFiveRead(traceFiveOldLedger, 128<<10)
	if e != nil || traceFiveSHA(old) != traceFiveOldSHA {
		t.Fatal("TRACE_FIVE_OLD_COUNTER_REFUSED")
	}
	ledger, e := traceFiveRead(traceFiveLedger, 32<<10)
	if e != nil || !traceFiveLedgerValid(ledger, id) {
		t.Fatal("TRACE_FIVE_LEDGER_REFUSED")
	}
	if trace.IsEnabled() {
		t.Fatal("TRACE_FIVE_ALREADY_TRACING")
	}
	if e = traceFiveClaim(traceFiveExecution, id); e != nil {
		t.Fatal("TRACE_FIVE_SLOT_REFUSED")
	}
	f, e := os.OpenFile(filepath.Join(traceFiveExecution, id+".trace"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		traceFiveStop(traceFiveExecution)
		t.Fatal("TRACE_FIVE_FILE_REFUSED")
	}
	writer := &traceFiveWriter{dst: f, limit: traceFiveCap}
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
			traceFiveStop(traceFiveExecution)
			t.Error("TRACE_FIVE_UNUSABLE")
		}
		marker := struct {
			Case       string `json:"case"`
			Code       string `json:"code"`
			Bytes      int64  `json:"bytes"`
			Overflow   bool   `json:"overflow"`
			TestPassed bool   `json:"test_passed"`
		}{id, code, n, writer.overflowed(), !t.Failed()}
		raw, _ := json.Marshal(marker)
		out, err := os.OpenFile(filepath.Join(traceFiveExecution, id+".complete.json"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err == nil {
			_, err = out.Write(append(raw, '\n'))
			ce := out.Close()
			if ce != nil {
				err = ce
			}
		}
		if err != nil {
			traceFiveStop(traceFiveExecution)
			t.Error("TRACE_FIVE_COMPLETION_REFUSED")
		}
	})
	if trace.Start(writer) != nil {
		t.Fatal("TRACE_FIVE_START_REFUSED")
	}
	started = true
	TestConfigurationV3ProviderStates(t)
	returned = true
}
