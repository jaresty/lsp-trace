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
	"lsp-trace/sessionruntime"
)

func TestOptedInTransportSourceBoundManagedPeer(t *testing.T) {
	workspace := t.TempDir()
	file := filepath.Join(workspace, "query.go")
	content := []byte("package fixture\nfunc Query() {}\n")
	if err := os.WriteFile(file, content, 0o600); err != nil {
		t.Fatal(err)
	}
	uri := (&url.URL{Scheme: "file", Path: filepath.ToSlash(file)}).String()
	manager, started, trace := startManagedMethodPeerWorkspace(t, workspace, true)
	prepared := manager.PrepareDocument(context.Background(), sessionruntime.DocumentRequest{SessionID: started.SessionID, Generation: started.Generation, URI: uri, LanguageID: "go", CaptureSupply: true})
	if prepared.Failure != "" || prepared.Supply == nil {
		t.Fatalf("ASSERT_MANAGED_SOURCE_SUPPLY: %+v", prepared)
	}
	expected := sessionruntime.OwnedDocumentBinding{URI: uri, Version: prepared.Version, SHA256: fmt.Sprintf("sha256:%x", sha256.Sum256(content))}
	params, _ := json.Marshal(map[string]any{"textDocument": map[string]string{"uri": uri}, "position": map[string]int{"line": 1, "character": 5}, "context": map[string]bool{"includeDeclaration": false}})
	req := transport.Request{SessionID: started.SessionID, Generation: started.Generation, Method: transport.MethodReferences, Params: params, Deadline: time.Now().Add(5 * time.Second), MaxMessages: 4, MaxBytes: 8192, CaptureOwnedMethodPair: true, ExpectedOwnedDocument: &expected}
	got := transport.New(manager).Execute(context.Background(), req)
	pair, ok := got.OwnedPair()
	if got.Outcome() != transport.OutcomeTransportSuccess || !ok || pair.Source == nil || *pair.Source != expected || string(got.Raw()) != `[]` || !bytes.Equal(pair.Params, params) {
		t.Fatalf("ASSERT_MANAGED_TRANSPORT_SOURCE_BINDING: outcome=%s ok=%v pair=%+v", got.Outcome(), ok, pair)
	}
	before, err := os.ReadFile(trace)
	if err != nil {
		t.Fatal(err)
	}
	expected.Version++
	failed := transport.New(manager).Execute(context.Background(), req)
	after, err := os.ReadFile(trace)
	if err != nil {
		t.Fatal(err)
	}
	if failed.Outcome() != transport.OutcomeTransportFailure || failed.Raw() != nil || !bytes.Equal(before, after) {
		t.Fatalf("ASSERT_SOURCE_MISMATCH_NO_METHOD_WRITE: outcome=%s trace=%q/%q", failed.Outcome(), before, after)
	}
	if _, present := failed.OwnedPair(); present {
		t.Fatal("ASSERT_SOURCE_FAILURE_WITHHELD")
	}
	pair.Source.Version++
	copied, present := got.OwnedPair()
	if !present || copied.Source == nil || copied.Source.Version != prepared.Version {
		t.Fatal("ASSERT_SOURCE_TRANSPORT_COPY")
	}
}
