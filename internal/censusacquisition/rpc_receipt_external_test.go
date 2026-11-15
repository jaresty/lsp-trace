package censusacquisition_test

import (
	"bytes"
	"context"
	"encoding/json"
	a "lsp-trace/internal/censusacquisition"
	assembly "lsp-trace/internal/censussourceassembly"
	continuation "lsp-trace/internal/censussourcecontinuation"
	plan "lsp-trace/internal/censussourceplan"
	"lsp-trace/internal/discoveryfailure"
	"lsp-trace/internal/lsp"
	"testing"
)

type receipt72Client struct{}

func (receipt72Client) SupportsDocumentSymbols() bool { return true }
func (receipt72Client) SupportsCallHierarchy() bool   { return true }
func (receipt72Client) DocumentSymbols(context.Context, lsp.DocumentSymbolParams) ([]lsp.DocumentSymbol, error) {
	r := lsp.Range{End: lsp.Position{Character: 1}}
	return []lsp.DocumentSymbol{{Name: "Run", Kind: 12, Range: r, SelectionRange: r}}, nil
}
func (receipt72Client) PrepareCallHierarchy(context.Context, lsp.PrepareCallHierarchyParams) ([]lsp.CallHierarchyItem, error) {
	return nil, receipt72Error{}
}

type receipt72Error struct{}

func (receipt72Error) Error() string                               { panic("native Error must not be inspected") }
func (receipt72Error) PrepareFailureCause() discoveryfailure.Cause { return discoveryfailure.RPCServer }
func (receipt72Error) PrepareFailureRPCCodeCategory() uint8 {
	return uint8(discoveryfailure.InternalError)
}

type receipt72Supplier struct{}

func (receipt72Supplier) Supply(context.Context, a.SourceFile) error { return nil }

func TestRPC72CanonicalReceipt(t *testing.T) {
	files := []a.SourceFile{{Path: "a.go", URI: "file:///offline/a.go", LanguageID: "go"}}
	p, err := plan.Build(files, "/offline", 1)
	if err != nil {
		t.Fatal(err)
	}
	d, err := (a.DiscoveryAdapter{Workspace: "/offline", Files: a.StaticFiles(files), Supplier: receipt72Supplier{}, Client: receipt72Client{}}).Discover(context.Background(), a.SessionIdentity{SessionID: "offline", Generation: 1})
	if err != nil {
		t.Fatal(err)
	}
	record, err := assembly.NewTerminalRecord(p, 1, 0, d)
	if err != nil {
		t.Fatal(err)
	}
	terminal, err := assembly.AdaptTerminal(p, 1, record)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := continuation.EncodeReceipt("offline", 1, "offline-claim", "offline-idempotency", terminal)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := continuation.DecodeReceipt(raw, "offline", p, 0)
	if err != nil || decoded.Authority != 0 || decoded.Outcome != continuation.OutcomeUnknown || decoded.Discovery.ProviderFailures[0].ServerCodeCategory != discoveryfailure.InternalError {
		t.Fatal("RPC72_CANONICAL_RECEIPT_CATEGORY", err)
	}
	again, err := continuation.EncodeReceipt("offline", 1, "offline-claim", "offline-idempotency", decoded)
	if err != nil || !bytes.Equal(raw, again) {
		t.Fatal("RPC72_CANONICAL_RECEIPT_BYTES")
	}
	// An old receipt has no category; remove only the optional field and require
	// the current canonical decoder/encoder to reproduce those exact bytes.
	old := bytes.Replace(raw, []byte(`,"ServerCodeCategory":"INTERNAL_ERROR"`), nil, 1)
	oldTerminal, err := continuation.DecodeReceipt(old, "offline", p, 0)
	if err != nil {
		t.Fatal("RPC72_OLD_RECEIPT_DECODE", err)
	}
	oldAgain, err := continuation.EncodeReceipt("offline", 1, "offline-claim", "offline-idempotency", oldTerminal)
	if err != nil || !bytes.Equal(old, oldAgain) {
		t.Fatal("RPC72_OLD_RECEIPT_EXACT_BYTES")
	}
	bad := bytes.Replace(raw, []byte(`"PrepareCause":"RPC_SERVER_ERROR"`), []byte(`"PrepareCause":"ENCLOSURE_BUDGET"`), 1)
	if _, err := continuation.DecodeReceipt(bad, "offline", p, 0); err == nil {
		t.Fatal("RPC72_CANONICAL_CONTRADICTION_REJECTED")
	}
	terminal.Discovery.ProviderFailures[0].PrepareCause = discoveryfailure.EnclosureBudget
	if _, err := continuation.EncodeReceipt("offline", 1, "offline-claim", "offline-idempotency", terminal); err == nil {
		t.Fatal("RPC72_CANONICAL_INVALID_ENCODE")
	}
	// New diagnostic field cannot carry raw data or silently extend the record.
	var failure a.ProviderFailure
	if json.Unmarshal([]byte(`{"Stage":"PREPARE_CALL_HIERARCHY","Code":"REQUEST_FAILED","PrepareCause":"RPC_SERVER_ERROR","Data":"secret"}`), &failure) == nil {
		t.Fatal("RPC72_RAW_EXTENSION_REJECTED")
	}
	t.Log("RPC72_CANONICAL_RECEIPT_CATEGORY")
}
