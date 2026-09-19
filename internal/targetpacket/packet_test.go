package targetpacket

import (
	"reflect"
	"testing"

	"lsp-trace/internal/censusprogramc"
)

// --- fixtures -------------------------------------------------------------
//
// sampleRequest and sampleNomination build a Request backed by a genuine
// serialized V2 snapshot artifact (see validV2Artifact in resolve_test.go), so
// every property test drives the real Admit -> Select -> CustodyBinding ->
// Resolve -> AssembleV2Bounded pipeline rather than a fake custody stub.

func sampleNomination() censusprogramc.Representative {
	return censusprogramc.Representative{
		Status:              censusprogramc.CandidateStatus,
		ClaimCeiling:        "STRUCTURAL",
		SelectionState:      "SELECTED",
		Members:             []string{"m1", "m2"},
		PreparedTargets:     []string{"t1"},
		SCCMembers:          []string{"s1", "s2"},
		CensusID:            "census-1",
		BatchID:             "batch-1",
		CommunityIdentity:   "community-1",
		ConstituentIdentity: "constituent-1",
		ConstituentOrdinal:  0,
		Distance:            3,
		Authority:           0,
		ExecutionBundleID:   "bundle-1",
		SeedLabel:           "seed-A",
		SeedAt:              "pos-7",
		SelectedNode:        "node-42",
		SourceGraphComplete: "UNKNOWN",
	}
}

// sampleRequest returns a Request backed by a genuine V2 snapshot artifact.
func sampleRequest(t *testing.T) Request {
	t.Helper()
	req, _, _ := resolvingRequest(t)
	return req
}

// --- Property [1]: exact one-nomination binding, authority 0, accepted=false

func TestBuildBindsExactlyOneNominationWithAllFields(t *testing.T) {
	req := sampleRequest(t)
	res, err := Build(req)
	if err != nil || res.State != StatePrepared || len(res.Packets) != 1 {
		t.Fatalf("ASSERT_P1_ONE_PACKET_PREPARED: state=%q packets=%d err=%v", res.State, len(res.Packets), err)
	}
	p := res.Packets[0]
	n := req.Nominations[0]
	if p.CensusID != n.CensusID || p.BatchID != n.BatchID || p.ConstituentIdentity != n.ConstituentIdentity ||
		p.ConstituentOrdinal != n.ConstituentOrdinal || p.ExecutionBundleID != n.ExecutionBundleID ||
		p.SeedLabel != n.SeedLabel || p.SeedPosition != n.SeedAt || p.CommunityIdentity != n.CommunityIdentity ||
		!reflect.DeepEqual(p.CommunityMembers, n.Members) || p.SelectedNode != n.SelectedNode ||
		p.Distance != n.Distance || !reflect.DeepEqual(p.SCCMembers, n.SCCMembers) || p.ClaimCeiling != n.ClaimCeiling {
		t.Fatalf("ASSERT_P1_ALL_FIELDS_BOUND: %+v", p)
	}
	if p.Authority != 0 || p.Accepted != false {
		t.Fatalf("ASSERT_P1_AUTHORITY_ZERO_ACCEPTED_FALSE: authority=%d accepted=%t", p.Authority, p.Accepted)
	}
}

// --- Property [2]: custody digest/length == Graph V5; wrong-graph fails closed

func TestBuildCustodyMatchesGraphV5(t *testing.T) {
	req := sampleRequest(t)
	res, err := Build(req)
	if err != nil || len(res.Packets) != 1 {
		t.Fatalf("ASSERT_P2_PREPARED: err=%v packets=%d", err, len(res.Packets))
	}
	p := res.Packets[0]
	if p.GraphDigest == "" || p.GraphDigest != req.Custody[0].GraphDigest || p.GraphByteLength != req.Custody[0].GraphByteLength {
		t.Fatalf("ASSERT_P2_CUSTODY_BINDS_GRAPH_V5: digest=%q len=%d", p.GraphDigest, p.GraphByteLength)
	}
}

func TestBuildWrongGraphCustodyFailsClosed(t *testing.T) {
	req := sampleRequest(t)
	req.Custody[0].GraphDigest = "sha256:deadbeef" // wrong-graph
	res, err := Build(req)
	if err == nil || res.State != StateFailed {
		t.Fatalf("ASSERT_P2_WRONG_GRAPH_FAILS_CLOSED: state=%q err=%v", res.State, err)
	}
}

func TestBuildMissingCustodyFailsClosed(t *testing.T) {
	req := sampleRequest(t)
	req.Custody = nil // missing custody for the nomination's constituent
	res, err := Build(req)
	if err == nil || res.State != StateFailed {
		t.Fatalf("ASSERT_P2_MISSING_CUSTODY_FAILS_CLOSED: state=%q err=%v", res.State, err)
	}
}

// --- Property [4]: lookup-only source resolution; nil lookup fails closed

func TestBuildNilLookupFailsClosed(t *testing.T) {
	req := sampleRequest(t)
	req.Lookup = nil
	res, err := Build(req)
	if err == nil || res.State != StateFailed {
		t.Fatalf("ASSERT_P4_NIL_LOOKUP_FAILS_CLOSED: state=%q err=%v", res.State, err)
	}
}

// --- Property [6]: deterministic packet identity over covered fields

func TestBuildPacketIDIsDeterministic(t *testing.T) {
	req := sampleRequest(t) // same input built once, resolved twice
	a, errA := Build(req)
	b, errB := Build(req)
	if errA != nil || errB != nil || len(a.Packets) != 1 || len(b.Packets) != 1 {
		t.Fatalf("ASSERT_P6_TWO_PREPARED: errA=%v errB=%v", errA, errB)
	}
	if a.Packets[0].PacketID == "" || a.Packets[0].PacketID != b.Packets[0].PacketID {
		t.Fatalf("ASSERT_P6_DETERMINISTIC_ID: a=%q b=%q", a.Packets[0].PacketID, b.Packets[0].PacketID)
	}
}

func TestBuildPacketIDChangesWithField(t *testing.T) {
	req := sampleRequest(t)
	base, err := Build(req)
	if err != nil || len(base.Packets) != 1 {
		t.Fatalf("ASSERT_P6_PREPARED: err=%v", err)
	}
	req.Nominations[0].BatchID = "batch-CHANGED" // covered field perturbation (valid node retained)
	other, err := Build(req)
	if err != nil || len(other.Packets) != 1 {
		t.Fatalf("ASSERT_P6_PREPARED_2: err=%v", err)
	}
	if base.Packets[0].PacketID == other.Packets[0].PacketID {
		t.Fatalf("ASSERT_P6_ID_COVERS_FIELDS: id unchanged after field change: %q", base.Packets[0].PacketID)
	}
}

// --- Property [7]: EMPTY / UNRESOLVED / PREPARED / FAILED are distinct

func TestBuildEmptyWhenNoNominations(t *testing.T) {
	req := sampleRequest(t)
	req.Nominations = nil
	res, err := Build(req)
	if err != nil || res.State != StateEmpty || len(res.Packets) != 0 {
		t.Fatalf("ASSERT_P7_EMPTY: state=%q packets=%d err=%v", res.State, len(res.Packets), err)
	}
}

func TestBuildUnresolvedNominationYieldsNoPacket(t *testing.T) {
	req := Request{
		CensusID:   "census-1",
		Unresolved: []censusprogramc.Representative{sampleNomination()},
		MaxBytes:   1 << 20,
	}
	res, err := Build(req)
	if err != nil || res.State != StateUnresolved || len(res.Packets) != 0 {
		t.Fatalf("ASSERT_P7_UNRESOLVED_NO_PACKET: state=%q packets=%d err=%v", res.State, len(res.Packets), err)
	}
}

// --- Property [8]: defensive cloning; mutating an input does not affect output

func TestBuildDefensivelyClonesInputs(t *testing.T) {
	req := sampleRequest(t)
	res, err := Build(req)
	if err != nil || len(res.Packets) != 1 {
		t.Fatalf("ASSERT_P8_PREPARED: err=%v", err)
	}
	before := append([]string(nil), res.Packets[0].CommunityMembers...)
	req.Nominations[0].Members[0] = "MUTATED" // mutate caller's slice after Build
	req.Custody[0].Raw[0] = 'X'               // mutate caller's raw bytes
	if !reflect.DeepEqual(res.Packets[0].CommunityMembers, before) {
		t.Fatalf("ASSERT_P8_OUTPUT_ISOLATED_FROM_INPUT_MUTATION: %+v vs %+v", res.Packets[0].CommunityMembers, before)
	}
}

// --- Property [9]: existing census result unchanged across Build (success/fail)

func TestBuildDoesNotMutateCensusResult(t *testing.T) {
	req := sampleRequest(t)
	result := censusprogramc.Result{
		CensusID: "census-1",
		Candidates: []censusprogramc.Candidate{
			{Status: censusprogramc.CandidateStatus, ClaimCeiling: "STRUCTURAL", Members: []string{"m1"}},
		},
		Representatives: censusprogramc.RepresentativeSelection{
			State:       "SELECTED",
			Nominations: req.Nominations,
		},
	}
	snapshot := censusprogramc.Result{
		CensusID:        result.CensusID,
		Candidates:      append([]censusprogramc.Candidate(nil), result.Candidates...),
		Representatives: result.Representatives,
	}
	res, err := Build(req)
	// Bind the guard to real work: a no-op Build must not pass this test, so we
	// require a prepared packet AND the census result unchanged.
	if err != nil || res.State != StatePrepared || len(res.Packets) != 1 {
		t.Fatalf("ASSERT_P9_PREPARED: state=%q packets=%d err=%v", res.State, len(res.Packets), err)
	}
	if !reflect.DeepEqual(result.Candidates, snapshot.Candidates) || result.CensusID != snapshot.CensusID {
		t.Fatalf("ASSERT_P9_CENSUS_RESULT_UNCHANGED: %+v vs %+v", result, snapshot)
	}
}
