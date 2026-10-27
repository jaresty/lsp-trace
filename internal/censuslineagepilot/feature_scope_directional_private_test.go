//go:build darwin || linux

package censuslineagepilot

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"lsp-trace/incomingops"
	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/directionalraw"
	"lsp-trace/internal/lsp"
	"lsp-trace/internal/lspwire"
	"lsp-trace/internal/operation"
	"lsp-trace/sessionruntime"
)

// The existing acquisition producer creates this reservation. This test-only
// runtime wrapper installs exactly the adapter's retained guard, not custody.
// It records the producer's frozen-supply blocker separately from native
// coordinator fixtures; it is not a reservation or receipt constructor.
type scopeDirectionalProducerFixture struct {
	*sessionruntime.Manager
	guard *sessionruntime.RetainedScopeGuardPrivate
}

func (f scopeDirectionalProducerFixture) RoundTripDirectionalPrivate(ctx context.Context, r sessionruntime.RoundTripRequest) (sessionruntime.RoundTripResult, directionalraw.Lease) {
	r.RetainedScopePrivate = f.guard
	return f.Manager.RoundTripDirectionalPrivate(ctx, r)
}
func TestScopeDirectionalPrivateExistingProducerFrozenSupplyBlocker(t *testing.T) {
	restart, m, id, child := adapterFixture(t)
	a, _, err := newScopeRuntimeAdapter(context.Background(), restart, m, id)
	if err != nil {
		t.Fatal(err)
	}
	input, _ := json.Marshal(map[string]any{"session_id": id.SessionID, "generation": id.Generation, "uri": a.files[0].URI, "language_id": "go", "line": 0, "character": 0, "max_depth": 1, "max_nodes": 8, "timeout_ms": 1000, "request_timeout_ms": 1000})
	_, failure, reservation := incomingops.NewExecutor(scopeDirectionalProducerFixture{m, a.guard}).ExecuteDirectionalPrivate(context.Background(), operation.Request{Name: incomingops.OperationIncoming, Input: input}, "scope-existing-producer")
	defer reservation.Release()
	// Do not override PrepareDocument with synthetic success or replace the
	// producer reservation. The frozen Manager boundary legitimately refuses it.
	if failure == nil || failure.Code != string(sessionruntime.DocumentSupplyUnavailable) || reservation.Attached() || len(semanticMessages(child)) != 0 {
		t.Fatalf("ASSERT_EXISTING_PRODUCER_FROZEN_SUPPLY_BLOCKER failure=%v attached=%t", failure, reservation.Attached())
	}
	if len(m.DirectionalObservationsPrivate()) != 0 {
		t.Fatal("ASSERT_EXISTING_PRODUCER_NO_DIRECTIONAL_CUSTODY")
	}
}

func TestScopeDirectionalPrivateNativeBinding(t *testing.T) {
	restart, m, id, child := adapterFixture(t)
	a, _, err := newScopeRuntimeAdapter(context.Background(), restart, m, id)
	if err != nil {
		t.Fatal(err)
	}
	reservation := directionalraw.NewReservation("scope-native-fixture")
	defer reservation.Release()
	ctx := reservation.Context(context.Background())
	zero := uint32(0)
	request := acquisition.Request{Mode: acquisition.Incoming, Context: acquisition.AcquisitionContext{ID: "private", SessionID: id.SessionID, Generation: id.Generation, PositionEncoding: "utf-16"}, Root: acquisition.Target{ID: "root", Locator: acquisition.Locator{URI: a.files[0].URI, LanguageID: "go", Line: &zero, Character: &zero}, DownDepth: 1, UpDepth: 1}, Limits: acquisition.Limits{MaxNodes: 8, MaxRequests: 8, MaxEvidenceBytes: 1 << 20, MaxPathWork: 100, Timeout: time.Second, RequestTimeout: time.Second, MaxResponseBytes: 2 << 20, MaxMessages: 64}}
	result, err := acquisition.Acquire(ctx, a.nativeDirectionalClientPrivate(), request)
	if err != nil || len(result.Targets) != 1 || result.Targets[0].Resolution.Status != acquisition.Resolved {
		t.Fatalf("ASSERT_NATIVE_DIRECTIONAL_RESOLVED: %+v %v", result, err)
	}
	selected, ok := directionalraw.Select(ctx, directionalraw.Request{Method: "callHierarchy/incomingCalls", Ordinal: 0})
	if !ok || !selected.View().SharesBackingPrivate() {
		t.Fatal("ASSERT_NATIVE_DIRECTIONAL_LEXICAL_BINDING")
	}
	if a.requests != 2 || a.captured == 0 || !reservation.Attached() || len(semanticMessages(child)) != 2 {
		t.Fatal("ASSERT_NATIVE_DIRECTIONAL_ACCOUNTING")
	}
	if !reservation.Release() || !reservation.Closed() {
		t.Fatal("ASSERT_NATIVE_DIRECTIONAL_RELEASE")
	}
	active, closed, started, released := m.DirectionalGenericTerminalPrivate()
	if active != 0 || !closed || !started || !released {
		t.Fatal("ASSERT_NATIVE_DIRECTIONAL_MANAGER_RELEASE")
	}
	if selected.View().SharesBackingPrivate() {
		t.Fatal("ASSERT_NATIVE_DIRECTIONAL_VIEW_RETIRED")
	}
}

func TestScopeDirectionalPrivateLexicalFidelity(t *testing.T) {
	for _, method := range []string{"textDocument/prepareCallHierarchy", "callHierarchy/incomingCalls", "callHierarchy/outgoingCalls"} {
		for _, shape := range []string{"null", "empty", "duplicates", "duplicate-key"} {
			t.Run(method+"/"+shape, func(t *testing.T) {
				restart, m, id, child := adapterFixture(t)
				a, _, err := newScopeRuntimeAdapter(context.Background(), restart, m, id)
				if err != nil {
					t.Fatal(err)
				}
				reservation := directionalraw.NewReservation("scope-lexical-fixture")
				defer reservation.Release()
				ctx := reservation.Context(context.Background())
				item := scopeAdapterItem(a.files[0].URI)
				rawItem, _ := json.Marshal(item)
				row := string(rawItem)
				var params any = lsp.PrepareCallHierarchyParams{TextDocument: lsp.TextDocumentIdentifier{URI: item.URI}}
				if method != "textDocument/prepareCallHierarchy" {
					key := "from"
					if method == "callHierarchy/outgoingCalls" {
						key = "to"
					}
					row = `{"` + key + `":` + row + `,"fromRanges":[]}`
					params = lsp.CallHierarchyIncomingCallsParams{Item: item}
				}
				other := strings.Replace(row, `"name":"Query"`, `"name":"Other"`, 1)
				raw := json.RawMessage("[ " + row + ",\n" + other + "," + row + " ]")
				switch shape {
				case "null":
					raw = json.RawMessage(`null`)
				case "empty":
					raw = json.RawMessage(`[]`)
				case "duplicate-key":
					raw = json.RawMessage(`[{"name":"a","name":"b"}]`)
				}
				child.mu.Lock()
				child.response = func(lspwire.Message) json.RawMessage { return raw }
				child.mu.Unlock()
				p, _ := json.Marshal(params)
				request := acquisition.WireRequest{Context: acquisition.AcquisitionContext{SessionID: id.SessionID, Generation: id.Generation}, Method: method, Params: p, MaxBytes: 2 << 20, MaxMessages: 64}
				var lease directionalraw.Lease
				err = a.invoke(func() error {
					var e error
					lease, e = a.nativeDirectionalRoundTripLocked(ctx, request)
					if e != nil {
						return e
					}
					return acquisition.BindDirectionalResponse(lease)
				})
				if shape == "duplicate-key" {
					if err == nil || a.captured != len(raw) || a.terminal == nil {
						t.Fatal("ASSERT_DIRECTIONAL_DUPLICATE_KEYS_STRICT_REFUSAL")
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				if lease.Null() != (shape == "null") {
					t.Fatal("ASSERT_DIRECTIONAL_NULL_DISTINCT_EMPTY")
				}
				// The fixture's JSON-RPC writer compacts RawMessage before wire.
				emitted, _ := json.Marshal(raw)
				if a.captured != len(emitted) {
					t.Fatalf("ASSERT_DIRECTIONAL_EXACT_RESULT_CHARGE got=%d wire=%d", a.captured, len(emitted))
				}
				for i := uint64(0); i < 3; i++ {
					selection, ok := directionalraw.Select(ctx, directionalraw.Request{Method: method, Ordinal: i})
					if shape != "duplicates" {
						if ok {
							t.Fatal("ASSERT_DIRECTIONAL_EMPTY_HAS_NO_ELEMENTS")
						}
						continue
					}
					lexical, appendOK := selection.AppendToPrivate(nil)
					expected := row
					if i == 1 {
						expected = other
					}
					if !ok || !appendOK || !selection.View().SharesBackingPrivate() || string(lexical) != expected {
						t.Fatal("ASSERT_DIRECTIONAL_DUPLICATE_ORDER_LEXICAL")
					}
				}
				messages := semanticMessages(child)
				if len(messages) != 1 || string(messages[0].Params) != string(p) {
					t.Fatal("ASSERT_DIRECTIONAL_FULL_ITEM_DATA_REQUEST")
				}
				if !reservation.Release() {
					t.Fatal("ASSERT_DIRECTIONAL_LEXICAL_RELEASE")
				}
			})
		}
	}
}

func TestScopeDirectionalPrivateWireBoundsAndTerminal(t *testing.T) {
	for _, kind := range []string{"bytes", "messages", "missing-reservation", "missing-guard", "external", "malformed", "budget"} {
		t.Run(kind, func(t *testing.T) {
			restart, m, id, child := adapterFixture(t)
			a, _, err := newScopeRuntimeAdapter(context.Background(), restart, m, id)
			if err != nil {
				t.Fatal(err)
			}
			reservation := directionalraw.NewReservation("scope-wire-fixture")
			defer reservation.Release()
			ctx := reservation.Context(context.Background())
			uri := a.files[0].URI
			if kind == "external" {
				uri = "file:///external.go"
			}
			params, _ := json.Marshal(lsp.PrepareCallHierarchyParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}})
			r := acquisition.WireRequest{Context: acquisition.AcquisitionContext{SessionID: id.SessionID, Generation: id.Generation}, Method: "textDocument/prepareCallHierarchy", Params: params, MaxBytes: 2 << 20, MaxMessages: 64}
			switch kind {
			case "bytes":
				r.MaxBytes = scopeMinimumResponseBody - 1
			case "messages":
				r.MaxMessages = 0
			case "missing-reservation":
				ctx = context.Background()
			case "missing-guard":
				a.guard = nil
			case "malformed":
				child.mu.Lock()
				child.response = func(_ lspwire.Message) json.RawMessage { return json.RawMessage(`[{"name":"missing fields"}]`) }
				child.mu.Unlock()
			case "budget":
				r.MaxBytes = scopeMinimumResponseBody
			}
			before := len(child.snapshot())
			var lease directionalraw.Lease
			err = a.invoke(func() error { var e error; lease, e = a.nativeDirectionalRoundTripLocked(ctx, r); return e })
			if err == nil {
				t.Fatal("ASSERT_DIRECTIONAL_REFUSAL")
			}
			consumed := kind == "malformed" || kind == "budget"
			if !consumed && (len(child.snapshot()) != before || reservation.Attached()) {
				t.Fatal("ASSERT_DIRECTIONAL_PREFLIGHT_ZERO_TRAFFIC")
			}
			if consumed {
				if a.terminal == nil || len(child.snapshot()) == before {
					t.Fatal("ASSERT_DIRECTIONAL_CONSUMED_TERMINAL")
				}
				n := len(child.snapshot())
				_ = a.invoke(func() error { _, e := a.nativeDirectionalRoundTripLocked(ctx, r); return e })
				if len(child.snapshot()) != n {
					t.Fatal("ASSERT_DIRECTIONAL_NO_RETRY")
				}
				if kind == "malformed" && a.captured == 0 {
					t.Fatal("ASSERT_DIRECTIONAL_MALFORMED_CHARGED")
				}
			}
			if consumed {
				_, _, allocations, _, _, _ := m.DirectionalAccountingPrivate()
				prepLive, _, prepAllocations, _, _ := m.DirectionalPreparationAccountingPrivate()
				if allocations+prepAllocations == 0 {
					t.Fatal("ASSERT_DIRECTIONAL_FAILURE_MANAGER_OWNERSHIP")
				}
				_ = prepLive
				if !reservation.Release() || !reservation.Closed() {
					t.Fatal("ASSERT_DIRECTIONAL_FAILURE_RELEASE")
				}
				if _, ok := lease.CompletedMetadataPrivate(); ok {
					t.Fatal("ASSERT_DIRECTIONAL_FAILURE_LEASE_RETIRED")
				}
			}
		})
	}
}
