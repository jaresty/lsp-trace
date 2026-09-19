package targetpacket

import (
	"crypto/sha256"
	"encoding/hex"
	"lsp-trace/internal/censusprogramc"
	"lsp-trace/internal/programcadmission"
	"lsp-trace/internal/retainedprojection"
	"lsp-trace/internal/retainedprojectiontestfixture"
	"lsp-trace/internal/sourceobject"
	"testing"
)

type fixtureLookup struct{ body []byte }

func (l fixtureLookup) Get(id sourceobject.Identity) (sourceobject.Object, error) {
	return sourceobject.Object{Identity: id, Bytes: append([]byte(nil), l.body...)}, nil
}
func validRequest(t *testing.T) Request {
	t.Helper()
	fx := retainedprojectiontestfixture.ValidV2Artifact(t)
	n := selected()
	n.Status = censusprogramc.CandidateStatus
	n.Authority = 0
	n.SourceGraphComplete = "UNKNOWN"
	n.SelectedNode = fx.NodeID
	n.ClaimCeiling = "STRUCTURAL"
	n.BatchID, n.CommunityIdentity = "batch", "community"
	n.ExecutionBundleID, n.SeedLabel, n.SeedAt = "bundle", "seed", "at"
	n.Members, n.SCCMembers = []string{"member"}, []string{"member"}
	c := censusprogramc.Result{CensusID: "census", Admission: programcadmission.Result{Artifact: programcadmission.Artifact{Constituents: []programcadmission.ConstituentReference{{Identity: "constituent", GraphSHA256: fx.GraphV5Digest, GraphByteLength: int(fx.GraphV5ByteLen)}}}}, Representatives: censusprogramc.RepresentativeSelection{State: "SELECTED", Nominations: []censusprogramc.Representative{n}}}
	return Request{Census: c, Snapshots: []Snapshot{{ConstituentIdentity: "constituent", ConstituentOrdinal: 0, Raw: fx.Raw}}, Lookup: fixtureLookup{fx.Content}, Policy: structPolicy(), ResolveLimits: retainedprojection.ResolveLimits{MaxDistinctObjects: 1, MaxUniqueSourceBytes: uint64(len(fx.Content)), MaxLogicalSelections: 1}, MaxResponseBytes: 1 << 20}
}
func TestASSERT_P1_P2_REAL_FIXTURE_PREPARED_TYPED_WIRE(t *testing.T) {
	r := validRequest(t)
	out, err := Build(r)
	if err != nil || out.State != StatePrepared || len(out.Packets) != 1 {
		t.Fatalf("prepared: %#v %v", out, err)
	}
	p := out.Packets[0]
	if len(p.Projection.Units) != 1 || len(p.Projection.Citations) != 1 || len(p.Projection.EmittedSpans) == 0 || p.Projection.CustodyBinding.GraphDigest != r.Census.Admission.Artifact.Constituents[0].GraphSHA256 {
		t.Fatalf("typed wire incomplete: %#v", p.Projection)
	}
}
func TestASSERT_P2_SNAPSHOT_RECONCILIATION_REJECTS_MISSING_AND_EXTRA(t *testing.T) {
	r := validRequest(t)
	r.Snapshots = nil
	if _, e := Build(r); e == nil {
		t.Fatal("missing")
	}
	r = validRequest(t)
	r.Snapshots = append(r.Snapshots, Snapshot{ConstituentIdentity: "extra", ConstituentOrdinal: 999})
	if _, e := Build(r); e == nil {
		t.Fatal("extra")
	}
}
func TestASSERT_P8_VALIDATE_DUPLICATE_KNOWN_KEY(t *testing.T) {
	r := validRequest(t)
	o, e := Build(r)
	if e != nil {
		t.Fatal(e)
	}
	raw, e := EncodeCanonical(o.Packets[0])
	if e != nil {
		t.Fatal(e)
	}
	raw = append([]byte(`{"packet_id":"x",`), raw[1:]...)
	if _, e = Validate(raw); e == nil {
		t.Fatal("duplicate packet_id accepted")
	}
	_ = sha256.Size
	_ = hex.EncodedLen(1)
}
