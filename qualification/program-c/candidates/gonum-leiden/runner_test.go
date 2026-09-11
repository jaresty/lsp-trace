package gonumleiden

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"
)

func kind(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil || FailureKind(err) != want {
		t.Fatalf("ASSERT_%s want=%s got=%v", want, want, err)
	}
	t.Logf("FAIL-WITNESS ASSERT_%s: %v", want, err)
}
func script(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "candidate")
	if err := os.WriteFile(p, []byte("#!/bin/sh\nsleep 0.1\nread _\n"+body+"\n"), 0755); err != nil {
		t.Fatal(err)
	}
	return p
}
func request() Request {
	return Request{Fixture: Fixture{SourceDigest: RetainedExportSHA256, Nodes: []string{"a", "b"}, Edges: []Edge{{From: "a", To: "b", Occurrences: 2}}}, Seed: 1}
}
func supervisor() Supervisor {
	return Supervisor{GOOS: "darwin", PS: PSPath, Limits: Limits{Wall: 2 * time.Second, RSSBytes: 512 << 20, Poll: 10 * time.Millisecond}}
}

func TestUnsupportedSupervisorPreventsExecution(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "ran")
	s := supervisor()
	s.GOOS = "linux"
	_, e := s.Run(script(t, "touch "+marker), request())
	kind(t, e, "MEMORY_UNSUPPORTED")
	if _, e = os.Stat(marker); !os.IsNotExist(e) {
		t.Fatal("ASSERT_PREEXECUTION_PREVENTION")
	}
	t.Log("PASS ASSERT_PREEXECUTION_PREVENTION")
}
func TestSupervisorSetupFailurePreventsExecution(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "ran")
	s := supervisor()
	s.BeforeStart = func() error { return Fail("x", "no process group authority") }
	_, e := s.Run(script(t, "touch "+marker), request())
	kind(t, e, "SUPERVISOR_UNSUPPORTED")
	if _, e = os.Stat(marker); !os.IsNotExist(e) {
		t.Fatal("ASSERT_PREEXECUTION_PREVENTION")
	}
}
func TestTimeoutKillsCandidate(t *testing.T) {
	s := supervisor()
	s.Limits.Wall = 40 * time.Millisecond
	_, e := s.Run(script(t, "sleep 5"), request())
	kind(t, e, "TIMEOUT")
}
func TestMemoryKillsCandidateBounded(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Darwin qualification guard")
	}
	s := supervisor()
	s.Limits.RSSBytes = 8 << 20
	_, e := s.Run(script(t, "exec /usr/bin/python3 -c 'import time; x=bytearray(32*1024*1024); time.sleep(5)'"), request())
	kind(t, e, "MEMORY_EXCEEDED")
}
func TestSupervisorHelperProcess(t *testing.T) {
	if os.Getenv("QUALIFICATION_HELPER") == "" {
		return
	}
	var req Request
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		os.Exit(97)
	}
	switch os.Getenv("QUALIFICATION_HELPER") {
	case "panic":
		_ = syscall.Kill(syscall.Getpid(), syscall.SIGABRT)
	case "nonzero":
		os.Exit(23)
	case "noncanonical":
		_, _ = os.Stdout.WriteString(`{"communities":[["b","a"]],"digest":"bad"}`)
	}
	os.Exit(0)
}
func helperSupervisor(mode string) Supervisor {
	s := supervisor()
	s.Args = []string{"-test.run=TestSupervisorHelperProcess"}
	s.Env = []string{"QUALIFICATION_HELPER=" + mode}
	return s
}
func TestPanicAndNonzero(t *testing.T) {
	_, e := helperSupervisor("panic").Run(os.Args[0], request())
	kind(t, e, "PANIC")
	_, e = helperSupervisor("nonzero").Run(os.Args[0], request())
	kind(t, e, "NONZERO_EXIT")
}
func TestNoncanonicalOutput(t *testing.T) {
	_, e := helperSupervisor("noncanonical").Run(os.Args[0], request())
	kind(t, e, "NONCANONICAL_OUTPUT")
}
func TestCapsAndCandidateValidation(t *testing.T) {
	r := request()
	r.Fixture.Nodes = make([]string, MaxNodes+1)
	_, e := Run(r)
	kind(t, e, "CAP_BREACH")
	r = request()
	r.Fixture.Edges = []Edge{{From: "a", To: "b", Occurrences: 0}}
	_, e = Run(r)
	kind(t, e, "DERIVATION_REJECTED")
}
func TestScheduleRejectsNondeterminismAndCardinality(t *testing.T) {
	o1 := Output{Communities: [][]string{{"a"}}}
	raw, _ := json.Marshal(o1.Communities)
	o1.Digest = Digest(raw)
	o2 := Output{Communities: [][]string{{"b"}}}
	raw, _ = json.Marshal(o2.Communities)
	o2.Digest = Digest(raw)
	rs := make([]SupervisedResult, 4)
	for i := range rs {
		rs[i].Output = o1
	}
	rs[3].Output = o2
	kind(t, ValidateSchedule(rs), "NONDETERMINISM")
	kind(t, ValidateSchedule(rs[:3]), "SCHEDULE_INVALID")
}
func TestRetainedInputDigestAndDerivationRejection(t *testing.T) {
	_, e := DeriveFixture([]byte("{}"))
	kind(t, e, "INPUT_DIGEST_MISMATCH")
	bad := []byte(`{"schema_version":"lsp-trace.retained-calls.v1","tables":{"endpoints":[],"groups":[]}}`)
	_, e = deriveFixture(bad, Digest(bad))
	kind(t, e, "DERIVATION_REJECTED")
}
func TestAuthoritativeDerivation(t *testing.T) {
	p := filepath.Join("..", "..", "..", "..", "internal", "retainedcalls", "testdata", "frozen-v1-export.json")
	raw, e := os.ReadFile(p)
	if e != nil {
		t.Fatal(e)
	}
	f, e := DeriveFixture(raw)
	if e != nil {
		t.Fatal(e)
	}
	if len(f.Edges) != 1 || f.Edges[0].Occurrences != 2 {
		t.Fatalf("ASSERT_DIRECTED_OCCURRENCE_MULTIPLICITY %+v", f.Edges)
	}
	t.Log("PASS ASSERT_DIRECTED_OCCURRENCE_MULTIPLICITY")
}
