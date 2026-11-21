package sessionruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	qt "lsp-trace/internal/adr0011querytarget"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/runtimeprofile"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPrivateDiscoveryDefinitionOccurrence(t *testing.T) {
	for _, mode := range []string{"owned", "outside-body", "missing-target", "wrong-target-uri", "wrong-query-uri", "ambiguous-container", "wrong-native", "ambiguous-native", "wrong-native-source", "wrong-native-version", "withheld-symbol", "withheld-method"} {
		t.Run(mode, func(t *testing.T) {
			root, e := filepath.EvalSymlinks(t.TempDir())
			if e != nil {
				t.Fatal(e)
			}
			body := []byte("package x\nfunc X() {}\n")
			path := filepath.Join(root, "x.go")
			if e = os.WriteFile(path, body, 0600); e != nil {
				t.Fatal(e)
			}
			uri := (&url.URL{Scheme: "file", Path: path}).String()
			profile, e := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "test", Workspace: root, Profile: "go", EnvironmentReference: "local"})
			if e != nil {
				t.Fatal(e)
			}
			pipe := newOwnedDiscoveryPipe(uri, "grouping")
			c := PrivateDiscoveryConstruction{CaptureOwnedMethods: true, CaptureCallDirections: PrivateCallsIncoming | PrivateCallsOutgoing, NewStarter: func() (Starter, error) { return oneChildStarter{pipe}, nil }, Start: StartRequest{Profile: runtimeprofile.Resolve(profile), LanguageID: "go"}, Deadline: time.Now().Add(10 * time.Second), Limits: discoveryLimitsTest(), Members: []PrivateDiscoveryMember{{Path: "x.go", URI: uri, LanguageID: "go"}}}
			position := lsp.Position{Line: 1, Character: 9}
			if mode == "outside-body" {
				position.Character = 11
			}
			observation, e := RunPrivateDiscovery(context.Background(), c, func(ctx context.Context, s *PrivateDiscoverySession) error {
				if e := s.Supply(ctx, uri, "go"); e != nil {
					return e
				}
				if _, e := s.DocumentSymbols(ctx, lsp.DocumentSymbolParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}}); e != nil {
					return e
				}
				if _, e := s.Definition(ctx, uri, position); e != nil {
					return e
				}
				_, e := s.PrepareCallHierarchy(ctx, lsp.PrepareCallHierarchyParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}, Position: lsp.Position{Line: 1, Character: 5}})
				return e
			})
			if e != nil || !observation.CleanupJoined {
				t.Fatal("capture", e)
			}
			owned := observation.OwnedMethods
			index := -1
			for i := 0; i < owned.GroupingQueryCount(); i++ {
				q, ok := owned.GroupingQuery(i)
				if ok && q.Method == "textDocument/definition" {
					index = i
				}
			}
			if index < 0 {
				t.Fatal("definition missing")
			}
			original := clonePrivateGroupingPair(owned.state.pairs[index])
			switch mode {
			case "missing-target":
				owned.state.pairs[index].Result = []byte(`[]`)
			case "wrong-target-uri":
				owned.state.pairs[index].Result = bytes.ReplaceAll(original.Result, []byte(uri), []byte("file:///unretained/target"))
			case "wrong-query-uri":
				owned.state.pairs[index].Params = bytes.ReplaceAll(original.Params, []byte(uri), []byte("file:///wrong/query"))
			case "withheld-symbol":
				owned.state.pairs[0] = OwnedMethodPair{}
			case "withheld-method":
				owned.state.pairs[index] = OwnedMethodPair{}
			case "ambiguous-container":
				raw := owned.state.pairs[0].Result
				owned.state.pairs[0].Result = append(append(append([]byte{}, raw[:len(raw)-1]...), ','), raw[1:]...)
			case "wrong-native-source":
				owned.state.calls[0].view.Supply.Content = []byte("different retained source")
			case "wrong-native-version":
				owned.state.calls[0].view.Supply.DocumentVersion++
			case "wrong-native", "ambiguous-native":
				var items []lsp.CallHierarchyItem
				if e = json.Unmarshal(owned.state.calls[0].view.Requests[0].Result, &items); e != nil {
					t.Fatal(e)
				}
				if mode == "wrong-native" {
					items[0].Range.End.Character--
				} else {
					items = append(items, items[0])
				}
				owned.state.calls[0].view.Requests[0].Result, _ = json.Marshal(items)
			}
			evidence, e := owned.ConsumeDiscoveryGroupingOwned(context.Background(), index, "occurrence", "revision", "private", func(v PrivateGroupingOwnedView) error {
				if !owned.MatchesGroupingNativeV1(0, v) {
					return fmt.Errorf("native association refused")
				}
				if v.Query.Line != position.Line || v.Query.Character != position.Character || !bytes.Equal(v.Methods[0].Params, original.Params) || !bytes.Equal(v.Methods[0].Result, original.Result) || qt.DefinitionQueryNodeV1(v.Query, v.Target) == v.Target.SymbolID {
					t.Fatal("ASSERT_OCCURRENCE_ORIGINAL_IDENTITIES")
				}
				return nil
			})
			if mode == "owned" {
				if e != nil {
					t.Fatal("ASSERT_RETAINED_DEFINITION_OCCURRENCE", e)
				}
				view, e := evidence.Snapshot(PrivateGroupingQueryMaxBytes)
				if e != nil || !bytes.Equal(view.Source, body) {
					t.Fatal("source", e)
				}
				t.Log("ASSERT_RETAINED_DEFINITION_OCCURRENCE: PASS")
			} else {
				if e == nil || evidence.SourceByteLength() != 0 {
					t.Fatal("ASSERT_OCCURRENCE_REFUSAL", mode)
				}
				t.Log("ASSERT_OCCURRENCE_REFUSAL: PASS", mode)
			}
			if _, e := owned.ConsumeDiscoveryGroupingOwned(context.Background(), index, "occurrence", "revision", "private", func(PrivateGroupingOwnedView) error { return nil }); e == nil {
				t.Fatal("ASSERT_OCCURRENCE_REPLAY_REFUSED")
			}
			pipe.mu.Lock()
			requests := len(pipe.methods)
			pipe.mu.Unlock()
			if requests != 5 {
				t.Fatal("ASSERT_OCCURRENCE_NO_REQUERY", requests)
			}
			t.Log("ASSERT_OCCURRENCE_NO_REQUERY: PASS; spent slot refuses")
		})
	}
}
