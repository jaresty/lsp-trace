//go:build darwin

package adr0011methodresult

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"lsp-trace/internal/adr0011querytarget"
	"lsp-trace/internal/publication"
	"lsp-trace/sessionruntime"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestADR0011ManagedMalformedDiagnostic(t *testing.T) {
	workspace := t.TempDir()
	source := []byte("package fixture\nfunc Query() {}\n")
	file := filepath.Join(workspace, "query.go")
	if err := os.WriteFile(file, source, 0600); err != nil {
		t.Fatal(err)
	}
	uri := (&url.URL{Scheme: "file", Path: filepath.ToSlash(file)}).String()
	manager, started, _ := startManagedMethodPeerWorkspace(t, workspace, true, "R-07")
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
			t.Fatalf("managed %s: %s", method, result.Failure)
		}
		return pair
	}
	symbolParams, _ := json.Marshal(map[string]any{"textDocument": map[string]string{"uri": uri}})
	symbol := call("textDocument/documentSymbol", symbolParams)
	refParams, _ := json.Marshal(map[string]any{"textDocument": map[string]string{"uri": uri}, "position": map[string]int{"line": 1, "character": 6}, "context": map[string]bool{"includeDeclaration": false}})
	refs := call("textDocument/references", refParams)
	q := adr0011querytarget.Query{OccurrenceID: "declared-1", URI: uri, Encoding: "utf-16", DocumentVersion: fmt.Sprint(prepared.Version), SourceDigest: binding.SHA256, SessionID: started.SessionID, Generation: started.Generation, Line: 1, Character: 6}
	commit := strings.Repeat("a", 40)
	command := func(out string, args ...string) HostGitCommand {
		return HostGitCommand{Args: args, Exit: 0, Stdout: out, Timestamp: time.Now().UTC().Format(time.RFC3339Nano)}
	}
	observation := HostGitObservation{Version: HostGitProbeVersion, Custody: "HOST_OBSERVED_GIT", WorkspaceRoot: workspace, HostExecutable: "/synthetic/test-peer", HostExecutableSHA256: privateDigest([]byte("synthetic-host")), TopLevel: command(workspace+"\n", "rev-parse", "--show-toplevel"), Head: command(commit+"\n", "rev-parse", "HEAD"), Status: command("", "status", "--porcelain=v1", "--untracked-files=all"), Worktrees: command("worktree "+workspace+"\nHEAD "+commit+"\nbranch refs/heads/main\n", "worktree", "list", "--porcelain")}
	git := HostGitEvidence{Before: observation, After: observation}
	expect := ChainExpectation{Query: q, Source: source, Revision: commit, Custody: "CALLER_ASSERTED", SymbolParams: symbolParams, ReferenceParams: refParams, TargetGit: git, MethodGit: git}
	dir := filepath.Join(t.TempDir(), "root")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	root, err := publication.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	target, err := PublishTarget(root, symbol, expect)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := PublishReferenceDiagnostic(root, target, symbol, refs, expect)
	if err != nil {
		t.Fatalf("ASSERT_ADR0011_MANAGED_MALFORMED_DIAGNOSTIC: %v", err)
	}
	got, err := ReplayReferenceDiagnostic(root, receipt, symbol, refs, expect)
	if err != nil || got.Outcome != "MALFORMED" || got.Disposition != "MALFORMED" || got.N != 1 || got.B != 1 || got.T != 1 || got.E != 2 || got.EB != 2 || got.ET != 2 || got.P != 0 || got.A != 0 || got.FailureOrdinal != 1 || len(got.Events) != 4 || got.Key != refs.Key || got.RawDigest != chainDigest(refs.Result) {
		t.Fatalf("ASSERT_ADR0011_MANAGED_MALFORMED_DIAGNOSTIC: got=%+v err=%v", got, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "adr0011-occurrences-") {
			t.Fatalf("ASSERT_ADR0011_MANAGED_MALFORMED_NO_OCCURRENCES: %s", entry.Name())
		}
	}
}
