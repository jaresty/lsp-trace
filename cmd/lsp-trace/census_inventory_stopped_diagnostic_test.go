package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"testing"

	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/censusresult"
	"lsp-trace/internal/censussourceassembly"
	continuation "lsp-trace/internal/censussourcecontinuation"
	"lsp-trace/internal/censussourcedriver"
	"lsp-trace/internal/censussourceplan"
	"lsp-trace/internal/discoveryfailure"
	"lsp-trace/internal/lsp"
	"lsp-trace/sessionruntime"
)

func TestStopped66ActualMapper(t *testing.T) {
	for _, tc := range []struct {
		stop censussourcedriver.Stop
		code censusresult.Code
	}{
		{censussourcedriver.StopRetained, "DISCOVERY_RETAINED"},
		{censussourcedriver.StopIndeterminateExecution, "DISCOVERY_INDETERMINATE"},
	} {
		for _, input := range []struct {
			name  string
			cause error
		}{
			{"nil", nil}, {"opaque", errors.New("secret://source/credential")},
			{"private-sentinel", sessionruntime.ErrPrivateDiscovery}, {"deadline", context.DeadlineExceeded},
		} {
			cause := input.cause
			t.Run(string(tc.stop)+"/"+input.name, func(t *testing.T) {
				d, err := censusInventoryStoppedDiagnostic(tc.stop, cause)
				if err != nil || d.Stage != censusresult.StageDiscovery || d.Code != tc.code || d.Retry || d.Status != "FAILED" {
					t.Fatalf("STOP_PRESERVED: stop=%s code=%s retry=%t stage=%s error=%v", tc.stop, d.Code, d.Retry, d.Stage, err)
				}
				raw, err := censusresult.MarshalDiagnostic(d)
				if err != nil {
					t.Fatal(err)
				}
				decoded, err := censusresult.DecodeDiagnostic(raw)
				if err != nil || decoded.Code != tc.code || decoded.Retry || bytes.Contains(raw, []byte("secret")) {
					t.Fatal("STOP_SAFE_ROUNDTRIP", err)
				}
			})
		}
	}
}

func TestStopped66RefusalCoexistence(t *testing.T) {
	for _, stop := range []censussourcedriver.Stop{censussourcedriver.StopRetained, censussourcedriver.StopIndeterminateExecution, censussourcedriver.StopFailed} {
		ref := sessionruntime.PrivateDiscoveryRefusal{Stage: sessionruntime.PrivateDiscoveryRead, Currency: sessionruntime.PrivateDiscoveryResponseBytes, Requested: 1, Remaining: 0, AmountsAvailable: true, CleanupUncertain: true}
		d, e := censusInventoryStoppedDiagnosticValue(stop, &ref)
		if e != nil || d.PrivateDiscoveryRefusal == nil || *d.PrivateDiscoveryRefusal != ref || d.Retry {
			t.Fatal("STOP_FIRST_REFUSAL_COEXISTS", e)
		}
		if stop == censussourcedriver.StopRetained && d.Code != censusresult.CodeDiscoveryRetained || stop == censussourcedriver.StopIndeterminateExecution && d.Code != censusresult.CodeDiscoveryIndeterminate || stop == censussourcedriver.StopFailed && d.Code != censusresult.CodeDiscoveryFailed {
			t.Fatal("REFUSAL_CANNOT_ERASE_STOP")
		}
		ref.Requested = 9
		if d.PrivateDiscoveryRefusal.Requested != 1 {
			t.Fatal("REFUSAL_VALUE_NO_ALIAS")
		}
		if _, e = censusresult.MarshalDiagnostic(d); e != nil {
			t.Fatal(e)
		}
	}
	if _, e := censusInventoryStoppedDiagnosticValue(censussourcedriver.StopRetained, &sessionruntime.PrivateDiscoveryRefusal{}); e == nil {
		t.Fatal("INVALID_REFUSAL_NOT_FORWARDED")
	}
}

func TestStopped66OrdinaryFallback(t *testing.T) {
	d, e := censusInventoryStoppedDiagnostic(censussourcedriver.StopFailed, errors.New("secret://source"))
	if e != nil || d.Code != censusresult.CodeInvalidConfig || !d.Retry {
		t.Fatal("ORDINARY_FALLBACK_UNCHANGED", e)
	}
}

// Structural integration only: this does not execute the authority-bearing bridge.
func TestStopped66ProductionCallsite(t *testing.T) {
	raw, e := os.ReadFile("census_inventory_bridge_private.go")
	if e != nil {
		t.Fatal(e)
	}
	f, e := parser.ParseFile(token.NewFileSet(), "bridge.go", raw, 0)
	if e != nil {
		t.Fatal(e)
	}
	found := false
	ast.Inspect(f, func(n ast.Node) bool {
		c, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		id, ok := c.Fun.(*ast.Ident)
		if !ok || id.Name != "writeCensusInventoryDiscoveryFailure" {
			return true
		}
		if len(c.Args) != 4 {
			return true
		}
		s, ok := c.Args[3].(*ast.SelectorExpr)
		if ok && s.Sel.Name == "Stop" {
			v, ok := s.X.(*ast.Ident)
			found = ok && v.Name == "result"
		}
		return true
	})
	if !found {
		t.Fatal("ACTUAL_BRIDGE_MUST_FORWARD_STOP")
	}
}

type memory66 struct {
	objects map[string][]byte
	head    []byte
}

func (m *memory66) Put(_ context.Context, b []byte) (string, error) {
	s := fmt.Sprintf("sha256:%x", sha256.Sum256(b))
	m.objects[s] = append([]byte(nil), b...)
	return s, nil
}
func (m *memory66) Get(_ context.Context, s string) ([]byte, error) {
	return append([]byte(nil), m.objects[s]...), nil
}
func (m *memory66) Load(context.Context, string) ([]byte, error) {
	return append([]byte(nil), m.head...), nil
}
func (m *memory66) CompareAndSwap(_ context.Context, _ string, expected continuation.HeadRevision, b []byte) continuation.CASResult {
	current, e := continuation.ParseHeadRevision(m.head)
	if e != nil || current != expected {
		return continuation.CASResult{Outcome: continuation.CASNotCommitted}
	}
	m.head = append([]byte(nil), b...)
	return continuation.CASResult{Outcome: continuation.CASCommitted}
}

type client66 struct{}

func (client66) SupportsDocumentSymbols() bool { return true }
func (client66) SupportsCallHierarchy() bool   { return true }
func (client66) DocumentSymbols(context.Context, lsp.DocumentSymbolParams) ([]lsp.DocumentSymbol, error) {
	var out []lsp.DocumentSymbol
	for _, line := range []uint32{1, 2} {
		p := lsp.Position{Line: line}
		r := lsp.Range{Start: p, End: lsp.Position{Line: line, Character: 3}}
		out = append(out, lsp.DocumentSymbol{Name: "Run", Kind: 12, Range: r, SelectionRange: r})
	}
	return out, nil
}
func (client66) PrepareCallHierarchy(_ context.Context, p lsp.PrepareCallHierarchyParams) ([]lsp.CallHierarchyItem, error) {
	if p.Position.Line == 2 {
		return nil, errors.New("secret://source")
	}
	r := lsp.Range{Start: p.Position, End: p.Position}
	return []lsp.CallHierarchyItem{{URI: p.TextDocument.URI, Name: "Run", Kind: 12, Range: r, SelectionRange: r}}, nil
}

type supplier66 struct{}

func (supplier66) Supply(context.Context, censusacquisition.SourceFile) error { return nil }
func TestStopped66ProductionAssemblyAndReceiptCompatibility(t *testing.T) {
	ctx := context.Background()
	files := []censusacquisition.SourceFile{{Path: "a.go", URI: "file:///offline/a.go", LanguageID: "go"}}
	plan, e := censussourceplan.Build(files, "/offline", 16)
	if e != nil {
		t.Fatal(e)
	}
	discover := func(ctx context.Context, _ int, fs []censusacquisition.SourceFile) (censusacquisition.Discovery, error) {
		return (censusacquisition.DiscoveryAdapter{Workspace: "/offline", Files: censusacquisition.StaticFiles(fs), Supplier: supplier66{}, Client: client66{}}).Discover(ctx, censusacquisition.SessionIdentity{"offline", 1})
	}
	m := &memory66{objects: map[string][]byte{}}
	result, terminal := censussourceassembly.Run(ctx, censussourceassembly.Request{Plan: plan, PartitionSize: 16, WorkflowKey: "offline", HeadKey: "offline", Objects: m, Heads: m, StepBound: 1, Discover: discover})
	if result.Stop != censussourcedriver.StopRetained || result.Err != nil || result.State.CommittedPrefix != 0 || result.Transitions != 0 || terminal != nil {
		t.Fatalf("PARTIAL_RETAINED_NOT_COMMITTED: %+v", result)
	}
	d, e := censusInventoryStoppedDiagnostic(result.Stop, result.Err)
	if e != nil || d.Code != censusresult.CodeDiscoveryRetained || d.Retry {
		t.Fatal("ACTUAL_ASSEMBLY_STOP_DIAGNOSTIC", e)
	}
	head, e := continuation.InspectHead(m.head, "offline", plan.Identity, 1)
	if e != nil || head.State != continuation.HeadStateClaimed || head.CommittedPrefix != 0 {
		t.Fatal("CUSTODY_STAYS_CLAIMED", e)
	}
	raw := m.objects[head.ReceiptSelector]
	old, e := continuation.DecodeReceipt(raw, "offline", plan, 0)
	if e != nil || old.Outcome != continuation.OutcomePartial || !old.Discovery.Complete || old.Discovery.Closure == censusacquisition.DiscoveryClosureSuccessful {
		t.Fatal("ENUMERATION_NOT_CALLABLE_CLOSURE", e)
	}
	if bytes.Contains(raw, []byte("PrepareCause")) {
		t.Fatal("OLD_UNKNOWN_FIELD_OMITTED")
	}
	// Source-independent legacy receipt remains byte-identical, including nil paths.
	var envelope struct {
		Workflow    string `json:"workflow_key"`
		Generation  uint64 `json:"claim_generation"`
		Selector    string `json:"claim_selector"`
		Idempotency string `json:"idempotency_key"`
	}
	if e := json.Unmarshal(raw, &envelope); e != nil {
		t.Fatal(e)
	}
	re, e := continuation.EncodeReceipt(envelope.Workflow, envelope.Generation, envelope.Selector, envelope.Idempotency, old)
	if e != nil || !bytes.Equal(re, raw) {
		t.Fatal("LEGACY_RECEIPT_CANONICAL_BYTES", e)
	}
	old.Discovery.ProviderFailures[0].PrepareCause = discoveryfailure.RPCServer
	classified, e := continuation.EncodeReceipt(envelope.Workflow, envelope.Generation, envelope.Selector, envelope.Idempotency, old)
	if e != nil || bytes.Equal(classified, raw) {
		t.Fatal("NEW_CAUSE_AFFECTS_NEW_CONTENT_ONLY", e)
	}
	known, e := continuation.DecodeReceipt(classified, "offline", plan, 0)
	if e != nil || known.Discovery.ProviderFailures[0].PrepareCause != discoveryfailure.RPCServer {
		t.Fatal("CLASSIFIED_RECEIPT_ROUNDTRIP", e)
	}
	// Exercise the unchanged inventory field-quoting codec with the new named
	// string, without opening a cursor, source, provider, or accounting root.
	bounded, e := censusInventoryMarshal(known, 4<<20)
	var boundedTerminal continuation.Terminal
	if e != nil || censusInventoryDecode(bounded, &boundedTerminal, 4<<20) != nil || boundedTerminal.Discovery.ProviderFailures[0].PrepareCause != discoveryfailure.RPCServer {
		t.Fatal("EXISTING_BOUNDED_CODEC_ACCEPTS_CLOSED_CAUSE", e)
	}
	bad := bytes.Replace(classified, []byte("RPC_SERVER_ERROR"), []byte("secret://source"), 1)
	if _, e := continuation.DecodeReceipt(bad, "offline", plan, 0); e == nil {
		t.Fatal("RECEIPT_UNKNOWN_CAUSE_REJECTED")
	}
}
