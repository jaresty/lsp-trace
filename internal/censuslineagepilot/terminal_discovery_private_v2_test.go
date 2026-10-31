//go:build darwin || linux

package censuslineagepilot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"lsp-trace/internal/censusacquisition"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/lspwire"
)

func terminalFixturePrivateV2(t *testing.T, count int, during ...func(context.Context, FrozenScopePrivateV2, TerminalDiscoveryOutcomePrivateV2)) (FrozenScopePrivateV2, func(bool) (EnumerationOriginPrivateV2, TerminalDiscoveryOutcomePrivateV2, ScopeDiscoveryReportPrivateV2, error)) {
	t.Helper()
	spec, m, id, child := producerFixturePrivateV2(t, 1, 0)
	body := "package owned\n\n"
	entries := count
	if entries == 0 {
		entries = 8
	}
	for i := 0; i < entries; i++ {
		if count == 0 {
			body += fmt.Sprintf("var C%d = 1\n\n", i)
		} else {
			body += fmt.Sprintf("func C%d() {}\n\n", i)
		}
	}
	if err := os.WriteFile(filepath.Join(spec.Root, spec.Members[0].CanonicalRelativePath), []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	spec.Members[0].ExpectedSHA256 = Digest([]byte(body))
	spec.Members[0].ByteLength = uint64(len(body))
	child.mu.Lock()
	child.response = func(msg lspwire.Message) json.RawMessage {
		var p lsp.PrepareCallHierarchyParams
		_ = json.Unmarshal(msg.Params, &p)
		item := func(i int) lsp.CallHierarchyItem {
			v := producerItemPrivateV2(p.TextDocument.URI, i)
			v.Name = fmt.Sprintf("C%d", i)
			v.Range.Start.Line += 2
			v.Range.End.Line += 2
			v.SelectionRange.Start.Line += 2
			v.SelectionRange.End.Line += 2
			return v
		}
		var value any = []any{}
		switch msg.Method {
		case "textDocument/documentSymbol":
			symbols := []lsp.DocumentSymbol{}
			n := count
			if n == 0 {
				n = 8
			}
			for i := 0; i < n; i++ {
				v := item(i)
				if count == 0 {
					v.Kind = 13
				}
				symbols = append(symbols, lsp.DocumentSymbol{Name: v.Name, Kind: v.Kind, Range: v.Range, SelectionRange: v.SelectionRange})
			}
			value = symbols
		case "textDocument/prepareCallHierarchy":
			value = []lsp.CallHierarchyItem{item(int((p.Position.Line - 2) / 2))}
		}
		raw, _ := json.Marshal(value)
		return raw
	}
	child.mu.Unlock()
	f, err := FreezeScopePrivateV2(context.Background(), spec)
	if err != nil {
		t.Fatal(err)
	}
	return f, func(old bool) (o EnumerationOriginPrivateV2, terminal TerminalDiscoveryOutcomePrivateV2, r ScopeDiscoveryReportPrivateV2, err error) {
		producerLeasePrivateV2(t, m, id, spec.Root, func(ctx context.Context) error {
			if old {
				o, r, err = DiscoverScopePrivateV2(ctx, f, m, id)
			} else {
				o, terminal, r, err = DiscoverScopeOutcomePrivateV2(ctx, f, m, id)
				if err == nil {
					for _, visit := range during {
						visit(ctx, f, terminal)
					}
				}
			}
			return nil
		})
		return
	}
}
func TestTerminalDiscoveryOwnedPrivateV2(t *testing.T) {
	for _, n := range []int{84, 0} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			_, run := terminalFixturePrivateV2(t, n)
			o, terminal, report, err := run(false)
			var d censusacquisition.Discovery
			if json.Unmarshal(report.Bytes(), &d) != nil || len(d.Targets) != n || d.Accounting.SymbolDenominator != map[bool]int{true: 8, false: 84}[n == 0] || d.Accounting.Validate() != nil {
				t.Fatal("fixture did not execute exact original discovery ledger", err)
			}
			t.Logf("ORIGINAL_DISCOVERY targets=%d symbols=%d complete=%t closure=%d", len(d.Targets), d.Accounting.SymbolDenominator, d.Complete, d.Closure)
			want := "TERMINAL_RESOURCE_GAP_TARGET_CAP"
			if n == 0 {
				want = "TERMINAL_NO_PREPARABLE_TARGETS"
			}
			if err != nil || terminal.Reason() != want || len(o.Bytes()) != 0 {
				t.Errorf("ASSERT_TERMINAL_OWNED_OUTCOME: targets=%d reason=%q error=%v", n, terminal.Reason(), err)
			} else {
				t.Log("ASSERT_TERMINAL_OWNED_OUTCOME PASS")
			}
			_, _, _, again := run(false)
			if again == nil {
				t.Fatal("ASSERT_TERMINAL_DISCOVERY_SPENT")
			}
		})
	}
}
func TestTerminalLegacyDiscoveryPrivateV2(t *testing.T) {
	for _, n := range []int{84, 0} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			_, run := terminalFixturePrivateV2(t, n)
			_, terminal, _, err := run(true)
			// The zero-target legacy path fails its original !Complete check first.
			if !errors.Is(err, ErrScopeDiscoveryIncompletePrivateV2) || terminal.Reason() != "" {
				t.Fatal("ASSERT_TERMINAL_LEGACY_UNCHANGED", err)
			}
		})
	}
}
