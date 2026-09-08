package graphprovenance

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"lsp-trace/internal/acquisition"
	"lsp-trace/internal/graph"
)

func TestV2RawCarrierLimits(t *testing.T) {
	r, root := coordinatorV2Fixture(t, nil)
	raw, err := CaptureV2(context.Background(), r, root)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]json.RawMessage
	_ = json.Unmarshal(raw, &doc)
	doc["graph_bytes"], _ = json.Marshal([]byte(strings.Repeat("[", 65) + "0" + strings.Repeat("]", 65)))
	bad, _ := json.Marshal(doc)
	bad = append([]byte(`{"schema_version":"duplicate",`), bad[1:]...)
	if _, err := ValidateFor(bad, Family, "v2"); err == nil || !strings.Contains(err.Error(), "depth limit") {
		t.Fatalf("ASSERT_V2_DECODED_GRAPH_DEPTH_FIRST: %v", err)
	}
	nodes := `{"nodes":[` + strings.Repeat(`{},`, 10000) + `{}]}`
	if err := preflightGraphV2([]byte(nodes)); err == nil || !strings.Contains(err.Error(), "row limit") {
		t.Fatalf("ASSERT_V2_RAW_NODE_LIMIT: %v", err)
	}
	e := EvidenceV2{Captures: []Receipt{{URI: "file:///a", ID: strings.Repeat("x", 71)}}, Bindings: make([]BindingV2, MaxBindings)}
	for i := range e.Bindings {
		e.Bindings[i] = BindingV2{Pointer: "/graph/" + strings.Repeat("x", 64), URI: "file:///a", Attribution: "SOURCE"}
	}
	if err := bindV2(&e); err == nil {
		t.Fatal("ASSERT_V2_BINDING_AMPLIFICATION_LIMIT")
	}
	for i := range e.Bindings {
		e.Bindings[i].Attribution = "NON_SOURCE"
		e.Bindings[i].URI = ""
	}
	if err := bindV2(&e); err == nil {
		t.Fatal("ASSERT_V2_NON_SOURCE_BINDING_LIMIT")
	}
	t.Log("ASSERT_V2_DECODED_GRAPH_DEPTH_FIRST: PASS; ASSERT_V2_RAW_NODE_LIMIT: PASS; ASSERT_V2_BINDING_AMPLIFICATION_LIMIT: PASS; ASSERT_V2_NON_SOURCE_BINDING_LIMIT: PASS")
}

func TestV2CensusNamespaces(t *testing.T) {
	r, _ := coordinatorV2Fixture(t, func(r *acquisition.Request) {
		n := graph.NewNode(graph.Item{Name: "A", Kind: 12, URI: r.Root.Locator.URI, Range: graph.Range{End: graph.Position{Character: 10}}, SelectionRange: graph.Range{End: graph.Position{Character: 1}}})
		r.RequiredTargets[0].ID = n.ID
	})
	if r.Request.RequiredTargets[0].ID != r.Graph.Nodes[0].ID {
		t.Fatal("fixture namespace collision missing")
	}
	bindings, err := CensusV2(r)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, b := range bindings {
		if b.Pointer == "/acquisition/request/required_targets/0/id" {
			found = true
			if b.Attribution != "NON_SOURCE" || b.URI != "" {
				t.Fatal("ASSERT_V2_TYPED_ID_NAMESPACE")
			}
		}
	}
	if !found {
		t.Fatal("ASSERT_V2_TYPED_ID_NAMESPACE_MISSING")
	}
	t.Log("ASSERT_V2_TYPED_ID_NAMESPACE: PASS")
}

func TestV2DirectCensusAdmission(t *testing.T) {
	r, _ := coordinatorV2Fixture(t, nil)
	r.Targets = r.Targets[:1]
	if _, err := CensusV2(r); err == nil {
		t.Fatal("ASSERT_V2_DIRECT_CENSUS_ACQUISITION_ADMISSION")
	}
	t.Log("ASSERT_V2_DIRECT_CENSUS_ACQUISITION_ADMISSION: PASS")
}

func TestV2SupplyNumberPrecision(t *testing.T) {
	large := int64(9007199254740993)
	receipt := Receipt{URI: "file:///a", Content: []byte("x"), Supply: &SupplyMetadata{Method: "textDocument/didChange", Version: int(large), Params: json.RawMessage(`{"textDocument":{"uri":"file:///a","version":9007199254740993},"contentChanges":[{"text":"x"}]}`)}}
	if err := validateSupplyV2(&receipt); err != nil {
		t.Fatal("ASSERT_V2_EXACT_SUPPLY_INTEGER:", err)
	}
	receipt.Supply.Params = json.RawMessage(`{"textDocument":{"uri":"file:///a","version":9007199254740992},"contentChanges":[{"text":"x"}]}`)
	if err := validateSupplyV2(&receipt); err == nil {
		t.Fatal("ASSERT_V2_SUPPLY_INTEGER_NEIGHBOR_REJECT")
	}
	t.Log("ASSERT_V2_EXACT_SUPPLY_INTEGER: PASS; ASSERT_V2_SUPPLY_INTEGER_NEIGHBOR_REJECT: PASS")
}

func TestV2NoNotificationObservationRemainsAbsent(t *testing.T) {
	base, root := coordinatorV2Fixture(t, nil)
	wire := acquisition.NewWireClient(func(context.Context, acquisition.WireRequest) (json.RawMessage, error) {
		return json.RawMessage(`[]`), nil
	})
	client := acquisition.WithDocumentSupply(wire, func(_ context.Context, _ acquisition.AcquisitionContext, l acquisition.Locator) (acquisition.Supply, error) {
		return acquisition.Supply{URI: l.URI}, nil
	})
	r, err := acquisition.Acquire(context.Background(), client, base.Request)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := CaptureV2(context.Background(), r, root)
	if err != nil {
		t.Fatal(err)
	}
	var e EvidenceV2
	if err := json.Unmarshal(raw, &e); err != nil {
		t.Fatal(err)
	}
	if len(e.Supplies) != 1 || e.Supplies[0].Receipt != nil || e.Supplies[0].Status != "NO_NOTIFICATION_OBSERVATION" {
		t.Fatal("ASSERT_V2_ABSENT_NOTIFICATION")
	}
	t.Log("ASSERT_V2_ABSENT_NOTIFICATION: PASS")
}
