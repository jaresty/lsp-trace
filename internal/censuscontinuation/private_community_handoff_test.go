package censuscontinuation

import (
	"reflect"
	"testing"

	"lsp-trace/internal/programc"
	"lsp-trace/internal/programcpresentation"
	"lsp-trace/internal/transientstructural"
)

func TestManagedMemberStructuralRequestUsesOnlyMachineTarget(t *testing.T) {
	callable := programcpresentation.MachineTarget{URI: "file:///workspace/validate.go", Line: 235, Character: 5, PositionEncoding: "utf-16", RangeRole: "SELECTION_RANGE", NodeID: "Validate", NodeKind: 12}
	got, err := managedMemberStructuralRequest("session", 7, callable)
	if err != nil || got.NodeID != "Validate" || got.URI != callable.URI || got.Request.Target.URI != callable.URI || got.Request.Target.Line == nil || *got.Request.Target.Line != 235 || got.Request.Target.Character == nil || *got.Request.Target.Character != 5 || got.Request.SourceOnlyTarget || got.Request.Analysis.Kind != transientstructural.AnalysisNeighborhood {
		t.Fatalf("ASSERT_PRIVATE_MACHINE_TARGET_CALLABLE_NEIGHBORHOOD: got=%+v err=%v", got, err)
	}
	nonCallable := callable
	nonCallable.NodeID, nonCallable.NodeKind, nonCallable.Line, nonCallable.Character = "field", 8, 38, 23
	got, err = managedMemberStructuralRequest("session", 7, nonCallable)
	if err != nil || !got.Request.SourceOnlyTarget || got.Request.UpDepth != 0 || got.Request.DownDepth != 0 || got.NodeID != "field" || got.URI != nonCallable.URI {
		t.Fatalf("ASSERT_PRIVATE_MACHINE_TARGET_SOURCE_ONLY: got=%+v err=%v", got, err)
	}
	bad := callable
	bad.URI = "file:///workspace/../sdk/fmt.go"
	if _, err := managedMemberStructuralRequest("session", 7, bad); err == nil {
		t.Fatal("ASSERT_PRIVATE_MACHINE_TARGET_MALFORMED_REJECTED")
	}
}

func TestPrivateCommunityHandoffRejectsForeignAndCapturesEveryMember(t *testing.T) {
	workspace, f, handoff := partitionCaptureFixture(t, false)
	boundary, err := programc.ComputeBoundary(f.result.Outcome, programc.BoundaryRequest{PageRankTopK: 7, HubTopK: 7})
	if err != nil {
		t.Fatal(err)
	}
	if len(boundary.Communities) == 0 {
		t.Fatal("ASSERT_PRIVATE_HANDOFF_FIXTURE_COMMUNITY")
	}
	selected := boundary.Communities[0]
	prepareCalls, resolverCalls := 0, 0
	deps := successfulManagedDependencies(t, workspace, &prepareCalls, &resolverCalls, false)
	limits := testContract(t).CaptureLimits()

	if _, err := captureAdmittedCommunity(handoff, f.result, boundary, "sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff", workspace, handoff.PositionEncoding(), limits, deps); err == nil {
		t.Fatal("ASSERT_PRIVATE_HANDOFF_REJECTS_FOREIGN_COMMUNITY")
	}
	checkpoint, err := captureAdmittedCommunity(handoff, f.result, boundary, selected.CommunityID, workspace, handoff.PositionEncoding(), limits, deps)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(checkpoint.memberIDs(), selected.Members) {
		t.Fatalf("ASSERT_PRIVATE_HANDOFF_MEMBERS_IMMUTABLE: got=%v want=%v", checkpoint.memberIDs(), selected.Members)
	}
	counts := map[string]int{}
	for _, constituent := range checkpoint.capture.Constituents {
		for _, outcome := range constituent.V6Capture.Outcomes {
			if outcome.Role == "TARGET" {
				counts[outcome.GraphSubjectID]++
			}
		}
	}
	for _, member := range selected.Members {
		if counts[member] != 1 {
			t.Fatalf("ASSERT_PRIVATE_HANDOFF_EXACT_ONE_MEMBER_V6: member=%s count=%d", member, counts[member])
		}
	}
	if len(counts) != len(selected.Members) || prepareCalls == 0 || resolverCalls == 0 {
		t.Fatalf("ASSERT_PRIVATE_HANDOFF_ALL_MEMBER_V6: counts=%v prepare=%d resolve=%d", counts, prepareCalls, resolverCalls)
	}
}
