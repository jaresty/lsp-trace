package v5sourcesnapshotv4

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"

	"lsp-trace/internal/source"
	"lsp-trace/internal/sourceobject"
	"lsp-trace/internal/v5sourcesnapshot"
	"lsp-trace/internal/v5sourcesnapshotv2"
	"lsp-trace/internal/v5sourcesnapshotv3"
)

type CaptureInput struct {
	// GraphV5Bytes selects the canonical one-reader path and constructs the exact
	// V3 parent from the same receipt acquisition. ParentV3 is accepted only for
	// pure callers that already own an exact validated parent.
	GraphV5Bytes     []byte
	ParentV3         []byte
	Workspace        string
	PositionEncoding string
	Endpoints        EndpointSet
	Limits           Limits
}

type SourceUnavailableError struct{ Endpoint, GraphSubjectID, LogicalSourceID string }

func (e *SourceUnavailableError) Error() string {
	return fmt.Sprintf("v5sourcesnapshotv4: %s SOURCE_UNAVAILABLE: %s\x00%s", e.Endpoint, e.GraphSubjectID, e.LogicalSourceID)
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
	limits          Limits
}

func Capture(in CaptureInput) (CaptureResult, error) {
	if err := checkLimits(in.Limits); err != nil {
		return CaptureResult{}, err
	}
	workspace, err := canonicalWorkspace(in.Workspace)
	if err != nil {
		return CaptureResult{}, err
	}
	if !validEncoding(in.PositionEncoding) {
		return CaptureResult{}, errors.New("v4 position encoding")
	}

	parent := append([]byte(nil), in.ParentV3...)
	uris := map[string]bool{}
	for _, e := range in.Endpoints.endpoints {
		uris[e.uri] = true
	}
	var nativeReceipts map[string]Receipt
	var nativeCallerURIs map[string]bool
	var nativeSiblingNodes []struct{ id, uri string }
	if len(in.GraphV5Bytes) > 0 {
		if in.Endpoints.graphDigest != digest(in.GraphV5Bytes) || in.Endpoints.graphLength != uint64(len(in.GraphV5Bytes)) {
			return CaptureResult{}, errors.New("v4 endpoint set constituent mismatch")
		}
		native, e := decodeGraphV5(in.GraphV5Bytes)
		if e != nil {
			return CaptureResult{}, e
		}
		nativeCallerURIs = map[string]bool{}
		nodes := map[string]struct{ uri string }{}
		for _, n := range native.Nodes {
			nodes[n.ID] = struct{ uri string }{n.URI}
		}
		for _, edge := range native.Edges {
			n, ok := nodes[edge.CallerNodeID]
			if !ok {
				return CaptureResult{}, errors.New("v4 caller absent")
			}
			uris[n.uri] = true
			nativeCallerURIs[n.uri] = true
		}
		for _, sibling := range native.SiblingCandidates {
			if sibling.Declaration == nil {
				return CaptureResult{}, errors.New("v4 historical declaration missing")
			}
			for _, n := range []struct{ id, uri string }{{sibling.Origin.ID, sibling.Origin.URI}, {sibling.Declaration.ID, sibling.Declaration.URI}, {sibling.Candidate.ID, sibling.Candidate.URI}} {
				uris[n.uri] = true
				nativeSiblingNodes = append(nativeSiblingNodes, n)
			}
		}
	}
	ordered := make([]string, 0, len(uris))
	for uri := range uris {
		ordered = append(ordered, uri)
	}
	sort.Strings(ordered)
	root, err := os.OpenRoot(workspace)
	if err != nil {
		return CaptureResult{}, err
	}
	defer root.Close()
	all := make([]Receipt, 0, len(ordered))
	nativeReceipts = map[string]Receipt{}
	total := 0
	for _, uri := range ordered {
		rel, e := relativeEndpoint(workspace, uri)
		if e != nil {
			if endpointMember(in.Endpoints, uri) {
				return CaptureResult{}, endpointUnavailable(in.Endpoints, uri)
			}
			return CaptureResult{}, e
		}
		content, e := source.ReadRegularInputBounded(root, rel, int64(in.Limits.MaxSourceBytes))
		if e != nil {
			if endpointMember(in.Endpoints, uri) {
				return CaptureResult{}, endpointUnavailable(in.Endpoints, uri)
			}
			return CaptureResult{}, fmt.Errorf("v4 capture %q: %w", uri, e)
		}
		total += len(content)
		if total > in.Limits.MaxTotalSourceBytes {
			return CaptureResult{}, errors.New("v4 total source byte limit")
		}
		_, retained, canonical, e := source.CanonicalizeReceipt(source.DiscoveredItem{ID: uri, Locator: uri}, source.Acquisition{Status: source.Readable, Provenance: source.Provenance{Mechanism: "bounded-workspace-file", Locator: uri}}, content)
		if e != nil {
			return CaptureResult{}, e
		}
		r := Receipt{digest(canonical), uri, digest(retained), append([]byte(nil), retained...), append([]byte(nil), canonical...)}
		all = append(all, r)
		nativeReceipts[uri] = r
	}
	if len(in.GraphV5Bytes) > 0 {
		v1Receipts := make([]v5sourcesnapshot.Receipt, 0, len(nativeSiblingNodes))
		seen := map[string]bool{}
		for _, n := range nativeSiblingNodes {
			if !seen[n.uri] {
				seen[n.uri] = true
				r := nativeReceipts[n.uri]
				v1Receipts = append(v1Receipts, v5sourcesnapshot.Receipt{ID: r.ID, URI: r.URI, ContentDigest: r.ContentDigest, Content: r.Content, CanonicalReceipt: r.CanonicalReceipt})
			}
		}
		sort.Slice(v1Receipts, func(i, j int) bool { return v1Receipts[i].URI < v1Receipts[j].URI })
		v1raw, e := v5sourcesnapshot.BuildFromReceipts(in.GraphV5Bytes, workspace, in.PositionEncoding, v1Receipts)
		if e != nil {
			return CaptureResult{}, e
		}
		var v1 v5sourcesnapshot.Artifact
		_ = json.Unmarshal(v1raw, &v1)
		displays := make([]v5sourcesnapshotv2.DisplayBinding, 0)
		for _, b := range v1.Bindings {
			if b.EndpointRole == "DECLARATION" && b.RangeRole == "DECLARATION_RANGE" {
				displays = append(displays, v5sourcesnapshotv2.DisplayBinding{GraphSubjectID: b.NodeID, LogicalSourceID: b.URI, DisplayRange: b.Range, DisplayRangePolicy: v5sourcesnapshotv2.DisplayRangePolicy, Provenance: v5sourcesnapshotv2.Provenance{Kind: v5sourcesnapshotv2.ProvenanceKind, Method: v5sourcesnapshotv2.ProvenanceMethod}, ReceiptID: b.ReceiptIDs[0], SourceDigest: b.SourceDigest, PositionEncoding: in.PositionEncoding, Status: v5sourcesnapshotv2.Status, Custody: v5sourcesnapshotv2.Custody})
			}
		}
		sort.Slice(displays, func(i, j int) bool {
			return displays[i].GraphSubjectID+"\x00"+displays[i].LogicalSourceID < displays[j].GraphSubjectID+"\x00"+displays[j].LogicalSourceID
		})
		v2raw, e := v5sourcesnapshotv2.Build(v1raw, displays)
		if e != nil {
			return CaptureResult{}, e
		}
		v3Receipts := make([]v5sourcesnapshotv3.Receipt, 0, len(nativeCallerURIs))
		for _, r := range all {
			if nativeCallerURIs[r.URI] {
				v3Receipts = append(v3Receipts, v5sourcesnapshotv3.Receipt{ID: r.ID, URI: r.URI, ContentDigest: r.ContentDigest, Content: r.Content, CanonicalReceipt: r.CanonicalReceipt})
			}
		}
		parent, e = v5sourcesnapshotv3.BuildFromReceipts(v2raw, v3Receipts, v5sourcesnapshotv3.Limits{MaxArtifactBytes: in.Limits.MaxParentBytes, MaxParentBytes: in.Limits.MaxParentBytes, MaxReceipts: in.Limits.MaxReceipts, MaxSourceBytes: in.Limits.MaxSourceBytes, MaxTotalSourceBytes: in.Limits.MaxTotalSourceBytes, MaxBindings: in.Limits.MaxBindings, MaxWork: in.Limits.MaxWork})
		if e != nil {
			return CaptureResult{}, e
		}
	}
	gd, gl, err := constituentIdentity(parent)
	if err != nil {
		return CaptureResult{}, err
	}
	if gd != in.Endpoints.graphDigest || gl != in.Endpoints.graphLength {
		return CaptureResult{}, errors.New("v4 parent constituent mismatch")
	}
	endpointReceipts := make([]Receipt, 0)
	endpointURIs := map[string]bool{}
	for _, e := range in.Endpoints.endpoints {
		endpointURIs[e.uri] = true
	}
	for _, r := range all {
		if endpointURIs[r.URI] {
			endpointReceipts = append(endpointReceipts, r)
		}
	}
	bindings := make([]EndpointBinding, len(in.Endpoints.endpoints))
	for i, e := range in.Endpoints.endpoints {
		r, ok := nativeReceipts[e.uri]
		if !ok {
			return CaptureResult{}, endpointUnavailable(in.Endpoints, e.uri)
		}
		bindings[i] = EndpointBinding{e.role, e.node, e.node, e.uri, e.display, DisplayRangePolicy, Provenance{ProvenanceKind, ProvenanceMethod}, r.ID, r.ContentDigest, uint64(len(r.Content)), in.PositionEncoding, i, Status, Custody, gd, gl, 0, false, Completeness}
	}
	raw, err := BuildFromReceipts(parent, endpointReceipts, bindings, in.Limits)
	if err != nil {
		return CaptureResult{}, err
	}
	objects := make([]sourceobject.Object, len(all))
	for i, r := range all {
		objects[i] = sourceobject.Object{Identity: sourceobject.Identity{Digest: r.ContentDigest, ByteLength: uint64(len(r.Content))}, Bytes: append([]byte(nil), r.Content...)}
	}
	return CaptureResult{raw, objects, gd, gl, "CAPTURED", 0, false, Completeness, in.Limits}, nil
}

const (
	SelectedStatusCaptured          = "CAPTURED"
	SelectedStatusSourceUnavailable = "SOURCE_UNAVAILABLE"
	SelectedCodeSourceUnavailable   = "EXACT_ENDPOINT_SOURCE_UNAVAILABLE"
)

type SelectedEndpointOutcome struct {
	Role, GraphSubjectID, LogicalSourceID, Status, Code string
}

type SelectedCaptureResult struct {
	Capture          CaptureResult
	Outcomes         []SelectedEndpointOutcome
	CapturedCount    int
	UnavailableCount int
}

// CaptureSelected captures only the closed EndpointSet against an exact V3
// parent. Endpoint failures are retained individually; successful endpoints
// alone are admitted to the otherwise unchanged V4 envelope.
func CaptureSelected(in CaptureInput) (SelectedCaptureResult, error) {
	if len(in.GraphV5Bytes) != 0 || len(in.ParentV3) == 0 {
		return SelectedCaptureResult{}, errors.New("v4 selected capture requires exact parent v3")
	}
	if err := checkLimits(in.Limits); err != nil {
		return SelectedCaptureResult{}, err
	}
	workspace, err := canonicalWorkspace(in.Workspace)
	if err != nil {
		return SelectedCaptureResult{}, err
	}
	if !validEncoding(in.PositionEncoding) {
		return SelectedCaptureResult{}, errors.New("v4 position encoding")
	}
	gd, gl, err := constituentIdentity(in.ParentV3)
	if err != nil {
		return SelectedCaptureResult{}, err
	}
	if gd != in.Endpoints.graphDigest || gl != in.Endpoints.graphLength {
		return SelectedCaptureResult{}, errors.New("v4 parent constituent mismatch")
	}
	root, err := os.OpenRoot(workspace)
	if err != nil {
		return SelectedCaptureResult{}, err
	}
	defer root.Close()

	type acquired struct {
		receipt Receipt
		err     error
	}
	byURI := map[string]acquired{}
	total := 0
	for _, endpoint := range in.Endpoints.endpoints {
		if _, seen := byURI[endpoint.uri]; seen {
			continue
		}
		rel, e := relativeEndpoint(workspace, endpoint.uri)
		if e == nil {
			var content []byte
			content, e = source.ReadRegularInputBounded(root, rel, int64(in.Limits.MaxSourceBytes))
			if e == nil {
				total += len(content)
				if total > in.Limits.MaxTotalSourceBytes {
					return SelectedCaptureResult{}, errors.New("v4 total source byte limit")
				}
				_, retained, canonical, canonicalErr := source.CanonicalizeReceipt(source.DiscoveredItem{ID: endpoint.uri, Locator: endpoint.uri}, source.Acquisition{Status: source.Readable, Provenance: source.Provenance{Mechanism: "bounded-workspace-file", Locator: endpoint.uri}}, content)
				if canonicalErr != nil {
					return SelectedCaptureResult{}, canonicalErr
				}
				byURI[endpoint.uri] = acquired{receipt: Receipt{digest(canonical), endpoint.uri, digest(retained), append([]byte(nil), retained...), append([]byte(nil), canonical...)}}
				continue
			}
		}
		byURI[endpoint.uri] = acquired{err: e}
	}

	result := SelectedCaptureResult{Outcomes: make([]SelectedEndpointOutcome, 0, len(in.Endpoints.endpoints))}
	receiptByURI := map[string]Receipt{}
	bindings := make([]EndpointBinding, 0, len(in.Endpoints.endpoints))
	for _, endpoint := range in.Endpoints.endpoints {
		a := byURI[endpoint.uri]
		outcome := SelectedEndpointOutcome{Role: endpoint.role, GraphSubjectID: endpoint.node, LogicalSourceID: endpoint.uri}
		if a.err != nil {
			outcome.Status, outcome.Code = SelectedStatusSourceUnavailable, SelectedCodeSourceUnavailable
			result.UnavailableCount++
		} else {
			outcome.Status = SelectedStatusCaptured
			result.CapturedCount++
			receiptByURI[endpoint.uri] = a.receipt
			bindings = append(bindings, EndpointBinding{endpoint.role, endpoint.node, endpoint.node, endpoint.uri, endpoint.display, DisplayRangePolicy, Provenance{ProvenanceKind, ProvenanceMethod}, a.receipt.ID, a.receipt.ContentDigest, uint64(len(a.receipt.Content)), in.PositionEncoding, len(bindings), Status, Custody, gd, gl, 0, false, Completeness})
		}
		result.Outcomes = append(result.Outcomes, outcome)
	}
	receipts := make([]Receipt, 0, len(receiptByURI))
	for _, receipt := range receiptByURI {
		receipts = append(receipts, receipt)
	}
	sort.Slice(receipts, func(i, j int) bool { return receipts[i].URI < receipts[j].URI })
	raw, err := BuildFromReceipts(in.ParentV3, receipts, bindings, in.Limits)
	if err != nil {
		return SelectedCaptureResult{}, err
	}
	objects := make([]sourceobject.Object, len(receipts))
	for i, receipt := range receipts {
		objects[i] = sourceobject.Object{Identity: sourceobject.Identity{Digest: receipt.ContentDigest, ByteLength: uint64(len(receipt.Content))}, Bytes: append([]byte(nil), receipt.Content...)}
	}
	result.Capture = CaptureResult{raw, objects, gd, gl, SelectedStatusCaptured, 0, false, Completeness, in.Limits}
	return result, nil
}

func endpointMember(set EndpointSet, uri string) bool {
	for _, e := range set.endpoints {
		if e.uri == uri {
			return true
		}
	}
	return false
}
func endpointUnavailable(set EndpointSet, uri string) error {
	for _, e := range set.endpoints {
		if e.uri == uri {
			role := e.role
			if role == "CALLER" {
				role = "OUTWARD"
			}
			return &SourceUnavailableError{role, e.node, e.uri}
		}
	}
	return &SourceUnavailableError{"TARGET", "", uri}
}
func ReplayCapture(in CaptureResult) ([]byte, MemoryLookup, error) {
	if in.Status != "CAPTURED" || in.Authority != 0 || in.Accepted || in.Completeness != Completeness || in.GraphDigest == "" || in.GraphByteLength == 0 {
		return nil, MemoryLookup{}, errors.New("invalid v4 capture")
	}
	raw, lookup, err := Replay(in.Raw, in.limits)
	if err != nil {
		return nil, MemoryLookup{}, err
	}
	for _, o := range in.Objects {
		got, e := lookup.Get(o.Identity)
		if e != nil || string(got.Bytes) != string(o.Bytes) {
			return nil, MemoryLookup{}, errors.New("v4 capture object mismatch")
		}
	}
	return raw, lookup, nil
}
