package filter

import (
	"reflect"
	"strings"
	"sync"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/schema"
)

func TestProjectPairwiseRejectsNonInspectionInput(t *testing.T) {
	_, err := ProjectPairwise([]byte(`{}`), "left", "right")
	if err == nil || !strings.Contains(err.Error(), "input must be lsp-trace.inspect.v1 ALL_SEEDS") {
		t.Fatalf("ASSERT_PAIRWISE_INPUT_ADMISSION: err=%v", err)
	}
}

func TestPartitionsPreserveNamespaceOrderAccountingAndReversal(t *testing.T) {
	globals := []string{"outside-before", "left", "shared", "right", "outside-after"}
	left, right := []string{"shared", "left"}, []string{"right", "shared"}
	want := StringPartition{Shared: []string{"shared"}, LeftOnly: []string{"left"}, RightOnly: []string{"right"}}
	for _, ns := range []schema.ReferenceNamespace{schema.ReferenceNode, schema.ReferenceCallRelation, schema.ReferenceDispatchRelationship, schema.ReferenceSiblingCandidate} {
		got := partitionStrings(ns, globals, left, right)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("ASSERT_PAIRWISE_NAMESPACE_ORDER: namespace=%s got=%#v", ns, got)
		}
		account := accountStrings(got, len(left), len(right))
		if account.SharedReferenceCount+account.LeftOnlyReferenceCount != account.LeftReferenceCount || account.SharedReferenceCount+account.RightOnlyReferenceCount != account.RightReferenceCount || account.SharedReferenceCount+account.LeftOnlyReferenceCount+account.RightOnlyReferenceCount != account.PairUniverseCount {
			t.Fatalf("ASSERT_PAIRWISE_ACCOUNTING: namespace=%s accounting=%#v", ns, account)
		}
		reversed := partitionStrings(ns, globals, right, left)
		if !reflect.DeepEqual(reversed.Shared, got.Shared) || !reflect.DeepEqual(reversed.LeftOnly, got.RightOnly) || !reflect.DeepEqual(reversed.RightOnly, got.LeftOnly) {
			t.Fatalf("ASSERT_PAIRWISE_REVERSAL: namespace=%s", ns)
		}
	}
	indexes := partitionIndexes(5, []int{2, 1}, []int{3, 2})
	if !reflect.DeepEqual(indexes, IndexPartition{Shared: []int{2}, LeftOnly: []int{1}, RightOnly: []int{3}}) {
		t.Fatalf("ASSERT_PAIRWISE_DIAGNOSTIC_ORDER: %#v", indexes)
	}
}

func TestSeedSummaryBoundaries(t *testing.T) {
	cases := []struct {
		seed inspectionSeed
		want string
	}{
		{inspectionSeed{PreparationStatus: "FAILED", Seed: graph.SeedResult{Label: "failed", Failure: &graph.SeedFailure{Phase: "prepare", Message: "failed"}}}, "FAILED"},
		{inspectionSeed{PreparationStatus: "SUCCEEDED", Seed: graph.SeedResult{Label: "empty"}}, "SUCCESSFUL_EMPTY"},
		{inspectionSeed{PreparationStatus: "SUCCEEDED", Seed: graph.SeedResult{Label: "node"}, NativeNodeIDs: []string{"n"}}, "SUCCESSFUL_WITH_EVIDENCE"},
		{inspectionSeed{PreparationStatus: "SUCCEEDED", Seed: graph.SeedResult{Label: "dispatch"}, SeedMemberships: []graph.SeedMembership{{EvidenceKind: "DISPATCH_ASSOCIATION", EndpointID: "d"}}}, "SUCCESSFUL_WITH_EVIDENCE"},
	}
	for _, tc := range cases {
		if got := seedSummary(tc.seed).State; got != tc.want {
			t.Fatalf("ASSERT_PAIRWISE_SEED_STATE: got=%q want=%q", got, tc.want)
		}
	}
}

func TestProjectPairwiseConcurrentInvalidInputIsDeterministic(t *testing.T) {
	const workers = 32
	errors := make(chan string, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := ProjectPairwise([]byte(`{}`), "left", "right")
			if err == nil {
				errors <- "<nil>"
				return
			}
			errors <- err.Error()
		}()
	}
	wg.Wait()
	close(errors)
	for got := range errors {
		if got != "input must be lsp-trace.inspect.v1 ALL_SEEDS" {
			t.Fatalf("ASSERT_PAIRWISE_CONCURRENT_DETERMINISM: err=%q", got)
		}
	}
}
