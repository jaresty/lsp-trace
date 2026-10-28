//go:build darwin || linux

package censuslineagepilot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/lspwire"
	"lsp-trace/sessionruntime"
)

// Generated protocol data, not retained live source or a new custody assertion.
// All four nominated files are visited; noncallables (including a nested node)
// count toward the same denominator as callables. Repeated names are preserved.
func discoverPolicyFixturePrivate(t *testing.T, policy scopedExecutionPolicyPrivate, total, callables int, badPrepare bool) censusacquisition.Discovery {
	t.Helper()
	restart, manager, id, child := adapterFixture(t)
	var got censusacquisition.Discovery
	err := manager.WithReadySessionLease(context.Background(), sessionruntime.SessionLeaseRequest{SessionID: id.SessionID, Generation: id.Generation, Workspace: restart.Scope().Selection().Workspace}, func(ctx context.Context) error {
		a, _, err := newScopeRuntimeAdapterWithPolicyPrivate(ctx, restart, manager, id, policy)
		if err != nil {
			return err
		}
		if len(a.files) != 4 || len(a.guard.MembersPrivate()) != 6 {
			t.Fatal("ASSERT_SCOPE_FOUR_FILES_SIX_GUARDS")
		}
		symbols := map[string][]lsp.DocumentSymbol{}
		prepared := map[string]map[lsp.Position]lsp.CallHierarchyItem{}
		for fi, file := range a.files {
			n := 40
			if fi == 3 {
				n = total - 120
			}
			prepared[file.URI] = map[lsp.Position]lsp.CallHierarchyItem{}
			for i := 0; i < n; i++ {
				pos := lsp.Position{Character: uint32(i * 2)}
				r := lsp.Range{Start: pos, End: lsp.Position{Character: pos.Character + 1}}
				name, kind := "repeated-noncallable", 13
				// Spread required preparations over all files, with the rest in the last.
				wanted := callables / 4
				if fi == 3 {
					wanted = callables - 3*wanted
				}
				if i < wanted {
					name = fmt.Sprintf("Call%d", i)
					kind = 12
				}
				s := lsp.DocumentSymbol{Name: name, Kind: kind, Range: r, SelectionRange: r}
				symbols[file.URI] = append(symbols[file.URI], s)
				prepared[file.URI][pos] = lsp.CallHierarchyItem{Name: name, Kind: kind, URI: file.URI, Range: r, SelectionRange: r}
			}
			// Same flattened node count; nested duplicate name/range/selection must
			// not collapse the symbol denominator or consume a callable slot.
			list := symbols[file.URI]
			list[len(list)-1].Range = list[len(list)-2].Range
			list[len(list)-1].SelectionRange = list[len(list)-2].SelectionRange
			list[len(list)-2].Children = []lsp.DocumentSymbol{list[len(list)-1]}
			symbols[file.URI] = list[:len(list)-1]
		}
		child.mu.Lock()
		child.response = func(msg lspwire.Message) json.RawMessage {
			var value any = []any{}
			switch msg.Method {
			case "textDocument/documentSymbol":
				var p lsp.DocumentSymbolParams
				_ = json.Unmarshal(msg.Params, &p)
				value = symbols[p.TextDocument.URI]
			case "textDocument/prepareCallHierarchy":
				var p lsp.PrepareCallHierarchyParams
				_ = json.Unmarshal(msg.Params, &p)
				item := prepared[p.TextDocument.URI][p.Position]
				if badPrepare {
					item.Name = "mismatched"
				}
				value = []lsp.CallHierarchyItem{item}
			}
			raw, _ := json.Marshal(value)
			return raw
		}
		child.mu.Unlock()
		got, err = a.Discover(ctx, id)
		if err != nil {
			return err
		}
		if got.Accounting.SymbolDenominator != total || len(got.EnumerationSymbols) != total || len(got.SymbolLedger.Entries) != total {
			t.Fatalf("ASSERT_ALL_SYMBOLS_COUNTED: %+v", got.Accounting)
		}
		docs, prepares, supplies := 0, 0, 0
		for _, msg := range child.snapshot() {
			switch msg.Method {
			case "textDocument/documentSymbol":
				docs++
				var p lsp.DocumentSymbolParams
				_ = json.Unmarshal(msg.Params, &p)
				if _, ok := symbols[p.TextDocument.URI]; !ok {
					t.Fatal("ASSERT_NO_SCOPE_EXPANSION")
				}
			case "textDocument/prepareCallHierarchy":
				prepares++
			case "textDocument/didOpen", "textDocument/didChange":
				supplies++
			}
		}
		if docs != 4 || supplies != 6 {
			t.Fatalf("ASSERT_SCOPE_UNCHANGED docs=%d supplies=%d", docs, supplies)
		}
		if policy == scopedExecutionPolicyV2Private() && prepares != callables {
			t.Fatalf("ASSERT_ALL_REQUIRED_PREPARATIONS got=%d want=%d", prepares, callables)
		}
		// Enumerated ordering and duplicate multiplicity must survive policy selection.
		duplicates := 0
		for i, e := range got.EnumerationSymbols {
			if e.Ordinal != i || got.SymbolLedger.Entries[i].Ordinal != i {
				t.Fatal("ASSERT_ORDINALS_PRESERVED")
			}
			if i > 0 {
				prev := got.EnumerationSymbols[i-1]
				if prev.FileOrdinal == e.FileOrdinal && prev.Name == e.Name && prev.SelectionStart == e.SelectionStart {
					duplicates++
				}
				if prev.FileOrdinal > e.FileOrdinal || (prev.FileOrdinal == e.FileOrdinal && prev.SelectionStart.Character > e.SelectionStart.Character) {
					t.Fatal("ASSERT_ORDER_PRESERVED")
				}
			}
		}
		if duplicates != 4 {
			t.Fatalf("ASSERT_EXACT_DUPLICATE_MULTIPLICITY got=%d want=4", duplicates)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestScopedDiscoveryPolicyPrivate219AndBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name           string
		policy         scopedExecutionPolicyPrivate
		total, omitted int
	}{
		{"v1-219", scopedExecutionPolicyFixedPrivate(), 219, 91},
		{"v2-219", scopedExecutionPolicyV2Private(), 219, 0},
		{"v2-256", scopedExecutionPolicyV2Private(), 256, 0},
		{"v2-257", scopedExecutionPolicyV2Private(), 257, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := discoverPolicyFixturePrivate(t, tc.policy, tc.total, 63, false)
			limits, children := 0, 0
			for _, c := range d.IncompleteCauses {
				switch c.Code {
				case "SYMBOL_LIMIT":
					if c.SymbolOrdinal == nil || *c.SymbolOrdinal != tc.total-tc.omitted+limits {
						t.Fatal("ASSERT_EXACT_OMITTED_ORDINALS")
					}
					limits++
				case "CHILD_INCOMPLETE":
					if c.FileOrdinal == nil || *c.FileOrdinal != 3 {
						t.Fatal("ASSERT_LAST_FILE_CHILD")
					}
					children++
				default:
					t.Fatalf("ASSERT_NO_OTHER_INCOMPLETE_CAUSE: %+v", c)
				}
			}
			if limits != tc.omitted || (children == 0) != (tc.omitted == 0) || children > 1 || len(d.ProviderFailures) != 0 {
				t.Fatalf("ASSERT_EXACT_CAUSES limits=%d children=%d failures=%+v", limits, children, d.ProviderFailures)
			}
			if d.Complete != (tc.omitted == 0) {
				t.Fatal("ASSERT_FINITE_SYMBOL_CLOSURE")
			}
			if tc.omitted == 0 && (d.Closure != censusacquisition.DiscoveryClosureSuccessful || len(d.Targets) != 63) {
				t.Fatal("ASSERT_VALID_PREPARED_COMPLETENESS")
			}
		})
	}
	d := discoverPolicyFixturePrivate(t, scopedExecutionPolicyV2Private(), 219, 63, true)
	if d.Complete || len(d.ProviderFailures) == 0 || len(d.IncompleteCauses) == 0 || len(d.Targets) != 0 {
		t.Fatal("ASSERT_RESOURCE_OR_PREPARATION_FAILURE_NOT_EMPTY_SUCCESS")
	}
	d = discoverPolicyFixturePrivate(t, scopedExecutionPolicyV2Private(), 219, 64, false)
	if !d.Complete || len(d.Targets) != 64 {
		t.Fatal("ASSERT_SYMBOL_LIMIT_SEPARATE_FROM_TARGET_POLICY")
	}
}

func TestScopedDiscoveryPolicyPrivateFixedVersions(t *testing.T) {
	v1, v2 := scopedExecutionPolicyFixedPrivate(), scopedExecutionPolicyV2Private()
	old, _ := json.Marshal(v1)
	if string(old) != `{"name":"one-native-v5-private-v1","down_depth":1,"up_depth":1,"max_targets":63}` {
		t.Fatalf("ASSERT_V1_POLICY_BYTES: %s", old)
	}
	for _, p := range []scopedExecutionPolicyPrivate{v1, v2} {
		limits, e := p.discoveryLimits()
		if e != nil || limits.MaxFiles != 4 || limits.MaxDepth != 64 || limits.MaxStringBytes != 1<<20 || p.MaxTargets != 63 {
			t.Fatal("ASSERT_FIXED_POLICY_RESOURCES")
		}
		want := 128
		if p == v2 {
			want = 256
		}
		if limits.MaxSymbols != want {
			t.Fatal("ASSERT_FIXED_SYMBOL_BUDGET")
		}
	}
	for _, p := range []scopedExecutionPolicyPrivate{{}, {Name: strings.ToUpper(v2.Name), DownDepth: 1, UpDepth: 1, MaxTargets: 63, MaxSymbols: 256}, {Name: v2.Name, DownDepth: 1, UpDepth: 1, MaxTargets: 63, MaxSymbols: 128}, {Name: v1.Name, DownDepth: 1, UpDepth: 1, MaxTargets: 63, MaxSymbols: 256}} {
		if _, e := p.discoveryLimits(); e == nil {
			t.Fatal("ASSERT_UNKNOWN_MIXED_POLICY_REJECTED")
		}
		if _, _, e := newScopeRuntimeAdapterWithPolicyPrivate(context.Background(), scopeRestartResult{}, nil, censusacquisition.SessionIdentity{}, p); e == nil {
			t.Fatal("ASSERT_POLICY_REJECTED_BEFORE_SEMANTICS")
		}
	}
}

func TestScopedDiscoveryPolicyPrivateClaimVersions(t *testing.T) {
	f := executionClaimFixture(t)
	old := f.owner(t)
	explicit, e := newScopedExecutionClaimWithPolicyPrivate(context.Background(), f.restart, f.revision, f.session, f.workspace, f.roots, scopedExecutionPolicyFixedPrivate())
	if e != nil || !bytes.Equal(old.HeadBytes(), explicit.HeadBytes()) {
		t.Fatal("ASSERT_DEFAULT_V1_BYTES_UNCHANGED", e)
	}
	v2, e := newScopedExecutionClaimWithPolicyPrivate(context.Background(), f.restart, f.revision, f.session, f.workspace, f.roots, scopedExecutionPolicyV2Private())
	if e != nil {
		t.Fatal(e)
	}
	h, e := parseScopedExecutionHeadPrivate(v2.HeadBytes())
	if e != nil || h.Schema != scopedExecutionClaimSchemaV2Private || h.Binding.Policy != scopedExecutionPolicyV2Private() || Digest(v2.HeadBytes()) == Digest(old.HeadBytes()) {
		t.Fatal("ASSERT_V2_CLAIM_EXPLICIT_HASH", e)
	}
	if v2.selector != old.selector {
		t.Fatal("ASSERT_POLICY_CANNOT_BYPASS_SPENT_COORDINATOR")
	}
	for _, mutate := range []func(*scopedExecutionHeadPrivate){
		func(h *scopedExecutionHeadPrivate) { h.Schema = scopedExecutionClaimSchemaPrivate },
		func(h *scopedExecutionHeadPrivate) { h.Binding.Policy.MaxSymbols = 0 },
		func(h *scopedExecutionHeadPrivate) { h.Binding.Policy.MaxSymbols = 257 },
		func(h *scopedExecutionHeadPrivate) { h.Binding.Policy.Name = strings.ToUpper(h.Binding.Policy.Name) },
	} {
		bad := h
		mutate(&bad)
		raw, _ := json.Marshal(bad)
		if _, e := parseScopedExecutionHeadPrivate(raw); e == nil {
			t.Fatal("ASSERT_MIXED_CLAIM_REJECTED")
		}
	}
	for _, raw := range [][]byte{
		bytes.Replace(v2.HeadBytes(), []byte(`"max_symbols":256`), []byte(`"max_symbols":9223372036854775808`), 1),
		bytes.Replace(v2.HeadBytes(), []byte(`"max_symbols":256`), []byte(`"Max_Symbols":256`), 1),
		bytes.Replace(v2.HeadBytes(), []byte(`"max_symbols":256`), []byte(`"arbitrary_budget":256`), 1),
	} {
		if _, e := parseScopedExecutionHeadPrivate(raw); e == nil {
			t.Fatal("ASSERT_CLOSED_CLAIM_PARSE")
		}
	}
	// Same coordinator remains spent even when a new fixed version is selected.
	store := &executionClaimMemoryPrivate{}
	if _, e := old.claim(context.Background(), store); e != nil {
		t.Fatal(e)
	}
	if _, e := v2.claim(context.Background(), store); e == nil || store.writes != 1 {
		t.Fatal("ASSERT_NO_REPLAY_ACROSS_POLICY_VERSIONS")
	}
}

func TestScopedDiscoveryPolicyPrivateV2Wiring(t *testing.T) {
	h, m, cfg, f := NewScopedRehearsalFixtureV2Private(t)
	if !h.MatchesConfigPrivate(cfg) {
		t.Fatal("ASSERT_V2_CONFIG_OWNER_MATCH")
	}
	// The store rejects a v1 candidate even when all root/input pins agree.
	head, e := parseScopedExecutionHeadPrivate(h.owner.HeadBytes())
	if e != nil {
		t.Fatal(e)
	}
	head.Schema = scopedExecutionClaimSchemaPrivate
	head.Binding.Policy = scopedExecutionPolicyFixedPrivate()
	mixed, _ := json.Marshal(head)
	if _, e := h.store.CompareAbsent(context.Background(), h.owner.selector, mixed); e == nil {
		t.Fatal("ASSERT_STORE_REJECTS_DIFFERENT_POLICY_CANDIDATE")
	}
	err := m.WithReadySessionLease(context.Background(), sessionruntime.SessionLeaseRequest{SessionID: cfg.Session.SessionID, Generation: cfg.Session.Generation, Workspace: cfg.Workspace}, func(ctx context.Context) error {
		if _, e := h.Bind(ctx, m); e != nil {
			return e
		}
		if h.adapter.policy != scopedExecutionPolicyV2Private() {
			t.Fatal("ASSERT_V2_CONFIG_CLAIM_ADAPTER_MATCH")
		}
		before, _ := f.Traffic()
		h.adapter.policy = scopedExecutionPolicyFixedPrivate()
		if _, e := h.Discover(ctx, cfg.Session); e == nil {
			t.Fatal("ASSERT_MIXED_ADAPTER_REJECTED")
		}
		after, _ := f.Traffic()
		if before != after {
			t.Fatal("ASSERT_MISMATCH_NO_SEMANTICS")
		}
		h.adapter.policy = scopedExecutionPolicyV2Private()
		d, e := h.Discover(ctx, cfg.Session)
		if e != nil || !d.Complete {
			t.Fatal("ASSERT_V2_GUARDED_DISCOVERY", e)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestScopedDiscoveryPolicyPrivateV2SixSupplyDrifts(t *testing.T) {
	for i := 0; i < 6; i++ {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			h, m, cfg, f := NewScopedRehearsalFixtureV2Private(t)
			file := h.restart.Scope().Files()[i]
			// Disposable generated fixture only; historical inputs are never opened here.
			if err := os.WriteFile(filepath.Join(cfg.Workspace, file.Path), []byte("package drift\n"), 0600); err != nil {
				t.Fatal(err)
			}
			err := m.WithReadySessionLease(context.Background(), sessionruntime.SessionLeaseRequest{SessionID: cfg.Session.SessionID, Generation: cfg.Session.Generation, Workspace: cfg.Workspace}, func(ctx context.Context) error { _, e := h.Bind(ctx, m); return e })
			if err == nil {
				t.Fatal("ASSERT_V2_ALL_SIX_SUPPLIES_REQUIRED")
			}
			semantic, _ := f.Traffic()
			if semantic != 0 {
				t.Fatal("ASSERT_V2_DRIFT_NO_DISCOVERY")
			}
		})
	}
}

func TestScopedDiscoveryPolicyPrivateConfigAndBinding(t *testing.T) {
	h, _, cfg, _ := NewScopedRehearsalFixturePrivate(t)
	// Parser-only canonical locations, never opened or created.
	cfg.OutputRoots = [3]string{"/explicit/primary", "/explicit/backup", "/explicit/coordinator"}
	raw1, _ := json.Marshal(cfg)
	cfg.Schema = "lsp-trace.scoped-rehearsal.private.v2"
	raw2, _ := json.Marshal(cfg)
	parsed, e := ParseScopedRehearsalConfigPrivate(raw2)
	if e != nil || !reflect.DeepEqual(parsed, cfg) || Digest(raw1) == Digest(raw2) {
		t.Fatal("ASSERT_V2_CONFIG_CANONICAL", e)
	}
	obs, e := parsed.DiscoveryPolicyObservationPrivate()
	if e != nil || obs == nil || obs.MaxSymbols != 256 || obs.MaxFiles != 4 || obs.MaxDepth != 64 || obs.MaxTargets != 63 {
		t.Fatal("ASSERT_V2_EFFECTIVE_OBSERVATION", e)
	}
	for _, raw := range [][]byte{
		bytes.Replace(raw2, []byte(`.private.v2`), []byte(`.private.V2`), 1),
		bytes.Replace(raw2, []byte(`"schema":`), []byte(`"Schema":`), 1),
		bytes.Replace(raw2, []byte(`"schema":`), []byte(`"unknown":`), 1),
		append(bytes.Clone(raw2), ' '),
		append(append(bytes.Clone(raw2[:len(raw2)-1]), []byte(`,"max_symbols":256`)...), '}'),
		append(append(bytes.Clone(raw2[:len(raw2)-1]), []byte(`,"request_policy":"one-native-v5-private-v1"`)...), '}'),
	} {
		if _, e := ParseScopedRehearsalConfigPrivate(raw); e == nil {
			t.Fatal("ASSERT_CLOSED_CONFIG_REJECTS_OVERRIDE")
		}
	}
	h.config.Schema = cfg.Schema
	if h.checkPolicy() == nil || h.MatchesConfigPrivate(h.config) {
		t.Fatal("ASSERT_CONFIG_CLAIM_MISMATCH_BEFORE_CLAIM")
	}
	h.config.Schema = "lsp-trace.scoped-rehearsal.private.v1"
	if h.checkPolicy() != nil {
		t.Fatal("ASSERT_V1_BINDING_RESTORED")
	}
}
