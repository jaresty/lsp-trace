// Package v5sourcesnapshotv5 retains selected Program C endpoint and call-site
// source custody directly against exact Graph Provenance V5 bytes.
package v5sourcesnapshotv5

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/schema"
	"lsp-trace/internal/source"
	"lsp-trace/internal/sourceobject"
	"lsp-trace/internal/strictjson"
)

const Version = "lsp-trace.graph-v5-source-snapshot.v5"
const Family = "graph-v5-source-snapshot"
const Policy = "EXACT_GRAPH_PROVENANCE_V5_AND_SELECTED_ENDPOINT_SOURCE_CUSTODY"
const Completeness = "UNKNOWN"
const SourceUnavailableCode = "EXACT_ENDPOINT_SOURCE_UNAVAILABLE"

type Limits struct{ MaxArtifactBytes, MaxGraphBytes, MaxReceipts, MaxSourceBytes, MaxTotalSourceBytes, MaxBindings, MaxOutcomes, MaxWork int }
type Receipt struct {
	ID               string `json:"id"`
	URI              string `json:"uri"`
	ContentDigest    string `json:"content_digest"`
	Content          []byte `json:"content"`
	CanonicalReceipt []byte `json:"canonical_receipt"`
}
type Occurrence struct {
	RelationID   string
	OccurrenceID string
	CallerNodeID string
	CalleeNodeID string
	Range        graph.Range
}
type Nomination struct {
	ID           string
	TargetNodeID string
	Incoming     []Occurrence
}
type EndpointBinding struct {
	NominationID     string      `json:"nomination_id"`
	Role             string      `json:"role"`
	GraphSubjectID   string      `json:"graph_subject_id"`
	NodeID           string      `json:"node_id"`
	LogicalSourceID  string      `json:"logical_source_id"`
	DisplayRange     graph.Range `json:"display_range"`
	ReceiptID        string      `json:"receipt_id"`
	SourceDigest     string      `json:"source_digest"`
	SourceByteLength uint64      `json:"source_byte_length"`
	CanonicalOrdinal int         `json:"canonical_ordinal"`
	Authority        int         `json:"authority"`
	Accepted         bool        `json:"accepted"`
	Completeness     string      `json:"completeness"`
}
type RelationBinding struct {
	NominationID          string      `json:"nomination_id"`
	RelationID            string      `json:"relation_id"`
	OccurrenceID          string      `json:"occurrence_id"`
	CallerNodeID          string      `json:"caller_node_id"`
	CalleeNodeID          string      `json:"callee_node_id"`
	CallerLogicalSourceID string      `json:"caller_logical_source_id"`
	Range                 graph.Range `json:"range"`
	ReceiptID             string      `json:"receipt_id"`
	SourceDigest          string      `json:"source_digest"`
	CanonicalOrdinal      int         `json:"canonical_ordinal"`
}
type Outcome struct {
	NominationID        string `json:"nomination_id"`
	Role                string `json:"role"`
	GraphSubjectID      string `json:"graph_subject_id"`
	LogicalSourceDigest string `json:"logical_source_digest"`
	Status              string `json:"status"`
	Code                string `json:"code,omitempty"`
	CanonicalOrdinal    int    `json:"canonical_ordinal"`
	Authority           int    `json:"authority"`
	Accepted            bool   `json:"accepted"`
	Completeness        string `json:"completeness"`
}
type Artifact struct {
	SchemaVersion     string            `json:"schema_version"`
	Policy            string            `json:"policy"`
	GraphV5Bytes      []byte            `json:"graph_v5_bytes"`
	GraphV5Digest     string            `json:"graph_v5_digest"`
	GraphV5ByteLength uint64            `json:"graph_v5_byte_length"`
	SessionID         string            `json:"session_id"`
	Generation        uint64            `json:"generation"`
	PositionEncoding  string            `json:"position_encoding"`
	Receipts          []Receipt         `json:"receipts"`
	EndpointBindings  []EndpointBinding `json:"endpoint_bindings"`
	RelationBindings  []RelationBinding `json:"relation_bindings"`
	Outcomes          []Outcome         `json:"outcomes"`
	Authority         int               `json:"authority"`
	Accepted          bool              `json:"accepted"`
	Completeness      string            `json:"completeness"`
}
type CaptureInput struct {
	GraphV5Bytes     []byte
	Workspace        string
	PositionEncoding string
	Nominations      []Nomination
	Limits           Limits
}
type CaptureResult struct {
	Raw              []byte
	Objects          []sourceobject.Object
	Outcomes         []Outcome
	CapturedCount    int
	UnavailableCount int
}

type selected struct {
	nomination, role string
	node             graph.Node
}

func CaptureSelected(in CaptureInput) (CaptureResult, error) {
	if err := checkLimits(in.Limits); err != nil {
		return CaptureResult{}, err
	}
	evidence, native, err := decode(in.GraphV5Bytes)
	if err != nil {
		return CaptureResult{}, err
	}
	if in.PositionEncoding != "utf-8" && in.PositionEncoding != "utf-16" && in.PositionEncoding != "utf-32" {
		return CaptureResult{}, errors.New("v5 position encoding")
	}
	if in.Workspace == "" || !filepath.IsAbs(in.Workspace) || filepath.Clean(in.Workspace) != in.Workspace {
		return CaptureResult{}, errors.New("v5 canonical workspace required")
	}
	nodes := map[string]graph.Node{}
	for _, n := range native.Nodes {
		if _, ok := nodes[n.ID]; ok {
			return CaptureResult{}, errors.New("v5 duplicate node")
		}
		nodes[n.ID] = n
	}
	edges := map[string]graph.Edge{}
	for _, e := range native.Edges {
		if _, ok := edges[e.RelationID]; ok {
			return CaptureResult{}, errors.New("v5 duplicate relation")
		}
		edges[e.RelationID] = e
	}
	var endpoints []selected
	var relations []struct {
		nomination string
		occurrence Occurrence
		caller     graph.Node
	}
	seenNom := map[string]bool{}
	for _, n := range in.Nominations {
		if n.ID == "" || seenNom[n.ID] {
			return CaptureResult{}, errors.New("v5 duplicate nomination")
		}
		seenNom[n.ID] = true
		target, ok := nodes[n.TargetNodeID]
		if !ok {
			return CaptureResult{}, errors.New("v5 unknown target")
		}
		endpoints = append(endpoints, selected{n.ID, "TARGET", target})
		seenOcc := map[string]bool{}
		for _, requested := range n.Incoming {
			e, ok := edges[requested.RelationID]
			callSiteMatches := 0
			for _, callSite := range e.CallSites {
				if callSite == requested.Range {
					callSiteMatches++
				}
			}
			if !ok || requested.OccurrenceID == "" || seenOcc[requested.OccurrenceID] || e.CallerNodeID != requested.CallerNodeID || e.CalleeNodeID != requested.CalleeNodeID || e.CalleeNodeID != target.ID || callSiteMatches != 1 {
				return CaptureResult{}, fmt.Errorf("v5 occurrence substitution: relation=%t occurrence=%t duplicate=%t caller=%t callee=%t target=%t range_matches=%d", ok, requested.OccurrenceID != "", seenOcc[requested.OccurrenceID], e.CallerNodeID == requested.CallerNodeID, e.CalleeNodeID == requested.CalleeNodeID, e.CalleeNodeID == target.ID, callSiteMatches)
			}
			seenOcc[requested.OccurrenceID] = true
			caller, ok := nodes[e.CallerNodeID]
			if !ok || caller.URI == "" {
				return CaptureResult{}, errors.New("v5 caller substitution")
			}
			endpoints = append(endpoints, selected{n.ID, "CALLER", caller})
			relations = append(relations, struct {
				nomination string
				occurrence Occurrence
				caller     graph.Node
			}{n.ID, requested, caller})
		}
	}
	sort.Slice(endpoints, func(i, j int) bool { return endpointKey(endpoints[i]) < endpointKey(endpoints[j]) })
	canonicalEndpoints := endpoints[:0]
	lastEndpoint := ""
	for _, endpoint := range endpoints {
		key := endpointKey(endpoint)
		if key == lastEndpoint {
			continue
		}
		canonicalEndpoints = append(canonicalEndpoints, endpoint)
		lastEndpoint = key
	}
	endpoints = canonicalEndpoints
	root, err := os.OpenRoot(in.Workspace)
	if err != nil {
		return CaptureResult{}, err
	}
	defer root.Close()
	byURI := map[string]Receipt{}
	unavailable := map[string]bool{}
	total := 0
	for _, e := range endpoints {
		if _, ok := byURI[e.node.URI]; ok || unavailable[e.node.URI] {
			continue
		}
		rel, er := relative(in.Workspace, e.node.URI)
		if er != nil {
			unavailable[e.node.URI] = true
			continue
		}
		content, er := source.ReadRegularInputBounded(root, rel, int64(in.Limits.MaxSourceBytes))
		if er != nil {
			unavailable[e.node.URI] = true
			continue
		}
		total += len(content)
		if total > in.Limits.MaxTotalSourceBytes {
			return CaptureResult{}, errors.New("v5 total source limit")
		}
		_, retained, canonical, er := source.CanonicalizeReceipt(source.DiscoveredItem{ID: e.node.URI, Locator: e.node.URI}, source.Acquisition{Status: source.Readable, Provenance: source.Provenance{Mechanism: "bounded-workspace-file", Locator: e.node.URI}}, content)
		if er != nil {
			return CaptureResult{}, er
		}
		byURI[e.node.URI] = Receipt{digest(canonical), e.node.URI, digest(retained), retained, canonical}
	}
	artifact := Artifact{SchemaVersion: Version, Policy: Policy, GraphV5Bytes: append([]byte(nil), in.GraphV5Bytes...), GraphV5Digest: digest(in.GraphV5Bytes), GraphV5ByteLength: uint64(len(in.GraphV5Bytes)), SessionID: evidence.SessionID, Generation: evidence.Generation, PositionEncoding: in.PositionEncoding, Receipts: []Receipt{}, EndpointBindings: []EndpointBinding{}, RelationBindings: []RelationBinding{}, Outcomes: []Outcome{}, Completeness: Completeness}
	result := CaptureResult{}
	for _, e := range endpoints {
		o := Outcome{NominationID: e.nomination, Role: e.role, GraphSubjectID: e.node.ID, LogicalSourceDigest: digest([]byte(e.node.URI)), CanonicalOrdinal: len(artifact.Outcomes), Completeness: Completeness}
		if r, ok := byURI[e.node.URI]; ok {
			o.Status = "CAPTURED"
			result.CapturedCount++
			artifact.EndpointBindings = append(artifact.EndpointBindings, EndpointBinding{e.nomination, e.role, e.node.ID, e.node.ID, e.node.URI, e.node.Range, r.ID, r.ContentDigest, uint64(len(r.Content)), len(artifact.EndpointBindings), 0, false, Completeness})
		} else {
			o.Status = "SOURCE_UNAVAILABLE"
			o.Code = SourceUnavailableCode
			result.UnavailableCount++
		}
		artifact.Outcomes = append(artifact.Outcomes, o)
	}
	for _, x := range relations {
		r, ok := byURI[x.caller.URI]
		if !ok {
			continue
		}
		artifact.RelationBindings = append(artifact.RelationBindings, RelationBinding{x.nomination, x.occurrence.RelationID, x.occurrence.OccurrenceID, x.occurrence.CallerNodeID, x.occurrence.CalleeNodeID, x.caller.URI, x.occurrence.Range, r.ID, r.ContentDigest, len(artifact.RelationBindings)})
	}
	for _, r := range byURI {
		artifact.Receipts = append(artifact.Receipts, r)
	}
	sort.Slice(artifact.Receipts, func(i, j int) bool { return artifact.Receipts[i].URI < artifact.Receipts[j].URI })
	raw, _ := json.Marshal(artifact)
	raw = append(raw, '\n')
	if len(raw) > in.Limits.MaxArtifactBytes {
		return CaptureResult{}, errors.New("v5 artifact limit")
	}
	if _, err = Validate(raw, in.Limits); err != nil {
		return CaptureResult{}, err
	}
	result.Raw = raw
	result.Outcomes = append([]Outcome(nil), artifact.Outcomes...)
	for _, r := range artifact.Receipts {
		result.Objects = append(result.Objects, sourceobject.Object{Identity: sourceobject.Identity{Digest: r.ContentDigest, ByteLength: uint64(len(r.Content))}, Bytes: append([]byte(nil), r.Content...)})
	}
	return result, nil
}
func Validate(raw []byte, l Limits) (string, error) {
	if err := checkLimits(l); err != nil {
		return "", err
	}
	if len(raw) > l.MaxArtifactBytes {
		return "", errors.New("v5 artifact limit")
	}
	if err := strictjson.RejectDuplicates(raw); err != nil {
		return "", err
	}
	var a Artifact
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&a); err != nil {
		return "", err
	}
	if err := d.Decode(&struct{}{}); err != io.EOF {
		return "", errors.New("v5 trailing json")
	}
	if a.SchemaVersion != Version || a.Policy != Policy || a.Authority != 0 || a.Accepted || a.Completeness != Completeness || a.GraphV5Digest != digest(a.GraphV5Bytes) || a.GraphV5ByteLength != uint64(len(a.GraphV5Bytes)) {
		return "", errors.New("v5 identity")
	}
	e, _, err := decode(a.GraphV5Bytes)
	if err != nil || a.SessionID != e.SessionID || a.Generation != e.Generation {
		return "", errors.New("v5 graph scalar substitution")
	}
	if len(a.Receipts) > l.MaxReceipts || len(a.EndpointBindings) > l.MaxBindings || len(a.Outcomes) > l.MaxOutcomes {
		return "", errors.New("v5 limits")
	}
	receipt := map[string]Receipt{}
	last := ""
	for _, r := range a.Receipts {
		if r.URI <= last || r.ID != digest(r.CanonicalReceipt) || r.ContentDigest != digest(r.Content) {
			return "", errors.New("v5 receipt")
		}
		last = r.URI
		receipt[r.ID] = r
	}
	for i, b := range a.EndpointBindings {
		r, ok := receipt[b.ReceiptID]
		if !ok || r.URI != b.LogicalSourceID || r.ContentDigest != b.SourceDigest || b.CanonicalOrdinal != i || b.NodeID != b.GraphSubjectID || b.Authority != 0 || b.Accepted || b.Completeness != Completeness {
			return "", errors.New("v5 endpoint binding")
		}
	}
	for i, o := range a.Outcomes {
		if o.CanonicalOrdinal != i || o.Authority != 0 || o.Accepted || o.Completeness != Completeness || (o.Status == "CAPTURED" && o.Code != "") || (o.Status == "SOURCE_UNAVAILABLE" && o.Code != SourceUnavailableCode) {
			return "", errors.New("v5 outcome")
		}
	}
	if detected, err := schema.ValidateFor(raw, schema.FamilyGraphV5SourceSnapshot, "v5"); err != nil || detected != Version {
		if err == nil {
			err = errors.New("v5 schema mismatch")
		}
		return "", err
	}
	return Version, nil
}

type MemoryLookup struct {
	objects map[sourceobject.Identity][]byte
}

func (l MemoryLookup) Get(id sourceobject.Identity) (sourceobject.Object, error) {
	b, ok := l.objects[id]
	if !ok {
		return sourceobject.Object{}, errors.New("source object missing")
	}
	return sourceobject.Object{Identity: id, Bytes: append([]byte(nil), b...)}, nil
}
func Replay(raw []byte, l Limits) ([]byte, MemoryLookup, error) {
	if _, err := Validate(raw, l); err != nil {
		return nil, MemoryLookup{}, err
	}
	var a Artifact
	_ = json.Unmarshal(raw, &a)
	m := map[sourceobject.Identity][]byte{}
	for _, r := range a.Receipts {
		m[sourceobject.Identity{Digest: r.ContentDigest, ByteLength: uint64(len(r.Content))}] = append([]byte(nil), r.Content...)
	}
	return append([]byte(nil), raw...), MemoryLookup{m}, nil
}
func decode(raw []byte) (graphprovenance.EvidenceV5, graph.Result, error) {
	var e graphprovenance.EvidenceV5
	var g graph.Result
	if _, err := graphprovenance.ValidateFor(raw, graphprovenance.Family, "v5"); err != nil {
		return e, g, err
	}
	if err := json.Unmarshal(raw, &e); err != nil {
		return e, g, err
	}
	b, err := base64.StdEncoding.DecodeString(e.GraphV5)
	if err != nil {
		return e, g, err
	}
	err = json.Unmarshal(b, &g)
	return e, g, err
}
func endpointKey(e selected) string {
	role := "1"
	if e.role == "TARGET" {
		role = "0"
	}
	return e.nomination + "\x00" + role + "\x00" + e.node.ID + "\x00" + e.node.URI
}
func relative(workspace, raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "file" || (u.Host != "" && u.Host != "localhost") || u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("v5 file uri")
	}
	p, err := url.PathUnescape(u.EscapedPath())
	if err != nil {
		return "", err
	}
	p = filepath.FromSlash(p)
	rel, err := filepath.Rel(workspace, p)
	if err != nil || rel == "." || filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("v5 outside workspace")
	}
	return filepath.ToSlash(rel), nil
}
func checkLimits(l Limits) error {
	if l.MaxArtifactBytes <= 0 || l.MaxGraphBytes <= 0 || l.MaxReceipts <= 0 || l.MaxSourceBytes <= 0 || l.MaxTotalSourceBytes <= 0 || l.MaxBindings <= 0 || l.MaxOutcomes <= 0 || l.MaxWork <= 0 {
		return errors.New("v5 limits required")
	}
	return nil
}
func digest(b []byte) string { s := sha256.Sum256(b); return "sha256:" + hex.EncodeToString(s[:]) }
