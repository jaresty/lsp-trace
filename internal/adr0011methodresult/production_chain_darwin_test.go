//go:build darwin

package adr0011methodresult

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lsp-trace/internal/adr0011querytarget"
	"lsp-trace/internal/publication"
	"lsp-trace/sessionruntime"
)

func publishPrivateChainForTest(root *publication.Root, symbol, refs sessionruntime.OwnedMethodPair, e ChainExpectation) (*ChainReceipt, error) {
	target, err := PublishTarget(root, symbol, e)
	if err != nil {
		return nil, err
	}
	return publishReferencesUnissued(root, target, symbol, refs, e)
}

func TestADR0011ChainTwoEqualManagedLocations(t *testing.T) {
	workspace := t.TempDir()
	source := []byte("package fixture\nfunc Query() {}\n")
	file := filepath.Join(workspace, "query.go")
	if err := os.WriteFile(file, source, 0600); err != nil {
		t.Fatal(err)
	}
	uri := (&url.URL{Scheme: "file", Path: filepath.ToSlash(file)}).String()
	manager, started, _ := startManagedMethodPeerWorkspace(t, workspace, true, "R-05")
	prepared := manager.PrepareDocument(context.Background(), sessionruntime.DocumentRequest{SessionID: started.SessionID, Generation: started.Generation, URI: uri, LanguageID: "go", CaptureSupply: true})
	if prepared.Failure != "" || prepared.Supply == nil || !bytes.Equal(prepared.Supply.Content, source) {
		t.Fatalf("fixture source: %+v", prepared)
	}
	binding := sessionruntime.OwnedDocumentBinding{URI: uri, Version: prepared.Version, SHA256: privateDigest(source)}
	call := func(method string, params []byte) sessionruntime.OwnedMethodPair {
		t.Helper()
		result := manager.RoundTrip(context.Background(), sessionruntime.RoundTripRequest{SessionID: started.SessionID, Generation: started.Generation, Method: method, Params: params, Deadline: time.Now().Add(5 * time.Second), MaxMessages: 4, MaxBytes: 8192, CaptureOwnedMethodPair: true, ExpectedOwnedDocument: &binding})
		pair, ok := result.CompletedOwnedMethodPair()
		if result.Failure != "" || !ok {
			t.Fatalf("fixture managed %s: %s", method, result.Failure)
		}
		return pair
	}
	symbolParams, _ := json.Marshal(map[string]any{"textDocument": map[string]string{"uri": uri}})
	symbol := call("textDocument/documentSymbol", symbolParams)
	refParams, _ := json.Marshal(map[string]any{"textDocument": map[string]string{"uri": uri}, "position": map[string]int{"line": 1, "character": 6}, "context": map[string]bool{"includeDeclaration": false}})
	refs := call("textDocument/references", refParams)
	q := adr0011querytarget.Query{OccurrenceID: "declared-1", URI: uri, Encoding: "utf-16", DocumentVersion: fmt.Sprint(prepared.Version), SourceDigest: binding.SHA256, SessionID: started.SessionID, Generation: started.Generation, Line: 1, Character: 6}
	commit := strings.Repeat("a", 40)
	fixtureCommand := func(out string, args ...string) HostGitCommand {
		return HostGitCommand{Args: args, Exit: 0, Stdout: out, Timestamp: time.Now().UTC().Format(time.RFC3339Nano)}
	}
	fixtureObservation := HostGitObservation{Version: HostGitProbeVersion, Custody: "HOST_OBSERVED_GIT", WorkspaceRoot: workspace, HostExecutable: "/synthetic/test-peer", HostExecutableSHA256: privateDigest([]byte("synthetic-host")), TopLevel: fixtureCommand(workspace+"\n", "rev-parse", "--show-toplevel"), Head: fixtureCommand(commit+"\n", "rev-parse", "HEAD"), Status: fixtureCommand("", "status", "--porcelain=v1", "--untracked-files=all"), Worktrees: fixtureCommand("worktree "+workspace+"\nHEAD "+commit+"\nbranch refs/heads/main\n", "worktree", "list", "--porcelain")}
	fixtureGit := HostGitEvidence{Before: fixtureObservation, After: fixtureObservation}
	e := ChainExpectation{Query: q, Source: source, Revision: commit, Custody: "CALLER_ASSERTED", SymbolParams: symbolParams, ReferenceParams: refParams, TargetGit: fixtureGit, MethodGit: fixtureGit}
	dir := filepath.Join(t.TempDir(), "root")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	root, err := publication.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	receipt, err := publishPrivateChainForTest(root, symbol, refs, e)
	if err != nil {
		t.Fatalf("ASSERT_ADR0011_CHAIN_TWO_EQUAL_LOCATIONS: %v", err)
	}
	ledger, err := ReplayChain(root, receipt, symbol, refs, e)
	if err != nil || ledger.Kind != ReferencesSymbolV1 || ledger.N != 1 || ledger.B != 1 || ledger.T != 0 || ledger.E != 2 || ledger.P != 2 || ledger.A != 0 || len(ledger.Occurrences) != 2 || ledger.Occurrences[0].Ordinal != 0 || ledger.Occurrences[1].Ordinal != 1 || ledger.Occurrences[0].ID == ledger.Occurrences[1].ID || ledger.Occurrences[0].URI != ledger.Occurrences[1].URI || ledger.Occurrences[0].Range != ledger.Occurrences[1].Range {
		t.Fatalf("ASSERT_ADR0011_CHAIN_TWO_EQUAL_LOCATIONS: ledger=%+v err=%v", ledger, err)
	}
	for _, o := range ledger.Occurrences {
		if o.SourceRole != "REFERENCING_OCCURRENCE" || o.TargetRole != "REFERENCED_SYMBOL" || o.Authority != 0 || o.Accepted || o.Completeness != "UNKNOWN" || o.ProducerAuthentication != "NO_PRODUCER_AUTHENTICATION" {
			t.Fatalf("ASSERT_ADR0011_CHAIN_CLAIM_CEILING: %+v", o)
		}
	}
	targetDir := filepath.Join(t.TempDir(), "target-only")
	if err := os.Mkdir(targetDir, 0700); err != nil {
		t.Fatal(err)
	}
	targetRoot, err := publication.OpenRoot(targetDir)
	if err != nil {
		t.Fatal(err)
	}
	defer targetRoot.Close()
	separateTarget, err := PublishTarget(targetRoot, symbol, e)
	if err != nil {
		t.Fatalf("fixture target: %v", err)
	}
	substitutedTarget := &TargetReceipt{Selector: separateTarget.Selector, Digest: privateDigest([]byte("wrong"))}
	if got, err := PublishReferences(targetRoot, substitutedTarget, symbol, refs, e); err == nil || got != nil {
		t.Fatalf("ASSERT_ADR0011_CHAIN_TARGET_REPLAY_BEFORE_METHOD: receipt=%+v err=%v", got, err)
	}
	entriesBefore, err := os.ReadDir(targetDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entriesBefore) != 1 {
		t.Fatalf("ASSERT_ADR0011_CHAIN_TARGET_REPLAY_BEFORE_METHOD: unexpected publications=%d", len(entriesBefore))
	}
	for _, tc := range []struct {
		name   string
		change func(*ChainExpectation)
	}{
		{"target", func(x *ChainExpectation) { x.Query.Character++ }},
		{"source", func(x *ChainExpectation) { x.Source = []byte("replacement") }},
		{"revision", func(x *ChainExpectation) { x.Revision = "replacement" }},
		{"params", func(x *ChainExpectation) { x.ReferenceParams = []byte(`{}`) }},
		{"host-target", func(x *ChainExpectation) { x.TargetGit.Before.Status.Stdout = "dirty" }},
		{"host-method", func(x *ChainExpectation) { x.MethodGit.After.Head.Stdout = strings.Repeat("b", 40) + "\n" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bad := e
			tc.change(&bad)
			if got, err := ReplayChain(root, receipt, symbol, refs, bad); err == nil || got.A != 0 {
				t.Fatalf("ASSERT_ADR0011_CHAIN_SUBSTITUTION_ZERO: ledger=%+v err=%v", got, err)
			}
		})
	}
	bad := refs
	bad.Result = []byte(`[{"uri":"file:///ok","range":{"start":{"line":1,"character":0},"end":{"line":1,"character":1}}},{"uri":9}]`)
	if got, err := ReplayChain(root, receipt, symbol, bad, e); err == nil || got.A != 0 || got.P != 0 {
		t.Fatalf("ASSERT_ADR0011_CHAIN_MALFORMED_PREFIX_ZERO: %+v %v", got, err)
	}
	for _, role := range []string{"method", "terminal", "occurrences"} {
		t.Run("encode-"+role, func(t *testing.T) {
			boundedDir := filepath.Join(t.TempDir(), "bounded")
			if err := os.Mkdir(boundedDir, 0700); err != nil {
				t.Fatal(err)
			}
			boundedRoot, err := publication.OpenRoot(boundedDir)
			if err != nil {
				t.Fatal(err)
			}
			defer boundedRoot.Close()
			target, err := PublishTarget(boundedRoot, symbol, e)
			if err != nil {
				t.Fatal(err)
			}
			attempts := []string{}
			chainEncodeTestHook = func(v any) bool {
				switch v.(type) {
				case chainPair, chainTerminal, chainOccurrences:
					return chainRole(v) != role
				}
				return true
			}
			chainBeforePublishTestHook = func(v string) { attempts = append(attempts, v) }
			defer func() { chainEncodeTestHook = nil; chainBeforePublishTestHook = nil }()
			issued, err := PublishReferences(boundedRoot, target, symbol, refs, e)
			if err == nil || issued != nil {
				t.Fatalf("ASSERT_ADR0011_CANONICAL_ERROR_ZERO: role=%s receipt=%+v err=%v", role, issued, err)
			}
			for _, attempted := range attempts {
				if attempted == role {
					t.Fatalf("ASSERT_ADR0011_CANONICAL_ERROR_BEFORE_PUBLICATION: role=%s attempts=%v", role, attempts)
				}
			}
		})
	}
	// A committed terminal that cannot pass post-commit verification must not
	// issue an occurrence ledger or a successful receipt.
	lateDir := filepath.Join(t.TempDir(), "late")
	if err := os.Mkdir(lateDir, 0700); err != nil {
		t.Fatal(err)
	}
	lateRoot, err := publication.OpenRoot(lateDir)
	if err != nil {
		t.Fatal(err)
	}
	defer lateRoot.Close()
	chainPostCommitTestHook = func(role string) bool { return role != "terminal" }
	defer func() { chainPostCommitTestHook = nil }()
	lateReceipt, lateErr := publishPrivateChainForTest(lateRoot, symbol, refs, e)
	if lateErr == nil || lateReceipt != nil {
		t.Fatalf("ASSERT_ADR0011_CHAIN_LATE_PUBLICATION_ZERO: receipt=%+v err=%v", lateReceipt, lateErr)
	}
	entries, err := os.ReadDir(lateDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if len(entry.Name()) >= 20 && entry.Name()[:20] == "adr0011-occurrences-" {
			t.Fatalf("ASSERT_ADR0011_CHAIN_LATE_PUBLICATION_ZERO: issued %s", entry.Name())
		}
	}
}
