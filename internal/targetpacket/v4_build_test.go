package targetpacket

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"lsp-trace/internal/censusprogramc"
	"lsp-trace/internal/retainedprojection"
	"lsp-trace/internal/v5sourcesnapshot"
	"lsp-trace/internal/v5sourcesnapshotv2"
	"lsp-trace/internal/v5sourcesnapshotv3"
	"lsp-trace/internal/v5sourcesnapshotv4"
)

func TestBuildV4EndpointAbsentFromHistoricalV1(t *testing.T) {
	fixture := buildV3PacketFixture(t, 1)
	v3raw := fixture.request.Snapshots[0].Raw
	var v3 v5sourcesnapshotv3.Artifact
	if err := json.Unmarshal(v3raw, &v3); err != nil {
		t.Fatal(err)
	}
	var v2 v5sourcesnapshotv2.Artifact
	if err := json.Unmarshal(v3.ParentSnapshot, &v2); err != nil {
		t.Fatal(err)
	}
	var v1 v5sourcesnapshot.Artifact
	if err := json.Unmarshal(v2.ParentSnapshot, &v1); err != nil {
		t.Fatal(err)
	}
	if len(v1.Receipts) == 0 {
		t.Fatal("ASSERT_V4_ENDPOINT_NOT_V1_BOUND: fixture receipt missing")
	}
	r := v1.Receipts[0]
	fakeID := "endpoint-absent-from-v1"
	for _, b := range v1.Bindings {
		if b.NodeID == fakeID {
			t.Fatal("ASSERT_V4_ENDPOINT_NOT_V1_BOUND: endpoint unexpectedly in V1")
		}
	}
	receipt := v5sourcesnapshotv4.Receipt{ID: r.ID, URI: r.URI, ContentDigest: r.ContentDigest, Content: append([]byte(nil), r.Content...), CanonicalReceipt: append([]byte(nil), r.CanonicalReceipt...)}
	binding := v5sourcesnapshotv4.EndpointBinding{Role: "TARGET", GraphSubjectID: fakeID, NodeID: fakeID, LogicalSourceID: r.URI, DisplayRange: v1.Bindings[0].Range, DisplayRangePolicy: v5sourcesnapshotv4.DisplayRangePolicy, Provenance: v5sourcesnapshotv4.Provenance{Kind: v5sourcesnapshotv4.ProvenanceKind, Method: v5sourcesnapshotv4.ProvenanceMethod}, ReceiptID: r.ID, SourceDigest: r.ContentDigest, SourceByteLength: uint64(len(r.Content)), PositionEncoding: v1.PositionEncoding, CanonicalOrdinal: 0, Status: v5sourcesnapshotv4.Status, Custody: v5sourcesnapshotv4.Custody, ConstituentGraphDigest: v1.GraphV5Digest, ConstituentGraphByteLength: uint64(len(v1.GraphV5Bytes)), Authority: 0, Accepted: false, Completeness: v5sourcesnapshotv4.Completeness}
	limits := v5sourcesnapshotv4.Limits{MaxArtifactBytes: 1 << 27, MaxParentBytes: 1 << 26, MaxReceipts: 100, MaxSourceBytes: 1 << 20, MaxTotalSourceBytes: 1 << 22, MaxBindings: 100, MaxWork: 1000}
	callerBinding := binding
	callerBinding.Role = "CALLER"
	callerBinding.GraphSubjectID = "caller-absent-from-v1"
	callerBinding.NodeID = callerBinding.GraphSubjectID
	callerBinding.CanonicalOrdinal = 1
	v4raw, err := v5sourcesnapshotv4.BuildFromReceipts(v3raw, []v5sourcesnapshotv4.Receipt{receipt}, []v5sourcesnapshotv4.EndpointBinding{binding, callerBinding}, limits)
	if err != nil {
		t.Fatal("ASSERT_V4_ENDPOINT_NOT_V1_BOUND:", err)
	}
	binding.CanonicalOrdinal, callerBinding.CanonicalOrdinal = 1, 0
	if _, err := v5sourcesnapshotv4.BuildFromReceipts(v3raw, []v5sourcesnapshotv4.Receipt{receipt}, []v5sourcesnapshotv4.EndpointBinding{callerBinding, binding}, limits); err == nil {
		t.Fatal("ASSERT_V4_CANONICAL_ORDER")
	}
	var v4 v5sourcesnapshotv4.Artifact
	_ = json.Unmarshal(v4raw, &v4)
	mutated := v4
	mutated.ConstituentGraphByteLength++
	mutatedRaw := append(mustJSONBytes(t, mutated), '\n')
	if _, err := v5sourcesnapshotv4.Validate(mutatedRaw, limits); err == nil {
		t.Fatal("ASSERT_V4_CONSTITUENT_IDENTITY_COMPLETE")
	}
	unknown := bytes.Replace(v4raw, []byte(`{"schema_version":`), []byte(`{"unknown":true,"schema_version":`), 1)
	if _, err := v5sourcesnapshotv4.Validate(unknown, limits); err == nil {
		t.Fatal("ASSERT_V4_CLOSED_SCHEMA")
	}
	if !bytes.Equal(v4.ParentSnapshot, v3raw) {
		t.Fatal("ASSERT_V4_PARENT_V3_EXACT_BYTES")
	}
	var embedded v5sourcesnapshotv3.Artifact
	_ = json.Unmarshal(v4.ParentSnapshot, &embedded)
	if !bytes.Equal(mustJSONBytes(t, embedded.Bindings), mustJSONBytes(t, v3.Bindings)) {
		t.Fatal("ASSERT_V4_V3_OCCURRENCES_UNCHANGED")
	}
	if v4.Authority != 0 || v4.Accepted || v4.Completeness != "UNKNOWN" || binding.Authority != 0 || binding.Accepted || binding.Completeness != "UNKNOWN" {
		t.Fatal("ASSERT_V4_AUTHORITY_TUPLE")
	}
	workspaceURI, _ := url.Parse(r.URI)
	if err := os.RemoveAll(filepath.Dir(workspaceURI.Path)); err != nil {
		t.Fatal(err)
	}
	_, replayLookup, err := v5sourcesnapshotv4.Replay(v4raw, limits)
	if err != nil {
		t.Fatal("ASSERT_V4_CAPTURE_ONLY_WORKSPACE_READ:", err)
	}
	admitted, err := retainedprojection.Admit(v4raw)
	if err != nil {
		t.Fatal("ASSERT_RETAINED_V4_ONE_ARTIFACT_SET:", err)
	}
	keys := admitted.DisplayKeys()
	if len(keys) != 2 || keys[0].GraphSubjectID != fakeID {
		t.Fatal("ASSERT_RETAINED_V4_ONE_ARTIFACT_SET")
	}
	fixture.request.Census.Representatives.Nominations[0].SelectedNode = fakeID
	fixture.request.Census.Representatives.Nominations[0].IncomingPredecessors = nil
	fixture.request.Snapshots[0].Raw = v4raw
	fixture.request.Lookup = replayLookup
	fixture.request.ResolveLimits = retainedprojection.ResolveLimits{MaxDistinctObjects: 1, MaxUniqueSourceBytes: 1 << 20, MaxLogicalSelections: 1}
	result, err := Build(fixture.request)
	if err != nil || result.State != StatePrepared || len(result.Packets) != 1 {
		t.Fatalf("ASSERT_TARGETPACKET_V4_RECONCILIATION: state=%s err=%v", result.State, err)
	}
	if len(result.Packets[0].Projection.Units) != 1 {
		t.Fatal("ASSERT_RETAINED_V4_ONE_PROJECT_PASS")
	}
}

func TestV4CaptureRejectsArbitraryEndpointsAndDoesNotSubstitute(t *testing.T) {
	fixture := buildV3PacketFixture(t, 1)
	var v3 v5sourcesnapshotv3.Artifact
	_ = json.Unmarshal(fixture.request.Snapshots[0].Raw, &v3)
	var v2 v5sourcesnapshotv2.Artifact
	_ = json.Unmarshal(v3.ParentSnapshot, &v2)
	var v1 v5sourcesnapshot.Artifact
	_ = json.Unmarshal(v2.ParentSnapshot, &v1)
	nomination := fixture.request.Census.Representatives.Nominations[0]
	set, err := v5sourcesnapshotv4.DeriveEndpointSet(v1.GraphV5Bytes, "constituent", 0, []censusprogramc.Representative{nomination})
	if err != nil {
		t.Fatal(err)
	}
	arbitrary := nomination
	arbitrary.SelectedNode = "arbitrary-addition"
	if _, err := v5sourcesnapshotv4.DeriveEndpointSet(v1.GraphV5Bytes, "constituent", 0, []censusprogramc.Representative{nomination, arbitrary}); err == nil {
		t.Fatal("ASSERT_V4_NO_ARBITRARY_ENDPOINTS")
	}
	targetURI, _ := url.Parse(fixture.target.URI)
	workspace := filepath.Dir(targetURI.Path)
	if err := os.Remove(targetURI.Path); err != nil {
		t.Fatal(err)
	}
	limits := v5sourcesnapshotv4.Limits{MaxArtifactBytes: 1 << 27, MaxParentBytes: 1 << 26, MaxReceipts: 100, MaxSourceBytes: 1 << 20, MaxTotalSourceBytes: 1 << 22, MaxBindings: 100, MaxWork: 1000}
	_, err = v5sourcesnapshotv4.Capture(v5sourcesnapshotv4.CaptureInput{GraphV5Bytes: v1.GraphV5Bytes, Workspace: workspace, PositionEncoding: v1.PositionEncoding, Endpoints: set, Limits: limits})
	var unavailable *v5sourcesnapshotv4.SourceUnavailableError
	if !errors.As(err, &unavailable) || unavailable.Endpoint != "TARGET" || unavailable.LogicalSourceID != fixture.target.URI {
		t.Fatalf("ASSERT_V4_SOURCE_UNAVAILABLE_NO_SUBSTITUTE: %v", err)
	}
}

func TestV4CaptureSelectedPreservesSuccessfulSubset(t *testing.T) {
	fixture := buildV3PacketFixture(t, 1)
	v3raw := fixture.request.Snapshots[0].Raw
	var v3 v5sourcesnapshotv3.Artifact
	_ = json.Unmarshal(v3raw, &v3)
	var v2 v5sourcesnapshotv2.Artifact
	_ = json.Unmarshal(v3.ParentSnapshot, &v2)
	var v1 v5sourcesnapshot.Artifact
	_ = json.Unmarshal(v2.ParentSnapshot, &v1)
	nomination := fixture.request.Census.Representatives.Nominations[0]
	set, err := v5sourcesnapshotv4.DeriveEndpointSet(v1.GraphV5Bytes, "constituent", 0, []censusprogramc.Representative{nomination})
	if err != nil {
		t.Fatal(err)
	}
	targetURI, _ := url.Parse(fixture.target.URI)
	if err := os.Remove(targetURI.Path); err != nil {
		t.Fatal(err)
	}
	limits := v5sourcesnapshotv4.Limits{MaxArtifactBytes: 1 << 27, MaxParentBytes: 1 << 26, MaxReceipts: 100, MaxSourceBytes: 1 << 20, MaxTotalSourceBytes: 1 << 22, MaxBindings: 100, MaxWork: 1000}
	selected, err := v5sourcesnapshotv4.CaptureSelected(v5sourcesnapshotv4.CaptureInput{ParentV3: v3raw, Workspace: filepath.Dir(targetURI.Path), PositionEncoding: v1.PositionEncoding, Endpoints: set, Limits: limits})
	if err != nil {
		t.Fatalf("ASSERT_V4_SELECTED_PARTIAL_CAPTURE: %v", err)
	}
	if len(selected.Outcomes) < 2 || selected.CapturedCount == 0 || selected.UnavailableCount == 0 {
		t.Fatalf("ASSERT_V4_SELECTED_EXACT_OUTCOMES: %#v", selected)
	}
	if _, err := v5sourcesnapshotv4.Validate(selected.Capture.Raw, limits); err != nil {
		t.Fatalf("ASSERT_V4_SELECTED_SUCCESSFUL_SUBSET_VALID: %v", err)
	}
}

func mustJSONBytes(t *testing.T, v any) []byte {
	t.Helper()
	b, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
