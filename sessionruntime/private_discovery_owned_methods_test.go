package sessionruntime

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"lsp-trace/internal/lsp"
	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/managedprocess"
	"lsp-trace/internal/runtimeprofile"
)

type ownedDiscoveryPipe struct {
	*discoveryPipeChild
	done    chan struct{}
	mu      sync.Mutex
	methods []string
	mode    string
}

func (c *ownedDiscoveryPipe) Teardown(ctx context.Context) managedprocess.TeardownObservation {
	o := c.discoveryPipeChild.Teardown(ctx)
	select {
	case <-c.done:
	case <-ctx.Done():
		o.Census.Bounded = false
	}
	if c.mode == "bad-cleanup" {
		o.Census.Bounded = false
	}
	return o
}
func newOwnedDiscoveryPipe(uri, mode string) *ownedDiscoveryPipe {
	input, stdin := io.Pipe()
	stdout, output := io.Pipe()
	c := &ownedDiscoveryPipe{discoveryPipeChild: &discoveryPipeChild{input: input, stdin: stdin, stdout: stdout, output: output}, done: make(chan struct{}), mode: mode}
	go func() {
		defer close(c.done)
		reader := lspwire.NewReader(input, lspwire.DefaultLimits())
		writer := lspwire.NewWriter(output, lspwire.DefaultLimits())
		for {
			m, err := reader.Read()
			if err != nil || m.Method == "exit" {
				return
			}
			if len(m.ID) == 0 {
				continue
			}
			result := json.RawMessage(`[]`)
			if m.Method == "initialize" {
				result = json.RawMessage(`{"serverInfo":{"name":"owned-method-fixture","version":"1"},"capabilities":{"positionEncoding":"utf-16","documentSymbolProvider":true,"definitionProvider":true,"referencesProvider":true,"callHierarchyProvider":true}}`)
			} else if m.Method == "shutdown" {
				result = json.RawMessage(`null`)
			} else {
				c.mu.Lock()
				c.methods = append(c.methods, m.Method)
				c.mu.Unlock()
				if m.Method == "textDocument/definition" || m.Method == "textDocument/references" {
					result, _ = json.Marshal([]lsp.Location{{URI: uri, Range: &lsp.Range{Start: lsp.Position{Line: 1, Character: 5}, End: lsp.Position{Line: 1, Character: 6}}}})
				}
			}
			item := lsp.CallHierarchyItem{Name: "X", Kind: 12, URI: uri, Range: lsp.Range{Start: lsp.Position{Line: 1}, End: lsp.Position{Line: 1, Character: 11}}, SelectionRange: lsp.Range{Start: lsp.Position{Line: 1, Character: 5}, End: lsp.Position{Line: 1, Character: 6}}}
			switch m.Method {
			case "textDocument/documentSymbol":
				if mode == "grouping" {
					result, _ = json.Marshal([]lsp.DocumentSymbol{{Name: item.Name, Kind: item.Kind, Range: item.Range, SelectionRange: item.SelectionRange}})
				}
			case "textDocument/prepareCallHierarchy":
				result, _ = json.Marshal([]lsp.CallHierarchyItem{item})
			case "callHierarchy/incomingCalls":
				result, _ = json.Marshal([]lsp.CallHierarchyIncomingCall{{From: item, FromRanges: []lsp.Range{item.SelectionRange}}})
			case "callHierarchy/outgoingCalls":
				result, _ = json.Marshal([]lsp.CallHierarchyOutgoingCall{{To: item, FromRanges: []lsp.Range{item.SelectionRange}}})
			}
			response := lspwire.Message{JSONRPC: lspwire.Version, ID: m.ID, Result: result}
			if mode == "withhold-definition" && m.Method == "textDocument/definition" || mode == "withhold-calls" && m.Method == "callHierarchy/outgoingCalls" {
				response.Result = nil
				response.Error = &lspwire.RPCError{Code: -32603, Message: "controlled withholding"}
			}
			if writer.Write(response) != nil {
				return
			}
		}
	}()
	return c
}

func TestPrivateDiscoveryOwnedMethodsSameRequests(t *testing.T) {
	for _, mode := range []string{"owned", "unretained", "withhold-definition", "corrupt-source", "bad-cleanup", "retention-underfunded"} {
		t.Run(mode, func(t *testing.T) {
			root, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			body := []byte("package x\nfunc X() {}\n")
			path := filepath.Join(root, "x.go")
			if err := os.WriteFile(path, body, 0600); err != nil {
				t.Fatal(err)
			}
			uri := (&url.URL{Scheme: "file", Path: path}).String()
			profile, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "test", Workspace: root, Profile: "go", EnvironmentReference: "local"})
			if err != nil {
				t.Fatal(err)
			}
			pipe := newOwnedDiscoveryPipe(uri, mode)
			starts := 0
			c := PrivateDiscoveryConstruction{CaptureOwnedMethods: mode != "unretained", NewStarter: func() (Starter, error) { starts++; return oneChildStarter{pipe}, nil }, Start: StartRequest{Profile: runtimeprofile.Resolve(profile), LanguageID: "go"}, Deadline: time.Now().Add(10 * time.Second), Limits: discoveryLimitsTest(), Members: []PrivateDiscoveryMember{{Path: "x.go", URI: uri, LanguageID: "go"}}}
			if mode == "retention-underfunded" {
				c.Limits.RetainedBytes = 32 << 20
			}
			observation, err := RunPrivateDiscovery(context.Background(), c, func(ctx context.Context, s *PrivateDiscoverySession) error {
				if err := s.Supply(ctx, uri, "go"); err != nil {
					return err
				}
				if _, err := s.DocumentSymbols(ctx, lsp.DocumentSymbolParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}}); err != nil {
					return err
				}
				if _, err := s.References(ctx, uri, lsp.Position{Line: 1, Character: 5}); err != nil {
					return err
				}
				if mode == "corrupt-source" {
					// Test-only fault: the Manager's actual supplied-document binding
					// must reject a changed retained source before the next wire effect.
					s.client.w.ownedMethods.sources[uri].Content = []byte("not the supplied document")
				}
				_, err := s.Definition(ctx, uri, lsp.Position{Line: 1, Character: 5})
				return err
			})
			pipe.mu.Lock()
			methods := append([]string(nil), pipe.methods...)
			pipe.mu.Unlock()
			if starts != 1 {
				t.Fatal("ASSERT_OWNED_SAME_ACQUISITION: duplicate construction", starts)
			}
			if mode == "owned" || mode == "unretained" {
				if err != nil || observation.Failed || !observation.CleanupJoined || len(methods) != 3 {
					t.Fatal("ASSERT_OWNED_SAME_ACQUISITION: failed original requests", err, methods)
				}
				if observation.OwnedMethods.Available() != (mode == "owned") {
					t.Fatal("ASSERT_OWNED_RETENTION: wrong opt-in outcome")
				}
				if mode == "owned" {
					originals := observation.OwnedMethods.state
					if originals.workspace != root || originals.metadata.ProviderName != "owned-method-fixture" || originals.metadata.ProviderVersion != "1" || originals.metadata.PositionEncoding != "utf-16" {
						t.Fatal("ASSERT_OWNED_RETENTION: runtime metadata lost")
					}
					if len(originals.pairs) != 3 || len(originals.sources) != 1 {
						t.Fatal("ASSERT_OWNED_RETENTION: lost originals")
					}
					for i, pair := range originals.pairs {
						if pair.SessionID != originals.sessionID || pair.Generation != originals.generation || pair.Method != methods[i] || pair.Source == nil || pair.Source.URI != uri || pair.Source.SHA256 != fmt.Sprintf("sha256:%x", sha256.Sum256(body)) || pair.Key.ID == 0 || pair.Key.Generation != pair.Generation {
							t.Fatal("ASSERT_OWNED_RETENTION: source/key mismatch", pair)
						}
					}
					t.Log("ASSERT_OWNED_RETENTION: PASS original symbol/D/R with exact source and request keys")
				}
			} else {
				if err == nil || observation.OwnedMethods.Available() {
					t.Fatal("ASSERT_OWNED_FAILURE_WITHHOLDS: originals escaped", err)
				}
				if mode == "corrupt-source" && len(methods) != 2 {
					t.Fatal("ASSERT_OWNED_SOURCE_BINDING: corrupted source reached wire", methods)
				}
				if mode == "retention-underfunded" && len(methods) != 1 {
					t.Fatal("ASSERT_OWNED_PREFUND: effect before retention debit", methods)
				}
				t.Log("ASSERT_OWNED_FAILURE_WITHHOLDS: PASS", mode)
			}
			t.Log("ASSERT_OWNED_SAME_ACQUISITION: PASS", methods)
		})
	}
}
