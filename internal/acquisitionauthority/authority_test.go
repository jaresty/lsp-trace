package acquisitionauthority

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"lsp-trace/internal/operation"
	"lsp-trace/internal/seedbinding"
	"lsp-trace/sessionruntime"
)

func testBinding(t *testing.T) (Binding, []byte) {
	t.Helper()
	workspace := t.TempDir()
	uri := (&url.URL{Scheme: "file", Path: filepath.Join(workspace, "a.go")}).String()
	input := map[string]any{
		"session_id": "s", "generation": uint64(7), "output_version": "lsp-trace.graph-provenance.v5",
		"seed_manifest": map[string]any{
			"schema_version": "lsp-trace.seed-manifest.v2", "coordinate_convention": "zero-based-session",
			"root":             map[string]any{"id": "root", "locator": map[string]any{"uri": uri, "line": uint32(0), "character": uint32(2)}, "down_depth": 0, "up_depth": 0},
			"required_targets": []any{},
		},
	}
	raw, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	seed := []byte(`{"schema_version":"lsp-trace.seeds.v2","coordinate_convention":"one-based","defaults":{"down_depth":0,"up_depth":0},"seeds":[{"type":"position","label":"root","path":"a.go","line":1,"column":3}]}`)
	return Binding{Operation: operation.Name("slice_v3"), Route: "explicit-trace", RequestID: "request-a", Workspace: workspace, SessionID: "s", Generation: 7, Input: raw}, seed
}

func mintTestSeed(t *testing.T, b Binding, seed []byte) SeedAuthority {
	t.Helper()
	a, err := MintSeedAuthority(b, seed, seedbinding.CallerAssertedLocal, false, "", true)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestSeedAuthorityRejectsMutationForgeryAndReplay(t *testing.T) {
	base, seed := testBinding(t)
	if _, err := ConsumeSeedAuthority(SeedAuthority{}, base); err == nil {
		t.Fatal("ASSERT_SEED_AUTHORITY_ZERO_VALUE_FORGERY_REJECTED")
	}
	mutations := map[string]func(*Binding){
		"operation":  func(b *Binding) { b.Operation = "incoming_v3" },
		"route":      func(b *Binding) { b.Route = "automatic-from-file" },
		"request":    func(b *Binding) { b.RequestID = "request-b" },
		"workspace":  func(b *Binding) { b.Workspace += "-other" },
		"session":    func(b *Binding) { b.SessionID = "other" },
		"generation": func(b *Binding) { b.Generation++ },
		"input":      func(b *Binding) { b.Input = append(bytes.Clone(b.Input), ' ') },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			a := mintTestSeed(t, base, seed)
			crossed := base
			mutate(&crossed)
			if _, err := ConsumeSeedAuthority(a, crossed); err == nil {
				t.Fatal("ASSERT_SEED_AUTHORITY_CROSS_BINDING_REJECTED")
			}
		})
	}
	a := mintTestSeed(t, base, seed)
	seed[0] = 'X'
	grant, err := ConsumeSeedAuthority(a, base)
	if err != nil || grant.Provenance != seedbinding.CallerAssertedLocal || !grant.RetainSeedSpec || grant.SeedSpec[0] != '{' {
		t.Fatalf("ASSERT_SEED_AUTHORITY_EXACT_DEEP_COPY_USE: grant=%+v err=%v", grant, err)
	}
	if _, err := ConsumeSeedAuthority(a, base); err == nil {
		t.Fatal("ASSERT_SEED_AUTHORITY_REPLAY_REJECTED")
	}
}

func TestSeedAuthorityConcurrentReplayAdmitsExactlyOnce(t *testing.T) {
	binding, seed := testBinding(t)
	a := mintTestSeed(t, binding, seed)
	var successes atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(copy SeedAuthority) {
			defer wg.Done()
			if _, err := ConsumeSeedAuthority(copy, binding); err == nil {
				successes.Add(1)
			}
		}(a)
	}
	wg.Wait()
	if successes.Load() != 1 {
		t.Fatalf("ASSERT_SEED_AUTHORITY_CONCURRENT_SINGLE_USE: successes=%d", successes.Load())
	}
}

type sourcePreparer struct{ document sessionruntime.DocumentResult }

func (p sourcePreparer) PrepareDocument(context.Context, sessionruntime.DocumentRequest) sessionruntime.DocumentResult {
	return p.document
}

func TestPreparedSourceCapabilityMutationAndConcurrentReplay(t *testing.T) {
	binding, _ := testBinding(t)
	uri := "file://" + filepath.Join(binding.Workspace, "a.go")
	req := sessionruntime.DocumentRequest{SessionID: "s", Generation: 7, URI: uri, LanguageID: "go", CaptureSupply: true}
	doc := sessionruntime.DocumentResult{URI: uri, LanguageID: "go", Version: 3, Supply: &sessionruntime.DocumentSupply{SessionID: "s", Generation: 7, URI: uri, DocumentVersion: 3, Method: "textDocument/didOpen", Content: []byte("package a\n"), Params: json.RawMessage(`{"textDocument":{"uri":"file:///a.go"}}`)}}
	_, capability, err := PrepareSource(context.Background(), sourcePreparer{document: doc}, req, binding)
	if err != nil {
		t.Fatal(err)
	}
	doc.Supply.Content[0] = 'X'
	crossed := binding
	crossed.RequestID = "other"
	if _, err := ConsumePreparedSource(capability, crossed); err == nil {
		t.Fatal("ASSERT_PREPARED_SOURCE_CROSS_BINDING_REJECTED")
	}
	var successes atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(copy PreparedSourceCapability) {
			defer wg.Done()
			got, consumeErr := ConsumePreparedSource(copy, binding)
			if consumeErr == nil {
				if string(got.Supply.Content) != "package a\n" {
					t.Errorf("ASSERT_PREPARED_SOURCE_DEEP_COPY: %q", got.Supply.Content)
				}
				successes.Add(1)
			}
		}(capability)
	}
	wg.Wait()
	if successes.Load() != 1 {
		t.Fatalf("ASSERT_PREPARED_SOURCE_CONCURRENT_SINGLE_USE: successes=%d", successes.Load())
	}
}

func TestSeedAuthorityRejectsCanonicalTargetMutations(t *testing.T) {
	binding, seed := testBinding(t)
	for name, mutated := range map[string][]byte{
		"noncanonical": append([]byte(" "), seed...),
		"position":     bytes.Replace(seed, []byte(`"column":3`), []byte(`"column":4`), 1),
		"schema":       bytes.Replace(seed, []byte(`lsp-trace.seeds.v2`), []byte(`lsp-trace.seeds.v1`), 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := MintSeedAuthority(binding, mutated, seedbinding.CallerAssertedLocal, false, "", true); err == nil {
				t.Fatal("ASSERT_SEED_AUTHORITY_CANONICAL_TARGET_MUTATION_REJECTED")
			}
		})
	}
}
