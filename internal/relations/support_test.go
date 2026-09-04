package relations

import (
	"reflect"
	"strings"
	"testing"
)

func TestMinimumDependenceValidatesUpstreamGraph(t *testing.T) {
	tests := []struct {
		name string
		in   []Observation
		want string
	}{
		{"missing parent", []Observation{{ID: "child", UpstreamObservationIDs: []string{"missing"}}}, "missing upstream observation"},
		{"cycle", []Observation{{ID: "a", UpstreamObservationIDs: []string{"b"}}, {ID: "b", UpstreamObservationIDs: []string{"a"}}}, "cyclic upstream observations"},
		{"duplicate id", []Observation{{ID: "a"}, {ID: "a"}}, "duplicate observation id"},
		{"empty id", []Observation{{ID: ""}}, "missing observation id"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := MinimumDependence(tt.in)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("ASSERT_UPSTREAM_GRAPH_COMPLETE_ACYCLIC: got %v, want %q", err, tt.want)
			}
		})
	}
}

func TestMinimumDependenceComputesCanonicalComponents(t *testing.T) {
	in := []Observation{
		{ID: "g", ReceiptID: "r2"},
		{ID: "f", UpstreamObservationIDs: []string{"e"}},
		{ID: "e", UpstreamObservationIDs: []string{"d"}},
		{ID: "d"},
		{ID: "c", ResponseID: "p", ReceiptID: "r2"},
		{ID: "b", ExecutionID: "x", ResponseID: "p"},
		{ID: "a", ExecutionID: "x"},
		{ID: "z"},
	}
	want := [][]string{{"a", "b", "c", "g"}, {"d", "e", "f"}, {"z"}}
	got, err := MinimumDependence(in)
	if err != nil {
		t.Fatalf("ASSERT_MINIMUM_DEPENDENCE_COMPONENTS: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ASSERT_MINIMUM_DEPENDENCE_COMPONENTS: got %#v, want %#v", got, want)
	}
}

func TestMinimumDependenceDoesNotGroupEmptyKeys(t *testing.T) {
	got, err := MinimumDependence([]Observation{{ID: "b"}, {ID: "a"}})
	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{{"a"}, {"b"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ASSERT_EMPTY_PROVENANCE_NOT_SHARED: got %#v, want %#v", got, want)
	}
}
