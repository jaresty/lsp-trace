//go:build darwin

package adr0011methodresult

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	transport "lsp-trace/internal/adr0011methodtransport"
	"lsp-trace/sessionruntime"
)

func TestADR0011PrivateManagedReferencesJoinMalformedLater(t *testing.T) {
	workspace := t.TempDir()
	file := filepath.Join(workspace, "query.go")
	source := []byte("package fixture\nfunc Query() {}\n")
	if err := os.WriteFile(file, source, 0600); err != nil {
		t.Fatal(err)
	}
	uri := (&url.URL{Scheme: "file", Path: filepath.ToSlash(file)}).String()
	manager, started, _ := startManagedMethodPeerWorkspace(t, workspace, true, "R-07")
	prepared := manager.PrepareDocument(context.Background(), sessionruntime.DocumentRequest{SessionID: started.SessionID, Generation: started.Generation, URI: uri, LanguageID: "go", CaptureSupply: true})
	if prepared.Failure != "" || prepared.Supply == nil {
		t.Fatalf("ASSERT_JOIN_MALFORMED_SOURCE: %+v", prepared)
	}
	binding := sessionruntime.OwnedDocumentBinding{URI: uri, Version: prepared.Version, SHA256: privateDigest(source)}
	params, _ := json.Marshal(map[string]any{"textDocument": map[string]string{"uri": uri}, "position": map[string]int{"line": 1, "character": 6}, "context": map[string]bool{"includeDeclaration": false}})
	observed := &methodWireObservedRuntime{Manager: manager}
	wire := transport.New(observed).Execute(context.Background(), transport.Request{SessionID: started.SessionID, Generation: started.Generation, Method: transport.MethodReferences, Params: params, Deadline: time.Now().Add(5 * time.Second), MaxMessages: 4, MaxBytes: 8192, CaptureOwnedMethodPair: true, ExpectedOwnedDocument: &binding})
	if wire.Outcome() != transport.OutcomeTransportSuccess {
		t.Fatalf("ASSERT_JOIN_MALFORMED_WIRE: %s", wire.Outcome())
	}
	var members []json.RawMessage
	if err := json.Unmarshal(wire.Raw(), &members); err != nil || len(members) != 2 {
		t.Fatalf("ASSERT_JOIN_MALFORMED_RETURNED_E: %d %v", len(members), err)
	}
	parsed, failure := parseRawUntrusted(transport.MethodReferences, wire.Raw(), ownerPairMaxCandidates)
	if failure == nil || failure.Ordinal != 1 || len(parsed.Items) != 0 {
		t.Fatalf("ASSERT_JOIN_MALFORMED_ALL_OR_NOTHING_A_ZERO: %+v %+v", failure, parsed)
	}
	// The canonical pair refuses a malformed result before any private publication.
	if pair, ok := wire.OwnedPair(); ok {
		if _, err := BuildOwnerPairCanonical(pair); err == nil {
			t.Fatal("ASSERT_JOIN_MALFORMED_PAIR_REJECTED: accepted")
		}
	}
}
