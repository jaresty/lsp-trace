// Package v5sourcesnapshotv2 validates retained full-definition display evidence
// bound to an already-admitted graph-v5 source snapshot without duplicating its
// retained source bytes.
package v5sourcesnapshotv2

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"

	"lsp-trace/internal/graph"
	"lsp-trace/internal/schema"
	"lsp-trace/internal/strictjson"
	"lsp-trace/internal/v5sourcesnapshot"
)

const Version = "lsp-trace.graph-v5-source-snapshot.v2"
const Family = "graph-v5-source-snapshot"
const Policy = "EXACT_V1_SNAPSHOT_AND_RETAINED_FULL_DEFINITION_DISPLAY_BINDINGS"
const DisplayRangePolicy = "FULL_DEFINITION"
const ProvenanceKind = "SERVER_REPORTED_DOCUMENT_SYMBOL"
const ProvenanceMethod = "textDocument/documentSymbol"
const Status = "RETAINED_DISPLAY_EVIDENCE"
const Custody = "RETAINED"
const MaxBytes = v5sourcesnapshot.MaxBytes
const MaxBindings = v5sourcesnapshot.MaxBindings

var digestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

type Provenance struct {
	Kind   string `json:"kind"`
	Method string `json:"method"`
}

type DisplayBinding struct {
	GraphSubjectID     string      `json:"graph_subject_id"`
	LogicalSourceID    string      `json:"logical_source_id"`
	DisplayRange       graph.Range `json:"display_range"`
	DisplayRangePolicy string      `json:"display_range_policy"`
	Provenance         Provenance  `json:"provenance"`
	ReceiptID          string      `json:"receipt_id"`
	SourceDigest       string      `json:"source_digest"`
	PositionEncoding   string      `json:"position_encoding"`
	Status             string      `json:"status"`
	Custody            string      `json:"custody"`
}

type Artifact struct {
	SchemaVersion        string           `json:"schema_version"`
	Policy               string           `json:"policy"`
	ParentSchemaVersion  string           `json:"parent_schema_version"`
	ParentSnapshotDigest string           `json:"parent_snapshot_digest"`
	ParentSnapshot       []byte           `json:"parent_snapshot"`
	DisplayBindings      []DisplayBinding `json:"display_bindings"`
}

type receiptIdentity struct {
	ID     string
	URI    string
	Digest string
}

func Validate(raw []byte) (string, error) {
	if len(raw) > MaxBytes {
		return "", admissionError("V2_BYTE_LIMIT", "artifact exceeds byte limit")
	}
	if err := strictjson.RejectDuplicates(raw); err != nil && bytes.Contains([]byte(err.Error()), []byte("duplicate JSON member")) {
		return "", admissionError("V2_JSON_DUPLICATE_MEMBER", err.Error())
	}
	var artifact Artifact
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&artifact); err != nil {
		return "", admissionError("V2_JSON_INVALID", err.Error())
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return "", admissionError("V2_JSON_TRAILING_VALUE", "trailing JSON value")
	}
	if artifact.SchemaVersion != Version {
		return "", admissionError("V2_SCHEMA_IDENTITY_MISMATCH", "schema_version substitution")
	}
	if artifact.Policy != Policy {
		return "", admissionError("V2_POLICY_MISMATCH", "policy substitution")
	}
	if artifact.ParentSchemaVersion != v5sourcesnapshot.Version {
		return "", admissionError("V2_PARENT_IDENTITY_MISMATCH", "parent schema_version substitution")
	}
	if !digestPattern.MatchString(artifact.ParentSnapshotDigest) {
		return "", admissionError("V2_PARENT_DIGEST_INVALID", "parent digest is not canonical sha256")
	}
	if artifact.ParentSnapshotDigest != digest(artifact.ParentSnapshot) {
		return "", admissionError("V2_PARENT_DIGEST_MISMATCH", "parent digest does not bind embedded snapshot")
	}
	if _, err := v5sourcesnapshot.Validate(artifact.ParentSnapshot); err != nil {
		return "", admissionError("V2_PARENT_INVALID", err.Error())
	}
	var parent v5sourcesnapshot.Artifact
	if err := json.Unmarshal(artifact.ParentSnapshot, &parent); err != nil {
		return "", admissionError("V2_PARENT_INVALID", err.Error())
	}
	if len(artifact.DisplayBindings) == 0 || len(artifact.DisplayBindings) > MaxBindings {
		return "", admissionError("V2_DISPLAY_BINDINGS_REQUIRED", "display binding count outside bounds")
	}
	receipts := make(map[string]receiptIdentity, len(parent.Receipts))
	for _, receipt := range parent.Receipts {
		receipts[receipt.ID] = receiptIdentity{ID: receipt.ID, URI: receipt.URI, Digest: receipt.ContentDigest}
	}
	parentSubjects := make(map[string]map[string]bool)
	for _, binding := range parent.Bindings {
		if parentSubjects[binding.NodeID] == nil {
			parentSubjects[binding.NodeID] = map[string]bool{}
		}
		parentSubjects[binding.NodeID][binding.URI] = true
	}
	keys := make([]string, 0, len(artifact.DisplayBindings))
	seen := make(map[string]bool, len(artifact.DisplayBindings))
	for _, binding := range artifact.DisplayBindings {
		if binding.GraphSubjectID == "" || binding.LogicalSourceID == "" || !parentSubjects[binding.GraphSubjectID][binding.LogicalSourceID] {
			return "", admissionError("V2_SELECTION_IDENTITY_MISMATCH", "display selection is absent from parent")
		}
		key := binding.GraphSubjectID + "\x00" + binding.LogicalSourceID
		if seen[key] {
			return "", admissionError("V2_AMBIGUOUS_DISPLAY_BINDING", "duplicate display selection key")
		}
		seen[key] = true
		keys = append(keys, key)
		if !validRange(binding.DisplayRange) {
			return "", admissionError("V2_DISPLAY_RANGE_INVALID", "display range is reversed")
		}
		if binding.DisplayRangePolicy != DisplayRangePolicy {
			return "", admissionError("V2_DISPLAY_POLICY_MISMATCH", "display range policy substitution")
		}
		if binding.Provenance.Kind != ProvenanceKind || binding.Provenance.Method != ProvenanceMethod {
			return "", admissionError("V2_PROVENANCE_MISMATCH", "display provenance substitution")
		}
		if binding.ReceiptID == "" {
			return "", admissionError("V2_RECEIPT_BINDING_REQUIRED", "receipt identity required")
		}
		if binding.SourceDigest == "" {
			return "", admissionError("V2_SOURCE_BINDING_REQUIRED", "source digest required")
		}
		receipt, ok := receipts[binding.ReceiptID]
		if !ok || receipt.URI != binding.LogicalSourceID {
			return "", admissionError("V2_RECEIPT_BINDING_MISMATCH", "receipt identity does not bind logical source")
		}
		if !digestPattern.MatchString(binding.SourceDigest) || binding.SourceDigest != receipt.Digest {
			return "", admissionError("V2_SOURCE_BINDING_MISMATCH", "source digest does not bind receipt")
		}
		if !validEncoding(binding.PositionEncoding) || binding.PositionEncoding != parent.PositionEncoding {
			return "", admissionError("V2_POSITION_ENCODING_INVALID", "position encoding does not match parent")
		}
		if binding.Status != Status {
			return "", admissionError("V2_STATUS_MISMATCH", "retained status substitution")
		}
		if binding.Custody != Custody {
			return "", admissionError("V2_CUSTODY_MISMATCH", "retained custody substitution")
		}
	}
	ordered := append([]string(nil), keys...)
	sort.Strings(ordered)
	for i := range keys {
		if keys[i] != ordered[i] {
			return "", admissionError("V2_BINDING_ORDER_INVALID", "display bindings are not lexicographically ordered by selection key")
		}
	}
	if detected, err := schema.ValidateFor(raw, schema.FamilyGraphV5SourceSnapshot, "v2"); err != nil || detected != Version {
		if err == nil {
			err = errors.New("schema version mismatch")
		}
		return "", admissionError("V2_SCHEMA_INVALID", err.Error())
	}
	return Version, nil
}

func validEncoding(value string) bool {
	return value == "utf-8" || value == "utf-16" || value == "utf-32"
}

func validRange(value graph.Range) bool {
	return value.Start.Line < value.End.Line || value.Start.Line == value.End.Line && value.Start.Character <= value.End.Character
}

func digest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func admissionError(code, detail string) error {
	return fmt.Errorf("v5sourcesnapshotv2: %s: %s", code, detail)
}
