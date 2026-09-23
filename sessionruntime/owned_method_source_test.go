package sessionruntime

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"lsp-trace/internal/runtimeprofile"
)

func ownedDocumentFixture(t *testing.T, capture bool) (*Manager, StartResult, *roundTripChild, OwnedDocumentBinding) {
	t.Helper()
	root := t.TempDir()
	file := filepath.Join(root, "query.go")
	content := []byte("package fixture\nfunc Query() {}\n")
	if err := os.WriteFile(file, content, 0o600); err != nil {
		t.Fatal(err)
	}
	profileSelector, err := runtimeprofile.Validate(runtimeprofile.Selector{TrustDomain: "test", Workspace: root, Profile: "go", EnvironmentReference: "local"})
	if err != nil {
		t.Fatal(err)
	}
	child := newRoundTripChild("references-empty")
	manager, err := New(Config{Limits: Limits{MaxSessions: 1, MaxRequests: 1, MaxChildren: 2, MaxCancels: 2, MaxTombstones: 4, MaxObservations: 64, MaxOperations: 2}, Starter: oneChildStarter{child}})
	if err != nil {
		t.Fatal(err)
	}
	started := manager.Start(context.Background(), StartRequest{Profile: runtimeprofile.Resolve(profileSelector)})
	if started.Failure != "" {
		t.Fatal(started.Failure)
	}
	if init := manager.ObserveInitialization(started.SessionID, started.Generation, true); init.Failure != "" {
		t.Fatal(init.Failure)
	}
	uri := (&url.URL{Scheme: "file", Path: filepath.ToSlash(file)}).String()
	prepared := manager.PrepareDocument(context.Background(), DocumentRequest{SessionID: started.SessionID, Generation: started.Generation, URI: uri, LanguageID: "go", CaptureSupply: capture})
	if prepared.Failure != "" {
		t.Fatalf("prepare: %+v", prepared)
	}
	if capture && prepared.Supply == nil {
		t.Fatal("missing completed supply")
	}
	return manager, started, child, OwnedDocumentBinding{URI: uri, Version: prepared.Version, SHA256: fmt.Sprintf("sha256:%x", sha256.Sum256(content))}
}

func methodForOwnedDocument(s StartResult, source OwnedDocumentBinding) RoundTripRequest {
	params, _ := json.Marshal(map[string]any{"textDocument": map[string]string{"uri": source.URI}, "position": map[string]int{"line": 1, "character": 5}, "context": map[string]bool{"includeDeclaration": false}})
	return RoundTripRequest{SessionID: s.SessionID, Generation: s.Generation, Method: "textDocument/references", Params: params, Deadline: time.Now().Add(time.Second), MaxMessages: 1, MaxBytes: 4096, CaptureOwnedMethodPair: true, ExpectedOwnedDocument: &source}
}

func TestOwnedMethodSourceRequiresExactPreparedSupplyBeforeMethod(t *testing.T) {
	for _, tc := range []struct {
		name    string
		capture bool
		change  func(*RoundTripRequest)
	}{
		{"missing-supply", false, func(*RoundTripRequest) {}},
		{"wrong-uri", true, func(r *RoundTripRequest) { r.ExpectedOwnedDocument.URI = "file:///other.go" }},
		{"wrong-version", true, func(r *RoundTripRequest) { r.ExpectedOwnedDocument.Version++ }},
		{"wrong-digest", true, func(r *RoundTripRequest) {
			r.ExpectedOwnedDocument.SHA256 = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
		}},
		{"wrong-query-uri", true, func(r *RoundTripRequest) {
			r.Params = json.RawMessage(`{"textDocument":{"uri":"file:///other.go"},"position":{"line":1,"character":5},"context":{"includeDeclaration":false}}`)
		}},
		{"no-capture", true, func(r *RoundTripRequest) { r.CaptureOwnedMethodPair = false }},
		{"duplicate-uri", true, func(r *RoundTripRequest) {
			r.Params = json.RawMessage(fmt.Sprintf(`{"textDocument":{"uri":"%s","uri":"%s"},"position":{"line":1,"character":5},"context":{"includeDeclaration":false}}`, r.ExpectedOwnedDocument.URI, r.ExpectedOwnedDocument.URI))
		}},
		{"case-alias-uri", true, func(r *RoundTripRequest) {
			r.Params = json.RawMessage(fmt.Sprintf(`{"textDocument":{"URI":"file:///other.go","uri":"%s"},"position":{"line":1,"character":5},"context":{"includeDeclaration":false}}`, r.ExpectedOwnedDocument.URI))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, s, child, source := ownedDocumentFixture(t, tc.capture)
			req := methodForOwnedDocument(s, source)
			tc.change(&req)
			got := m.RoundTrip(context.Background(), req)
			if got.Failure != DocumentSupplyUnavailable || got.Key.ID != 0 {
				t.Fatalf("ASSERT_SOURCE_PRE_DISPATCH_REJECT: %+v", got)
			}
			requests, _ := child.snapshot()
			for _, sent := range requests {
				if sent.Method == "textDocument/references" {
					t.Fatal("ASSERT_SOURCE_NO_METHOD_WRITE")
				}
			}
		})
	}
}

func TestOwnedMethodSourceExactMatchCarriesCheckedCoordinates(t *testing.T) {
	m, s, _, source := ownedDocumentFixture(t, true)
	req := methodForOwnedDocument(s, source)
	got := m.RoundTrip(context.Background(), req)
	pair, ok := got.CompletedOwnedMethodPair()
	if got.Failure != "" || !ok || pair.Source == nil || *pair.Source != source || string(pair.Result) != `[]` {
		t.Fatalf("ASSERT_SOURCE_MATCHED_PAIR: ok=%v pair=%+v failure=%v", ok, pair, got.Failure)
	}
	req.ExpectedOwnedDocument.URI = "file:///changed.go"
	pair.Source.Version++
	copied, present := got.CompletedOwnedMethodPair()
	if !present || copied.Source == nil || *copied.Source != source {
		t.Fatal("ASSERT_SOURCE_PAIR_COPIES_EXPECTATION")
	}
}
