package censuscontinuation

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	rg "lsp-trace/internal/referencegrouping"
	"strings"
	"testing"
)

// Structural guard: the private closure may copy already-bounded opaque views,
// but must not acquire payloads, evaluate, capture, prepare Describe, or execute.
func TestPrivateAllModeNoHiddenReplayOrSourcePath(t *testing.T) {
	forbidden := map[string]bool{"EvaluateGroupingCasePrivate": true, "CheckGroupingEvaluationPrivate": true, "ManagedReplay": true, "captureFresh": true, "Build": true, "BuildV2": true, "buildDescribeRequests": true, "runWorkers": true, "runWorkersV2": true, "Lookup": true, "Artifact": true, "CanonicalBytes": true}
	for _, file := range []string{"private_allmode_continuation.go", "private_allmode_handoff.go", "../referencegrouping/private_allmode_metadata.go", "../targetpacket/private_source_intent.go", "../programc/grouping_evaluation_private.go"} {
		parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range parsed.Decls {
			f, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			if file == "../programc/grouping_evaluation_private.go" && f.Name.Name != "BindGroupingPairPrivate" {
				continue
			}
			ast.Inspect(f.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				name := ""
				switch fn := call.Fun.(type) {
				case *ast.Ident:
					name = fn.Name
				case *ast.SelectorExpr:
					name = fn.Sel.Name
				}
				if forbidden[name] {
					t.Errorf("ASSERT_ALLMODE_NO_HIDDEN_REPLAY_OR_SOURCE_PATH file=%s func=%s call=%s", file, f.Name.Name, name)
				}
				return true
			})
		}
	}
	t.Log("ASSERT_ALLMODE_NO_HIDDEN_REPLAY_OR_SOURCE_PATH: PASS")
}

func privateMissingHandoff(t *testing.T) PrivateAllModeHandoff {
	t.Helper()
	var slots []PrivateAllModeSlot
	var entries []PrivateAllModeEntry
	for i, mode := range []string{"D", "R", "C", "RD", "CD", "CR", "CRD"} {
		slot := PrivateAllModeSlot{Ordinal: i, Mode: mode, RunID: "declared-run", SeedNamespace: "declared-original", Seed: 7}
		slots = append(slots, slot)
		entries = append(entries, PrivateAllModeEntry{Slot: slot, Disposition: PrivateAllModeDeclaredMissing, Lineage: "required-upstream-selector", MissingReason: "NO_AUTHENTICATED_AVAILABILITY_SUPPLIED"})
	}
	h, err := NewPrivateAllModeHandoff("controlled-workspace", "controlled-revision", slots, entries)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

type privateAllModePutSpy struct {
	*MemoryStore
	puts int
	max  int64
}

func (s *privateAllModePutSpy) MaxObjectBytes() int64 {
	if s.max > 0 {
		return s.max
	}
	return 1
}
func (s *privateAllModePutSpy) Put(context.Context, []byte) (string, error) {
	s.puts++
	return "", errors.New("controlled tiny store must not receive Put")
}

func TestPrivateAllModeLegacyExclusivity(t *testing.T) {
	f := newFixture(t)
	legacy, err := BuildHandoff(f.input)
	if err != nil {
		t.Fatal(err)
	}
	store := &privateAllModePutSpy{MemoryStore: NewMemoryStore(), max: 8 << 20}
	r := Request{Handoff: legacy, Contract: testContract(t), Store: store, Workspace: f.workspace, StopAfter: StopAfterDescribeRequests}
	control := Run(context.Background(), r)
	if control.PrivateCode != "CONTRACT_PERSIST" || store.puts != 1 {
		t.Fatal("CONTROL_LEGACY_STOP_REACHES_STORE", control.PrivateCode, store.puts)
	}
	store.puts = 0
	r.StopAfter = StopAfterSourceIntents
	rejected := Run(context.Background(), r)
	if rejected.Err == nil || rejected.PrivateCode != "STOP_AFTER_INVALID" || store.puts != 0 {
		t.Fatal("ASSERT_ALLMODE_LEGACY_PRIVATE_STOP_REFUSED", rejected.PrivateCode, store.puts)
	}
	h := privateMissingHandoff(t)
	r.PrivateAllMode = &h
	r.Workspace = "controlled-workspace"
	rejected = Run(context.Background(), r)
	if rejected.Err == nil || store.puts != 0 {
		t.Fatal("ASSERT_ALLMODE_MIXED_HANDOFF_REFUSED", store.puts)
	}
	t.Log("ASSERT_ALLMODE_LEGACY_PRIVATE_STOP_REFUSED: PASS")
	t.Log("ASSERT_ALLMODE_MIXED_HANDOFF_REFUSED: PASS")
}

func TestPrivateAllModeMissingAndFiniteBounds(t *testing.T) {
	h := privateMissingHandoff(t)
	store := cappedStore{MemoryStore: NewMemoryStore(), max: 8 << 20}
	contract := testContract(t)
	req := Request{PrivateAllMode: &h, Contract: contract, Store: store, Workspace: "controlled-workspace", StopAfter: StopAfterSourceIntents}
	out := Run(context.Background(), req)
	if out.Err != nil || out.Status != StatusPaused || out.PrivateAllMode == nil {
		t.Fatal("ASSERT_ALLMODE_MISSING_PAUSE", out.Err)
	}
	p := out.PrivateAllMode
	if p.Counts.Slots != 7 || p.Counts.Ready != 0 || p.Counts.Missing != 7 || p.Counts.Nominations != 0 || p.Counts.Intents != 7 || len(p.Diagnostics) != 7 {
		t.Fatal("ASSERT_ALLMODE_MISSING_NOT_EMPTY", p.Counts)
	}
	for _, intent := range p.Intents {
		if intent.CaseID != "" || intent.ResultID != "" || intent.Endpoint.Node != "" || intent.BindingState != "UNRESOLVED" || intent.Gap != PrivateAllModeDeclaredMissing || intent.ClaimCeiling != rg.EvaluationCeilingPrivate {
			t.Fatal("ASSERT_ALLMODE_MISSING_NO_FAKE_RESULT", intent)
		}
	}
	for _, max := range []int64{0, 1, 1024} {
		r := req
		r.Store = cappedStore{MemoryStore: NewMemoryStore(), max: max}
		if Run(context.Background(), r).Err == nil {
			t.Fatal("ASSERT_ALLMODE_FINITE_STORE_BOUND", max)
		}
	}
	spy := &privateAllModePutSpy{MemoryStore: NewMemoryStore()}
	boundedReq := req
	boundedReq.Store = spy
	if result := Run(context.Background(), boundedReq); result.Err == nil || spy.puts != 0 {
		t.Fatal("ASSERT_ALLMODE_BOUND_BEFORE_PUT", spy.puts, result.Err)
	}
	t.Log("ASSERT_ALLMODE_BOUND_BEFORE_PUT: PASS")
	low := contract
	low.wire.CaptureLimits.MaxWork = 1
	low.wire.ContractID = identityJSON("lsp-trace:census-continuation-contract:v1", low.wire, func(w *contractWire) { w.ContractID = "" })
	if err := low.Validate(); err != nil {
		t.Fatal("CONTROL_LOW_WORK_CONTRACT_INVALID", err)
	}
	boundedReq = req
	boundedReq.Contract = low
	if Run(context.Background(), boundedReq).Err == nil {
		t.Fatal("ASSERT_ALLMODE_WORK_BOUND")
	}
	t.Log("ASSERT_ALLMODE_WORK_BOUND: PASS")
	for _, size := range []int{rg.EvaluationMaxStringPrivate, rg.EvaluationMaxStringPrivate + 1} {
		altered := contract
		altered.wire.PacketSourcePolicy.PolicyID = strings.Repeat("p", size)
		altered.wire.ContractID = identityJSON("lsp-trace:census-continuation-contract:v1", altered.wire, func(w *contractWire) { w.ContractID = "" })
		if err := altered.Validate(); err != nil {
			t.Fatal("CONTROL_BOUNDED_METADATA_CONTRACT_INVALID", err)
		}
		r := req
		r.Contract = altered
		if size == rg.EvaluationMaxStringPrivate {
			if result := Run(context.Background(), r); result.Err != nil {
				t.Fatal("ASSERT_ALLMODE_CONTRACT_BOUND_EQUALITY", result.Err)
			}
		} else {
			spy := &privateAllModePutSpy{MemoryStore: NewMemoryStore(), max: 8 << 20}
			r.Store = spy
			if result := Run(context.Background(), r); result.Err == nil || spy.puts != 0 {
				t.Fatal("ASSERT_ALLMODE_CONTRACT_SCALAR_PREFLIGHT", spy.puts, result.Err)
			}
		}
	}
	t.Log("ASSERT_ALLMODE_CONTRACT_BOUND_EQUALITY: PASS")
	t.Log("ASSERT_ALLMODE_CONTRACT_SCALAR_PREFLIGHT: PASS")
	r := req
	r.Store = NewMemoryStore()
	if Run(context.Background(), r).Err == nil {
		t.Fatal("ASSERT_ALLMODE_UNBOUNDED_STORE_REFUSED")
	}
	r = req
	r.StopAfter = StopAfterDescribeRequests
	if Run(context.Background(), r).Err == nil {
		t.Fatal("ASSERT_ALLMODE_PRIVATE_STOP_EXCLUSIVE")
	}
	resume := ResumeRequest{PrivateAllMode: &PrivateAllModeResume{Version: PrivateAllModeVersion, Expected: h, Contract: contract}, Selector: out.CheckpointID, Workspace: req.Workspace, Store: store, StopAfter: StopAfterSourceIntents}
	again := Resume(context.Background(), resume)
	if again.Err != nil || again.CheckpointID != out.CheckpointID {
		t.Fatal("ASSERT_ALLMODE_MISSING_RESUME", again.Err)
	}
	for _, label := range []string{"ASSERT_ALLMODE_MISSING_PAUSE", "ASSERT_ALLMODE_MISSING_NOT_EMPTY", "ASSERT_ALLMODE_MISSING_NO_FAKE_RESULT", "ASSERT_ALLMODE_FINITE_STORE_BOUND", "ASSERT_ALLMODE_UNBOUNDED_STORE_REFUSED", "ASSERT_ALLMODE_PRIVATE_STOP_EXCLUSIVE", "ASSERT_ALLMODE_MISSING_RESUME"} {
		t.Log(label + ": PASS")
	}
}
