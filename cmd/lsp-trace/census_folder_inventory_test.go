//go:build darwin && lsp_trace_controlled_describe_fixture

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	methodsyntax "lsp-trace/internal/adr0011methodsyntax"
	qt "lsp-trace/internal/adr0011querytarget"
	"lsp-trace/internal/inventorysuccessor"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/referencegrouping"
	"lsp-trace/sessionruntime"
)

// This guard retains the actual normal-folder wire checks and now requires the
// inventory continuation terminal as well. The latter is a standing RED until
// same-acquisition native admission and continuation are connected. A terminal
// alone is not full acceptance: exact admitted D/R/C, report conservation and
// publication readback assertions must accompany that connection.
// No runNativeCatalog override, live server, or production fixture hook is used.
func TestCensusFolderCrossFileInventory(t *testing.T) {
	runCensusFolderCrossFileInventory(t, nil, "BOTH", true)
}

func TestCensusFolderCrossFileRetainedReferences(t *testing.T) {
	runCensusFolderCrossFileInventory(t, nil, "REFERENCES", false)
}

func TestCensusFolderCrossFileSelectedOriginals(t *testing.T) {
	runCensusFolderCrossFileInventory(t, nil, "SELECTED", false)
}

func TestCensusFolderCrossFileOwnedAcquisition(t *testing.T) {
	runCensusFolderCrossFileInventory(t, nil, "BOTH", false)
}

// Automatic inventory is not a legacy limited-direction query. The explicit
// legacy runtime direction guards remain unchanged in sessionruntime.
func TestCensusFolderCrossFileInventoryScope(t *testing.T) {
	runCensusFolderCrossFileInventory(t, []string{"--down-depth", "0", "--up-depth", "1"}, "BOTH", false)
}

func runCensusFolderCrossFileInventory(t *testing.T, extra []string, callMethod string, requireInventory bool) {
	t.Helper()
	workspace := folderInventoryTemp(t)
	store := folderInventoryTemp(t)
	for name, body := range map[string]string{
		"go.mod":      "module controlled.example/checkout\n\ngo 1.26\n",
		"checkout.go": "package checkout\nfunc Checkout() int { return price() }\n",
		"price.go":    "package checkout\nfunc price() int { return 7 }\n",
	} {
		if err := os.WriteFile(filepath.Join(workspace, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	wire := newFolderInventoryWire(t, workspace)
	deps := productionCensusRunnerDependencies()
	// Only substitute process transport. Parsing, folder preparation, native
	// dispatch, Manager, and all downstream production dependencies remain real.
	deps.lookPath = func(string) (string, error) { return "/controlled/checkout-provider", nil }
	deps.newStarter = func() (sessionruntime.Starter, error) { return terminalStarter{child: wire}, nil }
	caller := folderCallerFixture(t, false, false)
	var out, diagnostics bytes.Buffer
	args := append([]string{workspace, "--publication-root", store, "--machine"}, extra...)
	code := runCensusWithDependencies(args, &out, &diagnostics, deps)
	callerResult := <-caller
	if code != 0 {
		t.Fatalf("ASSERT_FOLDER_INVENTORY_CONTINUATION: FAIL code=%d stdout=%q stderr=%q caller=%v", code, out.String(), diagnostics.String(), callerResult.err)
	}
	verifyFolderCallerPublication(t, callerResult)
	observed := wire.snapshot()
	t.Log("PROCEDURE: normal folder CLI -> real preparation/native dispatch -> controlled two-file LSP wire; no native-dispatch override")
	if err := folderInventoryEvidence(observed, wire.checkout, wire.price); err != nil {
		t.Fatalf("ASSERT_FOLDER_CROSS_FILE_ACQUISITION: FAIL: %v; code=%d stdout=%q stderr=%q", err, code, out.String(), diagnostics.String())
	}
	t.Log("ASSERT_FOLDER_CROSS_FILE_ACQUISITION: PASS")
	if err := folderInventoryBothOriginals(observed, wire.checkout, wire.price); err != nil {
		t.Errorf("ASSERT_FOLDER_BOTH_ORIGINALS: FAIL: %v", err)
	} else {
		t.Log("ASSERT_FOLDER_BOTH_ORIGINALS: PASS")
	}
	if err := folderInventoryNativeOriginals(store, observed, wire.checkout, wire.price); err != nil {
		t.Errorf("ASSERT_FOLDER_NATIVE_ORIGINALS: FAIL: %v", err)
	} else {
		t.Log("ASSERT_FOLDER_NATIVE_ORIGINALS: PASS")
	}
	if requireInventory || callMethod == "SELECTED" || callMethod == "REFERENCES" {
		full := requireInventory || callMethod == "SELECTED"
		assertion := "ASSERT_FOLDER_SELECTED_REFERENCES"
		if full {
			assertion = "ASSERT_FOLDER_SELECTED_ORIGINALS"
		}
		if err := folderInventorySelectedOriginals(store, observed, full); err != nil {
			t.Errorf("%s: FAIL: %v", assertion, err)
		} else {
			t.Log(assertion + ": PASS")
		}
	}
	if requireInventory {
		var terminal applicationInventoryTerminalRecordV3
		if code != 0 || json.Unmarshal(out.Bytes(), &terminal) != nil || terminal.Status != "PROVISIONAL" || terminal.InventoryHead == "" || terminal.Authority != 0 || terminal.Accepted || terminal.Completeness != "UNKNOWN" {
			t.Errorf("ASSERT_FOLDER_INVENTORY_CONTINUATION: FAIL code=%d stdout=%q stderr=%q", code, out.String(), diagnostics.String())
		} else {
			t.Log("ASSERT_FOLDER_INVENTORY_CONTINUATION: PASS")
		}
	}
	t.Log("ASSERT_FOLDER_ACQUISITION_CLOSED: PASS; controller joined within retained lifetime")
	heads, err := filepath.Glob(filepath.Join(store, "run-*", "folder-acquisition", "inventory-successor", "heads", "private", "v2", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	observedHeads := 0
	for _, path := range heads {
		raw, err := os.ReadFile(path)
		var h struct {
			Event       string `json:"event"`
			Disposition string `json:"disposition"`
			Windows     uint64 `json:"consumed_windows"`
			Charged     struct {
				Requests uint64 `json:"provider_requests"`
			} `json:"cumulative_charged"`
		}
		if err != nil || json.Unmarshal(raw, &h) != nil {
			t.Fatal("ASSERT_FOLDER_FRESH_EXECUTION_ACCOUNTING: readback", err)
		}
		if h.Event == "OBSERVE" && h.Disposition == "OBSERVED" {
			observedHeads++
			if h.Windows != 1 || h.Charged.Requests != 128 {
				t.Fatal("ASSERT_FOLDER_FRESH_EXECUTION_ACCOUNTING: changed ceiling/window", h)
			}
		}
	}
	if observedHeads != 1 {
		t.Fatal("ASSERT_FOLDER_FRESH_EXECUTION_ACCOUNTING: expected one separately completed execution", observedHeads)
	}
	t.Log("ASSERT_FOLDER_FRESH_EXECUTION_ACCOUNTING: PASS")
	// A committed acquisition is NOT inventory publication or command success.
}

func folderInventoryTemp(t *testing.T) string {
	t.Helper()
	p, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(p, 0700); err != nil {
		t.Fatal(err)
	}
	return p
}

type folderInventoryReply struct {
	method string
	params json.RawMessage
	result json.RawMessage
	id     string
}
type folderInventoryWire struct {
	*inventoryPipePrivate // existing owned pipe lifecycle, no external process
	mu                    sync.Mutex
	replies               []folderInventoryReply
	checkout, price       string
}

func newFolderInventoryWire(t *testing.T, workspace string) *folderInventoryWire {
	t.Helper()
	a, b := io.Pipe()
	c, d := io.Pipe()
	f := &folderInventoryWire{
		inventoryPipePrivate: &inventoryPipePrivate{in: a, input: b, out: c, output: d, done: make(chan struct{})},
		checkout:             (&url.URL{Scheme: "file", Path: filepath.Join(workspace, "checkout.go")}).String(),
		price:                (&url.URL{Scheme: "file", Path: filepath.Join(workspace, "price.go")}).String(),
	}
	go f.serveFolder()
	t.Cleanup(func() {
		f.Close()
		select {
		case <-f.done:
		case <-time.After(time.Second):
			t.Error("controlled folder provider did not join")
		}
	})
	return f
}

// This fixture has no OS process or descendants. Report bounded empty census
// only AFTER both pipe directions are closed and the actual serve goroutine joins.
func (f *folderInventoryWire) Teardown(ctx context.Context) managedprocess.TeardownObservation {
	f.Close()
	select {
	case <-f.done:
		return managedprocess.TeardownObservation{Death: managedprocess.DeathObservation{Kind: managedprocess.DeathExited, Reap: managedprocess.ReapObservation{Kind: managedprocess.ReapComplete}}, Census: managedprocess.GroupCensusObservation{Bounded: true}}
	case <-ctx.Done():
		return managedprocess.TeardownObservation{}
	}
}

func (f *folderInventoryWire) snapshot() []folderInventoryReply {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]folderInventoryReply(nil), f.replies...)
}

func (f *folderInventoryWire) serveFolder() {
	defer close(f.done)
	r := lspwire.NewReader(f.in, lspwire.DefaultLimits())
	w := lspwire.NewWriter(f.output, lspwire.DefaultLimits())
	// This is a finite fixture, not a provider budget override.
	for n := 0; n < 128; n++ {
		m, err := r.Read()
		if err != nil || m.Method == "exit" {
			return
		}
		if m.Method == "initialized" || strings.HasPrefix(m.Method, "textDocument/did") {
			continue
		}
		value, handled := folderInventoryResponse(m, f.checkout, f.price)
		if os.Getenv("DURABLE_INVENTORY_MODE") == "three-item" {
			extra := strings.Replace(f.checkout, "checkout.go", "extra.go", 1)
			extra2 := strings.Replace(f.checkout, "checkout.go", "extra2.go", 1)
			if bytes.Contains(m.Params, []byte(extra2)) {
				extra = extra2
			}
			// This controlled provider explicitly reports observed-empty references.
			// Three separate original definition queries remain; no completeness claim.
			if m.Method == "textDocument/references" {
				value = []any{}
			}
			name := "ExtraOne"
			if extra == extra2 {
				name = "ExtraTwo"
			}
			item := folderInventoryItem(extra, name)
			item.Range.End.Character = 38
			item = scaleFixtureItem(item)
			site := lsp.Range{Start: lsp.Position{Line: 1, Character: 29}, End: lsp.Position{Line: 1, Character: 34}}
			target := folderInventoryItem(f.price, "price")
			if bytes.Contains(m.Params, []byte(extra)) {
				handled = true
				switch m.Method {
				case "textDocument/documentSymbol":
					value = []lsp.DocumentSymbol{{Name: item.Name, Kind: item.Kind, Range: item.Range, SelectionRange: item.SelectionRange}}
				case "textDocument/prepareCallHierarchy":
					value = []lsp.CallHierarchyItem{item}
				case "textDocument/definition":
					var p struct {
						Position lsp.Position `json:"position"`
					}
					_ = json.Unmarshal(m.Params, &p)
					value = []lsp.Location{}
					if p.Position.Line == 1 && p.Position.Character >= 29 && p.Position.Character < 34 {
						value = []lsp.Location{{URI: f.price, Range: &target.SelectionRange}}
					}
				case "textDocument/references":
					value = []lsp.Location{}
				case "callHierarchy/incomingCalls":
					value = []map[string]any{}
				case "callHierarchy/outgoingCalls":
					value = []map[string]any{{"to": target, "fromRanges": []lsp.Range{site}}}
				default:
					handled = false
				}
			}
		}
		if !handled {
			if w.Write(lspwire.Message{JSONRPC: lspwire.Version, ID: m.ID, Error: &lspwire.RPCError{Code: -32601, Message: "unsupported controlled method"}}) != nil {
				return
			}
			continue
		}
		raw, err := json.Marshal(value)
		if err != nil {
			return
		}
		if w.Write(lspwire.Message{JSONRPC: lspwire.Version, ID: m.ID, Result: raw}) != nil {
			return
		}
		f.mu.Lock()
		f.replies = append(f.replies, folderInventoryReply{m.Method, append(json.RawMessage(nil), m.Params...), raw, string(m.ID)})
		f.mu.Unlock()
	}
}

func folderInventoryItem(uri, name string) lsp.CallHierarchyItem {
	end := uint32(28)
	if name == "Checkout" {
		end = 38
	}
	return scaleFixtureItem(lsp.CallHierarchyItem{Name: name, Kind: 12, URI: uri,
		Range:          lsp.Range{Start: lsp.Position{Line: 1}, End: lsp.Position{Line: 1, Character: end}},
		SelectionRange: lsp.Range{Start: lsp.Position{Line: 1, Character: 5}, End: lsp.Position{Line: 1, Character: uint32(5 + len(name))}}})
}

func folderInventoryCallsite() lsp.Range {
	return lsp.Range{Start: lsp.Position{Line: 1, Character: 29}, End: lsp.Position{Line: 1, Character: 34}}
}

func folderInventoryResponse(m lspwire.Message, checkout, price string) (any, bool) {
	var p struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
		Item     lsp.CallHierarchyItem `json:"item"`
		Position lsp.Position          `json:"position"`
	}
	if len(m.Params) > 0 && json.Unmarshal(m.Params, &p) != nil {
		return nil, false
	}
	caller, callee := folderInventoryItem(checkout, "Checkout"), folderInventoryItem(price, "price")
	callsite := folderInventoryCallsite()
	switch m.Method {
	case "initialize":
		return map[string]any{"serverInfo": map[string]string{"name": "gopls", "version": "controlled-folder"}, "capabilities": map[string]any{"positionEncoding": "utf-16", "documentSymbolProvider": true, "callHierarchyProvider": true, "definitionProvider": true, "referencesProvider": true}}, true
	case "shutdown":
		return nil, true
	case "textDocument/documentSymbol":
		i := caller
		if p.TextDocument.URI == price {
			i = callee
		} else if p.TextDocument.URI != checkout {
			return []any{}, true
		}
		return []lsp.DocumentSymbol{{Name: i.Name, Kind: i.Kind, Range: i.Range, SelectionRange: i.SelectionRange}}, true
	case "textDocument/prepareCallHierarchy":
		if p.TextDocument.URI == checkout {
			return []lsp.CallHierarchyItem{caller}, true
		}
		if p.TextDocument.URI == price {
			return []lsp.CallHierarchyItem{callee}, true
		}
		return []any{}, true
	case "textDocument/definition":
		if p.TextDocument.URI == checkout && p.Position.Line == 1 && p.Position.Character >= 29 && p.Position.Character < 34 {
			return []lsp.Location{{URI: price, Range: &callee.SelectionRange}}, true
		}
		return []any{}, true
	case "textDocument/references":
		if p.TextDocument.URI == price && p.Position.Line == 1 && p.Position.Character >= 5 && p.Position.Character < 10 {
			return []lsp.Location{{URI: checkout, Range: &callsite}}, true
		}
		return []any{}, true
	case "callHierarchy/outgoingCalls":
		if p.Item.URI == checkout {
			return []map[string]any{{"to": callee, "fromRanges": []lsp.Range{folderInventoryCallsite()}}}, true
		}
		return []any{}, true
	case "callHierarchy/incomingCalls":
		if p.Item.URI == price {
			return []map[string]any{{"from": caller, "fromRanges": []lsp.Range{folderInventoryCallsite()}}}, true
		}
		return []any{}, true
	default:
		return nil, false
	}
}

// This observation compares exact cross-file locations, not method names or
// successful JSON alone. An empty/wrong endpoint is not positive evidence.
func folderInventoryEvidence(replies []folderInventoryReply, checkout, price string) error {
	definition, reference, call := false, false, false
	for _, reply := range replies {
		var request struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
			Item     lsp.CallHierarchyItem `json:"item"`
			Position lsp.Position          `json:"position"`
		}
		if json.Unmarshal(reply.params, &request) != nil {
			continue
		}
		switch reply.method {
		case "textDocument/definition", "textDocument/references":
			var locations []lsp.Location
			if json.Unmarshal(reply.result, &locations) != nil {
				continue
			}
			for _, location := range locations {
				if reply.method == "textDocument/definition" && request.TextDocument.URI == checkout && request.Position.Line == 1 && request.Position.Character >= 29 && request.Position.Character < 34 && location.URI == price && location.Range != nil && *location.Range == folderInventoryItem(price, "price").SelectionRange {
					definition = true
				}
				if reply.method == "textDocument/references" && request.TextDocument.URI == price && request.Position.Line == 1 && request.Position.Character >= 5 && request.Position.Character < 10 && location.URI == checkout && location.Range != nil && *location.Range == folderInventoryCallsite() {
					reference = true
				}
			}
		case "callHierarchy/outgoingCalls", "callHierarchy/incomingCalls":
			var calls []struct {
				To     lsp.CallHierarchyItem `json:"to"`
				From   lsp.CallHierarchyItem `json:"from"`
				Ranges []lsp.Range           `json:"fromRanges"`
			}
			if json.Unmarshal(reply.result, &calls) != nil {
				continue
			}
			for _, c := range calls {
				endpoint := folderInventorySameEndpoint(request.Item, folderInventoryItem(checkout, "Checkout")) && folderInventorySameEndpoint(c.To, folderInventoryItem(price, "price"))
				if reply.method == "callHierarchy/incomingCalls" {
					endpoint = folderInventorySameEndpoint(request.Item, folderInventoryItem(price, "price")) && folderInventorySameEndpoint(c.From, folderInventoryItem(checkout, "Checkout"))
				}
				if endpoint && len(c.Ranges) == 1 && c.Ranges[0] == folderInventoryCallsite() {
					call = true
				}
			}
		}
	}
	if !definition || !reference || !call {
		return fmt.Errorf("definition=%t reference=%t call=%t (require exact Checkout->price cross-file evidence)", definition, reference, call)
	}
	return nil
}

func folderInventorySelectedOriginals(store string, wire []folderInventoryReply, full bool) error {
	paths, err := filepath.Glob(filepath.Join(store, "run-*", "folder-selected-*", "inventory-successor", "heads", "private", "v2", "*.json"))
	if err != nil {
		return err
	}
	complete := 0
	for _, path := range paths {
		raw, e := os.ReadFile(path)
		if e != nil {
			return e
		}
		var h struct {
			Event, Disposition string
			Windows            uint64 `json:"consumed_windows"`
			Charged            struct {
				Requests uint64 `json:"provider_requests"`
			} `json:"cumulative_charged"`
		}
		if json.Unmarshal(raw, &h) != nil {
			return fmt.Errorf("selected head decode")
		}
		if h.Event == "OBSERVE" && h.Disposition == "OBSERVED" && h.Windows == 3 {
			if h.Charged.Requests != 0 {
				return fmt.Errorf("selected recapture credits")
			}
			complete++
		}
	}
	// Both D and R now complete even when this invocation checks R custody.
	want := 2
	if complete != want {
		return fmt.Errorf("completed existing M/P/S schedules=%d want=%d", complete, want)
	}
	paths, err = filepath.Glob(filepath.Join(store, "run-*", "folder-selected-*", "adr0011-private-grouping-*.json"))
	if err != nil {
		return err
	}
	methods := map[string]bool{}
	for _, path := range paths {
		raw, e := os.ReadFile(path)
		if e != nil {
			return e
		}
		var q struct {
			Query   qt.Query
			Target  qt.Candidate
			Methods []struct {
				Method         string
				RequestID      uint64
				Params, Result []byte
			}
		}
		if json.Unmarshal(raw, &q) != nil {
			continue
		}
		for _, p := range q.Methods {
			if p.Method == "textDocument/definition" {
				var params struct {
					TextDocument struct{ URI string }
					Position     struct{ Line, Character uint32 }
				}
				if json.Unmarshal(p.Params, &params) != nil || q.Query.URI != params.TextDocument.URI || q.Query.Line != params.Position.Line || q.Query.Character != params.Position.Character || q.Query.Line != 1 || q.Query.Character != 29 || q.Target.Version != qt.OccurrenceCandidateVersionV1 || q.Target.SymbolName != "Checkout" || qt.DefinitionQueryNodeV1(q.Query, q.Target) == q.Target.SymbolID {
					return fmt.Errorf("selected definition conflated occurrence/container/target")
				}
				if err := folderInventoryDefinitionMapping(filepath.Dir(path), q.Query, q.Target, p.Result); err != nil {
					return err
				}
			}
			matched := false
			for _, r := range wire {
				if r.method == p.Method && r.id == fmt.Sprint(p.RequestID) && bytes.Equal(r.params, p.Params) && bytes.Equal(r.result, p.Result) {
					matched = true
				}
			}
			if !matched {
				return fmt.Errorf("selected method lost original custody: %s", p.Method)
			}
			methods[p.Method] = true
		}
	}
	if (full && !methods["textDocument/definition"]) || !methods["textDocument/references"] {
		return fmt.Errorf("admitted original requested methods missing")
	}
	return nil
}

// Three-lead counterpart to the original two-lead assertion. It checks the
// actual retained M/P/S heads, zero selected recapture credits, exact wire
// request/result identity and produced definition mappings; no native override.
func folderInventoryThreeOriginals(store string, wire []folderInventoryReply) error {
	paths, e := filepath.Glob(filepath.Join(store, "run-*", "folder-selected-*", "inventory-successor", "heads", "private", "v2", "*.json"))
	if e != nil {
		return e
	}
	complete := 0
	for _, path := range paths {
		raw, e := os.ReadFile(path)
		if e != nil {
			return e
		}
		var h struct {
			Event, Disposition string
			Windows            uint64 `json:"consumed_windows"`
			Charged            struct {
				Requests uint64 `json:"provider_requests"`
			} `json:"cumulative_charged"`
		}
		if json.Unmarshal(raw, &h) != nil {
			return fmt.Errorf("three selected head decode")
		}
		if h.Event == "OBSERVE" && h.Disposition == "OBSERVED" && h.Windows == 3 {
			if h.Charged.Requests != 0 {
				return fmt.Errorf("three selected recapture")
			}
			complete++
		}
	}
	if complete != 3 {
		return fmt.Errorf("three completed M/P/S schedules=%d", complete)
	}
	paths, e = filepath.Glob(filepath.Join(store, "run-*", "folder-selected-*", "adr0011-private-grouping-*.json"))
	if e != nil {
		return e
	}
	queries := map[string]bool{}
	for _, path := range paths {
		raw, e := os.ReadFile(path)
		if e != nil {
			return e
		}
		var q struct {
			Query   qt.Query
			Target  qt.Candidate
			Methods []struct {
				Method         string
				RequestID      uint64
				Params, Result []byte
			}
		}
		if json.Unmarshal(raw, &q) != nil {
			continue
		}
		for _, p := range q.Methods {
			matches := 0
			for _, r := range wire {
				if r.method == p.Method && r.id == fmt.Sprint(p.RequestID) && bytes.Equal(r.params, p.Params) && bytes.Equal(r.result, p.Result) {
					matches++
				}
			}
			if matches != 1 {
				return fmt.Errorf("three original wire identity count=%d method=%s", matches, p.Method)
			}
			if p.Method == "textDocument/definition" {
				var params struct {
					TextDocument struct{ URI string }
					Position     struct{ Line, Character uint32 }
				}
				if json.Unmarshal(p.Params, &params) != nil || q.Query.URI != params.TextDocument.URI || q.Query.Line != 1 || q.Query.Character != 29 || params.Position.Line != q.Query.Line || params.Position.Character != q.Query.Character || q.Target.Version != qt.OccurrenceCandidateVersionV1 || qt.DefinitionQueryNodeV1(q.Query, q.Target) == q.Target.SymbolID {
					return fmt.Errorf("three definition identity conflated")
				}
				switch q.Target.SymbolName {
				case "Checkout", "ExtraOne", "ExtraTwo":
				default:
					return fmt.Errorf("three unexpected container")
				}
				if e := folderInventoryDefinitionMapping(filepath.Dir(path), q.Query, q.Target, p.Result); e != nil {
					return e
				}
				queries[q.Query.URI] = true
			}
		}
	}
	if len(queries) != 3 {
		return fmt.Errorf("three retained original definition queries=%d", len(queries))
	}
	return nil
}

// Inspect actual produced mappings, independently of schedule completion.
func folderInventoryDefinitionMapping(dir string, query qt.Query, container qt.Candidate, result []byte) error {
	parsed, failure := methodsyntax.ParseRaw("textDocument/definition", result, 256)
	if failure != nil || len(parsed.Items) != 1 {
		return fmt.Errorf("definition result unavailable")
	}
	item := parsed.Items[0]
	raw, _ := json.Marshal(struct {
		URI         string
		Range       methodsyntax.Range
		TargetRange *methodsyntax.Range
	}{item.URI, item.Range, item.TargetRange})
	to := fmt.Sprintf("sha256:%x", sha256.Sum256(append(raw, '\n')))
	paths, err := filepath.Glob(filepath.Join(dir, "adr0011-private-grouping-*.json"))
	if err != nil {
		return err
	}
	found := 0
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var mapping struct {
			Family   string
			Mappings []struct{ Kind, From, To string }
		}
		if json.Unmarshal(raw, &mapping) != nil || mapping.Family != "METHOD_MAPPING_RECEIPT" {
			continue
		}
		for _, m := range mapping.Mappings {
			if m.Kind != "RESOLVES_TO_DEFINITION" {
				continue
			}
			if m.From == container.SymbolID || m.From != qt.DefinitionQueryNodeV1(query, container) || m.To != to || m.From == m.To {
				return fmt.Errorf("published definition mapping conflated occurrence/container/target")
			}
			found++
		}
	}
	if found != 1 {
		return fmt.Errorf("published definition mapping count=%d want=1", found)
	}
	return nil
}

func folderInventoryNativeOriginals(store string, wire []folderInventoryReply, uris ...string) error {
	paths, err := filepath.Glob(filepath.Join(store, "run-*", "folder-native-*", "adr0011-folder-native-transfer-*.json"))
	if err != nil || len(paths) != len(uris) {
		return fmt.Errorf("native transfer count=%d want=%d: %v", len(paths), len(uris), err)
	}
	seen := map[string]bool{}
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		var summary inventorysuccessor.PrivateRetainedNativeSummaryV1
		if err != nil || json.Unmarshal(raw, &summary) != nil || seen[summary.URI] {
			return fmt.Errorf("native summary unavailable/duplicate")
		}
		seen[summary.URI] = true
		composite, err := os.ReadFile(filepath.Join(filepath.Dir(path), summary.CompositeSelector))
		if err != nil {
			return err
		}
		proofPath := filepath.Join(filepath.Dir(path), summary.ProofSelector)
		proof, err := os.ReadFile(proofPath)
		if err != nil {
			return err
		}
		if err := referencegrouping.ReplayPrivateNativeEvidenceV2(proof, composite); err != nil {
			return fmt.Errorf("native original validation: %w", err)
		}
		for i, method := range []string{"textDocument/prepareCallHierarchy", "callHierarchy/incomingCalls", "callHierarchy/outgoingCalls"} {
			found := false
			for _, r := range wire {
				if r.id == fmt.Sprint(summary.OriginalRequestIDs[i]) && r.method == method {
					found = true
				}
			}
			if !found {
				return fmt.Errorf("native original does not match paid wire %s", method)
			}
		}
		// Independently corrupt the actual reopened artifact, not a canned report.
		var decoded referencegrouping.PrivateNativeEvidenceV2
		if json.Unmarshal(proof, &decoded) != nil {
			return fmt.Errorf("native proof decode")
		}
		decoded.Requests[1].RequestID++
		corrupt, err := json.Marshal(decoded)
		if err != nil {
			return err
		}
		if err := os.WriteFile(proofPath, corrupt, 0600); err != nil {
			return err
		}
		reopened, err := os.ReadFile(proofPath)
		rejected := err == nil && referencegrouping.ReplayPrivateNativeEvidenceV2(reopened, composite) != nil
		if err := os.WriteFile(proofPath, proof, 0600); err != nil {
			return err
		}
		if !rejected {
			return fmt.Errorf("corrupted original request readback accepted")
		}
		decoded.Requests = decoded.Requests[:1]
		withheld, err := json.Marshal(decoded)
		if err != nil || referencegrouping.ReplayPrivateNativeEvidenceV2(withheld, composite) == nil {
			return fmt.Errorf("withheld original directions accepted")
		}
		restored, err := os.ReadFile(proofPath)
		if err != nil || referencegrouping.ReplayPrivateNativeEvidenceV2(restored, composite) != nil {
			return fmt.Errorf("restored native proof invalid")
		}
	}
	for _, uri := range uris {
		if !seen[uri] {
			return fmt.Errorf("native URI missing")
		}
	}
	return nil
}

// Require the original prepare and BOTH direction requests for each discovered
// callable. Distinct wire IDs and one request per key reject recapture.
func folderInventoryBothOriginals(replies []folderInventoryReply, uris ...string) error {
	counts := map[string]int{}
	ids := map[string]bool{}
	for _, r := range replies {
		if r.method != "textDocument/prepareCallHierarchy" && r.method != "callHierarchy/incomingCalls" && r.method != "callHierarchy/outgoingCalls" {
			continue
		}
		var p struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
			Item lsp.CallHierarchyItem `json:"item"`
		}
		if json.Unmarshal(r.params, &p) != nil {
			return fmt.Errorf("invalid original params")
		}
		uri := p.Item.URI
		if r.method == "textDocument/prepareCallHierarchy" {
			uri = p.TextDocument.URI
		}
		if r.id == "" || ids[r.id] {
			return fmt.Errorf("missing or repeated original request ID")
		}
		ids[r.id] = true
		counts[uri+"|"+r.method]++
	}
	for _, uri := range uris {
		for _, method := range []string{"textDocument/prepareCallHierarchy", "callHierarchy/incomingCalls", "callHierarchy/outgoingCalls"} {
			if counts[uri+"|"+method] != 1 {
				return fmt.Errorf("%s %s count=%d; require one actual original", uri, method, counts[uri+"|"+method])
			}
		}
	}
	return nil
}

func folderInventorySameEndpoint(a, b lsp.CallHierarchyItem) bool {
	return a.URI == b.URI && a.Name == b.Name && a.Kind == b.Kind && a.Range == b.Range && a.SelectionRange == b.SelectionRange
}

// Qualification of the first-edge observation only. These are test-created
// wire values, never native admission or a fabricated production publication.
func TestCensusFolderCrossFileEvidenceGuard(t *testing.T) {
	wire := newFolderInventoryWire(t, folderInventoryTemp(t))
	checkout, price := wire.checkout, wire.price
	// Exercise the same wire responder the normal-route test supplies; the
	// positive control is not merely a call to its response-construction helper.
	timer := time.AfterFunc(3*time.Second, func() { wire.Close() })
	defer timer.Stop()
	writer := lspwire.NewWriter(wire.input, lspwire.DefaultLimits())
	reader := lspwire.NewReader(wire.out, lspwire.DefaultLimits())
	var good []folderInventoryReply
	for _, method := range []string{"textDocument/definition", "textDocument/references", "callHierarchy/outgoingCalls"} {
		uri, position := checkout, folderInventoryCallsite().Start
		if method == "textDocument/references" {
			uri, position = price, folderInventoryItem(price, "price").SelectionRange.Start
		}
		params, _ := json.Marshal(map[string]any{"textDocument": map[string]string{"uri": uri}, "position": position, "item": folderInventoryItem(checkout, "Checkout")})
		if err := writer.Write(lspwire.Message{JSONRPC: lspwire.Version, ID: json.RawMessage(`1`), Method: method, Params: params}); err != nil {
			t.Fatal("fixture request", err)
		}
		response, err := reader.Read()
		if err != nil || response.Error != nil || string(response.ID) != "1" {
			t.Fatal("fixture response", err, response.Error)
		}
		good = append(good, folderInventoryReply{method, params, response.Result, string(response.ID)})
	}
	if err := folderInventoryEvidence(good, checkout, price); err != nil {
		t.Fatal("ASSERT_FOLDER_EVIDENCE_GUARD: positive rejected", err)
	}
	t.Log("ASSERT_FOLDER_CROSS_FILE_ACQUISITION: PASS (wire-observation control only)")
	for n := range good {
		for _, mode := range []string{"withheld", "wrong-file", "wrong-query"} {
			t.Run(good[n].method+"/"+mode, func(t *testing.T) {
				bad := append([]folderInventoryReply(nil), good...)
				if mode == "withheld" {
					bad[n].result = json.RawMessage(`[]`)
				} else if mode == "wrong-query" {
					bad[n].params = bytes.ReplaceAll(bytes.ReplaceAll(good[n].params, []byte(price), []byte("file:///wrong/price.go")), []byte(checkout), []byte("file:///wrong/checkout.go"))
				} else {
					bad[n].result = bytes.ReplaceAll(bytes.ReplaceAll(good[n].result, []byte(price), []byte("file:///wrong/price.go")), []byte(checkout), []byte("file:///wrong/checkout.go"))
				}
				if err := folderInventoryEvidence(bad, checkout, price); err == nil {
					t.Fatal("ASSERT_FOLDER_EVIDENCE_GUARD: accepted withheld or incorrect cross-file evidence")
				} else {
					t.Log("ASSERT_FOLDER_CROSS_FILE_ACQUISITION: FAIL (expected controlled perturbation):", err)
				}
			})
		}
	}
}
