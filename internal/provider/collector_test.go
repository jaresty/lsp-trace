package provider

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

type collectorAdmitter struct {
	calls     int
	selection Selection
	admission Admission
}

func (a *collectorAdmitter) Admit(_ context.Context, s Selection) (Admission, error) {
	a.calls++
	a.selection = s
	return a.admission, nil
}

type collectorExecutor struct {
	calls   int
	id      string
	request json.RawMessage
	limits  Limits
	receipt Receipt
}

func (e *collectorExecutor) Execute(_ context.Context, id string, request json.RawMessage, limits Limits) Receipt {
	e.calls++
	e.id = id
	e.request = append(json.RawMessage(nil), request...)
	e.limits = limits
	return e.receipt
}

type collectorAdapter struct {
	calls   int
	request StrictCollectorRequest
	receipt Receipt
	result  json.RawMessage
}

func (a *collectorAdapter) Adapt(_ context.Context, request StrictCollectorRequest, receipt Receipt) (json.RawMessage, error) {
	a.calls++
	a.request = request
	a.receipt = receipt
	return a.result, nil
}

func TestCollectorOmittedRelationsDoNotAdmitOrResolve(t *testing.T) {
	const assertion = "ASSERT_OMITTED_RELATIONS_NEVER_RESOLVE_OR_START_PROVIDER"
	a := &collectorAdmitter{}
	e := &collectorExecutor{}
	s := &collectorAdapter{}
	c, _ := NewCollector(e, a, s)
	if _, err := c.CollectRelations(context.Background(), nil, json.RawMessage(`{}`)); err == nil || a.calls != 0 || e.calls != 0 || s.calls != 0 {
		t.Fatalf("%s: err=%v admit=%d execute=%d adapt=%d", assertion, err, a.calls, e.calls, s.calls)
	}
	t.Log("PASS " + assertion)
}

func TestCollectorBuildsStrictAdmittedRequestAndIndependentReceipt(t *testing.T) {
	const assertionRequest = "ASSERT_STRICT_PROVIDER_REQUEST_FROM_MANAGED_CUSTODY_AND_LIMITS"
	const assertionAdapter = "ASSERT_SEMANTIC_ADAPTATION_NARROW_INJECTABLE_INTERFACE"
	const assertionReceipt = "ASSERT_PROVIDER_RECEIPTS_ARE_INVOCATION_INDEPENDENT"
	limits := Limits{RequestBytes: 4096, ResponseBytes: 8192, ProtocolMessages: 1, StderrBytes: 64, WallTime: time.Second, TerminationGrace: time.Millisecond}
	a := &collectorAdmitter{admission: Admission{ProviderID: "host-provider@1", AdapterID: "source-adapter@1", Limits: limits}}
	response := json.RawMessage(`{"provider":"raw"}`)
	e := &collectorExecutor{receipt: Receipt{ProviderID: "host-provider@1", Response: response, Messages: 1, Stderr: StderrReceipt{Bytes: []byte("bounded")}, Reaped: true}}
	result := json.RawMessage(`{"provider_id":"host-provider@1","terminal":"COMPLETE_WITHIN_BOUNDS","complete":true,"truncated":false,"bounds":{"max_nodes":20},"relations":[{"relation_id":"r1","kind":"PASSES_CALLBACK"}]}`)
	s := &collectorAdapter{result: result}
	c, _ := NewCollector(e, a, s)
	raw := json.RawMessage(`{"session_id":"managed-session","generation":7,"start_mode":"at","uri":"file:///workspace/component.gts","line":3,"character":4,"relations":["PASSES_CALLBACK"],"adapters":"auto","providers":["host-provider@1"],"workspace_revision":{"kind":"git","commit":"abc","custody":"CALLER_ASSERTED"},"fail_on_unknown_revision":true,"down_depth":2,"up_depth":3,"max_nodes":20,"max_messages":8,"max_bytes":2048,"timeout_ms":500,"request_timeout_ms":100}`)
	got, err := c.CollectRelations(context.Background(), []string{"PASSES_CALLBACK"}, raw)
	if err != nil {
		t.Fatalf("%s: %v", assertionRequest, err)
	}
	var wire StrictCollectorRequest
	if err := json.Unmarshal(e.request, &wire); err != nil {
		t.Fatal(err)
	}
	if e.calls != 1 || e.id != "host-provider@1" || !reflect.DeepEqual(e.limits, limits) || wire.SchemaVersion != CollectorRequestSchema || wire.Session.SessionID != "managed-session" || wire.Session.Generation != 7 || wire.Seed.URI != "file:///workspace/component.gts" || wire.Seed.Line == nil || *wire.Seed.Line != 3 || wire.Documents.OriginalURI != wire.Seed.URI || !wire.Documents.FailOnUnknown || !strings.Contains(string(wire.Documents.WorkspaceRevision), `"commit":"abc"`) || wire.Limits.MaxNodes != 20 || wire.Limits.MaxMessages != 8 || wire.Limits.MaxBytes != 2048 {
		t.Fatalf("%s: calls=%d id=%q limits=%+v wire=%+v", assertionRequest, e.calls, e.id, e.limits, wire)
	}
	t.Log("PASS " + assertionRequest)
	if a.calls != 1 || s.calls != 1 || s.request.ProviderID != "host-provider@1" || s.request.AdapterID != "source-adapter@1" {
		t.Fatalf("%s: admission=%d adapter=%d request=%+v", assertionAdapter, a.calls, s.calls, s.request)
	}
	t.Log("PASS " + assertionAdapter)
	e.receipt.Response[0] = 'X'
	e.receipt.Stderr.Bytes[0] = 'X'
	result[0] = 'X'
	if !json.Valid(got) || !json.Valid(s.receipt.Response) || string(s.receipt.Stderr.Bytes) != "bounded" {
		t.Fatalf("%s: got=%q adapted_receipt=%q stderr=%q", assertionReceipt, got, s.receipt.Response, s.receipt.Stderr.Bytes)
	}
	t.Log("PASS " + assertionReceipt)
}
