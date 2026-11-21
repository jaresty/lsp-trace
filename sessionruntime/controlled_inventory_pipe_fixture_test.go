//go:build lsp_trace_controlled_describe_fixture

package sessionruntime

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"lsp-trace/internal/lsp"
	"time"

	"lsp-trace/internal/managedprocess"
)

func controlledInventoryFixtureTest(t *testing.T) *ControlledInventoryPipeFixture {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	f, err := NewControlledInventoryPipeFixture(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := f.Close(ctx); err != nil {
			t.Error(err)
		}
	})
	return f
}

// This is a runtime-unit test, not an inventory-funded integration. It exercises
// the concrete Manager/transport/owned-method path without making a native grant
// or calling any native preparation, admission, retention, or publication API.
func TestControlledInventoryPipeRuntime16Plus1(t *testing.T) {
	f := controlledInventoryFixtureTest(t)
	var results [2]PrivateDiscoveryObservation
	total := 0
	for ordinal := 0; ordinal < 2; ordinal++ {
		c, err := f.Construction(ordinal, time.Now().Add(10*time.Second))
		if err != nil {
			t.Fatal(err)
		}
		expected := 16
		if ordinal == 1 {
			expected = 1
		}
		if len(c.Members) != expected {
			t.Fatal("ASSERT_FIXED_16_PLUS_1: FAIL")
		}
		if f.children[ordinal] != nil {
			t.Fatal("ASSERT_STARTER_LAZY: FAIL")
		}
		selected := c.Members[0].URI
		target := f.documents[16].uri
		if ordinal == 1 {
			target = f.documents[0].uri
		}
		var window *privateDiscoveryWindow
		result, err := RunPrivateDiscovery(context.Background(), c, func(ctx context.Context, s *PrivateDiscoverySession) error {
			window = s.client.w
			for _, m := range c.Members {
				if err := s.Supply(ctx, m.URI, m.LanguageID); err != nil {
					return err
				}
				symbols, err := s.DocumentSymbols(ctx, lsp.DocumentSymbolParams{TextDocument: lsp.TextDocumentIdentifier{URI: m.URI}})
				if err != nil {
					return err
				}
				if len(symbols) != 1 {
					return fmt.Errorf("fixed symbol missing")
				}
				total++
			}
			refs, err := s.References(ctx, selected, lsp.Position{Line: 1, Character: 5})
			if err != nil {
				return err
			}
			if len(refs) != 1 || refs[0].URI != target || refs[0].Range.Start.Line != 2 || refs[0].Range.Start.Character != 12 || refs[0].Range.End.Character != 19 {
				t.Errorf("ASSERT_CROSSBATCH_REFERENCE: FAIL: %+v", refs)
				return fmt.Errorf("fixed crossbatch reference missing")
			}
			return nil
		})
		if err != nil || result.Failed || !result.CleanupJoined {
			t.Fatalf("ASSERT_OWNED_RUNTIME_CLEANUP: FAIL: ordinal=%d err=%v failed=%v joined=%v", ordinal, err, result.Failed, result.CleanupJoined)
		}
		if !result.OwnedMethods.Available() {
			t.Fatal("ASSERT_GENUINE_OWNED_METHODS: FAIL")
		}
		owned := result.OwnedMethods.state
		if len(owned.sources) != expected || len(owned.pairs) != expected+1 {
			t.Fatal("ASSERT_OWNED_DENOMINATOR: FAIL")
		}
		for _, p := range owned.pairs {
			source := owned.sources[p.Source.URI]
			if source == nil || p.Source.SHA256 != fmt.Sprintf("sha256:%x", sha256.Sum256(source.Content)) || p.SessionID != owned.sessionID || p.Generation != owned.generation {
				t.Fatal("ASSERT_SOURCE_CUSTODY: FAIL")
			}
			if p.Method != "textDocument/documentSymbol" && p.Method != "textDocument/references" {
				t.Fatal("ASSERT_D_C_UNREQUESTED: FAIL")
			}
		}
		pair := owned.pairs[len(owned.pairs)-1]
		var refs []lsp.Location
		if pair.Method != "textDocument/references" || json.Unmarshal(pair.Result, &refs) != nil || len(refs) != 1 || refs[0].URI != target {
			t.Fatal("ASSERT_RETAINED_CROSSBATCH_REFERENCE: FAIL")
		}
		if _, err := c.NewStarter(); err == nil {
			t.Fatal("ASSERT_STARTER_ONE_SHOT: FAIL")
		}
		window.mu.Lock()
		r := window.remaining
		window.mu.Unlock()
		literalBytes := 0
		for _, source := range owned.sources {
			literalBytes += len(source.Content)
		}
		t.Logf("FIXTURE_RUNTIME_DEBITS ordinal=%d members=%d literal_bytes=%d source=%d request=%d response=%d wire=%d retained=%d work=%d messages=%d requests=%d; logical reservation counters, not physical IO/RAM", ordinal, expected, literalBytes, c.Limits.SourceBytes-r.SourceBytes, c.Limits.RequestBytes-r.RequestBytes, c.Limits.ResponseBytes-r.ResponseBytes, c.Limits.WireBytes-r.WireBytes, c.Limits.RetainedBytes-r.RetainedBytes, c.Limits.Work-r.Work, c.Limits.Messages-r.Messages, c.Limits.Requests-r.Requests)
		results[ordinal] = result
	}
	if total != 17 || results[0].OwnedMethods.state.sessionID == "" || results[1].OwnedMethods.state.sessionID == "" {
		t.Fatal("ASSERT_FIXED_16_PLUS_1: FAIL")
	}
	t.Log("ASSERT_FIXED_16_PLUS_1: PASS; ASSERT_CROSSBATCH_REFERENCE: PASS; ASSERT_RETAINED_CROSSBATCH_REFERENCE: PASS; ASSERT_SOURCE_CUSTODY: PASS; ASSERT_D_C_UNREQUESTED: PASS; ASSERT_OWNED_RUNTIME_CLEANUP: PASS; runtime unit only, no parent funding")
}

func TestControlledInventoryPipeStarterOneShot(t *testing.T) {
	f := controlledInventoryFixtureTest(t)
	c, err := f.Construction(0, time.Now().Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	outcomes := make(chan bool, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := c.NewStarter(); outcomes <- err == nil }()
	}
	wg.Wait()
	close(outcomes)
	count := 0
	for ok := range outcomes {
		if ok {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("ASSERT_STARTER_CONCURRENT_ONE_SHOT: FAIL: %d", count)
	}
	t.Log("ASSERT_STARTER_CONCURRENT_ONE_SHOT: PASS")
}

// The fixture has no OS processes. Its zero-member census is justified only
// after its sole protocol goroutine has joined and all four pipe ends closed.
func TestControlledDescribeDiscoveryCleanupCensus(t *testing.T) {
	f := controlledDescribeFixtureTest(t)
	got := f.child.Teardown(context.Background())
	select {
	case <-f.child.done:
	default:
		t.Fatal("ASSERT_FIXTURE_JOIN_BEFORE_CENSUS: FAIL")
	}
	if got.Death.Reap.Kind != managedprocess.ReapComplete || !got.Census.Bounded || got.Census.Truncated || got.Census.Members != 0 || got.Census.Err != nil || got.Census.Limit != 1 || got.Census.Evidence != "controlled io.Pipe joined; no OS process created" {
		t.Fatalf("ASSERT_FIXTURE_JOINED_ZERO_CENSUS: FAIL: %+v", got)
	}
	t.Log("ASSERT_FIXTURE_JOINED_ZERO_CENSUS: PASS")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := f.Close(ctx); err != nil {
		t.Fatal(err)
	}
}
