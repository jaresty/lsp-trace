//go:build darwin || linux

package censuslineagepilot_test

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/coverage"
	"strconv"
	"strings"
	"sync"
	"testing"

	pilot "lsp-trace/internal/censuslineagepilot"
	"lsp-trace/internal/managedprocess"
	prep "lsp-trace/internal/privatedescribeprep"
	"lsp-trace/sessionruntime"
)

var qualificationCoveredChild = flag.Bool("qualification-covered-child", false, "owned qualification coverage child only")

// Only temporary implementation mutants refer to these owned test values.
// The unmodified caller and production packages have no observer/test hooks.
var qualificationRetiredManager *sessionruntime.Manager
var qualificationCapturedSession string
var qualificationBeforeModelAttempt, qualificationAfterModelAttempt bool

type qualificationCleanupEntry struct {
	Receiver                           *sessionruntime.Manager
	Method, Session, Operation, Caller string
	Generation                         uint64
}

var qualificationCleanupMu sync.Mutex
var qualificationCleanupEntries []qualificationCleanupEntry

// Replaced only in the temporary instrumented test build.
func qualificationInstallCleanupObserver() {}

type qualificationCaptureEntry struct {
	Digest string
	Bytes  uint64
}

var qualificationCaptureEntries []qualificationCaptureEntry

func qualificationObserveCapture(c pilot.OpaqueSuccessfulV2Capture, err error) {
	if err != nil {
		return
	}
	qualificationCleanupMu.Lock()
	defer qualificationCleanupMu.Unlock()
	qualificationCaptureEntries = append(qualificationCaptureEntries, qualificationCaptureEntry{c.ArtifactDigest(), c.ArtifactByteLength()})
}
func qualificationCaptures() []qualificationCaptureEntry {
	qualificationCleanupMu.Lock()
	defer qualificationCleanupMu.Unlock()
	return append([]qualificationCaptureEntry(nil), qualificationCaptureEntries...)
}

func qualificationCleanupTrace() []qualificationCleanupEntry {
	qualificationCleanupMu.Lock()
	defer qualificationCleanupMu.Unlock()
	return append([]qualificationCleanupEntry(nil), qualificationCleanupEntries...)
}

func qualificationCoverageSnapshot(dir string) error {
	if e := os.MkdirAll(dir, 0700); e != nil {
		return e
	}
	if e := coverage.WriteMetaDir(dir); e != nil {
		return e
	}
	return coverage.WriteCountersDir(dir)
}

type qualificationStdin struct {
	io.WriteCloser
	beforeShutdown func()
}

func (w qualificationStdin) Write(p []byte) (int, error) {
	if bytes.Contains(p, []byte(`"method":"shutdown"`)) {
		w.beforeShutdown()
	}
	return w.WriteCloser.Write(p)
}

type qualificationChild struct {
	*variableOwnedChild
	beforeShutdown func()
}

func (c qualificationChild) Stdin() io.WriteCloser {
	return qualificationStdin{c.variableOwnedChild.Stdin(), c.beforeShutdown}
}

type qualificationStarter struct {
	delegate       variableOwnedStarter
	beforeShutdown func()
}

func (s qualificationStarter) Start(ctx context.Context, p managedprocess.Spec) (sessionruntime.Child, managedprocess.StartObservation) {
	c, o := s.delegate.Start(ctx, p)
	if c == nil {
		return nil, o
	}
	return qualificationChild{c.(*variableOwnedChild), s.beforeShutdown}, o
}
func qualificationCoverageText(t *testing.T, dir string) string {
	t.Helper()
	p := filepath.Join(dir, "counts.txt")
	b, e := os.ReadFile(p)
	if e != nil {
		t.Fatal(e)
	}
	return string(b)
}

// Read the entry counter of the actual compiled method, not a report counter.
func qualificationMethodCount(t *testing.T, text, file, name string) uint64 {
	t.Helper()
	fset := token.NewFileSet()
	f, e := parser.ParseFile(fset, file, nil, 0)
	if e != nil {
		t.Fatal(e)
	}
	start, end := 0, 0
	for _, d := range f.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok && fn.Name.Name == name {
			start = fset.Position(fn.Body.Lbrace).Line
			end = fset.Position(fn.Body.Rbrace).Line
			break
		}
	}
	if start == 0 {
		t.Fatalf("coverage setup missing function %s", name)
	}
	bestLine, bestCol := int(^uint(0)>>1), int(^uint(0)>>1)
	var count uint64
	found := false
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 3 {
			continue
		}
		colon := strings.LastIndex(fields[0], ":")
		if colon < 0 {
			continue
		}
		if filepath.Base(fields[0][:colon]) != filepath.Base(file) {
			continue
		}
		var sl, sc, el, ec int
		if _, e := fmt.Sscanf(fields[0][colon+1:], "%d.%d,%d.%d", &sl, &sc, &el, &ec); e != nil {
			continue
		}
		if sl < start || sl > end {
			continue
		}
		if sl < bestLine || sl == bestLine && sc < bestCol {
			n, e := strconv.ParseUint(fields[2], 10, 64)
			if e != nil {
				t.Fatal(e)
			}
			bestLine, bestCol, count, found = sl, sc, n, true
		}
	}
	if !found {
		t.Fatalf("coverage setup missing entry block %s", name)
	}
	return count
}

// Compiler-generated counters are confined to a temporary build overlay. The
// original production files stay byte-for-byte unchanged and acquire no hook.
func qualificationCoverageOverlay(t *testing.T) string {
	t.Helper()
	root, e := filepath.Abs(filepath.Join("..", ".."))
	if e != nil {
		t.Fatal(e)
	}
	dir := t.TempDir()
	replacements := map[string]string{}
	for _, pair := range [][2]string{{"sessionruntime.go", "QualificationMain"}, {"shutdown_response_budget_private_v2.go", "QualificationStop"}} {
		input := filepath.Join(root, "sessionruntime", pair[0])
		output := filepath.Join(dir, pair[0])
		cmd := exec.Command("go", "tool", "cover", "-mode=atomic", "-var="+pair[1], "-o", output, input)
		if b, e := cmd.CombinedOutput(); e != nil {
			t.Fatalf("compiler coverage setup: %v %s", e, b)
		}
		generated, err := os.ReadFile(output)
		if err != nil {
			t.Fatal(err)
		}
		if pair[0] == "shutdown_response_budget_private_v2.go" {
			needle := "func (m *Manager) StopWithShutdownResponseBudgetPrivateV2(ctx context.Context, id, caller string, b ShutdownResponseBudgetPrivateV2) session.LifecycleResult {"
			if strings.Count(string(generated), needle) != 1 {
				t.Fatal("stop observation binding")
			}
			observed := strings.Replace(string(generated), needle, strings.Replace(needle, ") session.LifecycleResult {", ") (qualificationResult session.LifecycleResult) {", 1)+`defer func(){ if QualificationCleanupEntry != nil { QualificationCleanupEntry(m,"Stop",id,qualificationResult.IntentID,caller,qualificationResult.Generation) } }();`, 1)
			if err = os.WriteFile(output, []byte(observed), 0600); err != nil {
				t.Fatal(err)
			}
		}
		if pair[0] == "sessionruntime.go" {
			generated, err := os.ReadFile(output)
			if err != nil {
				t.Fatal(err)
			}
			needle := "func (m *Manager) Shutdown(ctx context.Context) error {"
			if strings.Count(string(generated), needle) != 1 {
				t.Fatal("shutdown entry observation binding")
			}
			observed := strings.Replace(string(generated), needle, needle+`if QualificationCleanupEntry != nil { QualificationCleanupEntry(m,"Shutdown","","","",0) }; if QualificationShutdownEntry != nil { QualificationShutdownEntry() };`, 1)
			operationNeedle := "snapshot, ok := m.operations[id]"
			if strings.Count(observed, operationNeedle) != 1 {
				t.Fatal("operation observation binding")
			}
			observed = strings.Replace(observed, operationNeedle, operationNeedle+`; if QualificationCleanupEntry != nil { QualificationCleanupEntry(m,"Operation",snapshot.SessionID,id,snapshot.CallerID,snapshot.Generation) };`, 1)
			observed += "\n// Test-build observations only, absent from repository production source.\nvar QualificationShutdownEntry func()\nvar QualificationCleanupEntry func(*Manager,string,string,string,string,uint64)\n"
			if err = os.WriteFile(output, []byte(observed), 0600); err != nil {
				t.Fatal(err)
			}
		}
		replacements[input] = output
	}
	// Observe actual successful opaque returns, without decoding/reconstructing
	// capture authority or asking the Manager any additional questions.
	captureInput := filepath.Join(root, "internal", "censuslineagepilot", "scoped_batch_selection_private_v2.go")
	captureRaw, err := os.ReadFile(captureInput)
	if err != nil {
		t.Fatal(err)
	}
	captureNeedle := "func AcquireScopeBatchPrivateV2(ctx context.Context, m *sessionruntime.Manager, o EnumerationOriginPrivateV2, b ScopeBatchSelectionPrivateV2) (OpaqueSuccessfulV2Capture, error) {"
	if strings.Count(string(captureRaw), captureNeedle) != 1 {
		t.Fatal("capture return observation binding")
	}
	captureText := strings.Replace(string(captureRaw), captureNeedle, strings.Replace(captureNeedle, "(OpaqueSuccessfulV2Capture, error)", "(qualificationCapture OpaqueSuccessfulV2Capture, qualificationError error)", 1)+`defer func(){if QualificationCaptureReturn != nil {QualificationCaptureReturn(qualificationCapture,qualificationError)}}();`, 1)
	captureText += "\n// Test-build observation only.\nvar QualificationCaptureReturn func(OpaqueSuccessfulV2Capture,error)\n"
	captureOutput := filepath.Join(dir, "capture.go")
	if err = os.WriteFile(captureOutput, []byte(captureText), 0600); err != nil {
		t.Fatal(err)
	}
	replacements[captureInput] = captureOutput
	self := filepath.Join(root, "internal", "censuslineagepilot", "variable_scope_real_v2_qualification_coverage_test.go")
	raw, e := os.ReadFile(self)
	if e != nil {
		t.Fatal(e)
	}
	text := string(raw)
	start := strings.Index(text, "func qualificationCoverageSnapshot(dir string) error {")
	end := strings.Index(text, "type qualificationStdin struct")
	if start < 0 || end < start {
		t.Fatal("coverage overlay source binding")
	}
	body := `func qualificationCoverageSnapshot(dir string) error {
 if e:=os.MkdirAll(dir,0700);e!=nil{return e};f,e:=os.Create(filepath.Join(dir,"counts.txt"));if e!=nil{return e};defer f.Close()
 fmt.Fprintln(f,"mode: atomic")
 for i:=range sessionruntime.QualificationMain.Count {p:=sessionruntime.QualificationMain.Pos[3*i:3*i+3];fmt.Fprintf(f,"lsp-trace/sessionruntime/sessionruntime.go:%d.%d,%d.%d %d %d\n",p[0],p[2]&65535,p[1],p[2]>>16,sessionruntime.QualificationMain.NumStmt[i],atomic.LoadUint32(&sessionruntime.QualificationMain.Count[i]))}
 for i:=range sessionruntime.QualificationStop.Count {p:=sessionruntime.QualificationStop.Pos[3*i:3*i+3];fmt.Fprintf(f,"lsp-trace/sessionruntime/shutdown_response_budget_private_v2.go:%d.%d,%d.%d %d %d\n",p[0],p[2]&65535,p[1],p[2]>>16,sessionruntime.QualificationStop.NumStmt[i],atomic.LoadUint32(&sessionruntime.QualificationStop.Count[i]))}
 return nil
}

`
	text = text[:start] + body + text[end:]
	text = strings.Replace(text, "\"runtime/coverage\"", "\"sync/atomic\"", 1)
	text = strings.Replace(text, "func qualificationInstallCleanupObserver() {}", `func qualificationInstallCleanupObserver() {
 pilot.QualificationCaptureReturn = qualificationObserveCapture
 sessionruntime.QualificationCleanupEntry = func(m *sessionruntime.Manager, method, session, operation, caller string, generation uint64) {
 qualificationCleanupMu.Lock(); defer qualificationCleanupMu.Unlock()
 qualificationCleanupEntries = append(qualificationCleanupEntries, qualificationCleanupEntry{m,method,session,operation,caller,generation})
 }
}`, 1)
	text = strings.Replace(text, "\n\t\tqualificationRetiredManager = m", "\n\t\tsessionruntime.QualificationShutdownEntry = before; qualificationRetiredManager = m", 1)
	output := filepath.Join(dir, "qualification_test.go")
	if e = os.WriteFile(output, []byte(text), 0600); e != nil {
		t.Fatal(e)
	}
	replacements[self] = output
	manifest, e := json.Marshal(struct{ Replace map[string]string }{replacements})
	if e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(dir, "overlay.json")
	if e = os.WriteFile(path, manifest, 0600); e != nil {
		t.Fatal(e)
	}
	return path
}
func qualificationRunCovered(t *testing.T, selector string) {
	t.Helper()
	cmd := exec.Command("go", "test", "-race", "./internal/censuslineagepilot", "-run", selector, "-count=1", "-timeout=180s", "-v", "-overlay="+qualificationCoverageOverlay(t), "-args", "-qualification-covered-child")
	cmd.Dir = filepath.Join("..", "..")
	b, e := cmd.CombinedOutput()
	t.Logf("procedure: %s\n%s", strings.Join(cmd.Args, " "), b)
	if e != nil {
		t.Errorf("owned covered child failed: %v", e)
	}
}
func TestVariableCallerQualificationCleanupTraffic(t *testing.T) {
	if !*qualificationCoveredChild {
		// Outer -race does not propagate to a nested go test: the shared
		// observation runner explicitly race-checks the compiled child.
		qualificationRunCovered(t, "^TestVariableCallerQualificationCleanupTraffic$")
		return
	}
	qualificationInstallCleanupObserver()
	c, path, git := variableAutomaticConfig(t, 1)
	_, starts, ch, spec := variableOwnedFactory(t, "")
	base := t.TempDir()
	pre := filepath.Join(base, "pre")
	shutdown := filepath.Join(base, "shutdown")
	post := filepath.Join(base, "post")
	if e := qualificationCoverageSnapshot(pre); e != nil {
		t.Fatal("coverage setup: ", e)
	}
	var once sync.Once
	var snapshotError error
	before := func() { once.Do(func() { snapshotError = qualificationCoverageSnapshot(shutdown) }) }
	_ = before // Referenced by the temporary compiler observation overlay.
	factory := func() (*sessionruntime.Manager, error) {
		m, e := sessionruntime.New(sessionruntime.Config{Starter: qualificationStarter{variableOwnedStarter{child: ch, starts: starts, spec: spec}, func() {}}, Limits: variableRuntimeLimits()})
		qualificationRetiredManager = m
		qualificationCapturedSession = c.Session.SessionID
		return m, e
	}
	models := qualificationScript{nominal: true}.factory(t)
	observedModels := func(cfg prep.AutomaticHTTPConfigPrivateV2, scope string) (prep.AutomaticBackendPrivateV2, prep.AutomaticBackendPrivateV2, error) {
		return models(cfg, scope)
	}
	r, e := variableRunWithAutomatic(context.Background(), "execute-once", path, git, factory, variableClose, variableOwnedCodeInputs, observedModels)
	// This snapshot follows the actual terminal return, including all defers.
	if se := qualificationCoverageSnapshot(post); se != nil {
		t.Fatal("coverage setup: ", se)
	}
	if snapshotError != nil {
		t.Fatal("shutdown coverage setup: ", snapshotError)
	}
	beforeText, afterText := qualificationCoverageText(t, pre), qualificationCoverageText(t, post)
	shutdownText := ""
	if _, se := os.Stat(filepath.Join(shutdown, "counts.txt")); se == nil {
		shutdownText = qualificationCoverageText(t, shutdown)
	}
	runtimeFile := filepath.Join("..", "..", "sessionruntime", "sessionruntime.go")
	stopFile := filepath.Join("..", "..", "sessionruntime", "shutdown_response_budget_private_v2.go")
	delta := func(file, name string) uint64 {
		return qualificationMethodCount(t, afterText, file, name) - qualificationMethodCount(t, beforeText, file, name)
	}
	keyed := r.Cleanup.SessionID == c.Session.SessionID && r.Cleanup.Generation == c.Session.Generation && r.Cleanup.StopIntentID != "" && r.Cleanup.Terminal.ID == r.Cleanup.StopIntentID && r.Cleanup.Terminal.SessionID == c.Session.SessionID && r.Cleanup.Terminal.Generation == c.Session.Generation
	trace := qualificationCleanupTrace()
	var stops, shutdowns, operations []qualificationCleanupEntry
	for _, entry := range trace {
		switch entry.Method {
		case "Stop":
			stops = append(stops, entry)
		case "Shutdown":
			shutdowns = append(shutdowns, entry)
		case "Operation":
			operations = append(operations, entry)
		}
	}
	qualificationCheck(t, "report/cleanup/key-association", keyed && len(stops) == 1 && stops[0].Operation == r.Cleanup.StopIntentID, r.Cleanup.StopIntentID)
	stopKey := len(stops) == 1 && stops[0].Receiver == qualificationRetiredManager && stops[0].Session == c.Session.SessionID && stops[0].Generation == c.Session.Generation && stops[0].Operation != "" && stops[0].Caller == "private-variable-v2-caller-stop-once"
	shutdownKey := len(shutdowns) == 1 && shutdowns[0].Receiver == qualificationRetiredManager
	operationKey := len(operations) == 1 && len(stops) == 1 && operations[0].Receiver == qualificationRetiredManager && operations[0].Operation == stops[0].Operation && operations[0].Session == c.Session.SessionID && operations[0].Generation == c.Session.Generation && operations[0].Caller == "private-variable-v2-caller-stop-once"
	qualificationCheck(t, "Es/actual-keyed-Stop", stopKey && delta(stopFile, "StopWithShutdownResponseBudgetPrivateV2") == 1, fmt.Sprint("calls=", delta(stopFile, "StopWithShutdownResponseBudgetPrivateV2"), " trace=", stops))
	qualificationCheck(t, "Eh/actual-keyed-Shutdown", shutdownKey && delta(runtimeFile, "Shutdown") == 1, fmt.Sprint("calls=", delta(runtimeFile, "Shutdown"), " trace=", shutdowns))
	qualificationCheck(t, "Eo/actual-keyed-Operation", operationKey && delta(runtimeFile, "Operation") == 1, fmt.Sprint("calls=", delta(runtimeFile, "Operation"), " trace=", operations))
	if delta(runtimeFile, "Shutdown") == 0 {
		t.Log("ASSERT_QUAL/F/shutdown-boundary-observed: INAPPLICABLE no actual Shutdown entry")
	} else {
		qualificationCheck(t, "F/shutdown-boundary-observed", shutdownText != "", shutdownText != "")
	}
	for _, name := range []string{"RoundTrip", "PrepareDocument", "RefreshDocumentSupply", "WithReadySessionLease", "BeginRequest"} {
		if shutdownText == "" {
			t.Logf("ASSERT_QUAL/F/shutdown-return/%s: UNOBSERVED no manager shutdown entry", name)
			continue
		}
		n := qualificationMethodCount(t, afterText, runtimeFile, name) - qualificationMethodCount(t, shutdownText, runtimeFile, name)
		qualificationCheck(t, "F/shutdown-return/"+name, n == 0, n)
	}
	qualificationCheck(t, "F/actual-closed-transport", ch.writesAfterClose.Load() == 0, ch.writesAfterClose.Load())
	qualificationCheck(t, "A/covered-nominal", e == nil && r.Phase == "PROVISIONAL_COMPLETE", fmt.Sprint(r.Phase, " ", e))
}
