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
	"testing"
	"time"

	transport "lsp-trace/internal/adr0011methodtransport"
	"lsp-trace/internal/adr0011querytarget"
	"lsp-trace/internal/publication"
	"lsp-trace/sessionruntime"
)

func TestADR0011PrivateManagedReferencesJoin(t *testing.T) {
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
		t.Fatalf("ASSERT_JOIN_MANAGED_SOURCE: %+v", prepared)
	}
	binding := sessionruntime.OwnedDocumentBinding{URI: uri, Version: prepared.Version, SHA256: privateDigest(source)}
	observed := &methodWireObservedRuntime{Manager: manager}
	call := func(method string, params []byte) (sessionruntime.OwnedMethodPair, sessionruntime.RoundTripResult) {
		t.Helper()
		req := transport.Request{SessionID: started.SessionID, Generation: started.Generation, Method: method, Params: params, Deadline: time.Now().Add(5 * time.Second), MaxMessages: 4, MaxBytes: 8192, CaptureOwnedMethodPair: true, ExpectedOwnedDocument: &binding}
		wire := transport.New(observed).Execute(context.Background(), req)
		pair, ok := wire.OwnedPair()
		if wire.Outcome() != transport.OutcomeTransportSuccess || !ok {
			t.Fatalf("ASSERT_JOIN_MANAGED_PAIR: %s", wire.Outcome())
		}
		return pair, observed.last
	}
	symbolParams, _ := json.Marshal(map[string]any{"textDocument": map[string]string{"uri": uri}})
	symbolPair, symbolObservation := call("textDocument/documentSymbol", symbolParams)
	symbolWrite, wok := symbolObservation.CompletedRequestWrite()
	symbolRead, rok := symbolObservation.CompletedResponseRead()
	if !wok || !rok {
		t.Fatal("ASSERT_JOIN_SYMBOL_OBSERVATIONS")
	}
	symbolExpected := DocumentSymbolPairExpected{SessionID: started.SessionID, Generation: started.Generation, KeyID: symbolObservation.Key.ID, Method: "textDocument/documentSymbol", Params: symbolParams, Result: symbolObservation.Result, Write: symbolWrite, Read: symbolRead, Source: &binding}
	symbolBytes, err := BuildDocumentSymbolPairCanonical(symbolPair)
	if err != nil || VerifyDocumentSymbolPairCanonical(symbolBytes, symbolExpected) != nil {
		t.Fatalf("ASSERT_JOIN_SYMBOL_CANONICAL: %v", err)
	}
	query := adr0011querytarget.Query{OccurrenceID: "owned-query", URI: uri, Encoding: "utf-16", DocumentVersion: fmt.Sprint(prepared.Version), SourceDigest: binding.SHA256, SessionID: started.SessionID, Generation: started.Generation, Line: 1, Character: 6}
	rootPath := filepath.Join(t.TempDir(), "private")
	if err := os.Mkdir(rootPath, 0700); err != nil {
		t.Fatal(err)
	}
	root, err := publication.OpenRoot(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	target, err := PublishDocumentSymbolQueryReceipt(root, "target-pair.json", "target-query.json", symbolBytes, symbolExpected, query, source, "test-revision", "CALLER_ASSERTED")
	if err != nil {
		t.Fatalf("ASSERT_JOIN_TARGET_PUBLICATION: %v", err)
	}
	referenceParams, _ := json.Marshal(map[string]any{"textDocument": map[string]string{"uri": uri}, "position": map[string]int{"line": 1, "character": 6}, "context": map[string]bool{"includeDeclaration": false}})
	referencePair, referenceObservation := call(transport.MethodReferences, referenceParams)
	referenceWrite, wok := referenceObservation.CompletedRequestWrite()
	referenceRead, rok := referenceObservation.CompletedResponseRead()
	if !wok || !rok || referenceObservation.Key.ID == symbolObservation.Key.ID {
		t.Fatal("ASSERT_JOIN_INDEPENDENT_REFERENCE_OBSERVATIONS")
	}
	referenceExpected := OwnerPairExpected{SessionID: started.SessionID, Generation: started.Generation, KeyID: referenceObservation.Key.ID, Method: transport.MethodReferences, Params: referenceParams, Result: referenceObservation.Result, Write: referenceWrite, Read: referenceRead, Source: &binding}
	referenceBytes, err := BuildOwnerPairCanonical(referencePair)
	if err != nil || VerifyOwnerPairCanonical(referenceBytes, referenceExpected) != nil {
		t.Fatalf("ASSERT_JOIN_REFERENCE_CANONICAL: %v", err)
	}
	receipt, err := PublishPrivateReferencesJoin(root, "references-pair.json", "references-ledger.json", referenceBytes, referenceExpected, target, symbolExpected, query, source, "test-revision", "CALLER_ASSERTED")
	if err != nil {
		t.Fatalf("ASSERT_JOIN_PUBLICATION: %v", err)
	}
	ledger, err := ReplayPrivateReferencesJoin(root, receipt, referenceExpected, target, symbolExpected, query, source, "test-revision", "CALLER_ASSERTED")
	if err != nil {
		t.Fatalf("ASSERT_JOIN_OFFLINE_REPLAY: %v", err)
	}
	if ledger.N != 1 || ledger.E != 2 || ledger.P != 2 || ledger.A != 2 || len(ledger.Occurrences) != 2 || ledger.Occurrences[0].Ordinal != 0 || ledger.Occurrences[1].Ordinal != 1 || ledger.Occurrences[0].URI != ledger.Occurrences[1].URI || ledger.Occurrences[0].Range != ledger.Occurrences[1].Range || ledger.Occurrences[0].Identity == ledger.Occurrences[1].Identity || ledger.Occurrences[0].TargetSymbolID == "" || ledger.Occurrences[0].RelationKind != ReferencesSymbol || ledger.Occurrences[1].RelationKind != ReferencesSymbol || ledger.Occurrences[0].RelationKind == "CALLS" || ledger.Occurrences[1].RelationKind == "CALLS" || ledger.Occurrences[0].SourceRole != "REFERENCING_OCCURRENCE" || ledger.Occurrences[1].SourceRole != "REFERENCING_OCCURRENCE" || ledger.Occurrences[0].TargetRole != "REFERENCED_SYMBOL" || ledger.Occurrences[1].TargetRole != "REFERENCED_SYMBOL" {
		t.Fatalf("ASSERT_JOIN_DUPLICATE_ORDINAL_IDENTITIES: %+v", ledger)
	}
	for _, selector := range []string{receipt.PairSelector, receipt.LedgerSelector} {
		if _, err := publication.ReadVerifiedBoundFile(root, selector, privateQueryReceiptLimit); err != nil {
			t.Fatalf("ASSERT_JOIN_VERIFIED_READBACK: %s: %v", selector, err)
		}
	}
	for _, tc := range []struct {
		name   string
		change func(*OwnerPairExpected, **DocumentSymbolQueryReceipt, *DocumentSymbolPairExpected, *adr0011querytarget.Query, *[]byte, *string, **PrivateReferencesReceipt)
	}{
		{"reference-result", func(e *OwnerPairExpected, _ **DocumentSymbolQueryReceipt, _ *DocumentSymbolPairExpected, _ *adr0011querytarget.Query, _ *[]byte, _ *string, _ **PrivateReferencesReceipt) {
			e.Result = []byte(`[]`)
		}},
		{"reference-params", func(e *OwnerPairExpected, _ **DocumentSymbolQueryReceipt, _ *DocumentSymbolPairExpected, _ *adr0011querytarget.Query, _ *[]byte, _ *string, _ **PrivateReferencesReceipt) {
			e.Params = []byte(`{}`)
		}},
		{"target", func(_ *OwnerPairExpected, r **DocumentSymbolQueryReceipt, _ *DocumentSymbolPairExpected, _ *adr0011querytarget.Query, _ *[]byte, _ *string, _ **PrivateReferencesReceipt) {
			c := **r
			c.QueryDigest = privateDigest([]byte("replacement"))
			*r = &c
		}},
		{"target-result", func(_ *OwnerPairExpected, _ **DocumentSymbolQueryReceipt, e *DocumentSymbolPairExpected, _ *adr0011querytarget.Query, _ *[]byte, _ *string, _ **PrivateReferencesReceipt) {
			e.Result = []byte(`[]`)
		}},
		{"point", func(_ *OwnerPairExpected, _ **DocumentSymbolQueryReceipt, _ *DocumentSymbolPairExpected, q *adr0011querytarget.Query, _ *[]byte, _ *string, _ **PrivateReferencesReceipt) {
			q.Character++
		}},
		{"source", func(_ *OwnerPairExpected, _ **DocumentSymbolQueryReceipt, _ *DocumentSymbolPairExpected, _ *adr0011querytarget.Query, s *[]byte, _ *string, _ **PrivateReferencesReceipt) {
			*s = []byte("replacement")
		}},
		{"revision", func(_ *OwnerPairExpected, _ **DocumentSymbolQueryReceipt, _ *DocumentSymbolPairExpected, _ *adr0011querytarget.Query, _ *[]byte, r *string, _ **PrivateReferencesReceipt) {
			*r = "different"
		}},
		{"ledger", func(_ *OwnerPairExpected, _ **DocumentSymbolQueryReceipt, _ *DocumentSymbolPairExpected, _ *adr0011querytarget.Query, _ *[]byte, _ *string, p **PrivateReferencesReceipt) {
			c := **p
			c.LedgerDigest = privateDigest([]byte("replacement"))
			*p = &c
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, targetCopy, se, q, s, revision, receiptCopy := referenceExpected, target, symbolExpected, query, append([]byte(nil), source...), "test-revision", receipt
			tc.change(&e, &targetCopy, &se, &q, &s, &revision, &receiptCopy)
			if _, err := ReplayPrivateReferencesJoin(root, receiptCopy, e, targetCopy, se, q, s, revision, "CALLER_ASSERTED"); err == nil {
				t.Fatal("ASSERT_JOIN_REPLAY_SUBSTITUTION: accepted")
			}
		})
	}
	duplicateKey := referenceExpected
	duplicateKey.KeyID = symbolExpected.KeyID
	if _, err := ReplayPrivateReferencesJoin(root, receipt, duplicateKey, target, symbolExpected, query, source, "test-revision", "CALLER_ASSERTED"); err == nil {
		t.Fatal("ASSERT_JOIN_DUPLICATE_RESULT_KEY: accepted")
	}
	malformed := referenceExpected
	malformed.Result = []byte(`[` + loc + `,` + malformedSecondLocation + `]`)
	if _, err := privateReferencesExpected(root, "references-pair.json", referenceBytes, malformed, target, symbolExpected, query, source, "test-revision", "CALLER_ASSERTED"); err == nil {
		t.Fatal("ASSERT_JOIN_MALFORMED_LATER_MEMBER: accepted")
	}
}
