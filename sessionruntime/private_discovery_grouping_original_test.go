package sessionruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/runtimeprofile"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPrivateDiscoveryGroupingTransfer(t *testing.T) {
	for _, mode := range []string{"owned", "withheld-symbol", "withheld-method", "withheld-supply", "corrupt-source", "cancelled-consumer"} {
		t.Run(mode, func(t *testing.T) {
			root, e := filepath.EvalSymlinks(t.TempDir())
			if e != nil {
				t.Fatal(e)
			}
			body := []byte("package x\nfunc X() {}\n")
			path := filepath.Join(root, "x.go")
			if e := os.WriteFile(path, body, 0600); e != nil {
				t.Fatal(e)
			}
			uri := (&url.URL{Scheme: "file", Path: path}).String()
			profile, e := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "test", Workspace: root, Profile: "go", EnvironmentReference: "local"})
			if e != nil {
				t.Fatal(e)
			}
			pipe := newOwnedDiscoveryPipe(uri, "grouping")
			c := PrivateDiscoveryConstruction{CaptureOwnedMethods: true, NewStarter: func() (Starter, error) { return oneChildStarter{pipe}, nil }, Start: StartRequest{Profile: runtimeprofile.Resolve(profile), LanguageID: "go"}, Deadline: time.Now().Add(10 * time.Second), Limits: discoveryLimitsTest(), Members: []PrivateDiscoveryMember{{Path: "x.go", URI: uri, LanguageID: "go"}}}
			observation, e := RunPrivateDiscovery(context.Background(), c, func(ctx context.Context, s *PrivateDiscoverySession) error {
				if e := s.Supply(ctx, uri, "go"); e != nil {
					return e
				}
				if _, e := s.DocumentSymbols(ctx, lsp.DocumentSymbolParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}}); e != nil {
					return e
				}
				_, e := s.References(ctx, uri, lsp.Position{Line: 1, Character: 5})
				return e
			})
			if e != nil || !observation.CleanupJoined || !observation.OwnedMethods.Available() {
				t.Fatal("capture", e)
			}
			owned := observation.OwnedMethods
			index := -1
			for i := 0; i < owned.GroupingQueryCount(); i++ {
				q, ok := owned.GroupingQuery(i)
				if ok && q.Method == "textDocument/references" {
					index = i
				}
			}
			if index < 0 {
				t.Fatal("missing actual query")
			}
			switch mode {
			case "withheld-symbol":
				owned.state.pairs[0] = OwnedMethodPair{}
			case "withheld-method":
				owned.state.pairs[index] = OwnedMethodPair{}
			case "withheld-supply":
				delete(owned.state.sources, uri)
			case "corrupt-source":
				owned.state.sources[uri].Content[0] ^= 1
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			evidence, e := owned.ConsumeDiscoveryGroupingOwned(ctx, index, "actual-reference", "revision", "private", func(v PrivateGroupingOwnedView) error {
				if mode == "cancelled-consumer" {
					cancel()
					return nil
				}
				v.Source[0] ^= 1
				v.Symbol.Params[0] ^= 1
				v.Methods[0].Result[0] ^= 1
				return nil
			})
			if mode != "owned" {
				if e == nil || evidence.SourceByteLength() != 0 {
					t.Fatal("ASSERT_RETAINED_GROUPING_WITHHOLDS: source escaped", mode, e)
				}
				t.Log("ASSERT_RETAINED_GROUPING_WITHHOLDS: PASS", mode)
			} else {
				if e != nil {
					t.Fatal("transfer", e)
				}
				v, e := evidence.Snapshot(PrivateGroupingQueryMaxBytes)
				if e != nil || !bytes.Equal(v.Source, body) || !json.Valid(v.Symbol.Params) || !json.Valid(v.Methods[0].Result) || v.Query.Line != 1 || v.Query.Character != 5 {
					t.Fatal("ASSERT_RETAINED_GROUPING_ORIGINAL: clone/source/query", e)
				}
				t.Log("ASSERT_RETAINED_GROUPING_ORIGINAL: PASS actual source, query, defensive copies")
			}
			if _, e := owned.ConsumeDiscoveryGroupingOwned(context.Background(), index, "actual-reference", "revision", "private", func(PrivateGroupingOwnedView) error { return nil }); e == nil {
				t.Fatal("ASSERT_RETAINED_GROUPING_REPLAY")
			}
			pipe.mu.Lock()
			requests := len(pipe.methods)
			pipe.mu.Unlock()
			if requests != 2 {
				t.Fatal("ASSERT_RETAINED_GROUPING_RECAPTURE", requests)
			}
		})
	}
}
