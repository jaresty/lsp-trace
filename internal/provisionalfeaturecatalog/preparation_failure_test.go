package provisionalfeaturecatalog

import (
	"testing"

	"lsp-trace/internal/targetpacket"
)

func TestBuildWithPreparationFailuresAccountingAndMembers(t *testing.T) {
	failures := []PreparationFailureInput{
		{FailureID: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", NominationID: "n1", PacketIntentID: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", Role: "TARGET", Code: "EXACT_ENDPOINT_SOURCE_UNAVAILABLE", EvidenceIDs: []string{"sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"}},
		{FailureID: "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd", NominationID: "n2", PacketIntentID: "sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee", Role: "CALLER", Code: "EXACT_ENDPOINT_SOURCE_UNAVAILABLE", EvidenceIDs: []string{"sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"}},
	}
	catalog, err := BuildWithPreparationFailures(2, failures, targetpacketEmpty(), nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	a := catalog.Accounting()
	if a.NominationTotal != 2 || a.RequestTotal != 0 || a.DescribedEntries != 0 || a.PreRequestFailed != 2 || a.NominationTotal != a.RequestTotal+a.PreRequestFailed {
		t.Fatalf("ASSERT_CATALOG_PRE_REQUEST_ACCOUNTING: %#v", a)
	}
	entries := catalog.Entries()
	if len(entries) != 2 || catalog.Outcome() != OutcomeDegraded {
		t.Fatalf("ASSERT_CATALOG_FAILURE_TERMINAL_MEMBERS: entries=%#v outcome=%s", entries, catalog.Outcome())
	}
	for _, entry := range entries {
		if entry.TerminalOutcome != TerminalSourceUnavailable || entry.Lineage.ResponseID != "" || entry.Failure == nil || entry.Authority != 0 || entry.Accepted || entry.Completeness != CompletenessUnknown {
			t.Fatalf("ASSERT_CATALOG_FAILURE_MEMBER_SHAPE: %#v", entry)
		}
	}
}

func targetpacketEmpty() targetpacket.Result {
	return targetpacket.Result{State: targetpacket.StateEmpty, Packets: []targetpacket.Packet{}}
}
