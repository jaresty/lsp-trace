// Package v5sourcesnapshot binds immutable Graph Provenance V5 sibling evidence
// to exact, bounded retained source bytes read from the host-owned workspace.
package v5sourcesnapshot

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
	"sort"
	"strings"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/schema"
	"lsp-trace/internal/source"
	"lsp-trace/internal/strictjson"
)

const Version = "lsp-trace.graph-v5-source-snapshot.v1"
const Family = "graph-v5-source-snapshot"
const Policy = "EXACT_V5_AND_NATIVE_BOUNDED_RETAINED_SOURCE_BINDINGS"
const MaxBytes = 192 << 20
const MaxSourceBytes = 16 << 20
const MaxRetainedSourceBytes = 128 << 20
const MaxReceipts = 50000
const MaxBindings = 100000

type Receipt struct {
	ID               string `json:"id"`
	URI              string `json:"uri"`
	ContentDigest    string `json:"content_digest"`
	Content          []byte `json:"content"`
	CanonicalReceipt []byte `json:"canonical_receipt"`
}

type Binding struct {
	RelationID   string      `json:"relation_id"`
	EndpointRole string      `json:"endpoint_role"`
	RangeRole    string      `json:"range_role"`
	Pointer      string      `json:"pointer"`
	NodeID       string      `json:"node_id"`
	URI          string      `json:"uri"`
	Range        graph.Range `json:"range"`
	SourceDigest string      `json:"source_digest"`
	ReceiptIDs   []string    `json:"receipt_ids"`
	Status       string      `json:"status"`
}

type Artifact struct {
	SchemaVersion    string    `json:"schema_version"`
	Policy           string    `json:"policy"`
	GraphV5Bytes     []byte    `json:"graph_v5_bytes"`
	GraphV5Digest    string    `json:"graph_v5_digest"`
	SessionID        string    `json:"session_id"`
	Generation       uint64    `json:"generation"`
	PositionEncoding string    `json:"position_encoding"`
	Workspace        string    `json:"workspace"`
	Receipts         []Receipt `json:"receipts"`
	Bindings         []Binding `json:"bindings"`
}

func digest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func decodeV5(v5Bytes []byte) (graphprovenance.EvidenceV5, graph.Result, error) {
	var v5 graphprovenance.EvidenceV5
	var native graph.Result
	if _, err := graphprovenance.ValidateFor(v5Bytes, graphprovenance.Family, "v5"); err != nil {
		return v5, native, fmt.Errorf("V5 parent: %w", err)
	}
	if err := json.Unmarshal(v5Bytes, &v5); err != nil {
		return v5, native, err
	}
	raw, err := base64.StdEncoding.DecodeString(v5.GraphV5)
	if err != nil {
		return v5, native, err
	}
	if err := json.Unmarshal(raw, &native); err != nil {
		return v5, native, err
	}
	return v5, native, nil
}

func canonicalWorkspace(workspace string) (string, error) {
	if workspace == "" || !filepath.IsAbs(workspace) || filepath.Clean(workspace) != workspace {
		return "", errors.New("source snapshot requires canonical absolute workspace")
	}
	return workspace, nil
}

func relativeEndpoint(workspace, uri string) (string, error) {
	u, err := url.Parse(uri)
	if err != nil || u.Scheme != "file" || (u.Host != "" && u.Host != "localhost") || u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("source snapshot endpoint must be a local file URI")
	}
	path, err := url.PathUnescape(u.EscapedPath())
	if err != nil {
		return "", err
	}
	path = filepath.FromSlash(path)
	rel, err := filepath.Rel(workspace, path)
	if err != nil || rel == "." || filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("source snapshot endpoint outside workspace")
	}
	rel = filepath.ToSlash(rel)
	if filepath.ToSlash(filepath.Join(workspace, filepath.FromSlash(rel))) != filepath.ToSlash(path) {
		return "", errors.New("source snapshot endpoint is not canonical")
	}
	return rel, nil
}

func endpointNodes(native graph.Result) ([]graph.Node, error) {
	nodes := make([]graph.Node, 0, len(native.SiblingCandidates)*3)
	for _, sibling := range native.SiblingCandidates {
		if sibling.Declaration == nil {
			return nil, errors.New("source snapshot sibling declaration required")
		}
		nodes = append(nodes, sibling.Origin, *sibling.Declaration, sibling.Candidate)
	}
	return nodes, nil
}

func captureReceipts(native graph.Result, workspace string) ([]Receipt, error) {
	workspace, err := canonicalWorkspace(workspace)
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(workspace)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	nodes, err := endpointNodes(native)
	if err != nil {
		return nil, err
	}
	uris := map[string]bool{}
	for _, node := range nodes {
		if node.URI == "" {
			return nil, errors.New("source snapshot endpoint URI required")
		}
		uris[node.URI] = true
	}
	ordered := make([]string, 0, len(uris))
	for uri := range uris {
		ordered = append(ordered, uri)
	}
	sort.Strings(ordered)
	if len(ordered) > MaxReceipts {
		return nil, errors.New("source snapshot receipt limit")
	}
	receipts := make([]Receipt, 0, len(ordered))
	used := 0
	for _, uri := range ordered {
		rel, err := relativeEndpoint(workspace, uri)
		if err != nil {
			return nil, err
		}
		content, err := source.ReadRegularInputBounded(root, rel, MaxSourceBytes)
		if err != nil {
			return nil, fmt.Errorf("source snapshot read %q: %w", uri, err)
		}
		used += len(content)
		if used > MaxRetainedSourceBytes {
			return nil, errors.New("source snapshot retained source byte limit")
		}
		_, retained, canonical, err := source.CanonicalizeReceipt(
			source.DiscoveredItem{ID: uri, Locator: uri},
			source.Acquisition{Status: source.Readable, Provenance: source.Provenance{Mechanism: "bounded-workspace-file", Locator: uri}},
			content,
		)
		if err != nil {
			return nil, err
		}
		receipts = append(receipts, Receipt{ID: digest(canonical), URI: uri, ContentDigest: digest(retained), Content: retained, CanonicalReceipt: canonical})
	}
	return receipts, nil
}

func receiptIndex(receipts []Receipt, workspace string) (map[string]Receipt, error) {
	if len(receipts) > MaxReceipts {
		return nil, errors.New("source snapshot receipt limit")
	}
	if _, err := canonicalWorkspace(workspace); err != nil {
		return nil, err
	}
	index := map[string]Receipt{}
	ids := map[string]bool{}
	used := 0
	for _, receipt := range receipts {
		if receipt.ID == "" || receipt.URI == "" || ids[receipt.ID] {
			return nil, errors.New("source snapshot duplicate or missing receipt identity")
		}
		if _, exists := index[receipt.URI]; exists {
			return nil, errors.New("source snapshot ambiguous endpoint receipt")
		}
		if _, err := relativeEndpoint(workspace, receipt.URI); err != nil {
			return nil, err
		}
		if len(receipt.Content) > MaxSourceBytes {
			return nil, errors.New("source snapshot source byte limit")
		}
		used += len(receipt.Content)
		if used > MaxRetainedSourceBytes {
			return nil, errors.New("source snapshot retained source byte limit")
		}
		if receipt.ID != digest(receipt.CanonicalReceipt) || receipt.ContentDigest != digest(receipt.Content) {
			return nil, errors.New("source snapshot receipt identity mismatch")
		}
		var canonical source.Receipt
		if err := strictjson.RejectDuplicates(receipt.CanonicalReceipt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(receipt.CanonicalReceipt, &canonical); err != nil {
			return nil, err
		}
		_, retained, replay, err := source.CanonicalizeReceipt(
			source.DiscoveredItem{ID: receipt.URI, Locator: receipt.URI},
			source.Acquisition{Status: source.Readable, Provenance: source.Provenance{Mechanism: "bounded-workspace-file", Locator: receipt.URI}},
			receipt.Content,
		)
		if err != nil || !bytes.Equal(retained, receipt.Content) || !bytes.Equal(replay, receipt.CanonicalReceipt) || canonical.ContentIdentity == nil || canonical.ContentIdentity.Digest != receipt.ContentDigest {
			return nil, errors.New("source snapshot receipt replay mismatch")
		}
		ids[receipt.ID] = true
		index[receipt.URI] = receipt
	}
	return index, nil
}

func derive(native graph.Result, receipts []Receipt, workspace string) ([]Binding, error) {
	index, err := receiptIndex(receipts, workspace)
	if err != nil {
		return nil, err
	}
	bindings := make([]Binding, 0, len(native.SiblingCandidates)*6)
	for i, sibling := range native.SiblingCandidates {
		if sibling.Declaration == nil {
			return nil, errors.New("source snapshot sibling declaration required")
		}
		for _, endpoint := range []struct {
			role, name string
			node       graph.Node
		}{
			{"ORIGIN", "origin", sibling.Origin}, {"DECLARATION", "document_symbol", *sibling.Declaration}, {"PREPARED", "candidate", sibling.Candidate},
		} {
			receipt, ok := index[endpoint.node.URI]
			if !ok {
				return nil, errors.New("source snapshot endpoint has no exact retained bytes")
			}
			for _, rr := range []struct {
				role, field string
				value       graph.Range
			}{
				{"DECLARATION_RANGE", "range", endpoint.node.Range}, {"SELECTION_RANGE", "selection_range", endpoint.node.SelectionRange},
			} {
				bindings = append(bindings, Binding{RelationID: sibling.RelationID, EndpointRole: endpoint.role, RangeRole: rr.role, Pointer: fmt.Sprintf("/graph/sibling_candidates/%d/%s/%s", i, endpoint.name, rr.field), NodeID: endpoint.node.ID, URI: endpoint.node.URI, Range: rr.value, SourceDigest: receipt.ContentDigest, ReceiptIDs: []string{receipt.ID}, Status: "RETAINED_BYTES"})
			}
		}
	}
	if len(bindings) > MaxBindings {
		return nil, errors.New("source snapshot binding limit")
	}
	return bindings, nil
}

func validEncoding(value string) bool {
	return value == "utf-8" || value == "utf-16" || value == "utf-32"
}

func Build(v5Bytes []byte, workspace, positionEncoding string) ([]byte, error) {
	if len(v5Bytes) > MaxBytes {
		return nil, errors.New("source snapshot input byte limit")
	}
	if !validEncoding(positionEncoding) {
		return nil, errors.New("source snapshot position encoding")
	}
	v5, native, err := decodeV5(v5Bytes)
	if err != nil {
		return nil, err
	}
	if v5.SessionID == "" || v5.Generation == 0 {
		return nil, errors.New("source snapshot V5 parent session or generation missing")
	}
	receipts, err := captureReceipts(native, workspace)
	if err != nil {
		return nil, err
	}
	bindings, err := derive(native, receipts, workspace)
	if err != nil {
		return nil, err
	}
	artifact := Artifact{SchemaVersion: Version, Policy: Policy, GraphV5Bytes: append([]byte{}, v5Bytes...), GraphV5Digest: digest(v5Bytes), SessionID: v5.SessionID, Generation: v5.Generation, PositionEncoding: positionEncoding, Workspace: workspace, Receipts: receipts, Bindings: bindings}
	raw, err := json.Marshal(artifact)
	if err != nil {
		return nil, err
	}
	raw = append(raw, '\n')
	if len(raw) > MaxBytes {
		return nil, errors.New("source snapshot output byte limit")
	}
	if _, err := Validate(raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func Validate(raw []byte) (string, error) {
	if len(raw) > MaxBytes {
		return "", errors.New("source snapshot output byte limit")
	}
	if err := strictjson.RejectDuplicates(raw); err != nil {
		return "", err
	}
	if detected, err := schema.ValidateFor(raw, schema.FamilyGraphV5SourceSnapshot, "v1"); err != nil || detected != Version {
		if err == nil {
			err = errors.New("source snapshot schema version mismatch")
		}
		return "", err
	}
	var artifact Artifact
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&artifact); err != nil {
		return "", err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return "", errors.New("source snapshot trailing JSON")
	}
	if artifact.SchemaVersion != Version || artifact.Policy != Policy || artifact.GraphV5Digest != digest(artifact.GraphV5Bytes) || !validEncoding(artifact.PositionEncoding) {
		return "", errors.New("source snapshot identity mismatch")
	}
	v5, native, err := decodeV5(artifact.GraphV5Bytes)
	if err != nil {
		return "", err
	}
	if artifact.SessionID != v5.SessionID || artifact.Generation != v5.Generation {
		return "", errors.New("source snapshot parent identity mismatch")
	}
	want, err := derive(native, artifact.Receipts, artifact.Workspace)
	if err != nil {
		return "", err
	}
	if !reflect.DeepEqual(artifact.Bindings, want) {
		return "", errors.New("source snapshot derived bindings mismatch")
	}
	return Version, nil
}
