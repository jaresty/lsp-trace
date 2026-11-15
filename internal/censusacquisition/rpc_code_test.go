package censusacquisition

import (
	"bytes"
	"context"
	"encoding/json"
	"lsp-trace/internal/discoveryfailure"
	"testing"
)

// A dynamic closed-detail interface keeps this behavioral RED compilable before
// production adopts the category vocabulary. Its Error must never be inspected.
type rpc72Typed struct{ prepare66Typed }

func (rpc72Typed) PrepareFailureRPCCodeCategory() uint8 { return 6 }

func TestRPC72AdapterRetention(t *testing.T) {
	d, err := (DiscoveryAdapter{Workspace: "/offline", Files: StaticFiles{{Path: "a.go", URI: "file:///offline/a.go", LanguageID: "go"}}, Supplier: prepare66Supplier{}, Client: prepare66Client{err: rpc72Typed{prepare66Typed{discoveryfailure.RPCServer}}}}).Discover(context.Background(), SessionIdentity{"offline", 1})
	if err != nil || len(d.ProviderFailures) != 1 {
		t.Fatal("RPC72_ADAPTER_REACHED")
	}
	raw, err := json.Marshal(d)
	if err != nil || !bytes.Contains(raw, []byte(`"ServerCodeCategory":"SERVER_NOT_INITIALIZED"`)) {
		t.Fatal("RPC72_CATEGORY_RETAINED")
	}
	t.Log("RPC72_CATEGORY_RETAINED")
	cloned := cloneDiscovery(d)
	raw, err = json.Marshal(cloned)
	if err != nil {
		t.Fatal(err)
	}
	var decoded Discovery
	if json.Unmarshal(raw, &decoded) != nil || decoded.ProviderFailures[0].ServerCodeCategory != discoveryfailure.ServerNotInitialized || ValidateTerminalDiscovery(decoded, "UNKNOWN") != nil {
		t.Fatal("RPC72_CLONE_JSON_VALIDATION")
	}
	cloned.ProviderFailures[0].ServerCodeCategory = discoveryfailure.InternalError
	if d.ProviderFailures[0].ServerCodeCategory != discoveryfailure.ServerNotInitialized {
		t.Fatal("RPC72_CLONE_INDEPENDENT")
	}
}

func TestRPC72MetadataValidation(t *testing.T) {
	old := []byte(`{"Stage":"PREPARE_CALL_HIERARCHY","Code":"REQUEST_FAILED","FileOrdinal":null,"SymbolOrdinal":0}`)
	var legacy ProviderFailure
	if json.Unmarshal(old, &legacy) != nil {
		t.Fatal("RPC72_LEGACY_DECODE")
	}
	raw, err := json.Marshal(legacy)
	if err != nil || !bytes.Equal(raw, old) {
		t.Fatal("RPC72_LEGACY_EXACT_BYTES")
	}
	for _, category := range []discoveryfailure.ServerCodeCategory{discoveryfailure.ServerCodeUnknown, discoveryfailure.ParseError, discoveryfailure.InvalidRequest, discoveryfailure.MethodNotFound, discoveryfailure.InvalidParams, discoveryfailure.InternalError, discoveryfailure.ServerNotInitialized, discoveryfailure.UnknownErrorCode, discoveryfailure.RequestCancelled, discoveryfailure.ContentModified, discoveryfailure.ServerCancelled, discoveryfailure.RequestFailed, discoveryfailure.ServerCodeOther} {
		f := legacy
		f.PrepareCause = discoveryfailure.RPCServer
		f.ServerCodeCategory = category
		raw, err := json.Marshal(f)
		var back ProviderFailure
		if err != nil || json.Unmarshal(raw, &back) != nil || back.ServerCodeCategory != category {
			t.Fatal("RPC72_METADATA_ROUNDTRIP")
		}
		if category != discoveryfailure.ServerCodeUnknown && bytes.Equal(raw, old) {
			t.Fatal("RPC72_KNOWN_AFFECTS_BYTES")
		}
	}
	for _, cause := range []discoveryfailure.Cause{discoveryfailure.Unknown, discoveryfailure.Canceled, discoveryfailure.Deadline, discoveryfailure.EnclosureBudget, discoveryfailure.SessionUnavailable, discoveryfailure.SessionCrashed, discoveryfailure.InvalidResponse} {
		f := legacy
		f.PrepareCause = cause
		f.ServerCodeCategory = discoveryfailure.InternalError
		if _, err := json.Marshal(f); err == nil || f.Validate() == nil {
			t.Fatal("RPC72_CONTRADICTION_ENCODE")
		}
		raw := []byte(`{"Stage":"PREPARE_CALL_HIERARCHY","Code":"REQUEST_FAILED","PrepareCause":`)
		c, _ := json.Marshal(cause)
		raw = append(raw, c...)
		raw = append(raw, []byte(`,"ServerCodeCategory":"INTERNAL_ERROR"}`)...)
		if json.Unmarshal(raw, &f) == nil {
			t.Fatal("RPC72_CONTRADICTION_DECODE")
		}
	}
	for _, value := range []string{`"FUTURE"`, `"secret://payload"`, `null`, `0`, `""`, `{}`} {
		raw := []byte(`{"Stage":"PREPARE_CALL_HIERARCHY","Code":"REQUEST_FAILED","PrepareCause":"RPC_SERVER_ERROR","ServerCodeCategory":` + value + `}`)
		var f ProviderFailure
		if json.Unmarshal(raw, &f) == nil {
			t.Fatal("RPC72_INVALID_CATEGORY_DECODE")
		}
	}
	legacy.PrepareCause = discoveryfailure.RPCServer
	legacy.ServerCodeCategory = 255
	if _, err := json.Marshal(legacy); err == nil {
		t.Fatal("RPC72_INVALID_CATEGORY_ENCODE")
	}
}

func TestRPC72CanceledDispositionKeepsProducerDetail(t *testing.T) {
	terminal, cancel := context.WithCancel(context.Background())
	cancel()
	boundary := &prepare68BoundaryContext{context.Background()}
	client := prepare68BoundaryClient{prepare66Client: prepare66Client{err: rpc72Typed{prepare66Typed{discoveryfailure.RPCServer}}}, boundary: boundary, terminal: terminal}
	d, err := (DiscoveryAdapter{Workspace: "/offline", Files: StaticFiles{{Path: "a.go", URI: "file:///offline/a.go", LanguageID: "go"}}, Supplier: prepare66Supplier{}, Client: client}).Discover(boundary, SessionIdentity{"offline", 1})
	if err != nil || len(d.ProviderFailures) != 1 {
		t.Fatal("RPC72_CANCEL_BRANCH")
	}
	f := d.ProviderFailures[0]
	if f.Code != "CANCELED" || f.PrepareCause != discoveryfailure.RPCServer || f.ServerCodeCategory != discoveryfailure.ServerNotInitialized || ValidateTerminalDiscovery(d, "UNKNOWN") != nil {
		t.Fatal("RPC72_DISPOSITION_NOT_CAUSE_OVERRIDE")
	}
}
