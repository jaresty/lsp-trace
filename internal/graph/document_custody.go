package graph

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
)

// RevisionCustody records who can attest a revision identity.
type RevisionCustody string

const (
	CustodyCallerAsserted RevisionCustody = "CALLER_ASSERTED"
	CustodyProviderProved RevisionCustody = "PROVIDER_PROVED"
	CustodyUnknown        RevisionCustody = "UNKNOWN"
)

// RevisionIdentity is explicit about both identity and the authority asserting it.
type RevisionIdentity struct {
	Kind    string          `json:"kind,omitempty"`
	Value   string          `json:"value,omitempty"`
	Blob    string          `json:"blob,omitempty"`
	Custody RevisionCustody `json:"custody"`
}

type GeneratedCoordinatePolicy string

const (
	CoordinatesOriginal  GeneratedCoordinatePolicy = "ORIGINAL"
	CoordinatesGenerated GeneratedCoordinatePolicy = "GENERATED"
)

// VirtualDocumentMapping preserves the relation from generated text to an immutable original.
type VirtualDocumentMapping struct {
	MappingID          string `json:"mapping_id"`
	OriginalDocumentID string `json:"original_document_id"`
}

// SourceDocumentRecord is an immutable source identity plus optional virtual projection.
type SourceDocumentRecord struct {
	DocumentID    string                    `json:"document_id"`
	OriginalURI   string                    `json:"original_uri"`
	VirtualURI    string                    `json:"virtual_uri,omitempty"`
	CoordinateURI string                    `json:"coordinate_uri,omitempty"`
	ContentSHA256 string                    `json:"content_sha256"`
	Revision      RevisionIdentity          `json:"revision"`
	Mapping       *VirtualDocumentMapping   `json:"mapping,omitempty"`
	Coordinates   GeneratedCoordinatePolicy `json:"coordinates,omitempty"`
}

type DocumentCustodyRequest struct {
	WorkspaceRevision     RevisionIdentity       `json:"workspace_revision"`
	FailOnUnknownRevision bool                   `json:"fail_on_unknown_revision"`
	Documents             []SourceDocumentRecord `json:"documents"`
}

type DocumentCustodyConflict struct {
	Code        string   `json:"code"`
	DocumentIDs []string `json:"document_ids,omitempty"`
	Revisions   []string `json:"revisions,omitempty"`
}

type DocumentCustodyReceipt struct {
	WorkspaceRevision RevisionIdentity          `json:"workspace_revision"`
	Documents         []SourceDocumentRecord    `json:"documents"`
	Conflicts         []DocumentCustodyConflict `json:"conflicts"`
	LogicalDigest     string                    `json:"logical_digest"`
}

// ValidateDocumentCustody validates revision and mapping custody and returns a canonical receipt.
func ValidateDocumentCustody(req DocumentCustodyRequest) (DocumentCustodyReceipt, error) {
	receipt := DocumentCustodyReceipt{
		WorkspaceRevision: req.WorkspaceRevision,
		Documents:         append([]SourceDocumentRecord(nil), req.Documents...),
		Conflicts:         []DocumentCustodyConflict{},
	}

	if presentRevision(req.WorkspaceRevision) {
		if err := validateRevisionAuthority(req.WorkspaceRevision); err != nil {
			return receipt, err
		}
		if req.WorkspaceRevision.Kind == "git" && req.WorkspaceRevision.Value == "" {
			return receipt, fmt.Errorf("ASSERT_CUSTODY_REQUEST_REVISION_IDENTITY: git workspace revision requires commit")
		}
	}

	seenIDs := make(map[string]struct{}, len(req.Documents))
	knownRevisions := make(map[string][]string)
	unknownDocuments := make([]string, 0)
	for i := range receipt.Documents {
		doc := &receipt.Documents[i]
		if doc.DocumentID == "" {
			return receipt, fmt.Errorf("ASSERT_CUSTODY_IMMUTABLE_SOURCE_IDENTITY: empty document id")
		}
		if _, duplicate := seenIDs[doc.DocumentID]; duplicate {
			return receipt, fmt.Errorf("ASSERT_CUSTODY_IMMUTABLE_SOURCE_IDENTITY: duplicate document id %q", doc.DocumentID)
		}
		seenIDs[doc.DocumentID] = struct{}{}
		if !canonicalFileURI(doc.OriginalURI) || !sha256Hex(doc.ContentSHA256) {
			return receipt, fmt.Errorf("ASSERT_CUSTODY_IMMUTABLE_SOURCE_IDENTITY: document %q requires canonical file URI and SHA-256", doc.DocumentID)
		}
		if err := validateRevisionAuthority(doc.Revision); err != nil {
			return receipt, err
		}
		if doc.Revision.Kind == "git" && (doc.Revision.Value == "" || !hexDigest(doc.Revision.Blob)) {
			return receipt, fmt.Errorf("ASSERT_CUSTODY_GIT_COMMIT_BLOB: document %q requires git commit and blob", doc.DocumentID)
		}
		if doc.VirtualURI != "" {
			if doc.Mapping == nil || doc.Mapping.MappingID == "" || doc.Mapping.OriginalDocumentID == "" {
				return receipt, fmt.Errorf("ASSERT_CUSTODY_VIRTUAL_MAPPING_REQUIRED: document %q", doc.DocumentID)
			}
			if doc.VirtualURI == doc.OriginalURI {
				return receipt, fmt.Errorf("ASSERT_CUSTODY_DISTINCT_VIRTUAL_IDENTITY: document %q", doc.DocumentID)
			}
		}
		if doc.Coordinates == CoordinatesGenerated {
			if doc.VirtualURI == "" || doc.Mapping == nil {
				return receipt, fmt.Errorf("ASSERT_CUSTODY_GENERATED_COORDINATES_REQUIRE_MAPPING: document %q", doc.DocumentID)
			}
			if doc.CoordinateURI != doc.VirtualURI {
				return receipt, fmt.Errorf("ASSERT_CUSTODY_ORIGINAL_COORDINATES_NOT_GENERATED: document %q", doc.DocumentID)
			}
		}

		if knownRevision(doc.Revision) {
			knownRevisions[doc.Revision.Value] = append(knownRevisions[doc.Revision.Value], doc.DocumentID)
			if knownRevision(req.WorkspaceRevision) && doc.Revision.Value != req.WorkspaceRevision.Value {
				conflict := DocumentCustodyConflict{Code: "REQUEST_DOCUMENT_REVISION_CONFLICT", DocumentIDs: []string{doc.DocumentID}, Revisions: []string{req.WorkspaceRevision.Value, doc.Revision.Value}}
				receipt.Conflicts = append(receipt.Conflicts, conflict)
				return receipt, fmt.Errorf("ASSERT_CUSTODY_REQUEST_DOCUMENT_REVISION_CONFLICT: document %q", doc.DocumentID)
			}
		} else {
			unknownDocuments = append(unknownDocuments, doc.DocumentID)
		}
	}

	if len(knownRevisions) > 1 {
		conflict := DocumentCustodyConflict{Code: "MIXED_REVISIONS"}
		for revision, ids := range knownRevisions {
			conflict.Revisions = append(conflict.Revisions, revision)
			conflict.DocumentIDs = append(conflict.DocumentIDs, ids...)
		}
		sort.Strings(conflict.Revisions)
		sort.Strings(conflict.DocumentIDs)
		receipt.Conflicts = append(receipt.Conflicts, conflict)
		return receipt, fmt.Errorf("ASSERT_CUSTODY_MIXED_REVISIONS: %v", conflict.Revisions)
	}
	if req.FailOnUnknownRevision && len(unknownDocuments) != 0 {
		sort.Strings(unknownDocuments)
		receipt.Conflicts = append(receipt.Conflicts, DocumentCustodyConflict{Code: "UNKNOWN_REVISION", DocumentIDs: unknownDocuments})
		return receipt, fmt.Errorf("ASSERT_CUSTODY_UNKNOWN_REVISION: %v", unknownDocuments)
	}

	sort.Slice(receipt.Documents, func(i, j int) bool { return receipt.Documents[i].DocumentID < receipt.Documents[j].DocumentID })
	canonical, err := json.Marshal(struct {
		WorkspaceRevision RevisionIdentity          `json:"workspace_revision"`
		Documents         []SourceDocumentRecord    `json:"documents"`
		Conflicts         []DocumentCustodyConflict `json:"conflicts"`
	}{receipt.WorkspaceRevision, receipt.Documents, receipt.Conflicts})
	if err != nil {
		return receipt, fmt.Errorf("encode custody receipt: %w", err)
	}
	digest := sha256.Sum256(canonical)
	receipt.LogicalDigest = "sha256:" + hex.EncodeToString(digest[:])
	return receipt, nil
}

func validateRevisionAuthority(revision RevisionIdentity) error {
	switch revision.Custody {
	case CustodyCallerAsserted, CustodyProviderProved, CustodyUnknown:
		return nil
	default:
		return fmt.Errorf("ASSERT_CUSTODY_CLOSED_AUTHORITY_VOCABULARY: %q", revision.Custody)
	}
}

func presentRevision(revision RevisionIdentity) bool {
	return revision.Kind != "" || revision.Value != "" || revision.Blob != "" || revision.Custody != ""
}

func knownRevision(revision RevisionIdentity) bool {
	return revision.Custody != CustodyUnknown && revision.Value != ""
}

func canonicalFileURI(raw string) bool {
	parsed, err := url.Parse(raw)
	return err == nil && parsed.Scheme == "file" && parsed.Host == "" && parsed.Path != "" && parsed.String() == raw
}

func sha256Hex(value string) bool {
	return len(value) == sha256.Size*2 && hexDigest(value)
}

func hexDigest(value string) bool {
	if value == "" {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
