package targetpacket

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"lsp-trace/internal/censusprogramc"
	"lsp-trace/internal/retainedprojectiontestfixture"
	"lsp-trace/internal/sourceobject"
)

func digestOf(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// resolvingRequest builds a Request whose custody Raw is a genuine V2 artifact
// (from the shared retainedprojectiontestfixture) and whose nomination's
// SelectedNode maps to that artifact's single display binding.
func resolvingRequest(t *testing.T) (req Request, digest string, content []byte) {
	t.Helper()
	fx := retainedprojectiontestfixture.ValidV2Artifact(t)
	n := sampleNomination()
	n.SelectedNode = fx.NodeID
	custody := SnapshotCustody{
		ConstituentIdentity: n.ConstituentIdentity,
		GraphDigest:         fx.GraphV5Digest,
		GraphByteLength:     fx.GraphV5ByteLen,
		Raw:                 fx.Raw,
	}
	req = Request{
		CensusID:    "census-1",
		Nominations: []censusprogramc.Representative{n},
		Custody:     []SnapshotCustody{custody},
		Lookup:      contentLookup{digest: fx.SourceDigest, body: fx.Content},
		MaxBytes:    1 << 20,
	}
	return req, fx.SourceDigest, fx.Content
}

// contentLookup is a Lookup that returns the exact body for the requested
// identity, so identity/length/SHA reverification inside Resolve is exercised.
type contentLookup struct {
	digest string
	body   []byte
}

func (c contentLookup) Get(id sourceobject.Identity) (sourceobject.Object, error) {
	return sourceobject.Object{Identity: id, Bytes: append([]byte(nil), c.body...)}, nil
}

// --- Property [3]: selected node maps to exactly one logical source ---------

func TestBuildResolvesSelectedNodeToOneLogicalSource(t *testing.T) {
	req, _, _ := resolvingRequest(t)
	res, err := Build(req)
	if err != nil || res.State != StatePrepared || len(res.Packets) != 1 {
		t.Fatalf("ASSERT_P3_PREPARED: state=%q packets=%d err=%v", res.State, len(res.Packets), err)
	}
	if res.Packets[0].LogicalSourceID == "" {
		t.Fatalf("ASSERT_P3_LOGICAL_SOURCE_MAPPED: empty logical source id")
	}
}

func TestBuildUnknownSelectedNodeFailsClosed(t *testing.T) {
	req, _, _ := resolvingRequest(t)
	req.Nominations[0].SelectedNode = "no-such-node" // zero compatible logical sources
	res, err := Build(req)
	if err == nil || res.State != StateFailed {
		t.Fatalf("ASSERT_P3_UNKNOWN_NODE_FAILS_CLOSED: state=%q err=%v", res.State, err)
	}
}

// --- Property [5]/[6]: resolve + assemble body with bounded, typed outcome ---

func TestBuildResolvesBodyThroughLookup(t *testing.T) {
	req, _, content := resolvingRequest(t)
	res, err := Build(req)
	if err != nil || len(res.Packets) != 1 {
		t.Fatalf("ASSERT_P5_PREPARED: err=%v packets=%d", err, len(res.Packets))
	}
	p := res.Packets[0]
	if p.BodyDigest != digestOf(content) {
		t.Fatalf("ASSERT_P5_BODY_DIGEST_FROM_LOOKUP: got=%q want=%q", p.BodyDigest, digestOf(content))
	}
}

func TestBuildZeroMaxBytesFailsClosed(t *testing.T) {
	req, _, _ := resolvingRequest(t)
	req.MaxBytes = 0 // resource bound violated -> typed failure
	res, err := Build(req)
	if err == nil || res.State != StateFailed {
		t.Fatalf("ASSERT_P5_ZERO_BOUND_FAILS_CLOSED: state=%q err=%v", res.State, err)
	}
}
