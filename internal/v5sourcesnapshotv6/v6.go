// Package v5sourcesnapshotv6 retains prepared managed-document bytes and
// independently resolved full-definition endpoint custody. It never opens paths.
package v5sourcesnapshotv6

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"unicode/utf8"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/schema"
	"lsp-trace/internal/sourceobject"
	"lsp-trace/internal/strictjson"
)

const Version = "lsp-trace.graph-v5-source-snapshot.v6"
const Family = "graph-v5-source-snapshot"
const Policy = "EXACT_GRAPH_PROVENANCE_V5_AND_MANAGED_FULL_DEFINITION_SOURCE_CUSTODY"
const Completeness = "UNKNOWN"
const SourceUnavailableCode = "EXACT_ENDPOINT_SOURCE_UNAVAILABLE"
const DisplayUnavailableCode = "FULL_DEFINITION_DISPLAY_UNAVAILABLE"
const DisplayRangePolicy = "FULL_DEFINITION"
const ProvenanceKind = "SERVER_REPORTED_DOCUMENT_SYMBOL"
const ProvenanceMethod = "textDocument/documentSymbol"

type Limits struct{ MaxArtifactBytes, MaxGraphBytes, MaxReceipts, MaxSourceBytes, MaxTotalSourceBytes, MaxBindings, MaxOutcomes, MaxWork int }
type Receipt struct {
	ID               string `json:"id"`
	URI              string `json:"uri"`
	ContentDigest    string `json:"content_digest"`
	Content          []byte `json:"content"`
	CanonicalReceipt []byte `json:"canonical_receipt"`
	DocumentVersion  string `json:"document_version"`
}
type Occurrence struct {
	RelationID, OccurrenceID, CallerNodeID, CalleeNodeID string
	Range                                                graph.Range
}
type Nomination struct {
	ID, TargetNodeID string
	Incoming         []Occurrence
}
type PreparedDocument struct {
	URI                string
	Bytes              []byte
	Digest             string
	ByteLength         uint64
	Version, SessionID string
	Generation         uint64
	PositionEncoding   string
}
type ResolveRequest struct {
	URI, GraphSubjectID, Role, SessionID, PositionEncoding, DocumentDigest, DocumentVersion string
	Generation                                                                              uint64
	EvidenceRange, ItemRange, SelectionRange                                                graph.Range
	Bytes                                                                                   []byte
}
type ResolveResult struct {
	DisplayRange, ItemRange, SelectionRange                 graph.Range
	ProvenanceKind, Method, DocumentDigest, DocumentVersion string
	DocumentByteLength                                      uint64
}
type FullDefinitionResolver interface {
	ResolveFullDefinition(context.Context, ResolveRequest) (ResolveResult, error)
}
type ResolverFunc func(context.Context, ResolveRequest) (ResolveResult, error)

func (f ResolverFunc) ResolveFullDefinition(ctx context.Context, r ResolveRequest) (ResolveResult, error) {
	return f(ctx, r)
}

type EndpointBinding struct {
	NominationID          string      `json:"nomination_id"`
	Role                  string      `json:"role"`
	GraphSubjectID        string      `json:"graph_subject_id"`
	NodeID                string      `json:"node_id"`
	LogicalSourceID       string      `json:"logical_source_id"`
	EvidenceRange         graph.Range `json:"evidence_range"`
	ItemRange             graph.Range `json:"item_range"`
	SelectionRange        graph.Range `json:"selection_range"`
	DisplayRange          graph.Range `json:"display_range"`
	DisplayRangePolicy    string      `json:"display_range_policy"`
	DisplayProvenanceKind string      `json:"display_provenance_kind"`
	DisplayMethod         string      `json:"display_method"`
	ReceiptID             string      `json:"receipt_id"`
	SourceDigest          string      `json:"source_digest"`
	SourceByteLength      uint64      `json:"source_byte_length"`
	DocumentVersion       string      `json:"document_version"`
	PositionEncoding      string      `json:"position_encoding"`
	SessionID             string      `json:"session_id"`
	Generation            uint64      `json:"generation"`
	CanonicalOrdinal      int         `json:"canonical_ordinal"`
	Authority             int         `json:"authority"`
	Accepted              bool        `json:"accepted"`
	Completeness          string      `json:"completeness"`
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
	GraphV5Bytes                []byte
	PositionEncoding, SessionID string
	Generation                  uint64
	Nominations                 []Nomination
	Documents                   []PreparedDocument
	Resolver                    FullDefinitionResolver
	Limits                      Limits
}
type CaptureResult struct {
	Raw                             []byte
	Objects                         []sourceobject.Object `json:"-"`
	Outcomes                        []Outcome
	CapturedCount, UnavailableCount int
}

type LimitCategory string

const (
	LimitCategoryInput        LimitCategory = "input"
	LimitCategoryOutput       LimitCategory = "output"
	LimitCategoryUniqueSource LimitCategory = "unique_source"
)

type LimitField string

const (
	LimitFieldGraphBytes       LimitField = "graph_bytes"
	LimitFieldSourceBytes      LimitField = "source_bytes"
	LimitFieldTotalSourceBytes LimitField = "total_source_bytes"
	LimitFieldBindings         LimitField = "bindings"
	LimitFieldOutcomes         LimitField = "outcomes"
	LimitFieldArtifactBytes    LimitField = "artifact_bytes"
)

type LimitError struct {
	Component string
	Field     LimitField
	Limit     int
	Observed  int
	Category  LimitCategory
}

func (e *LimitError) Error() string             { return "v6 " + string(e.Category) + " limit" }
func (e *LimitError) ResourceComponent() string { return e.Component }
func (e *LimitError) ResourceLimit() int64      { return int64(e.Limit) }
func (e *LimitError) ResourceObserved() int64   { return int64(e.Observed) }
func (e *LimitError) ResourceCategory() string  { return string(e.Category) }
func (e *LimitError) ResourceField() string     { return string(e.Field) }

type selected struct {
	nomination, role string
	node             graph.Node
}

func CaptureSelected(ctx context.Context, in CaptureInput) (CaptureResult, error) {
	if err := checkLimits(in.Limits); err != nil {
		return CaptureResult{}, err
	}
	if len(in.GraphV5Bytes) > in.Limits.MaxGraphBytes {
		return CaptureResult{}, &LimitError{Component: "v5sourcesnapshotv6.CaptureSelected", Field: LimitFieldGraphBytes, Limit: in.Limits.MaxGraphBytes, Observed: len(in.GraphV5Bytes), Category: LimitCategoryInput}
	}
	evidence, native, err := decode(in.GraphV5Bytes)
	if err != nil {
		return CaptureResult{}, err
	}
	if in.Resolver == nil || in.SessionID == "" || in.SessionID != evidence.SessionID || in.Generation != evidence.Generation || !validEncoding(in.PositionEncoding) {
		return CaptureResult{}, errors.New("v6 committed resolver custody")
	}
	docs := map[string]PreparedDocument{}
	total := 0
	seenSource := map[string]bool{}
	for _, d := range in.Documents {
		if d.URI == "" || docs[d.URI].URI != "" || d.SessionID != in.SessionID || d.Generation != in.Generation || d.PositionEncoding != in.PositionEncoding || d.ByteLength != uint64(len(d.Bytes)) || d.Digest != digest(d.Bytes) || d.Version == "" {
			return CaptureResult{}, errors.New("v6 prepared document mismatch")
		}
		if len(d.Bytes) > in.Limits.MaxSourceBytes {
			return CaptureResult{}, &LimitError{Component: "v5sourcesnapshotv6.CaptureSelected", Field: LimitFieldSourceBytes, Limit: in.Limits.MaxSourceBytes, Observed: len(d.Bytes), Category: LimitCategoryUniqueSource}
		}
		if !seenSource[d.Digest] {
			if len(d.Bytes) > in.Limits.MaxTotalSourceBytes-total {
				return CaptureResult{}, &LimitError{Component: "v5sourcesnapshotv6.CaptureSelected", Field: LimitFieldTotalSourceBytes, Limit: in.Limits.MaxTotalSourceBytes, Observed: total + len(d.Bytes), Category: LimitCategoryUniqueSource}
			}
			total += len(d.Bytes)
			seenSource[d.Digest] = true
		}
		docs[d.URI] = cloneDoc(d)
	}
	nodes := map[string]graph.Node{}
	for _, n := range native.Nodes {
		if nodes[n.ID].ID != "" {
			return CaptureResult{}, errors.New("v6 duplicate node")
		}
		nodes[n.ID] = n
	}
	edges := map[string]graph.Edge{}
	for _, e := range native.Edges {
		edges[e.RelationID] = e
	}
	var endpoints []selected
	var relations []struct {
		nomination string
		occurrence Occurrence
		caller     graph.Node
	}
	seen := map[string]bool{}
	for _, nom := range in.Nominations {
		if nom.ID == "" || seen[nom.ID] {
			return CaptureResult{}, errors.New("v6 duplicate nomination")
		}
		seen[nom.ID] = true
		target, ok := nodes[nom.TargetNodeID]
		if !ok {
			return CaptureResult{}, errors.New("v6 unknown target")
		}
		endpoints = append(endpoints, selected{nom.ID, "TARGET", target})
		for _, o := range nom.Incoming {
			e, ok := edges[o.RelationID]
			if !ok || e.CallerNodeID != o.CallerNodeID || e.CalleeNodeID != o.CalleeNodeID || e.CalleeNodeID != target.ID {
				return CaptureResult{}, errors.New("v6 occurrence substitution")
			}
			caller, ok := nodes[e.CallerNodeID]
			if !ok {
				return CaptureResult{}, errors.New("v6 caller substitution")
			}
			endpoints = append(endpoints, selected{nom.ID, "CALLER", caller})
			relations = append(relations, struct {
				nomination string
				occurrence Occurrence
				caller     graph.Node
			}{nom.ID, o, caller})
		}
	}
	sort.Slice(endpoints, func(i, j int) bool { return endpointKey(endpoints[i]) < endpointKey(endpoints[j]) })
	uniq := endpoints[:0]
	last := ""
	for _, e := range endpoints {
		k := endpointKey(e)
		if k != last {
			uniq = append(uniq, e)
			last = k
		}
	}
	endpoints = uniq
	if len(endpoints) > in.Limits.MaxBindings {
		return CaptureResult{}, &LimitError{Component: "v5sourcesnapshotv6.CaptureSelected", Field: LimitFieldBindings, Limit: in.Limits.MaxBindings, Observed: len(endpoints), Category: LimitCategoryInput}
	}
	if len(endpoints) > in.Limits.MaxOutcomes {
		return CaptureResult{}, &LimitError{Component: "v5sourcesnapshotv6.CaptureSelected", Field: LimitFieldOutcomes, Limit: in.Limits.MaxOutcomes, Observed: len(endpoints), Category: LimitCategoryInput}
	}
	a := Artifact{SchemaVersion: Version, Policy: Policy, GraphV5Bytes: append([]byte(nil), in.GraphV5Bytes...), GraphV5Digest: digest(in.GraphV5Bytes), GraphV5ByteLength: uint64(len(in.GraphV5Bytes)), SessionID: in.SessionID, Generation: in.Generation, PositionEncoding: in.PositionEncoding, Receipts: []Receipt{}, EndpointBindings: []EndpointBinding{}, RelationBindings: []RelationBinding{}, Outcomes: []Outcome{}, Completeness: Completeness}
	result := CaptureResult{}
	receipts := map[string]Receipt{}
	for _, e := range endpoints {
		o := Outcome{NominationID: e.nomination, Role: e.role, GraphSubjectID: e.node.ID, LogicalSourceDigest: digest([]byte(e.node.URI)), CanonicalOrdinal: len(a.Outcomes), Completeness: Completeness}
		d, ok := docs[e.node.URI]
		if !ok {
			o.Status = "SOURCE_UNAVAILABLE"
			o.Code = SourceUnavailableCode
			result.UnavailableCount++
			a.Outcomes = append(a.Outcomes, o)
			continue
		}
		rr, er := in.Resolver.ResolveFullDefinition(ctx, ResolveRequest{URI: e.node.URI, GraphSubjectID: e.node.ID, Role: e.role, SessionID: in.SessionID, Generation: in.Generation, PositionEncoding: in.PositionEncoding, DocumentDigest: d.Digest, DocumentVersion: d.Version, EvidenceRange: e.node.Range, ItemRange: e.node.Range, SelectionRange: e.node.SelectionRange, Bytes: append([]byte(nil), d.Bytes...)})
		if er != nil {
			o.Status = "SOURCE_UNAVAILABLE"
			o.Code = DisplayUnavailableCode
			result.UnavailableCount++
			a.Outcomes = append(a.Outcomes, o)
			continue
		}
		if er = validateResolution(d, e.node, rr, in.PositionEncoding); er != nil {
			return CaptureResult{}, er
		}
		r := receipt(d)
		receipts[d.URI] = r
		o.Status = "CAPTURED"
		result.CapturedCount++
		a.EndpointBindings = append(a.EndpointBindings, EndpointBinding{e.nomination, e.role, e.node.ID, e.node.ID, e.node.URI, e.node.Range, rr.ItemRange, rr.SelectionRange, rr.DisplayRange, DisplayRangePolicy, rr.ProvenanceKind, rr.Method, r.ID, d.Digest, d.ByteLength, d.Version, in.PositionEncoding, in.SessionID, in.Generation, len(a.EndpointBindings), 0, false, Completeness})
		a.Outcomes = append(a.Outcomes, o)
	}
	for _, x := range relations {
		r, ok := receipts[x.caller.URI]
		if ok {
			a.RelationBindings = append(a.RelationBindings, RelationBinding{x.nomination, x.occurrence.RelationID, x.occurrence.OccurrenceID, x.occurrence.CallerNodeID, x.occurrence.CalleeNodeID, x.caller.URI, x.occurrence.Range, r.ID, r.ContentDigest, len(a.RelationBindings)})
		}
	}
	for _, r := range receipts {
		a.Receipts = append(a.Receipts, r)
	}
	sort.Slice(a.Receipts, func(i, j int) bool { return a.Receipts[i].URI < a.Receipts[j].URI })
	raw, _ := json.Marshal(a)
	raw = append(raw, '\n')
	if len(raw) > in.Limits.MaxArtifactBytes {
		return CaptureResult{}, &LimitError{Component: "v5sourcesnapshotv6.CaptureSelected", Field: LimitFieldArtifactBytes, Limit: in.Limits.MaxArtifactBytes, Observed: len(raw), Category: LimitCategoryOutput}
	}
	if _, err = Validate(raw, in.Limits); err != nil {
		return CaptureResult{}, err
	}
	result.Raw = raw
	result.Outcomes = append([]Outcome(nil), a.Outcomes...)
	for _, r := range a.Receipts {
		result.Objects = append(result.Objects, sourceobject.Object{Identity: sourceobject.Identity{Digest: r.ContentDigest, ByteLength: uint64(len(r.Content))}, Bytes: append([]byte(nil), r.Content...)})
	}
	return result, nil
}
func Validate(raw []byte, l Limits) (string, error) {
	if err := checkLimits(l); err != nil {
		return "", err
	}
	if len(raw) > l.MaxArtifactBytes {
		return "", &LimitError{Component: "v5sourcesnapshotv6.Validate", Field: LimitFieldArtifactBytes, Limit: l.MaxArtifactBytes, Observed: len(raw), Category: LimitCategoryInput}
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
		return "", errors.New("v6 trailing json")
	}
	if a.SchemaVersion != Version || a.Policy != Policy || a.Authority != 0 || a.Accepted || a.Completeness != Completeness || a.GraphV5Digest != digest(a.GraphV5Bytes) || a.GraphV5ByteLength != uint64(len(a.GraphV5Bytes)) {
		return "", errors.New("v6 identity")
	}
	e, _, err := decode(a.GraphV5Bytes)
	if err != nil || a.SessionID != e.SessionID || a.Generation != e.Generation || !validEncoding(a.PositionEncoding) {
		return "", errors.New("v6 graph scalar substitution")
	}
	if len(a.Receipts) > l.MaxReceipts || len(a.EndpointBindings) > l.MaxBindings || len(a.Outcomes) > l.MaxOutcomes {
		return "", errors.New("v6 limits")
	}
	rs := map[string]Receipt{}
	last := ""
	for _, r := range a.Receipts {
		if r.URI <= last || r.ID != digest(r.CanonicalReceipt) || r.ContentDigest != digest(r.Content) || r.DocumentVersion == "" {
			return "", errors.New("v6 receipt")
		}
		last = r.URI
		rs[r.ID] = r
	}
	for i, b := range a.EndpointBindings {
		r, ok := rs[b.ReceiptID]
		if !ok || r.URI != b.LogicalSourceID || r.ContentDigest != b.SourceDigest || b.SourceByteLength != uint64(len(r.Content)) || b.DocumentVersion != r.DocumentVersion || b.SessionID != a.SessionID || b.Generation != a.Generation || b.PositionEncoding != a.PositionEncoding || b.CanonicalOrdinal != i || b.DisplayRangePolicy != DisplayRangePolicy || b.DisplayProvenanceKind != ProvenanceKind || b.DisplayMethod != ProvenanceMethod || !contains(b.DisplayRange, b.ItemRange) || !contains(b.ItemRange, b.SelectionRange) || !contains(b.DisplayRange, b.EvidenceRange) || validateRange(r.Content, b.DisplayRange, b.PositionEncoding) != nil {
			return "", errors.New("v6 endpoint binding")
		}
	}
	for i, o := range a.Outcomes {
		if o.CanonicalOrdinal != i || o.Completeness != Completeness || o.Authority != 0 || o.Accepted || (o.Status == "CAPTURED" && o.Code != "") || (o.Status == "SOURCE_UNAVAILABLE" && o.Code != SourceUnavailableCode && o.Code != DisplayUnavailableCode) {
			return "", errors.New("v6 outcome")
		}
	}
	if detected, err := schema.ValidateFor(raw, schema.FamilyGraphV5SourceSnapshot, "v6"); err != nil || detected != Version {
		if err == nil {
			err = errors.New("v6 schema mismatch")
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
func validateResolution(d PreparedDocument, n graph.Node, r ResolveResult, enc string) error {
	if r.DocumentDigest != d.Digest || r.DocumentByteLength != d.ByteLength || r.DocumentVersion != d.Version {
		return errors.New("v6 resolved document mismatch")
	}
	if r.ProvenanceKind != ProvenanceKind || r.Method != ProvenanceMethod || r.ItemRange != n.Range || r.SelectionRange != n.SelectionRange || !contains(r.DisplayRange, r.ItemRange) || !contains(r.ItemRange, r.SelectionRange) || !contains(r.DisplayRange, n.Range) {
		return errors.New("v6 resolved range mismatch")
	}
	return validateRange(d.Bytes, r.DisplayRange, enc)
}
func validateRange(raw []byte, r graph.Range, enc string) error {
	lines := bytes.Split(raw, []byte("\n"))
	for _, p := range []graph.Position{r.Start, r.End} {
		if p.Line < 0 || int(p.Line) >= len(lines) {
			return errors.New("range line")
		}
		line := lines[p.Line]
		units := len(line)
		if enc == "utf-16" {
			units = 0
			for len(line) > 0 {
				rr, n := utf8.DecodeRune(line)
				if rr > 0xffff {
					units += 2
				} else {
					units++
				}
				line = line[n:]
			}
		} else if enc == "utf-32" {
			units = utf8.RuneCount(line)
		}
		if p.Character < 0 || int(p.Character) > units {
			return errors.New("range character")
		}
	}
	if !posLE(r.Start, r.End) {
		return errors.New("range order")
	}
	return nil
}
func receipt(d PreparedDocument) Receipt {
	canonical, _ := json.Marshal(struct {
		URI, Digest, Version string
		Length               uint64
	}{d.URI, d.Digest, d.Version, d.ByteLength})
	return Receipt{ID: digest(canonical), URI: d.URI, ContentDigest: d.Digest, Content: append([]byte(nil), d.Bytes...), CanonicalReceipt: canonical, DocumentVersion: d.Version}
}
func cloneDoc(d PreparedDocument) PreparedDocument {
	d.Bytes = append([]byte(nil), d.Bytes...)
	return d
}
func validEncoding(s string) bool    { return s == "utf-8" || s == "utf-16" || s == "utf-32" }
func contains(a, b graph.Range) bool { return posLE(a.Start, b.Start) && posLE(b.End, a.End) }
func posLE(a, b graph.Position) bool {
	return a.Line < b.Line || (a.Line == b.Line && a.Character <= b.Character)
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
func checkLimits(l Limits) error {
	if l.MaxArtifactBytes <= 0 || l.MaxGraphBytes <= 0 || l.MaxReceipts <= 0 || l.MaxSourceBytes <= 0 || l.MaxTotalSourceBytes <= 0 || l.MaxBindings <= 0 || l.MaxOutcomes <= 0 || l.MaxWork <= 0 {
		return errors.New("v6 limits required")
	}
	return nil
}
func digest(b []byte) string { s := sha256.Sum256(b); return "sha256:" + hex.EncodeToString(s[:]) }

var _ = fmt.Sprintf
