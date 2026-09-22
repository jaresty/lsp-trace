package featureattribution

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
)

const (
	AnchorGraphSubject       = "GRAPH_SUBJECT"
	AnchorSourceRange        = "SOURCE_RANGE"
	AnchorRelationOccurrence = "RELATION_OCCURRENCE"

	DispositionResolved   = "RESOLVED"
	DispositionUnresolved = "UNRESOLVED"
	DispositionAmbiguous  = "AMBIGUOUS"

	OutcomeNotReady     = "NOT_READY"
	CompletenessUnknown = "UNKNOWN"
)

type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

type SourceIdentity struct {
	LogicalURI       string `json:"logical_uri"`
	Digest           string `json:"digest"`
	ByteLength       int64  `json:"byte_length"`
	PositionEncoding string `json:"position_encoding"`
	DocumentVersion  int64  `json:"document_version"`
	Revision         string `json:"revision"`
	RevisionCustody  string `json:"revision_custody"`
}

type GraphSubjectAnchor struct {
	GraphSubjectID string         `json:"graph_subject_id"`
	EvidenceRange  Range          `json:"evidence_range"`
	ItemRange      Range          `json:"item_range"`
	SelectionRange Range          `json:"selection_range"`
	DisplayRange   Range          `json:"display_range"`
	Source         SourceIdentity `json:"source"`
}

type SourceRangeAnchor struct {
	Source SourceIdentity `json:"source"`
	Range  Range          `json:"range"`
}

type RelationOccurrenceAnchor struct {
	RelationOccurrenceID string         `json:"relation_occurrence_id"`
	EvidenceRange        Range          `json:"evidence_range"`
	Source               SourceIdentity `json:"source"`
}

type Anchor struct {
	Kind               string                    `json:"kind"`
	GraphSubject       *GraphSubjectAnchor       `json:"graph_subject,omitempty"`
	SourceRange        *SourceRangeAnchor        `json:"source_range,omitempty"`
	RelationOccurrence *RelationOccurrenceAnchor `json:"relation_occurrence,omitempty"`
}

func (a Anchor) Validate() error {
	payloads := 0
	if a.GraphSubject != nil {
		payloads++
	}
	if a.SourceRange != nil {
		payloads++
	}
	if a.RelationOccurrence != nil {
		payloads++
	}
	if payloads != 1 {
		return fmt.Errorf("anchor must contain exactly one payload, got %d", payloads)
	}
	switch a.Kind {
	case AnchorGraphSubject:
		if a.GraphSubject == nil || a.GraphSubject.GraphSubjectID == "" {
			return errors.New("GRAPH_SUBJECT requires graph_subject payload and identity")
		}
		return validateSource(a.GraphSubject.Source)
	case AnchorSourceRange:
		if a.SourceRange == nil {
			return errors.New("SOURCE_RANGE requires source_range payload")
		}
		if err := validateRange(a.SourceRange.Range); err != nil {
			return err
		}
		return validateSource(a.SourceRange.Source)
	case AnchorRelationOccurrence:
		if a.RelationOccurrence == nil || a.RelationOccurrence.RelationOccurrenceID == "" {
			return errors.New("RELATION_OCCURRENCE requires relation_occurrence payload and identity")
		}
		return validateSource(a.RelationOccurrence.Source)
	default:
		return fmt.Errorf("unknown anchor kind %q", a.Kind)
	}
}

type ProviderIdentity struct {
	Name       string `json:"name"`
	Version    string `json:"version"`
	InstanceID string `json:"instance_id"`
}

type ResolutionProvenance struct {
	SessionID         string           `json:"session_id"`
	Generation        int64            `json:"generation"`
	WorkspaceRevision string           `json:"workspace_revision"`
	RevisionCustody   string           `json:"revision_custody"`
	Provider          ProviderIdentity `json:"provider"`
}

type ResolutionEntry struct {
	LocatorID      string  `json:"locator_id"`
	Disposition    string  `json:"disposition"`
	Anchor         *Anchor `json:"anchor,omitempty"`
	CandidateCount int     `json:"candidate_count"`
}

type ReceiptAccounting struct {
	Submitted  int   `json:"submitted"`
	Resolved   int   `json:"resolved"`
	Unresolved int   `json:"unresolved"`
	Ambiguous  int   `json:"ambiguous"`
	Candidates int   `json:"candidates"`
	Work       int64 `json:"work"`
	Bytes      int64 `json:"bytes"`
}

type LocatorResolutionReceipt struct {
	Schema     string               `json:"schema"`
	ID         string               `json:"id,omitempty"`
	RequestID  string               `json:"request_id"`
	Provenance ResolutionProvenance `json:"provenance"`
	Entries    []ResolutionEntry    `json:"entries"`
	Accounting ReceiptAccounting    `json:"accounting"`
}

func (r LocatorResolutionReceipt) Validate() error {
	p := r.Provenance
	if r.Schema == "" || r.RequestID == "" || p.SessionID == "" || p.Generation < 1 || p.WorkspaceRevision == "" || p.RevisionCustody == "" || p.Provider.Name == "" || p.Provider.Version == "" || p.Provider.InstanceID == "" {
		return errors.New("receipt provenance is incomplete")
	}
	resolved, unresolved, ambiguous, candidates := 0, 0, 0, 0
	seen := map[string]struct{}{}
	for _, e := range r.Entries {
		if e.LocatorID == "" {
			return errors.New("empty locator identity")
		}
		if _, ok := seen[e.LocatorID]; ok {
			return fmt.Errorf("duplicate locator %q", e.LocatorID)
		}
		seen[e.LocatorID] = struct{}{}
		if e.CandidateCount < 0 {
			return errors.New("negative candidate count")
		}
		candidates += e.CandidateCount
		switch e.Disposition {
		case DispositionResolved:
			resolved++
			if e.Anchor == nil || e.CandidateCount != 1 {
				return errors.New("RESOLVED requires one exact anchor and candidate")
			}
			if err := e.Anchor.Validate(); err != nil {
				return fmt.Errorf("resolved anchor: %w", err)
			}
		case DispositionUnresolved:
			unresolved++
			if e.Anchor != nil || e.CandidateCount != 0 {
				return errors.New("UNRESOLVED cannot select an anchor or candidate")
			}
		case DispositionAmbiguous:
			ambiguous++
			if e.Anchor != nil || e.CandidateCount < 2 {
				return errors.New("AMBIGUOUS cannot select an anchor and requires multiple candidates")
			}
		default:
			return fmt.Errorf("unknown receipt disposition %q", e.Disposition)
		}
	}
	a := r.Accounting
	if a.Submitted != len(r.Entries) || a.Resolved != resolved || a.Unresolved != unresolved || a.Ambiguous != ambiguous || a.Candidates != candidates || a.Work < 0 || a.Bytes < 0 {
		return errors.New("receipt accounting mismatch")
	}
	return nil
}

type ReceiptSelector struct {
	ReceiptID string `json:"receipt_id"`
	EntryID   string `json:"entry_id"`
}

type AttributionInput struct {
	Anchor          *Anchor          `json:"anchor,omitempty"`
	ReceiptSelector *ReceiptSelector `json:"receipt_selector,omitempty"`
}

type Subject struct {
	SubjectID string             `json:"subject_id"`
	Inputs    []AttributionInput `json:"inputs"`
}

type ArtifactSelector struct {
	ID         string `json:"id"`
	Schema     string `json:"schema"`
	ByteLength int64  `json:"byte_length"`
}

type AttributionRequest struct {
	Schema       string           `json:"schema"`
	ID           string           `json:"id,omitempty"`
	Inventory    ArtifactSelector `json:"inventory"`
	Membership   ArtifactSelector `json:"membership"`
	Subjects     []Subject        `json:"subjects"`
	Authority    int              `json:"authority"`
	Accepted     bool             `json:"accepted"`
	Completeness string           `json:"completeness"`
}

func (r AttributionRequest) Validate() error {
	if r.Schema == "" || r.Inventory.ID == "" || r.Inventory.Schema == "" || r.Inventory.ByteLength < 1 || r.Membership.ID == "" || r.Membership.Schema == "" || r.Membership.ByteLength < 1 {
		return errors.New("immutable inventory and membership selectors are required")
	}
	if r.Authority != 0 || r.Accepted || r.Completeness != CompletenessUnknown {
		return errors.New("attribution authority ceiling violated")
	}
	seenSubjects := map[string]struct{}{}
	for _, s := range r.Subjects {
		if s.SubjectID == "" || len(s.Inputs) == 0 {
			return errors.New("subject identity and input are required")
		}
		if _, ok := seenSubjects[s.SubjectID]; ok {
			return fmt.Errorf("duplicate subject %q", s.SubjectID)
		}
		seenSubjects[s.SubjectID] = struct{}{}
		seenInputs := map[string]struct{}{}
		for _, in := range s.Inputs {
			if (in.Anchor == nil) == (in.ReceiptSelector == nil) {
				return errors.New("attribution input must contain exactly one anchor or receipt selector")
			}
			if in.Anchor != nil {
				if err := in.Anchor.Validate(); err != nil {
					return err
				}
			}
			if in.ReceiptSelector != nil && (in.ReceiptSelector.ReceiptID == "" || in.ReceiptSelector.EntryID == "") {
				return errors.New("receipt selector identities are required")
			}
			key, err := CanonicalBytes(in)
			if err != nil {
				return err
			}
			if _, ok := seenInputs[string(key)]; ok {
				return errors.New("duplicate attribution input")
			}
			seenInputs[string(key)] = struct{}{}
		}
	}
	return nil
}

type AttributionResult struct {
	Outcome      string `json:"outcome"`
	Authority    int    `json:"authority"`
	Accepted     bool   `json:"accepted"`
	Completeness string `json:"completeness"`
	Reason       string `json:"reason"`
}

func Attribute(AttributionRequest) AttributionResult {
	return AttributionResult{Outcome: OutcomeNotReady, Authority: 0, Accepted: false, Completeness: CompletenessUnknown, Reason: "immutable ADR0007 inventory membership is not available"}
}

func CanonicalBytes(v any) ([]byte, error) {
	normalized := normalize(v)
	return json.Marshal(normalized)
}

func Identity(v any) (string, error) {
	b, err := canonicalIdentityBytes(v)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func DecodeStrict(data []byte, out any) error {
	if err := rejectDuplicateKeys(data); err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return err
	}
	if err := requireEOF(dec); err != nil {
		return err
	}
	if v, ok := out.(interface{ Validate() error }); ok {
		if err := v.Validate(); err != nil {
			return err
		}
	}
	var supplied string
	switch x := out.(type) {
	case *AttributionRequest:
		supplied = x.ID
	case *LocatorResolutionReceipt:
		supplied = x.ID
	}
	if supplied != "" {
		actual, err := Identity(out)
		if err != nil {
			return err
		}
		if supplied != actual {
			return fmt.Errorf("identity mismatch: supplied %s computed %s", supplied, actual)
		}
	}
	return nil
}

func normalize(v any) any {
	switch x := v.(type) {
	case AttributionRequest:
		x.Subjects = append([]Subject(nil), x.Subjects...)
		for i := range x.Subjects {
			x.Subjects[i].Inputs = append([]AttributionInput(nil), x.Subjects[i].Inputs...)
			sort.Slice(x.Subjects[i].Inputs, func(a, b int) bool {
				aa, _ := json.Marshal(x.Subjects[i].Inputs[a])
				bb, _ := json.Marshal(x.Subjects[i].Inputs[b])
				return bytes.Compare(aa, bb) < 0
			})
		}
		sort.Slice(x.Subjects, func(i, j int) bool { return x.Subjects[i].SubjectID < x.Subjects[j].SubjectID })
		return x
	case *AttributionRequest:
		return normalize(*x)
	case LocatorResolutionReceipt:
		x.Entries = append([]ResolutionEntry(nil), x.Entries...)
		sort.Slice(x.Entries, func(i, j int) bool { return x.Entries[i].LocatorID < x.Entries[j].LocatorID })
		return x
	case *LocatorResolutionReceipt:
		return normalize(*x)
	default:
		return v
	}
}

func canonicalIdentityBytes(v any) ([]byte, error) {
	switch x := v.(type) {
	case AttributionRequest:
		x.ID = ""
		return json.Marshal(normalize(x))
	case *AttributionRequest:
		copy := *x
		copy.ID = ""
		return json.Marshal(normalize(copy))
	case LocatorResolutionReceipt:
		x.ID = ""
		return json.Marshal(normalize(x))
	case *LocatorResolutionReceipt:
		copy := *x
		copy.ID = ""
		return json.Marshal(normalize(copy))
	default:
		return json.Marshal(normalize(v))
	}
}

func validateSource(s SourceIdentity) error {
	if s.LogicalURI == "" || !strings.HasPrefix(s.Digest, "sha256:") || len(s.Digest) != 71 || s.ByteLength < 1 || (s.PositionEncoding != "utf-8" && s.PositionEncoding != "utf-16" && s.PositionEncoding != "utf-32") || s.DocumentVersion < 0 || s.Revision == "" || s.RevisionCustody == "" {
		return errors.New("source identity is incomplete")
	}
	return nil
}

func validateRange(r Range) error {
	if r.Start.Line < 0 || r.Start.Character < 0 || r.End.Line < 0 || r.End.Character < 0 || r.End.Line < r.Start.Line || (r.End.Line == r.Start.Line && r.End.Character < r.Start.Character) {
		return errors.New("invalid half-open range")
	}
	return nil
}

func requireEOF(dec *json.Decoder) error {
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("trailing JSON content")
		}
		return err
	}
	return nil
}

func rejectDuplicateKeys(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	var walk func() error
	walk = func() error {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		delim, ok := tok.(json.Delim)
		if !ok {
			return nil
		}
		switch delim {
		case '{':
			seen := map[string]struct{}{}
			for dec.More() {
				kt, err := dec.Token()
				if err != nil {
					return err
				}
				key := kt.(string)
				if _, ok := seen[key]; ok {
					return fmt.Errorf("duplicate key %q", key)
				}
				seen[key] = struct{}{}
				if err := walk(); err != nil {
					return err
				}
			}
			_, err = dec.Token()
			return err
		case '[':
			for dec.More() {
				if err := walk(); err != nil {
					return err
				}
			}
			_, err = dec.Token()
			return err
		default:
			return nil
		}
	}
	if err := walk(); err != nil {
		return err
	}
	if dec.More() {
		return errors.New("trailing JSON content")
	}
	return nil
}
