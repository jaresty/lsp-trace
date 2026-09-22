// Package v5sourcesnapshotv3 retains source custody for every native Graph V5
// call-site occurrence while embedding an exact validated V2 predecessor.
package v5sourcesnapshotv3

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

	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/schema"
	"lsp-trace/internal/source"
	"lsp-trace/internal/strictjson"
	"lsp-trace/internal/v5sourcesnapshot"
	"lsp-trace/internal/v5sourcesnapshotv2"
)

const Version = "lsp-trace.graph-v5-source-snapshot.v3"
const Family = "graph-v5-source-snapshot"
const Policy = "EXACT_V2_PARENT_AND_NATIVE_CALL_OCCURRENCE_SOURCE_CUSTODY"

const (
	Direction    = "CALLER_TO_CALLEE"
	Provenance   = "NATIVE_GRAPH_V5_CALL_SITE"
	Status       = "RETAINED_BYTES"
	Custody      = "RETAINED"
	Completeness = "UNKNOWN"
)

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

type Binding struct {
	OccurrenceID     string      `json:"occurrence_id"`
	RelationID       string      `json:"relation_id"`
	Direction        string      `json:"direction"`
	CallerNodeID     string      `json:"caller_node_id"`
	CalleeNodeID     string      `json:"callee_node_id"`
	CallerURI        string      `json:"caller_uri"`
	Range            graph.Range `json:"range"`
	CanonicalOrdinal int         `json:"canonical_ordinal"`
	ReceiptID        string      `json:"receipt_id"`
	SourceDigest     string      `json:"source_digest"`
	Provenance       string      `json:"provenance"`
	Status           string      `json:"status"`
	Custody          string      `json:"custody"`
	Authority        int         `json:"authority"`
	Accepted         bool        `json:"accepted"`
	Completeness     string      `json:"completeness"`
}

type Artifact struct {
	SchemaVersion        string    `json:"schema_version"`
	Policy               string    `json:"policy"`
	ParentSchemaVersion  string    `json:"parent_schema_version"`
	ParentSnapshotDigest string    `json:"parent_snapshot_digest"`
	ParentSnapshot       []byte    `json:"parent_snapshot"`
	NativeGraphDigest    string    `json:"native_graph_digest"`
	Receipts             []Receipt `json:"receipts"`
	Bindings             []Binding `json:"bindings"`
}

type occurrence struct {
	relation, caller, callee, uri string
	r                             graph.Range
}

func checkLimits(l Limits) error {
	if l.MaxArtifactBytes <= 0 || l.MaxParentBytes <= 0 || l.MaxReceipts <= 0 || l.MaxSourceBytes <= 0 || l.MaxTotalSourceBytes <= 0 || l.MaxBindings <= 0 || l.MaxWork <= 0 {
		return errors.New("v3 limits must be positive")
	}
	return nil
}

func Build(parentV2 []byte, workspace string, limits Limits) ([]byte, error) {
	if err := checkLimits(limits); err != nil {
		return nil, err
	}
	if len(parentV2) > limits.MaxParentBytes {
		return nil, errors.New("v3 parent byte limit")
	}
	if _, err := v5sourcesnapshotv2.Validate(parentV2); err != nil {
		return nil, fmt.Errorf("v3 parent: %w", err)
	}
	parent, v1, v5, native, nativeRaw, err := decodeParent(parentV2)
	if err != nil {
		return nil, err
	}
	occs, err := occurrences(native, limits)
	if err != nil {
		return nil, err
	}
	uris := make([]string, 0)
	seenURI := map[string]bool{}
	nodes := nodeIndex(native)
	for _, o := range occs {
		if !seenURI[o.uri] {
			seenURI[o.uri] = true
			uris = append(uris, o.uri)
		}
		if nodes[o.caller].URI != o.uri {
			return nil, errors.New("v3 caller URI mismatch")
		}
	}
	sort.Strings(uris)
	if len(uris) > limits.MaxReceipts {
		return nil, errors.New("v3 receipt limit")
	}
	receipts, err := buildReceipts(uris, v1.Receipts, workspace, limits)
	if err != nil {
		return nil, err
	}
	bindings, err := makeBindings(occs, receipts, digest(parentV2), digest(nativeRaw), limits)
	if err != nil {
		return nil, err
	}
	artifact := Artifact{Version, Policy, v5sourcesnapshotv2.Version, digest(parentV2), append([]byte(nil), parentV2...), v5.GraphV5SHA256, receipts, bindings}
	_ = parent
	raw, err := json.Marshal(artifact)
	if err != nil {
		return nil, err
	}
	raw = append(raw, '\n')
	if len(raw) > limits.MaxArtifactBytes {
		return nil, errors.New("v3 artifact byte limit")
	}
	if _, err := Validate(raw, limits); err != nil {
		return nil, err
	}
	return raw, nil
}

// BuildFromReceipts constructs V3 purely from an exact V2 parent and the
// explicit closed call-occurrence receipt set. It performs no workspace reads.
func BuildFromReceipts(parentV2 []byte, receipts []Receipt, limits Limits) ([]byte, error) {
	if err := checkLimits(limits); err != nil {
		return nil, err
	}
	if len(parentV2) > limits.MaxParentBytes {
		return nil, errors.New("v3 parent byte limit")
	}
	if _, err := v5sourcesnapshotv2.Validate(parentV2); err != nil {
		return nil, fmt.Errorf("v3 parent: %w", err)
	}
	_, v1, v5, native, nativeRaw, err := decodeParent(parentV2)
	if err != nil {
		return nil, err
	}
	occs, err := occurrences(native, limits)
	if err != nil {
		return nil, err
	}
	cloned := make([]Receipt, len(receipts))
	for i, r := range receipts {
		cloned[i] = r
		cloned[i].Content = append([]byte(nil), r.Content...)
		cloned[i].CanonicalReceipt = append([]byte(nil), r.CanonicalReceipt...)
	}
	if err := validateReceipts(cloned, v1.Receipts, limits); err != nil {
		return nil, err
	}
	bindings, err := makeBindings(occs, cloned, digest(parentV2), digest(nativeRaw), limits)
	if err != nil {
		return nil, err
	}
	artifact := Artifact{Version, Policy, v5sourcesnapshotv2.Version, digest(parentV2), append([]byte(nil), parentV2...), v5.GraphV5SHA256, cloned, bindings}
	raw, err := json.Marshal(artifact)
	if err != nil {
		return nil, err
	}
	raw = append(raw, '\n')
	if len(raw) > limits.MaxArtifactBytes {
		return nil, errors.New("v3 artifact byte limit")
	}
	if _, err := Validate(raw, limits); err != nil {
		return nil, err
	}
	return raw, nil
}

func Validate(raw []byte, limits Limits) (string, error) {
	if err := checkLimits(limits); err != nil {
		return "", err
	}
	if len(raw) > limits.MaxArtifactBytes {
		return "", errors.New("v3 artifact byte limit")
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
		return "", errors.New("v3 trailing JSON")
	}
	if a.SchemaVersion != Version || a.Policy != Policy || a.ParentSchemaVersion != v5sourcesnapshotv2.Version {
		return "", errors.New("v3 identity substitution")
	}
	if len(a.ParentSnapshot) > limits.MaxParentBytes || !digestPattern.MatchString(a.ParentSnapshotDigest) || a.ParentSnapshotDigest != digest(a.ParentSnapshot) {
		return "", errors.New("v3 parent identity mismatch")
	}
	if _, err := v5sourcesnapshotv2.Validate(a.ParentSnapshot); err != nil {
		return "", fmt.Errorf("v3 parent: %w", err)
	}
	_, v1, v5, native, nativeRaw, err := decodeParent(a.ParentSnapshot)
	if err != nil {
		return "", err
	}
	if !digestPattern.MatchString(a.NativeGraphDigest) || a.NativeGraphDigest != digest(nativeRaw) || a.NativeGraphDigest != v5.GraphV5SHA256 {
		return "", errors.New("v3 native graph digest mismatch")
	}
	if len(a.Receipts) > limits.MaxReceipts {
		return "", errors.New("v3 receipt limit")
	}
	if err := validateReceipts(a.Receipts, v1.Receipts, limits); err != nil {
		return "", err
	}
	occs, err := occurrences(native, limits)
	if err != nil {
		return "", err
	}
	want, err := makeBindings(occs, a.Receipts, a.ParentSnapshotDigest, a.NativeGraphDigest, limits)
	if err != nil {
		return "", err
	}
	if !reflect.DeepEqual(a.Bindings, want) {
		return "", errors.New("v3 derived bindings mismatch")
	}
	if detected, err := schema.ValidateFor(raw, schema.FamilyGraphV5SourceSnapshot, "v3"); err != nil || detected != Version {
		if err == nil {
			err = errors.New("v3 schema mismatch")
		}
		return "", err
	}
	return Version, nil
}

func decodeParent(raw []byte) (v5sourcesnapshotv2.Artifact, v5sourcesnapshot.Artifact, graphprovenance.EvidenceV5, graph.Result, []byte, error) {
	var p v5sourcesnapshotv2.Artifact
	var v1 v5sourcesnapshot.Artifact
	var v5 graphprovenance.EvidenceV5
	var native graph.Result
	if err := json.Unmarshal(raw, &p); err != nil {
		return p, v1, v5, native, nil, err
	}
	if err := json.Unmarshal(p.ParentSnapshot, &v1); err != nil {
		return p, v1, v5, native, nil, err
	}
	if err := json.Unmarshal(v1.GraphV5Bytes, &v5); err != nil {
		return p, v1, v5, native, nil, err
	}
	nativeRaw, err := base64.StdEncoding.DecodeString(v5.GraphV5)
	if err != nil {
		return p, v1, v5, native, nil, err
	}
	if err := json.Unmarshal(nativeRaw, &native); err != nil {
		return p, v1, v5, native, nil, err
	}
	return p, v1, v5, native, nativeRaw, nil
}

func occurrences(native graph.Result, l Limits) ([]occurrence, error) {
	nodes := nodeIndex(native)
	out := make([]occurrence, 0)
	seen := map[string]bool{}
	for _, e := range native.Edges {
		caller, ok := nodes[e.CallerNodeID]
		if !ok || nodes[e.CalleeNodeID].ID == "" || e.RelationID == "" {
			return nil, errors.New("v3 native edge identity")
		}
		for _, r := range e.CallSites {
			if !validRange(r) {
				return nil, errors.New("v3 range invalid")
			}
			o := occurrence{e.RelationID, e.CallerNodeID, e.CalleeNodeID, caller.URI, r}
			k := occurrenceKey(o)
			if seen[k] {
				return nil, errors.New("v3 semantically indistinguishable duplicate occurrence")
			}
			seen[k] = true
			out = append(out, o)
		}
	}
	if len(out) > l.MaxBindings {
		return nil, errors.New("v3 binding limit")
	}
	sort.Slice(out, func(i, j int) bool { return occurrenceKey(out[i]) < occurrenceKey(out[j]) })
	return out, nil
}
func nodeIndex(n graph.Result) map[string]graph.Node {
	m := map[string]graph.Node{}
	for _, x := range n.Nodes {
		m[x.ID] = x
	}
	return m
}
func occurrenceKey(o occurrence) string {
	b, _ := json.Marshal(o.r)
	return strings.Join([]string{o.relation, o.caller, o.callee, o.uri, string(b)}, "\x00")
}

func makeBindings(os []occurrence, rs []Receipt, parentDigest, nativeDigest string, l Limits) ([]Binding, error) {
	uris := make([]string, 0, len(os))
	seen := map[string]bool{}
	for _, o := range os {
		if !seen[o.uri] {
			seen[o.uri] = true
			uris = append(uris, o.uri)
		}
	}
	sort.Strings(uris)
	if len(os)+len(rs)+len(uris) > l.MaxWork {
		return nil, errors.New("v3 work limit")
	}
	if len(rs) != len(uris) {
		return nil, errors.New("v3 receipt set mismatch")
	}
	idx := make(map[string]Receipt, len(rs))
	for i, r := range rs {
		if r.URI != uris[i] {
			return nil, errors.New("v3 receipt set mismatch")
		}
		idx[r.URI] = r
	}
	out := make([]Binding, len(os))
	for i, o := range os {
		r, ok := idx[o.uri]
		if !ok {
			return nil, errors.New("v3 caller source receipt missing")
		}
		pre := struct {
			Parent, Native, Relation, Caller, Callee, URI string
			Range                                         graph.Range
			Ordinal                                       int
		}{parentDigest, nativeDigest, o.relation, o.caller, o.callee, o.uri, o.r, i}
		pb, _ := json.Marshal(pre)
		out[i] = Binding{digest(pb), o.relation, Direction, o.caller, o.callee, o.uri, o.r, i, r.ID, r.ContentDigest, Provenance, Status, Custody, 0, false, Completeness}
	}
	return out, nil
}

func buildReceipts(uris []string, inherited []v5sourcesnapshot.Receipt, workspace string, l Limits) ([]Receipt, error) {
	workspace, err := canonicalWorkspace(workspace)
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(workspace)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	old := map[string]v5sourcesnapshot.Receipt{}
	for _, r := range inherited {
		old[r.URI] = r
	}
	out := make([]Receipt, 0, len(uris))
	total := 0
	for _, uri := range uris {
		if r, ok := old[uri]; ok {
			out = append(out, Receipt{r.ID, r.URI, r.ContentDigest, append([]byte(nil), r.Content...), append([]byte(nil), r.CanonicalReceipt...)})
			total += len(r.Content)
			continue
		}
		rel, err := relativeEndpoint(workspace, uri)
		if err != nil {
			return nil, err
		}
		content, err := source.ReadRegularInputBounded(root, rel, int64(l.MaxSourceBytes))
		if err != nil {
			return nil, err
		}
		_, retained, canonical, err := source.CanonicalizeReceipt(source.DiscoveredItem{ID: uri, Locator: uri}, source.Acquisition{Status: source.Readable, Provenance: source.Provenance{Mechanism: "bounded-workspace-file", Locator: uri}}, content)
		if err != nil {
			return nil, err
		}
		out = append(out, Receipt{digest(canonical), uri, digest(retained), append([]byte(nil), retained...), append([]byte(nil), canonical...)})
		total += len(retained)
		if total > l.MaxTotalSourceBytes {
			return nil, errors.New("v3 total source byte limit")
		}
	}
	return out, nil
}

func validateReceipts(rs []Receipt, inherited []v5sourcesnapshot.Receipt, l Limits) error {
	old := map[string]v5sourcesnapshot.Receipt{}
	for _, r := range inherited {
		old[r.URI] = r
	}
	last := ""
	ids := map[string]bool{}
	total := 0
	for _, r := range rs {
		if r.URI == "" || r.URI <= last || ids[r.ID] || len(r.Content) > l.MaxSourceBytes {
			return errors.New("v3 receipt identity or order")
		}
		last = r.URI
		ids[r.ID] = true
		total += len(r.Content)
		if total > l.MaxTotalSourceBytes {
			return errors.New("v3 total source byte limit")
		}
		if r.ID != digest(r.CanonicalReceipt) || r.ContentDigest != digest(r.Content) {
			return errors.New("v3 receipt digest mismatch")
		}
		if x, ok := old[r.URI]; ok && !reflect.DeepEqual(r, Receipt{x.ID, x.URI, x.ContentDigest, x.Content, x.CanonicalReceipt}) {
			return errors.New("v3 inherited receipt mismatch")
		}
		var cr source.Receipt
		if err := strictjson.RejectDuplicates(r.CanonicalReceipt); err != nil {
			return err
		}
		if err := json.Unmarshal(r.CanonicalReceipt, &cr); err != nil {
			return err
		}
		_, retained, replay, err := source.CanonicalizeReceipt(source.DiscoveredItem{ID: r.URI, Locator: r.URI}, source.Acquisition{Status: source.Readable, Provenance: source.Provenance{Mechanism: "bounded-workspace-file", Locator: r.URI}}, r.Content)
		if err != nil || !bytes.Equal(retained, r.Content) || !bytes.Equal(replay, r.CanonicalReceipt) || cr.ContentIdentity == nil || cr.ContentIdentity.Digest != r.ContentDigest {
			return errors.New("v3 receipt replay mismatch")
		}
	}
	return nil
}

func canonicalWorkspace(w string) (string, error) {
	if w == "" || !filepath.IsAbs(w) || filepath.Clean(w) != w {
		return "", errors.New("v3 canonical absolute workspace required")
	}
	return w, nil
}
func relativeEndpoint(workspace, raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "file" || (u.Host != "" && u.Host != "localhost") || u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("v3 local file URI required")
	}
	p, err := url.PathUnescape(u.EscapedPath())
	if err != nil {
		return "", err
	}
	p = filepath.FromSlash(p)
	rel, err := filepath.Rel(workspace, p)
	if err != nil || rel == "." || filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("v3 source outside workspace")
	}
	if filepath.Clean(filepath.Join(workspace, rel)) != p {
		return "", errors.New("v3 noncanonical endpoint")
	}
	return filepath.ToSlash(rel), nil
}
func validRange(r graph.Range) bool {
	return r.Start.Line < r.End.Line || r.Start.Line == r.End.Line && r.Start.Character <= r.End.Character
}
func digest(b []byte) string { s := sha256.Sum256(b); return "sha256:" + hex.EncodeToString(s[:]) }
