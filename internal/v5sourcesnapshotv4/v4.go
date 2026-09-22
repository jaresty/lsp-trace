// Package v5sourcesnapshotv4 adds independently retained Program C endpoint
// definitions to an exact, validated V3 source snapshot.
package v5sourcesnapshotv4

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
	"reflect"
	"regexp"
	"sort"
	"strings"

	"lsp-trace/internal/censusprogramc"
	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/schema"
	"lsp-trace/internal/source"
	"lsp-trace/internal/sourceobject"
	"lsp-trace/internal/strictjson"
	"lsp-trace/internal/v5sourcesnapshot"
	"lsp-trace/internal/v5sourcesnapshotv2"
	"lsp-trace/internal/v5sourcesnapshotv3"
)

const Version = "lsp-trace.graph-v5-source-snapshot.v4"
const Family = "graph-v5-source-snapshot"
const Policy = "EXACT_V3_PARENT_AND_INDEPENDENT_PROGRAM_C_ENDPOINT_CUSTODY"
const DisplayRangePolicy = "FULL_DEFINITION"
const ProvenanceKind = "SERVER_REPORTED_DOCUMENT_SYMBOL"
const ProvenanceMethod = "textDocument/documentSymbol"
const Status = "RETAINED_DISPLAY_EVIDENCE"
const Custody = "RETAINED"
const Completeness = "UNKNOWN"

var digestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

type Limits struct {
	MaxArtifactBytes, MaxParentBytes, MaxReceipts, MaxSourceBytes, MaxTotalSourceBytes, MaxBindings, MaxWork int
}

type Receipt struct {
	ID               string `json:"id"`
	URI              string `json:"uri"`
	ContentDigest    string `json:"content_digest"`
	Content          []byte `json:"content"`
	CanonicalReceipt []byte `json:"canonical_receipt"`
}

type Provenance struct {
	Kind   string `json:"kind"`
	Method string `json:"method"`
}

type EndpointBinding struct {
	Role                       string      `json:"role"`
	GraphSubjectID             string      `json:"graph_subject_id"`
	NodeID                     string      `json:"node_id"`
	LogicalSourceID            string      `json:"logical_source_id"`
	DisplayRange               graph.Range `json:"display_range"`
	DisplayRangePolicy         string      `json:"display_range_policy"`
	Provenance                 Provenance  `json:"provenance"`
	ReceiptID                  string      `json:"receipt_id"`
	SourceDigest               string      `json:"source_digest"`
	SourceByteLength           uint64      `json:"source_byte_length"`
	PositionEncoding           string      `json:"position_encoding"`
	CanonicalOrdinal           int         `json:"canonical_ordinal"`
	Status                     string      `json:"status"`
	Custody                    string      `json:"custody"`
	ConstituentGraphDigest     string      `json:"constituent_graph_digest"`
	ConstituentGraphByteLength uint64      `json:"constituent_graph_byte_length"`
	Authority                  int         `json:"authority"`
	Accepted                   bool        `json:"accepted"`
	Completeness               string      `json:"completeness"`
}

type Artifact struct {
	SchemaVersion              string            `json:"schema_version"`
	Policy                     string            `json:"policy"`
	ParentSchemaVersion        string            `json:"parent_schema_version"`
	ParentSnapshotDigest       string            `json:"parent_snapshot_digest"`
	ParentSnapshot             []byte            `json:"parent_snapshot"`
	ConstituentGraphDigest     string            `json:"constituent_graph_digest"`
	ConstituentGraphByteLength uint64            `json:"constituent_graph_byte_length"`
	EndpointReceipts           []Receipt         `json:"endpoint_receipts"`
	EndpointBindings           []EndpointBinding `json:"endpoint_bindings"`
	Authority                  int               `json:"authority"`
	Accepted                   bool              `json:"accepted"`
	Completeness               string            `json:"completeness"`
}

type endpoint struct {
	role, node, uri string
	display         graph.Range
}
type EndpointSet struct {
	graphDigest string
	graphLength uint64
	endpoints   []endpoint
}

func DeriveEndpointSet(graphV5 []byte, constituentIdentity string, constituentOrdinal int, nominations []censusprogramc.Representative) (EndpointSet, error) {
	if constituentIdentity == "" || constituentOrdinal < 0 {
		return EndpointSet{}, errors.New("v4 constituent identity required")
	}
	native, err := decodeGraphV5(graphV5)
	if err != nil {
		return EndpointSet{}, err
	}
	nodes := map[string]graph.Node{}
	for _, n := range native.Nodes {
		if _, ok := nodes[n.ID]; ok {
			return EndpointSet{}, errors.New("v4 duplicate graph node")
		}
		nodes[n.ID] = n
	}
	byKey := map[string]endpoint{}
	for _, n := range nominations {
		if n.ConstituentIdentity != constituentIdentity || n.ConstituentOrdinal != constituentOrdinal || n.SelectedNode == "" {
			return EndpointSet{}, errors.New("v4 foreign nomination")
		}
		t, ok := nodes[n.SelectedNode]
		if !ok {
			return EndpointSet{}, errors.New("v4 target absent from exact graph")
		}
		putEndpoint(byKey, endpoint{"TARGET", t.ID, t.URI, t.Range})
		seen := map[string]bool{}
		for _, p := range n.IncomingPredecessors {
			if p.RelationID == "" || p.OccurrenceID == "" || p.TargetID != t.ID || p.CallerID == "" || seen[p.OccurrenceID] {
				return EndpointSet{}, errors.New("v4 invalid exact incoming predecessor")
			}
			seen[p.OccurrenceID] = true
			c, ok := nodes[p.CallerID]
			if !ok {
				return EndpointSet{}, errors.New("v4 outward endpoint absent from exact graph")
			}
			putEndpoint(byKey, endpoint{"CALLER", c.ID, c.URI, c.Range})
		}
	}
	out := make([]endpoint, 0, len(byKey))
	for _, e := range byKey {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return endpointKey(out[i]) < endpointKey(out[j]) })
	return EndpointSet{digest(graphV5), uint64(len(graphV5)), out}, nil
}
func putEndpoint(m map[string]endpoint, e endpoint) {
	k := e.node + "\x00" + e.uri
	if old, ok := m[k]; !ok || old.role == "CALLER" && e.role == "TARGET" {
		m[k] = e
	}
}
func endpointKey(e endpoint) string {
	role := "1"
	if e.role == "TARGET" {
		role = "0"
	}
	return role + "\x00" + e.node + "\x00" + e.uri
}

func BuildFromReceipts(parentV3 []byte, receipts []Receipt, bindings []EndpointBinding, limits Limits) ([]byte, error) {
	if err := checkLimits(limits); err != nil {
		return nil, err
	}
	if len(parentV3) > limits.MaxParentBytes {
		return nil, errors.New("v4 parent byte limit")
	}
	if _, err := v5sourcesnapshotv3.Validate(parentV3, v3Limits(parentV3)); err != nil {
		return nil, fmt.Errorf("v4 parent: %w", err)
	}
	graphDigest, graphLength, err := constituentIdentity(parentV3)
	if err != nil {
		return nil, err
	}
	a := Artifact{Version, Policy, v5sourcesnapshotv3.Version, digest(parentV3), append([]byte(nil), parentV3...), graphDigest, graphLength, cloneReceipts(receipts), append([]EndpointBinding(nil), bindings...), 0, false, Completeness}
	raw, err := json.Marshal(a)
	if err != nil {
		return nil, err
	}
	raw = append(raw, '\n')
	if len(raw) > limits.MaxArtifactBytes {
		return nil, errors.New("v4 artifact byte limit")
	}
	if _, err = Validate(raw, limits); err != nil {
		return nil, err
	}
	return raw, nil
}

func Validate(raw []byte, limits Limits) (string, error) {
	if err := checkLimits(limits); err != nil {
		return "", err
	}
	if len(raw) > limits.MaxArtifactBytes {
		return "", errors.New("v4 artifact byte limit")
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
	var trailing any
	if err := d.Decode(&trailing); err != io.EOF {
		return "", errors.New("v4 trailing JSON")
	}
	if a.SchemaVersion != Version || a.Policy != Policy || a.ParentSchemaVersion != v5sourcesnapshotv3.Version {
		return "", errors.New("v4 identity substitution")
	}
	if a.Authority != 0 || a.Accepted || a.Completeness != Completeness {
		return "", errors.New("v4 authority tuple")
	}
	if len(a.ParentSnapshot) > limits.MaxParentBytes || !digestPattern.MatchString(a.ParentSnapshotDigest) || a.ParentSnapshotDigest != digest(a.ParentSnapshot) {
		return "", errors.New("v4 parent identity mismatch")
	}
	if _, err := v5sourcesnapshotv3.Validate(a.ParentSnapshot, v3Limits(a.ParentSnapshot)); err != nil {
		return "", err
	}
	gd, gl, err := constituentIdentity(a.ParentSnapshot)
	if err != nil || gd != a.ConstituentGraphDigest || gl != a.ConstituentGraphByteLength {
		return "", errors.New("v4 constituent identity mismatch")
	}
	if len(a.EndpointReceipts) > limits.MaxReceipts || len(a.EndpointBindings) > limits.MaxBindings || len(a.EndpointReceipts)+len(a.EndpointBindings) > limits.MaxWork {
		return "", errors.New("v4 limits exceeded")
	}
	if err := validateReceipts(a.EndpointReceipts, limits); err != nil {
		return "", err
	}
	if err := validateBindings(a, limits); err != nil {
		return "", err
	}
	if detected, err := schema.ValidateFor(raw, schema.FamilyGraphV5SourceSnapshot, "v4"); err != nil || detected != Version {
		if err == nil {
			err = errors.New("v4 schema mismatch")
		}
		return "", err
	}
	return Version, nil
}

func validateReceipts(rs []Receipt, l Limits) error {
	last := ""
	total := 0
	ids := map[string]bool{}
	for _, r := range rs {
		if r.URI == "" || r.URI <= last || ids[r.ID] || len(r.Content) > l.MaxSourceBytes {
			return errors.New("v4 receipt identity or order")
		}
		last = r.URI
		ids[r.ID] = true
		total += len(r.Content)
		if total > l.MaxTotalSourceBytes || r.ID != digest(r.CanonicalReceipt) || r.ContentDigest != digest(r.Content) {
			return errors.New("v4 receipt digest mismatch")
		}
		_, retained, canonical, err := source.CanonicalizeReceipt(source.DiscoveredItem{ID: r.URI, Locator: r.URI}, source.Acquisition{Status: source.Readable, Provenance: source.Provenance{Mechanism: "bounded-workspace-file", Locator: r.URI}}, r.Content)
		if err != nil || !bytes.Equal(retained, r.Content) || !bytes.Equal(canonical, r.CanonicalReceipt) {
			return errors.New("v4 receipt replay mismatch")
		}
	}
	return nil
}
func validateBindings(a Artifact, l Limits) error {
	idx := map[string]Receipt{}
	for _, r := range a.EndpointReceipts {
		idx[r.ID] = r
	}
	last := ""
	for i, b := range a.EndpointBindings {
		roleOrder := "1"
		if b.Role == "TARGET" {
			roleOrder = "0"
		}
		k := roleOrder + "\x00" + b.GraphSubjectID + "\x00" + b.LogicalSourceID
		if k <= last || b.CanonicalOrdinal != i {
			return errors.New("v4 binding order")
		}
		last = k
		r, ok := idx[b.ReceiptID]
		if !ok || r.URI != b.LogicalSourceID || r.ContentDigest != b.SourceDigest || uint64(len(r.Content)) != b.SourceByteLength {
			return errors.New("v4 binding receipt mismatch")
		}
		if (b.Role != "TARGET" && b.Role != "CALLER") || b.GraphSubjectID == "" || b.NodeID != b.GraphSubjectID || !validRange(b.DisplayRange) || b.DisplayRangePolicy != DisplayRangePolicy || b.Provenance != (Provenance{ProvenanceKind, ProvenanceMethod}) || !validEncoding(b.PositionEncoding) || b.Status != Status || b.Custody != Custody || b.ConstituentGraphDigest != a.ConstituentGraphDigest || b.ConstituentGraphByteLength != a.ConstituentGraphByteLength || b.Authority != 0 || b.Accepted || b.Completeness != Completeness {
			return errors.New("v4 binding invariant")
		}
	}
	return nil
}

func Replay(raw []byte, limits Limits) ([]byte, MemoryLookup, error) {
	if _, err := Validate(raw, limits); err != nil {
		return nil, MemoryLookup{}, err
	}
	var a Artifact
	_ = json.Unmarshal(raw, &a)
	m := map[sourceobject.Identity][]byte{}
	for _, r := range a.EndpointReceipts {
		id := sourceobject.Identity{Digest: r.ContentDigest, ByteLength: uint64(len(r.Content))}
		m[id] = append([]byte(nil), r.Content...)
	}
	var p v5sourcesnapshotv3.Artifact
	_ = json.Unmarshal(a.ParentSnapshot, &p)
	for _, r := range p.Receipts {
		id := sourceobject.Identity{Digest: r.ContentDigest, ByteLength: uint64(len(r.Content))}
		if old, ok := m[id]; ok && !bytes.Equal(old, r.Content) {
			return nil, MemoryLookup{}, errors.New("v4 source substitution")
		}
		m[id] = append([]byte(nil), r.Content...)
	}
	return append([]byte(nil), raw...), MemoryLookup{m}, nil
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

func constituentIdentity(parent []byte) (string, uint64, error) {
	var v3 v5sourcesnapshotv3.Artifact
	if err := json.Unmarshal(parent, &v3); err != nil {
		return "", 0, err
	}
	var v2 v5sourcesnapshotv2.Artifact
	if err := json.Unmarshal(v3.ParentSnapshot, &v2); err != nil {
		return "", 0, err
	}
	var v1 v5sourcesnapshot.Artifact
	if err := json.Unmarshal(v2.ParentSnapshot, &v1); err != nil {
		return "", 0, err
	}
	return v1.GraphV5Digest, uint64(len(v1.GraphV5Bytes)), nil
}
func decodeGraphV5(raw []byte) (graph.Result, error) {
	var v5 graphprovenance.EvidenceV5
	var native graph.Result
	if _, err := graphprovenance.ValidateFor(raw, graphprovenance.Family, "v5"); err != nil {
		return native, err
	}
	if err := json.Unmarshal(raw, &v5); err != nil {
		return native, err
	}
	b, err := base64.StdEncoding.DecodeString(v5.GraphV5)
	if err != nil {
		return native, err
	}
	err = json.Unmarshal(b, &native)
	return native, err
}
func checkLimits(l Limits) error {
	if l.MaxArtifactBytes <= 0 || l.MaxParentBytes <= 0 || l.MaxReceipts <= 0 || l.MaxSourceBytes <= 0 || l.MaxTotalSourceBytes <= 0 || l.MaxBindings <= 0 || l.MaxWork <= 0 {
		return errors.New("v4 limits must be positive")
	}
	return nil
}
func v3Limits(raw []byte) v5sourcesnapshotv3.Limits {
	return v5sourcesnapshotv3.Limits{MaxArtifactBytes: max(1, len(raw)), MaxParentBytes: max(1, len(raw)), MaxReceipts: 100000, MaxSourceBytes: 16 << 20, MaxTotalSourceBytes: 128 << 20, MaxBindings: 100000, MaxWork: 300000}
}
func cloneReceipts(in []Receipt) []Receipt {
	out := make([]Receipt, len(in))
	for i, r := range in {
		out[i] = r
		out[i].Content = append([]byte(nil), r.Content...)
		out[i].CanonicalReceipt = append([]byte(nil), r.CanonicalReceipt...)
	}
	return out
}
func validRange(r graph.Range) bool {
	return r.Start.Line < r.End.Line || r.Start.Line == r.End.Line && r.Start.Character <= r.End.Character
}
func validEncoding(s string) bool { return s == "utf-8" || s == "utf-16" || s == "utf-32" }
func digest(b []byte) string      { s := sha256.Sum256(b); return "sha256:" + hex.EncodeToString(s[:]) }

func canonicalWorkspace(w string) (string, error) {
	if w == "" || !filepath.IsAbs(w) || filepath.Clean(w) != w {
		return "", errors.New("v4 canonical absolute workspace required")
	}
	return w, nil
}
func relativeEndpoint(workspace, raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "file" || (u.Host != "" && u.Host != "localhost") || u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("v4 local file URI required")
	}
	p, err := url.PathUnescape(u.EscapedPath())
	if err != nil {
		return "", err
	}
	p = filepath.FromSlash(p)
	rel, err := filepath.Rel(workspace, p)
	if err != nil || rel == "." || filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("v4 source outside workspace")
	}
	if filepath.Clean(filepath.Join(workspace, rel)) != p {
		return "", errors.New("v4 noncanonical endpoint")
	}
	return filepath.ToSlash(rel), nil
}

var _ = os.OpenRoot
var _ = reflect.DeepEqual
