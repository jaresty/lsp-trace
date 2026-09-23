//go:build darwin

package adr0011methodresult

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	transport "lsp-trace/internal/adr0011methodtransport"
	"lsp-trace/internal/publication"
	"lsp-trace/sessionruntime"
)

func TestADR0011PrivateManagedMalformedTerminal(t *testing.T) {
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
		t.Fatalf("ASSERT_TERMINAL_PREPARED_SOURCE: %+v", prepared)
	}
	binding := sessionruntime.OwnedDocumentBinding{URI: uri, Version: prepared.Version, SHA256: privateDigest(source)}
	params, _ := json.Marshal(map[string]any{"textDocument": map[string]string{"uri": uri}, "position": map[string]int{"line": 1, "character": 6}, "context": map[string]bool{"includeDeclaration": false}})
	observed := &methodWireObservedRuntime{Manager: manager}
	wire := transport.New(observed).Execute(context.Background(), transport.Request{SessionID: started.SessionID, Generation: started.Generation, Method: transport.MethodReferences, Params: params, Deadline: time.Now().Add(5 * time.Second), MaxMessages: 4, MaxBytes: 8192, CaptureOwnedMethodPair: true, ExpectedOwnedDocument: &binding})
	pair, ok := wire.OwnedPair()
	write, wok := observed.last.CompletedRequestWrite()
	read, rok := observed.last.CompletedResponseRead()
	if wire.Outcome() != transport.OutcomeTransportSuccess || !ok || !wok || !rok || !bytes.Equal(observed.last.Result, wire.Raw()) || observed.last.Key != pair.Key {
		t.Fatalf("ASSERT_TERMINAL_MANAGER_OWNERSHIP: outcome=%s pair=%v write=%v read=%v", wire.Outcome(), ok, wok, rok)
	}
	expected := OwnerPairExpected{SessionID: started.SessionID, Generation: started.Generation, KeyID: observed.last.Key.ID, Method: transport.MethodReferences, Params: params, Result: observed.last.Result, Write: write, Read: read, Source: &binding}
	if _, err := BuildOwnerPairCanonical(pair); err == nil {
		t.Fatal("ASSERT_TERMINAL_VALID_ONLY_PAIR_ACCEPTED")
	}
	record, err := BuildPrivateMalformedTerminal(pair, expected, source, 1, 6, "test-revision", "CALLER_ASSERTED")
	if err != nil {
		t.Fatalf("ASSERT_TERMINAL_BUILD: %v", err)
	}
	terminal, err := VerifyPrivateMalformedTerminal(record, expected, source, 1, 6, "test-revision", "CALLER_ASSERTED")
	if err != nil || terminal.Terminal != "MALFORMED" || terminal.N != 1 || terminal.E != 2 || terminal.P != 0 || terminal.A != 0 || terminal.DiagnosticValidPrefix != 1 || terminal.ErrorOrdinal != 1 || terminal.Transport != "SUCCESS" || terminal.ResultLength != len(observed.last.Result) || terminal.ResultDigest != privateDigest(observed.last.Result) {
		t.Fatalf("ASSERT_TERMINAL_MALFORMED_N1_E2_P0_A0_PREFIX1: %+v %v", terminal, err)
	}
	dir := filepath.Join(t.TempDir(), "private")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	root, err := publication.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	receipt, err := PublishPrivateMalformedTerminal(root, "terminal.json", record, expected, source, 1, 6, "test-revision", "CALLER_ASSERTED")
	if err != nil {
		t.Fatalf("ASSERT_TERMINAL_PUBLISH: %v", err)
	}
	if _, err := PublishPrivateMalformedTerminal(root, "terminal.json", record, expected, source, 1, 6, "test-revision", "CALLER_ASSERTED"); !errors.Is(err, os.ErrExist) {
		t.Fatalf("ASSERT_TERMINAL_NO_REPLACE: %v", err)
	}
	if got, err := ReplayPrivateMalformedTerminal(root, receipt, expected, source, 1, 6, "test-revision", "CALLER_ASSERTED"); err != nil || got != terminal {
		t.Fatalf("ASSERT_TERMINAL_REPLAY: %+v %v", got, err)
	}
	for _, tc := range []struct {
		name   string
		change func(*OwnerPairExpected, *[]byte, *string)
	}{
		{"raw", func(e *OwnerPairExpected, _ *[]byte, _ *string) { e.Result = []byte(`[]`) }},
		{"key", func(e *OwnerPairExpected, _ *[]byte, _ *string) { e.KeyID++ }},
		{"source", func(_ *OwnerPairExpected, s *[]byte, _ *string) { *s = []byte("replacement") }},
		{"revision", func(_ *OwnerPairExpected, _ *[]byte, r *string) { *r = "replacement" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, s, r := expected, append([]byte(nil), source...), "test-revision"
			tc.change(&e, &s, &r)
			if _, err := ReplayPrivateMalformedTerminal(root, receipt, e, s, 1, 6, r, "CALLER_ASSERTED"); err == nil {
				t.Fatal("ASSERT_TERMINAL_SUBSTITUTION_ACCEPTED")
			}
		})
	}
	nilSource := expected
	nilSource.Source = nil
	if _, err := VerifyPrivateMalformedTerminal(record, nilSource, source, 1, 6, "test-revision", "CALLER_ASSERTED"); err == nil {
		t.Fatal("ASSERT_TERMINAL_MISSING_SOURCE_ACCEPTED")
	}
	validResult := expected
	validResult.Result = []byte(`[]`)
	if _, err := BuildPrivateMalformedTerminal(pair, validResult, source, 1, 6, "test-revision", "CALLER_ASSERTED"); err == nil {
		t.Fatal("ASSERT_TERMINAL_VALID_RESULT_ACCEPTED")
	}
	wrongSelector := *receipt
	wrongSelector.Selector = "replacement.json"
	if _, err := ReplayPrivateMalformedTerminal(root, &wrongSelector, expected, source, 1, 6, "test-revision", "CALLER_ASSERTED"); err == nil {
		t.Fatal("ASSERT_TERMINAL_SELECTOR_SUBSTITUTION_ACCEPTED")
	}
	for _, raw := range [][]byte{append(append([]byte(nil), record...), ' '), append(append([]byte(nil), record...), '{'), bytes.Replace(record, []byte(`"Terminal":"MALFORMED"`), []byte(`"Terminal":"MALFORMED","Terminal":"MALFORMED"`), 1), bytes.Replace(record, []byte(`"Terminal":"MALFORMED"`), []byte(`"Unknown":true,"Terminal":"MALFORMED"`), 1), bytes.Repeat([]byte("x"), 1500001)} {
		if _, err := VerifyPrivateMalformedTerminal(raw, expected, source, 1, 6, "test-revision", "CALLER_ASSERTED"); err == nil {
			t.Fatal("ASSERT_TERMINAL_NONCANONICAL_ACCEPTED")
		}
	}
}
