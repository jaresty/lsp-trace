package retainedmanifest

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"lsp-trace/internal/graphprovenance"
	"lsp-trace/internal/sourceobject"
	"lsp-trace/internal/sourceprojection"
	"lsp-trace/internal/strictjson"
)

const (
	Version             = "lsp-trace.retained-availability-manifest.v1"
	SemanticOwner       = "CUSTODY_AND_AVAILABILITY_ONLY"
	SourceGraphComplete = "UNKNOWN"
)

type GraphBinding struct {
	SchemaID   string `json:"schema_id"`
	Digest     string `json:"digest"`
	ByteLength int    `json:"byte_length"`
	CaptureID  string `json:"capture_id"`
}

type EntryInput struct {
	Source                sourceobject.Identity  `json:"source_identity"`
	StorageClass          string                 `json:"storage_class"`
	Qualification         string                 `json:"qualification"`
	PrivacyClassification string                 `json:"privacy_classification"`
	Availability          string                 `json:"availability"`
	Role                  string                 `json:"role"`
	GraphSubjectID        string                 `json:"graph_subject_id"`
	OccurrenceID          string                 `json:"occurrence_id,omitempty"`
	LogicalSourceID       string                 `json:"logical_source_id"`
	Range                 sourceprojection.Range `json:"range"`
	PositionEncoding      string                 `json:"position_encoding"`
	PolicyID              string                 `json:"policy_id"`
	CustodyIdentity       string                 `json:"custody_identity"`
}

type Entry struct {
	EntryID string `json:"entry_id"`
	EntryInput
}

type Manifest struct {
	SchemaVersion       string       `json:"schema_version"`
	SemanticOwner       string       `json:"semantic_owner"`
	Authority           int          `json:"authority"`
	Accepted            bool         `json:"accepted"`
	SourceGraphComplete string       `json:"source_graph_complete"`
	GraphFactsAdded     int          `json:"graph_facts_added"`
	Graph               GraphBinding `json:"graph"`
	Entries             []Entry      `json:"entries"`
}

func Build(graphBytes, captureBytes []byte, entries []EntryInput) ([]byte, string, error) {
	if len(graphBytes) == 0 || len(captureBytes) == 0 {
		return nil, "", errors.New("exact graph and capture bytes required")
	}
	if detected, err := graphprovenance.ValidateFor(captureBytes, graphprovenance.Family, "v5"); err != nil || detected != graphprovenance.VersionV5 {
		return nil, "", errors.New("exact admitted Graph Provenance V5 required")
	}
	var admitted graphprovenance.EvidenceV5
	if err := json.Unmarshal(captureBytes, &admitted); err != nil {
		return nil, "", err
	}
	graphDigest := digest(graphBytes)
	if admitted.GraphV5SchemaID != graphprovenance.GraphV5SchemaID || admitted.GraphV5SHA256 != graphDigest {
		return nil, "", errors.New("admitted V5 graph binding mismatch")
	}
	canonical := make([]Entry, len(entries))
	for i := range entries {
		if err := validateEntryInput(entries[i]); err != nil {
			return nil, "", fmt.Errorf("entry %d: %w", i, err)
		}
		canonical[i] = Entry{EntryID: entryID(entries[i]), EntryInput: entries[i]}
	}
	sort.Slice(canonical, func(i, j int) bool { return canonical[i].EntryID < canonical[j].EntryID })
	for i := 1; i < len(canonical); i++ {
		if canonical[i-1].EntryID == canonical[i].EntryID {
			return nil, "", errors.New("duplicate retained availability entry")
		}
	}
	manifest := Manifest{
		SchemaVersion: Version, SemanticOwner: SemanticOwner, Authority: 0, Accepted: false,
		SourceGraphComplete: SourceGraphComplete, GraphFactsAdded: 0,
		Graph:   GraphBinding{SchemaID: graphprovenance.GraphV5SchemaID, Digest: graphDigest, ByteLength: len(graphBytes), CaptureID: digest(captureBytes)},
		Entries: canonical,
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		return nil, "", err
	}
	return raw, digest(raw), nil
}

func Admit(raw, graphBytes, captureBytes []byte) (Manifest, string, error) {
	var zero Manifest
	if err := strictjson.RejectDuplicates(raw); err != nil {
		return zero, "", err
	}
	var manifest Manifest
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return zero, "", err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return zero, "", errors.New("manifest trailing JSON")
	}
	if manifest.SchemaVersion != Version || manifest.SemanticOwner != SemanticOwner || manifest.Authority != 0 || manifest.Accepted || manifest.SourceGraphComplete != SourceGraphComplete || manifest.GraphFactsAdded != 0 {
		return zero, "", errors.New("manifest semantic ceiling mismatch")
	}
	if manifest.Graph.SchemaID != graphprovenance.GraphV5SchemaID || manifest.Graph.Digest != digest(graphBytes) || manifest.Graph.ByteLength != len(graphBytes) || manifest.Graph.CaptureID != digest(captureBytes) {
		return zero, "", errors.New("manifest exact V5 binding mismatch")
	}
	if detected, err := graphprovenance.ValidateFor(captureBytes, graphprovenance.Family, "v5"); err != nil || detected != graphprovenance.VersionV5 {
		return zero, "", errors.New("exact admitted Graph Provenance V5 required")
	}
	var admitted graphprovenance.EvidenceV5
	if err := json.Unmarshal(captureBytes, &admitted); err != nil || admitted.GraphV5SHA256 != manifest.Graph.Digest || admitted.GraphV5SchemaID != manifest.Graph.SchemaID {
		return zero, "", errors.New("manifest admitted capture mismatch")
	}
	for i := range manifest.Entries {
		entry := manifest.Entries[i]
		if err := validateEntryInput(entry.EntryInput); err != nil || entry.EntryID != entryID(entry.EntryInput) {
			return zero, "", fmt.Errorf("manifest entry %d identity mismatch", i)
		}
		if i > 0 && manifest.Entries[i-1].EntryID >= entry.EntryID {
			return zero, "", errors.New("manifest entry ordering or uniqueness mismatch")
		}
	}
	return manifest, digest(raw), nil
}

func entryID(entry EntryInput) string {
	raw, err := json.Marshal(entry)
	if err != nil {
		panic(err)
	}
	return digest(raw)
}

func validateEntryInput(entry EntryInput) error {
	if !canonicalDigest(entry.Source.Digest) || entry.Source.ByteLength < 0 || entry.GraphSubjectID == "" || entry.LogicalSourceID == "" || entry.PolicyID == "" || entry.CustodyIdentity == "" || entry.PrivacyClassification == "" {
		return errors.New("invalid retained availability identity")
	}
	if !oneOf(entry.StorageClass, "GIT_BLOB", "CONTENT_ADDRESS", "EMBEDDED_IMMUTABLE", "UNAVAILABLE") ||
		!oneOf(entry.Qualification, "QUALIFIED", "UNQUALIFIED") ||
		!oneOf(entry.Availability, "AVAILABLE", "WITHHELD", "UNAVAILABLE", "COLLECTED") ||
		!oneOf(entry.Role, "ENDPOINT", "RELATION", "ANCILLARY") ||
		!oneOf(entry.PositionEncoding, "utf-8", "utf-16", "utf-32") {
		return errors.New("invalid retained availability vocabulary")
	}
	if entry.Role == "RELATION" && entry.OccurrenceID == "" {
		return errors.New("relation occurrence identity required")
	}
	if entry.Role != "RELATION" && entry.OccurrenceID != "" {
		return errors.New("occurrence identity is relation-only")
	}
	if after(entry.Range.Start, entry.Range.End) {
		return errors.New("invalid retained availability range")
	}
	return nil
}

func after(left, right sourceprojection.Position) bool {
	return left.Line > right.Line || left.Line == right.Line && left.Character > right.Character
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func canonicalDigest(value string) bool {
	if len(value) != len("sha256:")+sha256.Size*2 || !strings.HasPrefix(value, "sha256:") {
		return false
	}
	_, err := hex.DecodeString(strings.TrimPrefix(value, "sha256:"))
	return err == nil
}

func digest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}
