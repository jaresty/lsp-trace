package operation

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"sync"
	"testing"

	semanticfilter "lsp-trace/internal/filter"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/inspection"
)

func admittedInspectionBytes(t *testing.T) []byte {
	t.Helper()
	n1 := graph.NewNode(graph.Item{Name: "one", URI: "file:///one.go"})
	n2 := graph.NewNode(graph.Item{Name: "two", URI: "file:///two.go"})
	edges := graph.MergeEdge(nil, graph.Edge{CallerNodeID: n1.ID, CalleeNodeID: n2.ID})
	bundle := inspection.Bundle{
		SchemaVersion: graph.SchemaVersionV3,
		Invocation:    graph.Invocation{Seeds: []graph.InvocationSeed{{Label: "second"}, {Label: "first"}}},
		Nodes:         []graph.Node{n1, n2},
		Edges:         edges,
		Seeds: []graph.SeedResult{
			{Label: "first", ReachedNodeIDs: []string{n1.ID}},
			{Label: "second", ReachedNodeIDs: []string{n2.ID}, ReachedRelationIDs: []string{edges[0].RelationID}},
		},
	}
	graphBytes, err := json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	projection, err := inspection.ProjectAllSeeds(graphBytes)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(projection)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func filterRequest(t *testing.T, inspectionBytes []byte, labels ...string) Request {
	t.Helper()
	input, err := json.Marshal(struct {
		Input  json.RawMessage `json:"input"`
		Filter struct {
			CompareSeeds []string `json:"compare_seeds"`
		} `json:"filter"`
	}{
		Input: inspectionBytes,
		Filter: struct {
			CompareSeeds []string `json:"compare_seeds"`
		}{CompareSeeds: labels},
	})
	if err != nil {
		t.Fatal(err)
	}
	return Request{Name: Filter, RequestID: "filter-request", Input: input}
}

func TestFilterHandlerPreservesPairwiseProjectionAndExactBytes(t *testing.T) {
	const assertion = "ASSERT_FILTER_HANDLER_EXACT_PAIRWISE_PARITY"
	t.Log("ASSERTION: " + assertion)
	inspectionBytes := admittedInspectionBytes(t)
	handler := NewFilterHandler()
	objectRequest := filterRequest(t, inspectionBytes, "second", "first")
	var stringInput map[string]any
	if err := json.Unmarshal(objectRequest.Input, &stringInput); err != nil {
		t.Fatal(err)
	}
	stringInput["input"] = string(inspectionBytes)
	stringBytes, err := json.Marshal(stringInput)
	if err != nil {
		t.Fatal(err)
	}
	requests := []Request{objectRequest, {Name: Filter, RequestID: "filter-string-request", Input: stringBytes}}

	want, err := semanticfilter.ProjectPairwise(inspectionBytes, "second", "first")
	if err != nil {
		t.Fatal(err)
	}
	wantBytes, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	wantBytes = append(wantBytes, '\n')
	for _, request := range requests {
		got, failure := handler(context.Background(), request)
		if failure != nil {
			t.Fatalf("%s: request=%s failure=%v", assertion, request.RequestID, failure)
		}
		if !reflect.DeepEqual(got.Value, want) || !bytes.Equal(got.Artifact, wantBytes) {
			t.Fatalf("%s: request=%s value_equal=%t artifact=%q want=%q", assertion, request.RequestID, reflect.DeepEqual(got.Value, want), got.Artifact, wantBytes)
		}
	}
}

func TestFilterHandlerRejectsNonPairwiseLabelsDeterministically(t *testing.T) {
	const assertion = "ASSERT_FILTER_HANDLER_PAIRWISE_CONSTRAINTS"
	t.Log("ASSERTION: " + assertion)
	handler := NewFilterHandler()
	inspectionBytes := admittedInspectionBytes(t)
	cases := [][]string{nil, {"first"}, {"first", ""}, {"first", "first"}, {"first", "second", "third"}}
	for _, labels := range cases {
		_, failure := handler(context.Background(), filterRequest(t, inspectionBytes, labels...))
		if failure == nil || failure.Code != "INVALID_INPUT" {
			t.Fatalf("%s: labels=%q failure=%#v", assertion, labels, failure)
		}
	}
}

func TestFilterHandlerConcurrentOutputIsDeterministic(t *testing.T) {
	const assertion = "ASSERT_FILTER_HANDLER_CONCURRENT_DETERMINISM"
	t.Log("ASSERTION: " + assertion)
	handler := NewFilterHandler()
	request := filterRequest(t, admittedInspectionBytes(t), "second", "first")
	const workers = 32
	outputs := make([][]byte, workers)
	failures := make([]*Failure, workers)
	var wg sync.WaitGroup
	for i := range outputs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			result, failure := handler(context.Background(), request)
			outputs[i] = result.Artifact
			failures[i] = failure
		}(i)
	}
	wg.Wait()
	for i := range outputs {
		if failures[i] != nil || !bytes.Equal(outputs[i], outputs[0]) {
			t.Fatalf("%s: worker=%d failure=%v differs=%t", assertion, i, failures[i], !bytes.Equal(outputs[i], outputs[0]))
		}
	}
}
