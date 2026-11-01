//go:build darwin

package censuslineagepilot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/publication"
	"lsp-trace/internal/runtimeprofile"
	"lsp-trace/sessionruntime"
)

// Only LENGTHS from the retained failure are used as controlled child payload
// sizing inputs. No old artifact, source snapshot, successful capture or Census
// origin is replayed. Resulting artifacts are new synthetic observations and do
// not claim exact real semantic content, throughput or serialized lengths.
var pageShapeLengthsPrivateV3 = []int{118314, 122012, 121964, 122411, 827897, 117924, 331764, 118321, 185323, 97127, 590523, 184926, 99553, 99553, 121200, 246996, 120085, 142112, 80573, 163692}

func pagePeerItemPrivateV3(uri string, i int) lsp.CallHierarchyItem {
	line := uint32(1 + i)
	data := fmt.Sprintf("{\"n\":1.000e+2,\"ordered\":[null,null],\"pad\":%q}", strings.Repeat("x", pageShapeLengthsPrivateV3[i]/8))
	return lsp.CallHierarchyItem{Name: fmt.Sprintf("Owned%02d", i), Kind: 12, URI: uri, Range: lsp.Range{Start: lsp.Position{Line: line}, End: lsp.Position{Line: line, Character: 17}}, SelectionRange: lsp.Range{Start: lsp.Position{Line: line, Character: 5}, End: lsp.Position{Line: line, Character: 12}}, Data: json.RawMessage(data)}
}
func TestProducerPagesPrivateV3OwnedStdioPeer(t *testing.T) {
	if os.Getenv("LSP_TRACE_OWNED_PAGE_PEER") != "1" {
		return
	}
	reader := lspwire.NewReader(os.Stdin, lspwire.DefaultLimits())
	writer := lspwire.NewWriter(os.Stdout, lspwire.DefaultLimits())
	for {
		msg, e := reader.Read()
		if e != nil {
			os.Exit(2)
		}
		var value any = []any{}
		switch msg.Method {
		case "initialize":
			value = map[string]any{"capabilities": map[string]any{"positionEncoding": "utf-16", "documentSymbolProvider": true, "callHierarchyProvider": true}, "serverInfo": map[string]any{"name": "owned-page-peer", "version": "1"}}
		case "textDocument/documentSymbol":
			var p lsp.DocumentSymbolParams
			if json.Unmarshal(msg.Params, &p) != nil {
				os.Exit(3)
			}
			symbols := []lsp.DocumentSymbol{}
			for i := 0; i < 20; i++ {
				item := pagePeerItemPrivateV3(p.TextDocument.URI, i)
				symbols = append(symbols, lsp.DocumentSymbol{Name: item.Name, Kind: item.Kind, Range: item.Range, SelectionRange: item.SelectionRange})
			}
			value = symbols
		case "textDocument/prepareCallHierarchy":
			var p lsp.PrepareCallHierarchyParams
			if json.Unmarshal(msg.Params, &p) != nil || p.Position.Line < 1 || p.Position.Line > 20 {
				os.Exit(3)
			}
			value = []lsp.CallHierarchyItem{pagePeerItemPrivateV3(p.TextDocument.URI, int(p.Position.Line)-1)}
		case "callHierarchy/incomingCalls":
			value = nil
		case "callHierarchy/outgoingCalls":
			var p lsp.CallHierarchyOutgoingCallsParams
			if json.Unmarshal(msg.Params, &p) != nil {
				os.Exit(3)
			}
			call := lsp.CallHierarchyOutgoingCall{To: p.Item, FromRanges: []lsp.Range{{Start: p.Item.SelectionRange.Start, End: p.Item.SelectionRange.End}}}
			value = []lsp.CallHierarchyOutgoingCall{call, call}
		case "shutdown":
			value = nil
		case "exit":
			os.Exit(0)
		default:
			if len(msg.ID) == 0 {
				continue
			}
		}
		raw, _ := json.Marshal(value)
		if writer.Write(lspwire.Message{JSONRPC: lspwire.Version, ID: msg.ID, Result: raw}) != nil {
			os.Exit(4)
		}
	}
}

// ReadControlledPagePairPrivateV3 is a test-only independent physical oracle.
func ReadControlledPagePairPrivateV3(t *testing.T, p ProducerPagesPrivateV3, id string, max int64) []byte {
	t.Helper()
	var out []byte
	for _, root := range p.state.roots.state.outputs {
		raw, err := p.state.ops.read(context.Background(), root, id, max)
		if err != nil || Digest(raw) != id || out != nil && !bytes.Equal(out, raw) {
			t.Fatal("ASSERT_V3_PHYSICAL_REFERENCE_PAIR", id, err)
		}
		out = raw
	}
	if err := publication.CheckProducerPacketObjectsDistinctPrivateV2(context.Background(), p.state.roots.state.outputs, id); err != nil {
		t.Fatal("ASSERT_V3_REFERENCE_DISTINCT_INODES", err)
	}
	return out
}

func TestProducerPagesPrivateV3OwnedTwentyStdio(t *testing.T) {
	ControlledTwentyPagesPrivateV3(t, pageAllowancePrivateV3())
}

// Exported only in the test binary so external-package integration guards can
// consume actual original owners without any production fixture constructor.
func ControlledTwentyPagesPrivateV3(t *testing.T, allowance PageCostPrivateV3) (ProducerPagesPrivateV3, PageRevisionPrivateV3) {
	t.Helper()
	ctx := context.Background()
	root, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	spec := producerSpecPrivateV2(t, root, 1, 0)
	spec.EffectivePolicy.Planning.MaxBatchTargets = 1
	body := "package owned\n"
	for i := 0; i < 20; i++ {
		body += fmt.Sprintf("func Owned%02d() {}\n", i)
	}
	if e = os.WriteFile(filepath.Join(root, spec.Members[0].CanonicalRelativePath), []byte(body), 0600); e != nil {
		t.Fatal(e)
	}
	spec.Members[0].ExpectedSHA256 = Digest([]byte(body))
	spec.Members[0].ByteLength = uint64(len(body))
	f, e := FreezeScopePrivateV2(ctx, spec)
	if e != nil {
		t.Fatal(e)
	}
	supervisor, e := managedprocess.NewLocalDarwinSupervisor(managedprocess.Options{StderrLimit: 4096, GracePeriod: 100 * time.Millisecond})
	if e != nil {
		t.Fatal(e)
	}
	manager, e := sessionruntime.New(sessionruntime.Config{Limits: sessionruntime.Limits{MaxSessions: 1, MaxRequests: 2, MaxChildren: 2, MaxCancels: 2, MaxTombstones: 256, MaxObservations: 4096, MaxOperations: 2}, Starter: sessionruntime.ManagedStarter{Manager: supervisor}})
	if e != nil {
		t.Fatal(e)
	}
	defer func() {
		if e := manager.Shutdown(ctx); e != nil {
			t.Error(e)
		}
	}()
	validated, e := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "test", Workspace: root, Profile: "go", EnvironmentReference: "local"})
	if e != nil {
		t.Fatal(e)
	}
	executable, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	start := manager.Start(ctx, sessionruntime.StartRequest{Profile: runtimeprofile.Resolve(validated), LanguageID: "go", Process: managedprocess.Spec{Path: executable, Args: []string{"-test.run=^TestProducerPagesPrivateV3OwnedStdioPeer$"}, Dir: root, Env: []string{"LSP_TRACE_OWNED_PAGE_PEER=1"}}})
	if start.Failure != "" {
		t.Fatal(start)
	}
	ready := manager.BeginReadiness(ctx, start.SessionID, start.Generation, time.Now().Add(5*time.Second))
	ready, ok := manager.WaitReadiness(ctx, ready.ID)
	if !ok || ready.Failure != "" {
		t.Fatal(ready, ok)
	}
	identity := censusacquisition.SessionIdentity{SessionID: start.SessionID, Generation: start.Generation}
	var o EnumerationOriginPrivateV2
	var captures []OpaqueSuccessfulV2Capture
	producerLeasePrivateV2(t, manager, identity, root, func(ctx context.Context) error {
		var e error
		var report ScopeDiscoveryReportPrivateV2
		o, report, e = DiscoverScopePrivateV2(ctx, f, manager, identity)
		if e != nil {
			return fmt.Errorf("owned discovery: %w; observation=%s failure=%+v", e, report.Bytes(), report.failure)
		}
		if o.BatchCount() != 20 {
			return fmt.Errorf("ASSERT_TWENTY_ORIGINAL_BATCHES: %d", o.BatchCount())
		}
		for i := 0; i < o.BatchCount(); i++ {
			b, e := o.Batch(i)
			if e != nil {
				return e
			}
			c, e := AcquireScopeBatchPrivateV2(ctx, manager, o, b)
			if e != nil {
				return fmt.Errorf("owned capture %d: %w", i, e)
			}
			captures = append(captures, c)
		}
		return nil
	})
	stop := manager.Stop(ctx, start.SessionID, "owned-page-fixture-complete")
	if stop.Failure != "" {
		t.Fatal(stop)
	}
	roots, _, _ := packetRootsPrivateV2(t, f)
	ledger, _, _ := ledgerFixturePrivateV3(t, allowance)
	t.Cleanup(func() { _ = ledger.Close() })
	p, e := NewProducerPagesPrivateV3(ctx, f, o, captures, roots, PagePolicyPrivateV3{32768, 1 << 20, Digest([]byte("owned twenty config"))}, ledger)
	if e != nil {
		t.Fatal(e)
	}
	r, id, e := p.Advance(ctx, 1, pageAllowancePrivateV3(), "")
	if e != nil {
		t.Fatal(e)
	}
	if r.BatchDenominator != 20 || r.ObjectDenominator != 88 || r.Status != "COMPLETE_ORIGIN_PAGES" {
		t.Fatal("ASSERT_ACTUAL_OWNED_TWENTY_DENOMINATOR", r)
	}
	if e = p.VerifyPrefix(ctx, r, pageAllowancePrivateV3()); e != nil {
		t.Fatal(e)
	}
	total := 0
	for i, c := range captures {
		total += len(c.state.artifact)
		raw, e := p.ReadObject(ctx, r, uint64(7+len(f.state.files)+4*i+2), pageAllowancePrivateV3())
		if e != nil || Digest(raw) != Digest(c.state.artifact) {
			t.Fatal("ASSERT_ALL_TWENTY_ORIGINAL_ARTIFACT_BYTES", i, e)
		}
	}
	t.Logf("CONTROLLED_NOT_LIVE: targets=20 batches=20 raw_artifact_bytes=%d pages=%d revision=%s", total, r.Cursor.PageCount, id)
	return p, r
}
