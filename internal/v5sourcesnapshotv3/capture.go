package v5sourcesnapshotv3

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/source"
	"lsp-trace/internal/sourceobject"
	"lsp-trace/internal/v5sourcesnapshot"
	"lsp-trace/internal/v5sourcesnapshotv2"
)

type DisplaySelection struct {
	GraphSubjectID  string
	LogicalSourceID string
	DisplayRange    graph.Range
	Endpoint        string
}

type CaptureInput struct {
	GraphV5Bytes     []byte
	Workspace        string
	PositionEncoding string
	Selections       []DisplaySelection
	Limits           Limits
}

type SourceUnavailableError struct {
	Endpoint        string
	GraphSubjectID  string
	LogicalSourceID string
}

func (e *SourceUnavailableError) Error() string {
	return fmt.Sprintf("v5sourcesnapshotv3: SOURCE_UNAVAILABLE: %s: %s\x00%s", e.Endpoint, e.GraphSubjectID, e.LogicalSourceID)
}

type CaptureResult struct {
	Raw             []byte
	Objects         []sourceobject.Object
	GraphDigest     string
	GraphByteLength uint64
	Status          string
	Authority       int
	Accepted        bool
	Completeness    string
}

type MemoryLookup struct {
	objects map[sourceobject.Identity][]byte
}

func (l MemoryLookup) Get(id sourceobject.Identity) (sourceobject.Object, error) {
	raw, ok := l.objects[id]
	if !ok {
		return sourceobject.Object{}, errors.New("source object missing")
	}
	return sourceobject.Object{Identity: id, Bytes: append([]byte(nil), raw...)}, nil
}

func Capture(in CaptureInput) (CaptureResult, error) {
	if err := checkLimits(in.Limits); err != nil {
		return CaptureResult{}, err
	}
	if _, err := canonicalWorkspace(in.Workspace); err != nil {
		return CaptureResult{}, err
	}
	_, native, err := decodeGraphV5(in.GraphV5Bytes)
	if err != nil {
		return CaptureResult{}, err
	}

	uris := map[string]bool{}
	nodes, err := historicalNodes(native)
	if err != nil {
		return CaptureResult{}, err
	}
	for _, n := range nodes {
		uris[n.URI] = true
	}
	occs, err := occurrences(native, in.Limits)
	if err != nil {
		return CaptureResult{}, err
	}
	for _, o := range occs {
		uris[o.uri] = true
	}
	if len(uris) > in.Limits.MaxReceipts {
		return CaptureResult{}, errors.New("v3 receipt limit")
	}

	ordered := make([]string, 0, len(uris))
	for uri := range uris {
		ordered = append(ordered, uri)
	}
	sort.Strings(ordered)
	root, err := os.OpenRoot(in.Workspace)
	if err != nil {
		return CaptureResult{}, err
	}
	defer root.Close()
	receipts := make([]v5sourcesnapshot.Receipt, 0, len(ordered))
	total := 0
	for _, uri := range ordered {
		rel, err := relativeEndpoint(in.Workspace, uri)
		if err != nil {
			return CaptureResult{}, err
		}
		content, err := source.ReadRegularInputBounded(root, rel, int64(in.Limits.MaxSourceBytes))
		if err != nil {
			return CaptureResult{}, fmt.Errorf("v3 capture %q: %w", uri, err)
		}
		total += len(content)
		if total > in.Limits.MaxTotalSourceBytes {
			return CaptureResult{}, errors.New("v3 total source byte limit")
		}
		_, retained, canonical, err := source.CanonicalizeReceipt(source.DiscoveredItem{ID: uri, Locator: uri}, source.Acquisition{Status: source.Readable, Provenance: source.Provenance{Mechanism: "bounded-workspace-file", Locator: uri}}, content)
		if err != nil {
			return CaptureResult{}, err
		}
		receipts = append(receipts, v5sourcesnapshot.Receipt{ID: captureDigest(canonical), URI: uri, ContentDigest: captureDigest(retained), Content: append([]byte(nil), retained...), CanonicalReceipt: append([]byte(nil), canonical...)})
	}

	v1raw, err := v5sourcesnapshot.BuildFromReceipts(in.GraphV5Bytes, in.Workspace, in.PositionEncoding, receipts)
	if err != nil {
		return CaptureResult{}, err
	}
	var v1 v5sourcesnapshot.Artifact
	if err := json.Unmarshal(v1raw, &v1); err != nil {
		return CaptureResult{}, err
	}
	parent := map[string][]v5sourcesnapshot.Binding{}
	for _, b := range v1.Bindings {
		parent[b.NodeID+"\x00"+b.URI] = append(parent[b.NodeID+"\x00"+b.URI], b)
	}
	displays := make([]v5sourcesnapshotv2.DisplayBinding, 0, len(in.Selections))
	seen := map[string]bool{}
	for _, s := range in.Selections {
		key := s.GraphSubjectID + "\x00" + s.LogicalSourceID
		rows := parent[key]
		endpoint := s.Endpoint
		if endpoint == "" {
			endpoint = "TARGET"
		}
		if len(rows) == 0 {
			return CaptureResult{}, &SourceUnavailableError{endpoint, s.GraphSubjectID, s.LogicalSourceID}
		}
		if seen[key] {
			return CaptureResult{}, errors.New("duplicate display selection")
		}
		seen[key] = true
		var receipt v5sourcesnapshot.Receipt
		for _, r := range v1.Receipts {
			if r.URI == s.LogicalSourceID {
				receipt = r
				break
			}
		}
		if receipt.ID == "" {
			return CaptureResult{}, &SourceUnavailableError{endpoint, s.GraphSubjectID, s.LogicalSourceID}
		}
		displays = append(displays, v5sourcesnapshotv2.DisplayBinding{GraphSubjectID: s.GraphSubjectID, LogicalSourceID: s.LogicalSourceID, DisplayRange: s.DisplayRange, DisplayRangePolicy: v5sourcesnapshotv2.DisplayRangePolicy, Provenance: v5sourcesnapshotv2.Provenance{Kind: v5sourcesnapshotv2.ProvenanceKind, Method: v5sourcesnapshotv2.ProvenanceMethod}, ReceiptID: receipt.ID, SourceDigest: receipt.ContentDigest, PositionEncoding: in.PositionEncoding, Status: v5sourcesnapshotv2.Status, Custody: v5sourcesnapshotv2.Custody})
	}
	v2raw, err := v5sourcesnapshotv2.Build(v1raw, displays)
	if err != nil {
		return CaptureResult{}, err
	}
	callerURIs := map[string]bool{}
	for _, o := range occs {
		callerURIs[o.uri] = true
	}
	v3Receipts := make([]Receipt, 0, len(callerURIs))
	for _, r := range receipts {
		if callerURIs[r.URI] {
			v3Receipts = append(v3Receipts, Receipt{ID: r.ID, URI: r.URI, ContentDigest: r.ContentDigest, Content: append([]byte(nil), r.Content...), CanonicalReceipt: append([]byte(nil), r.CanonicalReceipt...)})
		}
	}
	raw, err := BuildFromReceipts(v2raw, v3Receipts, in.Limits)
	if err != nil {
		return CaptureResult{}, err
	}
	objects := make([]sourceobject.Object, len(receipts))
	for i, r := range receipts {
		objects[i] = sourceobject.Object{Identity: sourceobject.Identity{Digest: r.ContentDigest, ByteLength: uint64(len(r.Content))}, Bytes: append([]byte(nil), r.Content...)}
	}
	return cloneCaptureResult(CaptureResult{Raw: raw, Objects: objects, GraphDigest: captureDigest(in.GraphV5Bytes), GraphByteLength: uint64(len(in.GraphV5Bytes)), Status: "CAPTURED", Authority: 0, Accepted: false, Completeness: Completeness})
}

func Replay(in CaptureResult) ([]byte, MemoryLookup, error) {
	if in.Status != "CAPTURED" || in.Authority != 0 || in.Accepted || in.Completeness != Completeness || in.GraphDigest != captureDigestGraph(in.Raw, in.LimitsForReplay()) {
		return nil, MemoryLookup{}, errors.New("invalid capture result")
	}
	if _, err := Validate(in.Raw, LimitsForRaw(in.Raw)); err != nil {
		return nil, MemoryLookup{}, err
	}
	objects := map[sourceobject.Identity][]byte{}
	for _, o := range in.Objects {
		if o.Identity.Digest != captureDigest(o.Bytes) || o.Identity.ByteLength != uint64(len(o.Bytes)) {
			return nil, MemoryLookup{}, errors.New("capture source object mismatch")
		}
		if _, exists := objects[o.Identity]; exists {
			return nil, MemoryLookup{}, errors.New("duplicate capture source object")
		}
		objects[o.Identity] = append([]byte(nil), o.Bytes...)
	}
	return append([]byte(nil), in.Raw...), MemoryLookup{objects: objects}, nil
}

func (CaptureResult) LimitsForReplay() Limits { return Limits{} }
func LimitsForRaw(raw []byte) Limits {
	return Limits{MaxArtifactBytes: len(raw), MaxParentBytes: len(raw), MaxReceipts: 100000, MaxSourceBytes: 16 << 20, MaxTotalSourceBytes: 128 << 20, MaxBindings: 100000, MaxWork: 300000}
}
func captureDigestGraph(raw []byte, _ Limits) string {
	var a Artifact
	if json.Unmarshal(raw, &a) != nil {
		return ""
	}
	var p v5sourcesnapshotv2.Artifact
	if json.Unmarshal(a.ParentSnapshot, &p) != nil {
		return ""
	}
	var v1 v5sourcesnapshot.Artifact
	if json.Unmarshal(p.ParentSnapshot, &v1) != nil {
		return ""
	}
	return captureDigest(v1.GraphV5Bytes)
}
func cloneCaptureResult(in CaptureResult) (CaptureResult, error) {
	out := in
	out.Raw = append([]byte(nil), in.Raw...)
	out.Objects = make([]sourceobject.Object, len(in.Objects))
	for i, o := range in.Objects {
		out.Objects[i] = sourceobject.Object{Identity: o.Identity, Bytes: append([]byte(nil), o.Bytes...)}
	}
	return out, nil
}
func captureDigest(b []byte) string {
	s := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(s[:])
}
func decodeGraphV5(raw []byte) (graphprovenance.EvidenceV5, graph.Result, error) {
	var v5 graphprovenance.EvidenceV5
	var native graph.Result
	if _, err := graphprovenance.ValidateFor(raw, graphprovenance.Family, "v5"); err != nil {
		return v5, native, err
	}
	if err := json.Unmarshal(raw, &v5); err != nil {
		return v5, native, err
	}
	nativeRaw, err := base64.StdEncoding.DecodeString(v5.GraphV5)
	if err != nil {
		return v5, native, err
	}
	if err := json.Unmarshal(nativeRaw, &native); err != nil {
		return v5, native, err
	}
	return v5, native, nil
}
func historicalNodes(native graph.Result) ([]graph.Node, error) {
	out := []graph.Node{}
	for _, s := range native.SiblingCandidates {
		if s.Declaration == nil {
			return nil, errors.New("historical V1 sibling declaration required")
		}
		out = append(out, s.Origin, *s.Declaration, s.Candidate)
	}
	return out, nil
}
