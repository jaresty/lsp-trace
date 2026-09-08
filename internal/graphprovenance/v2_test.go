package graphprovenance

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/lsp"
)

func coordinatorV2Fixture(t *testing.T, configure func(*acquisition.Request)) (acquisition.Result, string) {
	t.Helper()
	root := t.TempDir()
	uri := (&url.URL{Scheme: "file", Path: filepath.Join(root, "a.go")}).String()
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("package p\n"), 0600); err != nil {
		t.Fatal(err)
	}
	line, char := uint32(0), uint32(0)
	target := acquisition.Target{ID: "root", Locator: acquisition.Locator{URI: uri, Line: &line, Character: &char}, DownDepth: 1, UpDepth: 1}
	r := acquisition.Request{Mode: acquisition.Slice, Context: acquisition.AcquisitionContext{ID: "context", SessionID: "session", Generation: 1, PositionEncoding: "utf-16"}, Root: target, Limits: acquisition.Limits{MaxNodes: 10000, MaxRequests: 100, MaxEvidenceBytes: 1 << 20, MaxPathWork: 1000, Timeout: time.Second, RequestTimeout: time.Second, MaxResponseBytes: 1 << 20, MaxMessages: 64}}
	alias := target
	alias.ID = "alias"
	r.RequiredTargets = []acquisition.Target{alias}
	if configure != nil {
		configure(&r)
	}
	client := acquisition.NewWireClient(func(_ context.Context, request acquisition.WireRequest) (json.RawMessage, error) {
		if request.Method == "textDocument/documentSymbol" && r.Root.Locator.Symbol == "Ambiguous" {
			s := lsp.DocumentSymbol{Name: "Ambiguous", Kind: 12, Range: lsp.Range{End: lsp.Position{Character: 10}}, SelectionRange: lsp.Range{End: lsp.Position{Character: 1}}}
			return json.Marshal([]lsp.DocumentSymbol{s, s})
		}
		if request.Method == "textDocument/prepareCallHierarchy" {
			return json.Marshal([]lsp.CallHierarchyItem{{Name: "A", Kind: 12, URI: uri, Range: lsp.Range{End: lsp.Position{Character: 10}}, SelectionRange: lsp.Range{End: lsp.Position{Character: 1}}, Data: json.RawMessage(`{"large":9007199254740993,"uri":"file:///opaque-not-a-source.go"}`)}})
		}
		return json.RawMessage(`[]`), nil
	})
	result, err := acquisition.Acquire(context.Background(), client, r)
	if err != nil {
		t.Fatal(err)
	}
	return result, root
}

func TestV2CaptureVertical(t *testing.T) {
	result, root := coordinatorV2Fixture(t, nil)
	out, err := CaptureV2(context.Background(), result, root)
	if err != nil {
		t.Fatalf("ASSERT_V2_CAPTURE_VERTICAL: %v", err)
	}
	var envelope struct {
		GraphBytes  []byte          `json:"graph_bytes"`
		Acquisition json.RawMessage `json:"acquisition"`
		Captures    []Receipt       `json:"captures"`
	}
	if err := json.Unmarshal(out, &envelope); err != nil {
		t.Fatal(err)
	}
	native, _ := json.Marshal(result.Graph)
	if !bytes.Equal(native, envelope.GraphBytes) || len(envelope.Acquisition) == 0 || len(envelope.Captures) != 1 {
		t.Fatal("ASSERT_V2_CAPTURE_VERTICAL: evidence lost")
	}
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateFor(out, Family, "v2"); err != nil {
		t.Fatalf("ASSERT_V2_CAPTURE_VERTICAL: offline admission: %v", err)
	}
	t.Log("ASSERT_V2_CAPTURE_VERTICAL: PASS")
}
