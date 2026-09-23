//go:build darwin

package adr0011methodresult

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	transport "lsp-trace/internal/adr0011methodtransport"
	"lsp-trace/internal/adr0011querytarget"
	"lsp-trace/internal/publication"
	"lsp-trace/sessionruntime"
)

// Test-owned, unadmitted synthetic workspace and peer: no provider authentication or CALLS assertion.
func TestADR0011PrivateManagedDocumentSymbolQueryReceipt(t *testing.T) {
	workspace := t.TempDir()
	source := []byte("package fixture\nfunc Query() {}\n")
	file := filepath.Join(workspace, "query.go")
	if err := os.WriteFile(file, source, 0600); err != nil {
		t.Fatal(err)
	}
	uri := (&url.URL{Scheme: "file", Path: filepath.ToSlash(file)}).String()
	manager, started, _ := startManagedMethodPeerWorkspace(t, workspace, true)
	prepared := manager.PrepareDocument(context.Background(), sessionruntime.DocumentRequest{SessionID: started.SessionID, Generation: started.Generation, URI: uri, LanguageID: "go", CaptureSupply: true})
	if prepared.Failure != "" || prepared.Supply == nil || !bytes.Equal(prepared.Supply.Content, source) {
		t.Fatalf("ASSERT_QUERY_MANAGED_SUPPLY: %+v", prepared)
	}
	binding := sessionruntime.OwnedDocumentBinding{URI: uri, Version: prepared.Version, SHA256: fmt.Sprintf("sha256:%x", sha256.Sum256(prepared.Supply.Content))}
	params, _ := json.Marshal(map[string]any{"textDocument": map[string]string{"uri": uri}})
	request := transport.Request{SessionID: started.SessionID, Generation: started.Generation, Method: "textDocument/documentSymbol", Params: params, Deadline: time.Now().Add(5 * time.Second), MaxMessages: 4, MaxBytes: 8192, CaptureOwnedMethodPair: true, ExpectedOwnedDocument: &binding}
	observed := &methodWireObservedRuntime{Manager: manager}
	wire := transport.New(observed).Execute(context.Background(), request)
	write, writeOK := observed.last.CompletedRequestWrite()
	read, readOK := observed.last.CompletedResponseRead()
	if !writeOK || !readOK || observed.last.Key.ID == 0 {
		t.Fatalf("ASSERT_QUERY_INDEPENDENT_MANAGER_OBSERVATIONS: key=%+v write=%v read=%v", observed.last.Key, writeOK, readOK)
	}
	expected := DocumentSymbolPairExpected{SessionID: started.SessionID, Generation: started.Generation, KeyID: observed.last.Key.ID, Method: request.Method, Params: append([]byte(nil), params...), Result: append([]byte(nil), observed.last.Result...), Write: write, Read: read, Source: &binding}
	pair, ok := wire.OwnedPair()
	if wire.Outcome() != transport.OutcomeTransportSuccess || !ok || pair.SessionID != expected.SessionID || pair.Generation != expected.Generation || pair.Key != observed.last.Key || pair.Method != expected.Method || !bytes.Equal(pair.Params, expected.Params) || !bytes.Equal(pair.Result, expected.Result) || pair.Write != expected.Write || pair.Read != expected.Read || pair.Source == nil || *pair.Source != binding {
		t.Fatalf("ASSERT_QUERY_MANAGED_PAIR: %s %+v", wire.Outcome(), pair)
	}
	pairBytes, err := BuildDocumentSymbolPairCanonical(pair)
	if err != nil || VerifyDocumentSymbolPairCanonical(pairBytes, expected) != nil {
		t.Fatalf("ASSERT_QUERY_PAIR_CANONICAL: %v", err)
	}
	query := adr0011querytarget.Query{OccurrenceID: "test-owned-query-1", URI: uri, Encoding: "utf-16", DocumentVersion: fmt.Sprint(prepared.Version), SourceDigest: binding.SHA256, SessionID: started.SessionID, Generation: started.Generation, Line: 1, Character: 6}
	candidate, err := adr0011querytarget.SelectDocumentSymbolCandidateV1(query, wire.Raw())
	if err != nil || candidate.SymbolName != "Query" || candidate.QueryOccurrenceID != "test-owned-query-1" || candidate.QueryLine != 1 || candidate.QueryCharacter != 6 || candidate.SelectionRange.Start != (adr0011querytarget.Position{Line: 1, Character: 5}) || candidate.SelectionRange.End != (adr0011querytarget.Position{Line: 1, Character: 10}) {
		t.Fatalf("ASSERT_QUERY_EXACT_SELECTION: %+v %v", candidate, err)
	}
	rootPath := filepath.Join(t.TempDir(), "private")
	if err := os.Mkdir(rootPath, 0700); err != nil {
		t.Fatal(err)
	}
	root, err := publication.OpenRoot(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	receipt, err := PublishDocumentSymbolQueryReceipt(root, "pair.json", "query.json", pairBytes, expected, query, source, "test-revision", "CALLER_ASSERTED")
	if err != nil || receipt == nil {
		t.Fatalf("ASSERT_QUERY_VERIFIED_PUBLICATION: %+v %v", receipt, err)
	}
	persistedPair, err := publication.ReadVerifiedBoundFile(root, receipt.PairSelector, 1600000)
	if err != nil || !bytes.Equal(persistedPair, pairBytes) {
		t.Fatalf("ASSERT_QUERY_IMMUTABLE_PAIR_READBACK: %v", err)
	}
	persistedQuery, err := publication.ReadVerifiedBoundFile(root, receipt.QuerySelector, 1600000)
	if err != nil || len(persistedQuery) == 0 || bytes.Equal(persistedQuery, persistedPair) {
		t.Fatalf("ASSERT_QUERY_DISTINCT_IMMUTABLE_RECORD: %v", err)
	}
	if err := ReplayDocumentSymbolQueryReceipt(root, receipt, expected, query, source, "test-revision", "CALLER_ASSERTED"); err != nil {
		t.Fatalf("ASSERT_QUERY_OFFLINE_REPLAY: %v", err)
	}
	for _, tc := range []struct {
		name   string
		mutate func(*adr0011querytarget.Query, *DocumentSymbolPairExpected, *[]byte, *string)
	}{
		{"point", func(q *adr0011querytarget.Query, _ *DocumentSymbolPairExpected, _ *[]byte, _ *string) { q.Character++ }},
		{"source", func(_ *adr0011querytarget.Query, _ *DocumentSymbolPairExpected, s *[]byte, _ *string) {
			*s = []byte("substituted")
		}},
		{"pair-bytes", func(_ *adr0011querytarget.Query, e *DocumentSymbolPairExpected, _ *[]byte, _ *string) {
			e.Result = []byte(`[]`)
		}},
		{"candidate", func(q *adr0011querytarget.Query, _ *DocumentSymbolPairExpected, _ *[]byte, _ *string) {
			q.OccurrenceID = "substituted"
		}},
		{"revision", func(_ *adr0011querytarget.Query, _ *DocumentSymbolPairExpected, _ *[]byte, r *string) {
			*r = "other-revision"
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			q, e, s, r := query, expected, append([]byte(nil), source...), "test-revision"
			tc.mutate(&q, &e, &s, &r)
			if err := ReplayDocumentSymbolQueryReceipt(root, receipt, e, q, s, r, "CALLER_ASSERTED"); err == nil {
				t.Fatal("ASSERT_QUERY_REPLAY_SUBSTITUTION: accepted")
			}
		})
	}
	substitutedReceipt := *receipt
	substitutedReceipt.PairDigest = fmt.Sprintf("sha256:%x", sha256.Sum256([]byte("substituted pair")))
	if err := ReplayDocumentSymbolQueryReceipt(root, &substitutedReceipt, expected, query, source, "test-revision", "CALLER_ASSERTED"); err == nil {
		t.Fatal("ASSERT_QUERY_PAIR_ARTIFACT_SUBSTITUTION: accepted")
	}
	badSource := append([]byte(nil), source...)
	badSource[0] = 'X'
	if _, err := PublishDocumentSymbolQueryReceipt(root, "mismatch-pair.json", "mismatch-query.json", pairBytes, expected, query, badSource, "test-revision", "CALLER_ASSERTED"); err == nil {
		t.Fatal("ASSERT_QUERY_SOURCE_MISMATCH: accepted")
	}
	if _, err := root.ReadSelector("mismatch-pair.json", 8192); !os.IsNotExist(err) {
		t.Fatalf("ASSERT_QUERY_SOURCE_MISMATCH_ZERO_PUBLICATION: %v", err)
	}
	unresolved := query
	unresolved.Line = 40
	if _, err := PublishDocumentSymbolQueryReceipt(root, "unresolved-pair.json", "unresolved-query.json", pairBytes, expected, unresolved, source, "test-revision", "CALLER_ASSERTED"); err == nil {
		t.Fatal("ASSERT_QUERY_UNRESOLVED: accepted")
	}
	if _, err := root.ReadSelector("unresolved-pair.json", 8192); !os.IsNotExist(err) {
		t.Fatalf("ASSERT_QUERY_UNRESOLVED_ZERO_PUBLICATION: %v", err)
	}
	calls := 0
	late, err := publication.PublishBoundFile(root, "late-failure.json", pairBytes, func(raw []byte) error {
		calls++
		if calls == 2 {
			return fmt.Errorf("post-read verification rejected")
		}
		return VerifyDocumentSymbolPairCanonical(raw, expected)
	})
	if err != nil || late == nil || late.VerificationStatus != "COMMITTED_VERIFICATION_FAILED" {
		t.Fatalf("ASSERT_QUERY_REJECT_LATE_FAILURE: %+v %v", late, err)
	}
}
