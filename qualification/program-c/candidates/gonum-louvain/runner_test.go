package gonumlouvain

import (
	"encoding/json"
	"fmt"
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
		_, _ = os.Stdout.WriteString(`{"communities":[["b","a"],["a"]]}`)
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
func TestExternalCanonicalization(t *testing.T) {
	out, e := Canonicalize(RawOutput{Communities: [][]string{{"b", "a"}}}, []string{"a", "b"})
	if e != nil || len(out.Communities) != 1 || len(out.Communities[0]) != 2 || out.Communities[0][0] != "a" || out.Communities[0][1] != "b" || out.Digest == "" {
		t.Fatalf("ASSERT_EXTERNAL_CANONICALIZATION_PASS output=%+v error=%v", out, e)
	}
	t.Log("PASS ASSERT_EXTERNAL_CANONICALIZATION_PASS")
	_, e = Canonicalize(RawOutput{Communities: [][]string{{"a"}, {"a"}}}, []string{"a", "b"})
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
func repositoryRoot() string {
	return filepath.Join("..", "..", "..", "..")
}

func TestFixtureInventory(t *testing.T) {
	inventory, fixtures, e := LoadFixtureInventory(repositoryRoot(), "qualification/program-c/a-06-fixture-inventory.v1.json")
	if e != nil || len(fixtures) != 4 || len(inventory.Seeds) != 2 {
		t.Fatalf("ASSERT_FIXTURE_INVENTORY_PASS inventory=%+v fixtures=%d error=%v", inventory, len(fixtures), e)
	}
	var nodes, edges int
	for _, fixture := range fixtures {
		nodes += fixture.Record.NodeCount
		edges += fixture.Record.DirectedWeightedEdgeCount
	}
	if nodes != 21 || edges != 25 {
		t.Fatalf("ASSERT_FIXTURE_INVENTORY_COUNTS nodes=%d edges=%d", nodes, edges)
	}
	t.Log("PASS ASSERT_FIXTURE_INVENTORY_PASS fixtures=4 seeds=2 classes=6 nodes=21 edges=25")
}

func fixtureTestRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	paths := []string{
		"qualification/program-c/a-06-fixture-inventory.v1.json",
		"qualification/program-c/fixtures/retained-calls-v1.json",
		"qualification/program-c/fixtures/disconnected-singleton.json",
		"qualification/program-c/fixtures/high-degree-hub.json",
		"qualification/program-c/fixtures/adversarial-order.json",
		"internal/retainedcalls/testdata/frozen-v1-export.json",
	}
	for _, relative := range paths {
		from := filepath.Join(repositoryRoot(), relative)
		to := filepath.Join(root, relative)
		if e := os.MkdirAll(filepath.Dir(to), 0755); e != nil {
			t.Fatal(e)
		}
		b, e := os.ReadFile(from)
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(to, b, 0644); e != nil {
			t.Fatal(e)
		}
	}
	return root
}

func inventoryMutation(t *testing.T, name, want string, mutate func(map[string]any)) {
	t.Helper()
	root := fixtureTestRoot(t)
	path := filepath.Join(root, "qualification/program-c/a-06-fixture-inventory.v1.json")
	var value map[string]any
	b, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(b, &value); e != nil {
		t.Fatal(e)
	}
	mutate(value)
	b, e = json.MarshalIndent(value, "", "  ")
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(path, append(b, '\n'), 0644); e != nil {
		t.Fatal(e)
	}
	_, _, e = LoadFixtureInventory(root, "qualification/program-c/a-06-fixture-inventory.v1.json")
	kind(t, e, want)
	t.Logf("FAIL-WITNESS %s: %v", name, e)
}

func TestFixtureInventoryMutations(t *testing.T) {
	inventoryMutation(t, "ASSERT_FIXTURE_GRAPH_DIGEST", "INPUT_DIGEST_MISMATCH", func(v map[string]any) {
		v["fixtures"].([]any)[0].(map[string]any)["graph"].(map[string]any)["sha256"] = "sha256:" + fmt.Sprintf("%064d", 0)
	})
	inventoryMutation(t, "ASSERT_FIXTURE_CLASSES", "DERIVATION_REJECTED", func(v map[string]any) {
		v["required_classes"] = v["required_classes"].([]any)[:5]
	})
	inventoryMutation(t, "ASSERT_FIXTURE_SEEDS", "SCHEDULE_INVALID", func(v map[string]any) {
		v["seed_inventory"] = []any{float64(1)}
	})
	inventoryMutation(t, "ASSERT_FIXTURE_COUNTS", "DERIVATION_REJECTED", func(v map[string]any) {
		v["fixtures"].([]any)[0].(map[string]any)["node_count"] = float64(3)
	})
	inventoryMutation(t, "ASSERT_FIXTURE_CANDIDATES", "DERIVATION_REJECTED", func(v map[string]any) {
		v["candidate_scope"].([]any)[0].(map[string]any)["version"] = "v0.17.1"
	})
	inventoryMutation(t, "ASSERT_FIXTURE_LIMITS", "CAP_BREACH", func(v map[string]any) {
		v["limits"].(map[string]any)["maximum_nodes_per_fixture"] = float64(10001)
	})
}

func TestAuthoritativeDerivation(t *testing.T) {
	p := filepath.Join(repositoryRoot(), "internal", "retainedcalls", "testdata", "frozen-v1-export.json")
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
