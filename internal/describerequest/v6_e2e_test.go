package describerequest

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"os"
	"testing"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/sourceobject"
	"lsp-trace/internal/targetpacket"
	"lsp-trace/internal/v5sourcesnapshot"
	"lsp-trace/internal/v5sourcesnapshotv2"
	"lsp-trace/internal/v5sourcesnapshotv3"
	"lsp-trace/internal/v5sourcesnapshotv6"
)

func TestV6TargetPacketDescribeRequestOfflineE2E(t *testing.T) {
	fx := resolvedPacketResult(t, 2)
	var v3 v5sourcesnapshotv3.Artifact
	if err := json.Unmarshal(fx.request.Snapshots[0].Raw, &v3); err != nil {
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

	nomination := fx.request.Census.Representatives.Nominations[0]
	nodes := map[string]graph.Node{}
	var native graph.Result
	if err := json.Unmarshal(v1.GraphV5Bytes, &native); err == nil {
		for _, node := range native.Nodes {
			nodes[node.ID] = node
		}
	}
	// V1 retains the Graph Provenance carrier rather than bare native bytes.
	if len(nodes) == 0 {
		var carrier struct {
			GraphV5 []byte `json:"graph_v5"`
		}
		_ = json.Unmarshal(v1.GraphV5Bytes, &carrier)
		if err := json.Unmarshal(carrier.GraphV5, &native); err != nil {
			t.Fatal(err)
		}
		for _, node := range native.Nodes {
			nodes[node.ID] = node
		}
	}
	ids := []string{nomination.SelectedNode}
	for _, predecessor := range nomination.IncomingPredecessors {
		ids = append(ids, predecessor.CallerID)
	}
	documents := make([]v5sourcesnapshotv6.PreparedDocument, 0, len(ids))
	for _, id := range ids {
		node := nodes[id]
		u, _ := url.Parse(node.URI)
		body, err := os.ReadFile(u.Path)
		if err != nil {
			t.Fatal(err)
		}
		documents = append(documents, v5sourcesnapshotv6.PreparedDocument{URI: node.URI, Bytes: body, Digest: resolvedDigest(body), ByteLength: uint64(len(body)), Version: "9", SessionID: "session", Generation: 1, PositionEncoding: "utf-16"})
	}
	incoming := make([]v5sourcesnapshotv6.Occurrence, 0, len(nomination.IncomingPredecessors))
	for i, predecessor := range nomination.IncomingPredecessors {
		incoming = append(incoming, v5sourcesnapshotv6.Occurrence{RelationID: predecessor.RelationID, OccurrenceID: predecessor.OccurrenceID, CallerNodeID: predecessor.CallerID, CalleeNodeID: predecessor.TargetID, Range: predecessor.CallSite})
		_ = i
	}
	limits := v5sourcesnapshotv6.Limits{MaxArtifactBytes: 16 << 20, MaxGraphBytes: 8 << 20, MaxReceipts: 100, MaxSourceBytes: 1 << 20, MaxTotalSourceBytes: 4 << 20, MaxBindings: 100, MaxOutcomes: 100, MaxWork: 1000}
	captured, err := v5sourcesnapshotv6.CaptureSelected(context.Background(), v5sourcesnapshotv6.CaptureInput{GraphV5Bytes: v1.GraphV5Bytes, PositionEncoding: "utf-16", SessionID: "session", Generation: 1, Nominations: []v5sourcesnapshotv6.Nomination{{ID: "v6-e2e", TargetNodeID: nomination.SelectedNode, Incoming: incoming}}, Documents: documents, Resolver: v5sourcesnapshotv6.ResolverFunc(func(_ context.Context, in v5sourcesnapshotv6.ResolveRequest) (v5sourcesnapshotv6.ResolveResult, error) {
		return v5sourcesnapshotv6.ResolveResult{DisplayRange: graph.Range{Start: graph.Position{}, End: graph.Position{Line: 2, Character: 0}}, ItemRange: in.ItemRange, SelectionRange: in.SelectionRange, ProvenanceKind: v5sourcesnapshotv6.ProvenanceKind, Method: v5sourcesnapshotv6.ProvenanceMethod, DocumentDigest: in.DocumentDigest, DocumentByteLength: uint64(len(in.Bytes)), DocumentVersion: in.DocumentVersion}, nil
	}), Limits: limits})
	if err != nil {
		t.Fatal("ASSERT_V6_E2E_CAPTURE:", err)
	}
	raw, replay, err := v5sourcesnapshotv6.Replay(captured.Raw, limits)
	if err != nil {
		t.Fatal("ASSERT_V6_E2E_OFFLINE_REPLAY:", err)
	}
	lookup := resolvedLookup{}
	for _, object := range captured.Objects {
		got, getErr := replay.Get(object.Identity)
		if getErr != nil {
			t.Fatal(getErr)
		}
		lookup[sourceobject.Identity{Digest: object.Identity.Digest, ByteLength: object.Identity.ByteLength}] = got.Bytes
	}
	request := fx.request
	request.Snapshots = []targetpacket.Snapshot{{ConstituentIdentity: "constituent", Raw: raw}}
	request.Lookup = lookup
	first, err := targetpacket.Build(request)
	if err != nil {
		t.Fatal("ASSERT_V6_E2E_PACKET:", err)
	}
	second, err := targetpacket.Build(request)
	if err != nil {
		t.Fatal(err)
	}
	firstBytes, _ := targetpacket.EncodeCanonical(first.Packets[0])
	secondBytes, _ := targetpacket.EncodeCanonical(second.Packets[0])
	if !bytes.Equal(firstBytes, secondBytes) {
		t.Fatal("ASSERT_V6_E2E_PACKET_DETERMINISTIC")
	}
	records1, err := Build(first, 90000)
	if err != nil {
		t.Fatal("ASSERT_V6_E2E_REQUEST:", err)
	}
	records2, _ := Build(second, 90000)
	requestBytes1, _ := Bytes(records1)
	requestBytes2, _ := Bytes(records2)
	if !bytes.Equal(requestBytes1, requestBytes2) || len(records1) != 2 {
		t.Fatal("ASSERT_V6_E2E_REQUEST_DETERMINISTIC")
	}
	for _, marker := range []string{"func C(){}", "func A(){C();C()}", "func B(){C()}"} {
		if !bytes.Contains(firstBytes, []byte(marker)) || !bytes.Contains(requestBytes1, []byte(marker)) {
			t.Fatalf("ASSERT_V6_E2E_COMPLETE_BODY %s", marker)
		}
	}
	var artifact v5sourcesnapshotv6.Artifact
	_ = json.Unmarshal(captured.Raw, &artifact)
	if len(artifact.EndpointBindings) != 3 || len(artifact.RelationBindings) != 2 {
		t.Fatalf("ASSERT_V6_E2E_EXACT_CARDINALITY endpoints=%d relations=%d", len(artifact.EndpointBindings), len(artifact.RelationBindings))
	}
	if artifact.EndpointBindings[0].DisplayProvenanceKind != v5sourcesnapshotv6.ProvenanceKind || artifact.EndpointBindings[0].DisplayMethod != v5sourcesnapshotv6.ProvenanceMethod {
		t.Fatal("ASSERT_V6_E2E_EXACT_PROVENANCE")
	}
	tampered := append([]byte(nil), captured.Raw...)
	tampered[len(tampered)/2] ^= 1
	if _, _, err := v5sourcesnapshotv6.Replay(tampered, limits); err == nil {
		t.Fatal("ASSERT_V6_E2E_TAMPER_REJECTED")
	}
}
