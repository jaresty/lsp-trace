//go:build darwin || linux

package censuslineagepilot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"lsp-trace/internal/lsp"
	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/publication"
	"lsp-trace/sessionruntime"
)

// Arm the existing one-over wire-message fault only after discovery. The four
// nomination prepares themselves succeed; the first acquisition request fails.
// The shared provider-budget fixture uses 64 (at cap), not an actual overflow.
func ArmScopedPartialAcquisitionBudgetFailurePrivate(f *ScopedBatchFixturePrivate) {
	f.Fault("rehearsal-empty-directions")
	f.child.mu.Lock()
	response := f.child.response
	prepares := 0
	f.child.response = func(msg lspwire.Message) json.RawMessage {
		if msg.Method == "textDocument/prepareCallHierarchy" {
			prepares++
			if prepares == 4 {
				f.child.mu.Lock()
				f.child.notifications = 65
				f.child.mu.Unlock()
			}
		}
		return response(msg)
	}
	f.child.mu.Unlock()
}

// Reuse the reviewed exact table. Noncallable symbols preserve its original
// census ordinals without adding targets, preparation requests or seed inputs.
func NewScopedPartialExact25RehearsalFixturePrivate(t *testing.T, faults ...string) (*ScopedRehearsalPrivate, *sessionruntime.Manager, ScopedRehearsalConfigPrivate, *ScopedBatchFixturePrivate) {
	t.Helper()
	h, m, cfg, f := newScopedRehearsalFixturePrivate(t, "lsp-trace.scoped-rehearsal.private.v3", func(root string) {
		for _, path := range fixtureScopePaths {
			if e := os.WriteFile(filepath.Join(root, path), []byte("package fixture\n"+strings.Repeat("// deterministic owned mock padding ................................\n", 301)), 0600); e != nil {
				t.Fatal(e)
			}
		}
	})
	uri := func(path string) string {
		return (&url.URL{Scheme: "file", Path: filepath.Join(cfg.Workspace, path)}).String()
	}
	contextURI := uri(fixtureScopePaths[5])
	outside := uri("outside.go")
	if len(faults) > 0 && faults[0] == "outside-workspace" {
		// Lexical URI only: never create or read this outside-workspace path.
		outside = (&url.URL{Scheme: "file", Path: filepath.Join(filepath.Dir(cfg.Workspace), "uncaptured-outside.go")}).String()
	}
	f.ContextURI = contextURI
	f.child.mu.Lock()
	f.child.response = func(msg lspwire.Message) json.RawMessage {
		var p struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
			Position lsp.Position          `json:"position"`
			Item     lsp.CallHierarchyItem `json:"item"`
		}
		if e := json.Unmarshal(msg.Params, &p); e != nil {
			t.Error(e)
		}
		var value any = []any{}
		switch msg.Method {
		case "textDocument/documentSymbol":
			var symbols []lsp.DocumentSymbol
			// Fixed file ordinal intervals follow the table's sparse historical labels.
			starts, ends := []int{0, 9, 24, 214}, []int{9, 24, 214, 219}
			for file, path := range fixtureScopePaths[:4] {
				if p.TextDocument.URI != uri(path) {
					continue
				}
				for ordinal := starts[file]; ordinal < ends[file]; ordinal++ {
					name, kind, line, ch := fmt.Sprintf("noncallable-%06d", ordinal), 13, uint32(300), uint32(0)
					for _, row := range partialInspectionExact25 {
						if row.path != path {
							continue
						}
						wanted := 0
						if row.id != "root" {
							wanted, _ = strconv.Atoi(strings.TrimPrefix(row.id, "target-"))
						}
						if ordinal == wanted {
							name, kind, line, ch = row.id, 12, row.line, row.character
							break
						}
						if ordinal < wanted {
							line, ch = row.line, 0
							break
						}
					}
					selection := lsp.Range{Start: lsp.Position{Line: line, Character: ch}, End: lsp.Position{Line: line, Character: ch + 1}}
					symbols = append(symbols, lsp.DocumentSymbol{Name: name, Kind: kind, Range: lsp.Range{Start: lsp.Position{Line: line}, End: lsp.Position{Line: line, Character: 64}}, SelectionRange: selection})
				}
			}
			value = symbols
		case "textDocument/prepareCallHierarchy":
			for _, row := range partialInspectionExact25 {
				if p.TextDocument.URI == uri(row.path) && p.Position.Line == row.line && p.Position.Character == row.character {
					item := scopeAdapterItem(p.TextDocument.URI)
					item.Name = row.id
					item.Range = lsp.Range{Start: lsp.Position{Line: row.line}, End: lsp.Position{Line: row.line, Character: 64}}
					item.SelectionRange = lsp.Range{Start: p.Position, End: lsp.Position{Line: row.line, Character: row.character + 1}}
					value = []lsp.CallHierarchyItem{item}
					break
				}
			}
		case "callHierarchy/outgoingCalls":
			if p.Item.Name == "root" {
				if len(faults) > 0 && faults[0] == "source-alias" {
					allowed := filepath.Join(cfg.Workspace, fixtureScopePaths[5])
					outsideFile := filepath.Join(cfg.Workspace, "alias-outside.go")
					if e := os.WriteFile(outsideFile, []byte("forbidden alias source"), 0600); e != nil {
						t.Fatal(e)
					}
					if e := os.Rename(allowed, allowed+"-saved"); e != nil {
						t.Fatal(e)
					}
					if e := os.Symlink(outsideFile, allowed); e != nil {
						t.Fatal(e)
					}
				}
				sites := []lsp.Range{{Start: lsp.Position{}, End: lsp.Position{Character: 1}}}
				b := lsp.CallHierarchyOutgoingCall{To: scopeAdapterItem(outside), FromRanges: sites}
				value = []lsp.CallHierarchyOutgoingCall{b, b, {To: scopeAdapterItem(contextURI), FromRanges: sites}}
			}
		case "callHierarchy/incomingCalls":
			if p.Item.URI == contextURI {
				value = []lsp.CallHierarchyIncomingCall{{From: scopeAdapterItem(contextURI), FromRanges: []lsp.Range{{Start: lsp.Position{}, End: lsp.Position{Character: 1}}}}}
			}
		}
		raw, e := json.Marshal(value)
		if e != nil {
			t.Error(e)
		}
		return raw
	}
	f.child.mu.Unlock()
	if len(faults) > 0 && strings.HasPrefix(faults[0], "backup-") {
		kind := faults[0]
		ops := defaultScopeRetentionOps()
		read := ops.read
		ops.read = func(ctx context.Context, r *publication.ContentRetentionRoot, id string, n int64) ([]byte, error) {
			raw, e := read(ctx, r, id, n)
			if r == h.roots[1] && len(h.records) >= 5 && e == nil {
				switch kind {
				case "backup-read":
					return nil, errors.New("backup unavailable")
				case "backup-hash":
					return []byte("corrupt"), nil
				case "backup-close":
					return raw, errors.New("read close uncertain")
				}
			}
			return raw, e
		}
		h.retentionOps = ops
	}
	return h, m, cfg, f
}
