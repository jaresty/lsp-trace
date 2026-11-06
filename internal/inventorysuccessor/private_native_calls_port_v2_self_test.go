package inventorysuccessor_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"lsp-trace/internal/inventorysuccessor"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/lspwire"
)

// Unexported test-only marker, never a production authorization or provider.
func TestPrivateNativeConstructorSELFResponderV2(t *testing.T) {
	index := -1
	for i, arg := range os.Args {
		if arg == "native-constructor-self-v2" {
			index = i
			break
		}
	}
	if index < 0 {
		return
	}
	if len(os.Args) != index+3 {
		os.Exit(71)
	}
	workspace := os.Args[index+1]
	asymmetric := os.Args[index+2] == "true"
	uri := func(name string) string {
		return (&url.URL{Scheme: "file", Path: filepath.Join(workspace, name)}).String()
	}
	item := func(name, u string) lsp.CallHierarchyItem {
		return lsp.CallHierarchyItem{Name: name, URI: u, Kind: 12, Range: lsp.Range{Start: lsp.Position{Line: 1}, End: lsp.Position{Line: 1, Character: 21}}, SelectionRange: lsp.Range{Start: lsp.Position{Line: 1, Character: 5}, End: lsp.Position{Line: 1, Character: 9}}}
	}
	root := item("Root", uri("root.go"))
	caller, callee := root, root
	if asymmetric {
		caller = item("Caller", uri("caller.go"))
		callee = item("Callee", uri("callee.go"))
	}
	site := lsp.Range{Start: lsp.Position{Line: 1, Character: 13}, End: lsp.Position{Line: 1, Character: 17}}
	reader := lspwire.NewReader(os.Stdin, lspwire.DefaultLimits())
	for {
		msg, err := reader.Read()
		if err != nil {
			os.Exit(0)
		}
		if msg.Method == "exit" {
			os.Exit(0)
		}
		if len(msg.ID) == 0 {
			continue
		}
		var result any
		switch msg.Method {
		case "initialize":
			result = map[string]any{"serverInfo": map[string]string{"name": "controlled-SELF", "version": "native-constructor-v2"}, "capabilities": map[string]any{"positionEncoding": "utf-16", "callHierarchyProvider": true, "documentSymbolProvider": true}}
		case "textDocument/prepareCallHierarchy":
			result = []lsp.CallHierarchyItem{root}
		case "callHierarchy/incomingCalls":
			result = []lsp.CallHierarchyIncomingCall{{From: caller, FromRanges: []lsp.Range{site, site}}}
		case "callHierarchy/outgoingCalls":
			result = []lsp.CallHierarchyOutgoingCall{{To: callee, FromRanges: []lsp.Range{site, site}}}
		}
		raw, _ := json.Marshal(result)
		body := fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"result":%s}`, msg.ID, raw)
		if _, err = fmt.Fprintf(os.Stdout, "Content-Length: %d\r\n\r\n%s", len(body), body); err != nil {
			os.Exit(72)
		}
	}
}

func TestPrivateNativeConstructorDiscriminatorManagedSELF(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("local Darwin controlled SELF only; Linux is compile-only")
	}
	for _, asymmetric := range []bool{false, true} {
		t.Run(fmt.Sprint(asymmetric), func(t *testing.T) {
			m, q, _ := nativeConstructorFixtureV2(t, asymmetric, true)
			var got inventorysuccessor.PrivateNativeCallsResultV2
			err := m.WithReadySessionLease(context.Background(), q.Session, func(ctx context.Context) error {
				var e error
				got, e = inventorysuccessor.ConstructNativeCallsPrivateV2(ctx, m, q)
				return e
			})
			want := 1
			if asymmetric {
				want = 2
			}
			if err != nil || !got.Admission.Admission.Valid() || len(got.RawJoins) != 4 || len(got.Admission.Admission.Occurrences()) != want || len(got.Observations) != 3 {
				t.Fatalf("ASSERT_MANAGED_SELF_%v: err=%v raw=%d", asymmetric, err, len(got.RawJoins))
			}
			for i, o := range got.Observations {
				if o.Ordinal != i || o.Generation != q.Session.Generation || o.RequestID == 0 {
					t.Fatalf("ASSERT_MANAGED_SELF_ACTUAL_ORDINAL_%v: %+v", asymmetric, o)
				}
			}
			t.Logf("ASSERT_MANAGED_SELF_%v: PASS; LocalDarwinSupervisor -> ManagedStarter -> current test executable; raw=4 canonical=%d authority=0", asymmetric, want)
		})
	}
}
